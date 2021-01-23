package proxynode

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/zilliztech/milvus-distributed/internal/msgstream/pulsarms"

	grpcproxyservice "github.com/zilliztech/milvus-distributed/internal/distributed/proxyservice"

	"github.com/zilliztech/milvus-distributed/internal/proto/internalpb2"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go/config"

	"google.golang.org/grpc"

	"github.com/zilliztech/milvus-distributed/internal/allocator"
	"github.com/zilliztech/milvus-distributed/internal/msgstream"
	"github.com/zilliztech/milvus-distributed/internal/proto/masterpb"
	"github.com/zilliztech/milvus-distributed/internal/util/typeutil"
)

type UniqueID = typeutil.UniqueID
type Timestamp = typeutil.Timestamp

type NodeImpl struct {
	ctx    context.Context
	cancel func()
	wg     sync.WaitGroup

	proxyServiceClient *grpcproxyservice.Client
	initParams         *internalpb2.InitParams
	ip                 string
	port               int

	masterConn   *grpc.ClientConn
	masterClient masterpb.MasterServiceClient
	sched        *TaskScheduler
	tick         *timeTick

	idAllocator  *allocator.IDAllocator
	tsoAllocator *allocator.TimestampAllocator
	segAssigner  *allocator.SegIDAssigner

	manipulationMsgStream *pulsarms.PulsarMsgStream
	queryMsgStream        *pulsarms.PulsarMsgStream

	tracer opentracing.Tracer
	closer io.Closer

	// Add callback functions at different stages
	startCallbacks []func()
	closeCallbacks []func()
}

func CreateProxyNodeImpl(ctx context.Context) (*NodeImpl, error) {
	rand.Seed(time.Now().UnixNano())
	ctx1, cancel := context.WithCancel(ctx)
	node := &NodeImpl{
		ctx:    ctx1,
		cancel: cancel,
	}

	return node, nil
}

func (node *NodeImpl) Init() error {
	//Params.Init()

	var err error

	cfg := &config.Configuration{
		ServiceName: "proxynode",
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
	}
	node.tracer, node.closer, err = cfg.NewTracer()
	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}
	opentracing.SetGlobalTracer(node.tracer)

	pulsarAddress := Params.PulsarAddress()

	node.queryMsgStream = pulsarms.NewPulsarMsgStream(node.ctx, Params.MsgStreamSearchBufSize())
	node.queryMsgStream.SetPulsarClient(pulsarAddress)
	node.queryMsgStream.CreatePulsarProducers(Params.SearchChannelNames())

	masterAddr := Params.MasterAddress()
	idAllocator, err := allocator.NewIDAllocator(node.ctx, masterAddr)

	if err != nil {
		return err
	}
	node.idAllocator = idAllocator
	node.idAllocator.PeerID = Params.ProxyID()

	tsoAllocator, err := allocator.NewTimestampAllocator(node.ctx, masterAddr)
	if err != nil {
		return err
	}
	node.tsoAllocator = tsoAllocator
	node.tsoAllocator.PeerID = Params.ProxyID()

	segAssigner, err := allocator.NewSegIDAssigner(node.ctx, masterAddr, node.lastTick)
	if err != nil {
		panic(err)
	}
	node.segAssigner = segAssigner
	node.segAssigner.PeerID = Params.ProxyID()

	node.manipulationMsgStream = pulsarms.NewPulsarMsgStream(node.ctx, Params.MsgStreamInsertBufSize())
	node.manipulationMsgStream.SetPulsarClient(pulsarAddress)
	node.manipulationMsgStream.CreatePulsarProducers(Params.InsertChannelNames())
	repackFuncImpl := func(tsMsgs []msgstream.TsMsg, hashKeys [][]int32) (map[int32]*msgstream.MsgPack, error) {
		return insertRepackFunc(tsMsgs, hashKeys, node.segAssigner, false)
	}
	node.manipulationMsgStream.SetRepackFunc(repackFuncImpl)

	node.sched, err = NewTaskScheduler(node.ctx, node.idAllocator, node.tsoAllocator)
	if err != nil {
		return err
	}

	node.tick = newTimeTick(node.ctx, node.tsoAllocator, time.Millisecond*200, node.sched.TaskDoneTest)

	return nil
}

func (node *NodeImpl) Start() error {
	err := node.connectMaster()
	if err != nil {
		return err
	}
	initGlobalMetaCache(node.ctx, node)
	node.manipulationMsgStream.Start()
	node.queryMsgStream.Start()
	node.sched.Start()
	node.idAllocator.Start()
	node.tsoAllocator.Start()
	node.segAssigner.Start()
	node.tick.Start()

	// Start callbacks
	for _, cb := range node.startCallbacks {
		cb()
	}

	return nil
}

func (node *NodeImpl) Stop() error {
	node.cancel()

	node.tsoAllocator.Close()
	node.idAllocator.Close()
	node.segAssigner.Close()
	node.sched.Close()
	node.manipulationMsgStream.Close()
	node.queryMsgStream.Close()
	node.tick.Close()

	node.wg.Wait()

	if node.closer != nil {
		err := node.closer.Close()
		if err != nil {
			return err
		}
	}

	for _, cb := range node.closeCallbacks {
		cb()
	}

	return nil
}

// AddStartCallback adds a callback in the startServer phase.
func (node *NodeImpl) AddStartCallback(callbacks ...func()) {
	node.startCallbacks = append(node.startCallbacks, callbacks...)
}

func (node *NodeImpl) lastTick() Timestamp {
	return node.tick.LastTick()
}

// AddCloseCallback adds a callback in the Close phase.
func (node *NodeImpl) AddCloseCallback(callbacks ...func()) {
	node.closeCallbacks = append(node.closeCallbacks, callbacks...)
}

func (node *NodeImpl) connectMaster() error {
	masterAddr := Params.MasterAddress()
	log.Printf("NodeImpl connected to master, master_addr=%s", masterAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, masterAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Printf("NodeImpl connect to master failed, error= %v", err)
		return err
	}
	log.Printf("NodeImpl connected to master, master_addr=%s", masterAddr)
	node.masterConn = conn
	node.masterClient = masterpb.NewMasterServiceClient(conn)
	return nil
}