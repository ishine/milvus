// Copyright (C) 2019-2020 Zilliz. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License
// is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
// or implied. See the License for the specific language governing permissions and limitations under the License.

package masterservice

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"

	"github.com/milvus-io/milvus/internal/allocator"
	etcdkv "github.com/milvus-io/milvus/internal/kv/etcd"
	"github.com/milvus-io/milvus/internal/log"
	ms "github.com/milvus-io/milvus/internal/msgstream"
	"github.com/milvus-io/milvus/internal/proto/commonpb"
	"github.com/milvus-io/milvus/internal/proto/datapb"
	"github.com/milvus-io/milvus/internal/proto/indexpb"
	"github.com/milvus-io/milvus/internal/proto/internalpb"
	"github.com/milvus-io/milvus/internal/proto/masterpb"
	"github.com/milvus-io/milvus/internal/proto/milvuspb"
	"github.com/milvus-io/milvus/internal/proto/proxypb"
	"github.com/milvus-io/milvus/internal/proto/querypb"
	"github.com/milvus-io/milvus/internal/tso"
	"github.com/milvus-io/milvus/internal/types"
	"github.com/milvus-io/milvus/internal/util/retry"
	"github.com/milvus-io/milvus/internal/util/tsoutil"
	"github.com/milvus-io/milvus/internal/util/typeutil"
)

//  internalpb -> internalpb
//  proxypb(proxy_service)
//  querypb(query_service)
//  datapb(data_service)
//  indexpb(index_service)
//  milvuspb -> milvuspb
//  masterpb2 -> masterpb (master_service)

// ------------------ struct -----------------------

// master core
type Core struct {
	/*
		ProxyServiceClient Interface:
		get proxy service time tick channel,InvalidateCollectionMetaCache

		DataService Interface:
		Segment States Channel, from DataService, if create new segment, data service should put the segment id into this channel, and let the master add the segment id to the collection meta
		Segment Flush Watcher, monitor if segment has flushed into disk

		IndexService Interface
		IndexService Sch, tell index service to build index
	*/

	MetaTable *metaTable
	//id allocator
	idAllocator       func(count uint32) (typeutil.UniqueID, typeutil.UniqueID, error)
	idAllocatorUpdate func() error

	//tso allocator
	tsoAllocator       func(count uint32) (typeutil.Timestamp, error)
	tsoAllocatorUpdate func() error

	//inner members
	ctx     context.Context
	cancel  context.CancelFunc
	etcdCli *clientv3.Client
	kvBase  *etcdkv.EtcdKV
	metaKV  *etcdkv.EtcdKV

	//setMsgStreams, receive time tick from proxy service time tick channel
	ProxyTimeTickChan chan typeutil.Timestamp

	//setMsgStreams, send time tick into dd channel and time tick channel
	SendTimeTick func(t typeutil.Timestamp) error

	//setMsgStreams, send create collection into dd channel
	DdCreateCollectionReq func(ctx context.Context, req *internalpb.CreateCollectionRequest) error

	//setMsgStreams, send drop collection into dd channel, and notify the proxy to delete this collection
	DdDropCollectionReq func(ctx context.Context, req *internalpb.DropCollectionRequest) error

	//setMsgStreams, send create partition into dd channel
	DdCreatePartitionReq func(ctx context.Context, req *internalpb.CreatePartitionRequest) error

	//setMsgStreams, send drop partition into dd channel
	DdDropPartitionReq func(ctx context.Context, req *internalpb.DropPartitionRequest) error

	//setMsgStreams segment channel, receive segment info from data service, if master create segment
	DataServiceSegmentChan chan *datapb.SegmentInfo

	//setMsgStreams ,if segment flush completed, data node would put segment id into msg stream
	DataNodeSegmentFlushCompletedChan chan typeutil.UniqueID

	//get binlog file path from data service,
	GetBinlogFilePathsFromDataServiceReq func(segID typeutil.UniqueID, fieldID typeutil.UniqueID) ([]string, error)
	GetNumRowsReq                        func(segID typeutil.UniqueID, isFromFlushedChan bool) (int64, error)

	//call index builder's client to build index, return build id
	BuildIndexReq func(ctx context.Context, binlog []string, typeParams []*commonpb.KeyValuePair, indexParams []*commonpb.KeyValuePair, indexID typeutil.UniqueID, indexName string) (typeutil.UniqueID, error)
	DropIndexReq  func(ctx context.Context, indexID typeutil.UniqueID) error

	//proxy service interface, notify proxy service to drop collection
	InvalidateCollectionMetaCache func(ctx context.Context, ts typeutil.Timestamp, dbName string, collectionName string) error

	//query service interface, notify query service to release collection
	ReleaseCollection func(ctx context.Context, ts typeutil.Timestamp, dbID typeutil.UniqueID, collectionID typeutil.UniqueID) error

	// put create index task into this chan
	indexTaskQueue chan *CreateIndexTask

	//dd request scheduler
	ddReqQueue      chan reqTask //dd request will be push into this chan
	lastDdTimeStamp typeutil.Timestamp

	//time tick loop
	lastTimeTick typeutil.Timestamp

	//states code
	stateCode atomic.Value

	//call once
	initOnce  sync.Once
	startOnce sync.Once
	//isInit    atomic.Value

	msFactory ms.Factory
}

// --------------------- function --------------------------

func NewCore(c context.Context, factory ms.Factory) (*Core, error) {
	ctx, cancel := context.WithCancel(c)
	rand.Seed(time.Now().UnixNano())
	core := &Core{
		ctx:       ctx,
		cancel:    cancel,
		msFactory: factory,
	}
	core.UpdateStateCode(internalpb.StateCode_Abnormal)
	return core, nil
}

func (c *Core) UpdateStateCode(code internalpb.StateCode) {
	c.stateCode.Store(code)
}

func (c *Core) checkInit() error {
	if c.MetaTable == nil {
		return fmt.Errorf("MetaTable is nil")
	}
	if c.idAllocator == nil {
		return fmt.Errorf("idAllocator is nil")
	}
	if c.idAllocatorUpdate == nil {
		return fmt.Errorf("idAllocatorUpdate is nil")
	}
	if c.tsoAllocator == nil {
		return fmt.Errorf("tsoAllocator is nil")
	}
	if c.tsoAllocatorUpdate == nil {
		return fmt.Errorf("tsoAllocatorUpdate is nil")
	}
	if c.etcdCli == nil {
		return fmt.Errorf("etcdCli is nil")
	}
	if c.metaKV == nil {
		return fmt.Errorf("metaKV is nil")
	}
	if c.kvBase == nil {
		return fmt.Errorf("kvBase is nil")
	}
	if c.ProxyTimeTickChan == nil {
		return fmt.Errorf("ProxyTimeTickChan is nil")
	}
	if c.ddReqQueue == nil {
		return fmt.Errorf("ddReqQueue is nil")
	}
	if c.DdCreateCollectionReq == nil {
		return fmt.Errorf("DdCreateCollectionReq is nil")
	}
	if c.DdDropCollectionReq == nil {
		return fmt.Errorf("DdDropCollectionReq is nil")
	}
	if c.DdCreatePartitionReq == nil {
		return fmt.Errorf("DdCreatePartitionReq is nil")
	}
	if c.DdDropPartitionReq == nil {
		return fmt.Errorf("DdDropPartitionReq is nil")
	}
	if c.DataServiceSegmentChan == nil {
		return fmt.Errorf("DataServiceSegmentChan is nil")
	}
	if c.GetBinlogFilePathsFromDataServiceReq == nil {
		return fmt.Errorf("GetBinlogFilePathsFromDataServiceReq is nil")
	}
	if c.GetNumRowsReq == nil {
		return fmt.Errorf("GetNumRowsReq is nil")
	}
	if c.BuildIndexReq == nil {
		return fmt.Errorf("BuildIndexReq is nil")
	}
	if c.DropIndexReq == nil {
		return fmt.Errorf("DropIndexReq is nil")
	}
	if c.InvalidateCollectionMetaCache == nil {
		return fmt.Errorf("InvalidateCollectionMetaCache is nil")
	}
	if c.indexTaskQueue == nil {
		return fmt.Errorf("indexTaskQueue is nil")
	}
	if c.DataNodeSegmentFlushCompletedChan == nil {
		return fmt.Errorf("DataNodeSegmentFlushCompletedChan is nil")
	}
	if c.ReleaseCollection == nil {
		return fmt.Errorf("ReleaseCollection is nil")
	}
	return nil
}

func (c *Core) startDdScheduler() {
	for {
		select {
		case <-c.ctx.Done():
			log.Debug("close dd scheduler, exit task execution loop")
			return
		case task, ok := <-c.ddReqQueue:
			if !ok {
				log.Debug("dd chan is closed, exit task execution loop")
				return
			}
			ts, err := task.Ts()
			if err != nil {
				task.Notify(err)
				break
			}
			if !task.IgnoreTimeStamp() && ts <= c.lastDdTimeStamp {
				task.Notify(fmt.Errorf("input timestamp = %d, last dd time stamp = %d", ts, c.lastDdTimeStamp))
				break
			}
			err = task.Execute(task.Ctx())
			task.Notify(err)
			if ts > c.lastDdTimeStamp {
				c.lastDdTimeStamp = ts
			}
		}
	}
}

func (c *Core) startTimeTickLoop() {
	for {
		select {
		case <-c.ctx.Done():
			log.Debug("close master time tick loop")
			return
		case tt, ok := <-c.ProxyTimeTickChan:
			if !ok {
				log.Warn("proxyTimeTickStream is closed, exit time tick loop")
				return
			}
			if tt <= c.lastTimeTick {
				log.Warn("master time tick go back", zap.Uint64("last time tick", c.lastTimeTick), zap.Uint64("input time tick ", tt))
			}
			if err := c.SendTimeTick(tt); err != nil {
				log.Warn("master send time tick into dd and time_tick channel failed", zap.String("error", err.Error()))
			}
			c.lastTimeTick = tt
		}
	}
}

//data service send segment info to master when create segment
func (c *Core) startDataServiceSegmentLoop() {
	for {
		select {
		case <-c.ctx.Done():
			log.Debug("close data service segment loop")
			return
		case seg, ok := <-c.DataServiceSegmentChan:
			if !ok {
				log.Debug("data service segment is closed, exit loop")
				return
			}
			if seg == nil {
				log.Warn("segment from data service is nil")
			} else if err := c.MetaTable.AddSegment(seg); err != nil {
				//what if master add segment failed, but data service success?
				log.Warn("add segment info meta table failed ", zap.String("error", err.Error()))
			} else {
				log.Debug("add segment", zap.Int64("collection id", seg.CollectionID), zap.Int64("partition id", seg.PartitionID), zap.Int64("segment id", seg.ID))
			}
		}
	}
}

//create index loop
func (c *Core) startCreateIndexLoop() {
	for {
		select {
		case <-c.ctx.Done():
			log.Debug("close create index loop")
			return
		case t, ok := <-c.indexTaskQueue:
			if !ok {
				log.Debug("index task chan has closed, exit loop")
				return
			}
			if err := t.BuildIndex(); err != nil {
				log.Warn("create index failed", zap.String("error", err.Error()))
			} else {
				log.Debug("create index", zap.String("index name", t.indexName), zap.String("field name", t.fieldSchema.Name), zap.Int64("segment id", t.segmentID))
			}
		}
	}
}

func (c *Core) startSegmentFlushCompletedLoop() {
	for {
		select {
		case <-c.ctx.Done():
			log.Debug("close segment flush completed loop")
			return
		case seg, ok := <-c.DataNodeSegmentFlushCompletedChan:
			if !ok {
				log.Debug("data node segment flush completed chan has closed, exit loop")
			}
			log.Debug("flush segment", zap.Int64("id", seg))
			coll, err := c.MetaTable.GetCollectionBySegmentID(seg)
			if err != nil {
				log.Warn("GetCollectionBySegmentID error", zap.Error(err))
				break
			}
			err = c.MetaTable.AddFlushedSegment(seg)
			if err != nil {
				log.Warn("AddFlushedSegment error", zap.Error(err))
			}
			for _, f := range coll.FieldIndexes {
				idxInfo, err := c.MetaTable.GetIndexByID(f.IndexID)
				if err != nil {
					log.Warn("index not found", zap.Int64("index id", f.IndexID))
					continue
				}

				fieldSch, err := GetFieldSchemaByID(coll, f.FiledID)
				if err == nil {
					t := &CreateIndexTask{
						ctx:               c.ctx,
						core:              c,
						segmentID:         seg,
						indexName:         idxInfo.IndexName,
						indexID:           idxInfo.IndexID,
						fieldSchema:       fieldSch,
						indexParams:       idxInfo.IndexParams,
						isFromFlushedChan: true,
					}
					c.indexTaskQueue <- t
				}
			}
		}
	}
}

func (c *Core) tsLoop() {
	tsoTicker := time.NewTicker(tso.UpdateTimestampStep)
	defer tsoTicker.Stop()
	ctx, cancel := context.WithCancel(c.ctx)
	defer cancel()
	for {
		select {
		case <-tsoTicker.C:
			if err := c.tsoAllocatorUpdate(); err != nil {
				log.Warn("failed to update timestamp: ", zap.Error(err))
				continue
			}
			if err := c.idAllocatorUpdate(); err != nil {
				log.Warn("failed to update id: ", zap.Error(err))
				continue
			}
		case <-ctx.Done():
			// Server is closed and it should return nil.
			log.Debug("tsLoop is closed")
			return
		}
	}
}
func (c *Core) setMsgStreams() error {
	if Params.PulsarAddress == "" {
		return fmt.Errorf("PulsarAddress is empty")
	}
	if Params.MsgChannelSubName == "" {
		return fmt.Errorf("MsgChannelSubName is emptyr")
	}

	//proxy time tick stream,
	if Params.ProxyTimeTickChannel == "" {
		return fmt.Errorf("ProxyTimeTickChannel is empty")
	}

	var err error
	m := map[string]interface{}{
		"PulsarAddress":  Params.PulsarAddress,
		"ReceiveBufSize": 1024,
		"PulsarBufSize":  1024}
	err = c.msFactory.SetParams(m)
	if err != nil {
		return err
	}

	proxyTimeTickStream, _ := c.msFactory.NewMsgStream(c.ctx)
	proxyTimeTickStream.AsConsumer([]string{Params.ProxyTimeTickChannel}, Params.MsgChannelSubName)
	log.Debug("master AsConsumer: " + Params.ProxyTimeTickChannel + " : " + Params.MsgChannelSubName)
	proxyTimeTickStream.Start()

	// master time tick channel
	if Params.TimeTickChannel == "" {
		return fmt.Errorf("TimeTickChannel is empty")
	}
	timeTickStream, _ := c.msFactory.NewMsgStream(c.ctx)
	timeTickStream.AsProducer([]string{Params.TimeTickChannel})
	log.Debug("masterservice AsProducer: " + Params.TimeTickChannel)

	// master dd channel
	if Params.DdChannel == "" {
		return fmt.Errorf("DdChannel is empty")
	}
	ddStream, _ := c.msFactory.NewMsgStream(c.ctx)
	ddStream.AsProducer([]string{Params.DdChannel})
	log.Debug("masterservice AsProducer: " + Params.DdChannel)

	c.SendTimeTick = func(t typeutil.Timestamp) error {
		msgPack := ms.MsgPack{}
		baseMsg := ms.BaseMsg{
			BeginTimestamp: t,
			EndTimestamp:   t,
			HashValues:     []uint32{0},
		}
		timeTickResult := internalpb.TimeTickMsg{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_TimeTick,
				MsgID:     0,
				Timestamp: t,
				SourceID:  int64(Params.NodeID),
			},
		}
		timeTickMsg := &ms.TimeTickMsg{
			BaseMsg:     baseMsg,
			TimeTickMsg: timeTickResult,
		}
		msgPack.Msgs = append(msgPack.Msgs, timeTickMsg)
		if err := timeTickStream.Broadcast(&msgPack); err != nil {
			return err
		}
		if err := ddStream.Broadcast(&msgPack); err != nil {
			return err
		}
		return nil
	}

	c.DdCreateCollectionReq = func(ctx context.Context, req *internalpb.CreateCollectionRequest) error {
		msgPack := ms.MsgPack{}
		baseMsg := ms.BaseMsg{
			Ctx:            ctx,
			BeginTimestamp: req.Base.Timestamp,
			EndTimestamp:   req.Base.Timestamp,
			HashValues:     []uint32{0},
		}
		collMsg := &ms.CreateCollectionMsg{
			BaseMsg:                 baseMsg,
			CreateCollectionRequest: *req,
		}
		msgPack.Msgs = append(msgPack.Msgs, collMsg)
		if err := ddStream.Broadcast(&msgPack); err != nil {
			return err
		}
		return nil
	}

	c.DdDropCollectionReq = func(ctx context.Context, req *internalpb.DropCollectionRequest) error {
		msgPack := ms.MsgPack{}
		baseMsg := ms.BaseMsg{
			Ctx:            ctx,
			BeginTimestamp: req.Base.Timestamp,
			EndTimestamp:   req.Base.Timestamp,
			HashValues:     []uint32{0},
		}
		collMsg := &ms.DropCollectionMsg{
			BaseMsg:               baseMsg,
			DropCollectionRequest: *req,
		}
		msgPack.Msgs = append(msgPack.Msgs, collMsg)
		if err := ddStream.Broadcast(&msgPack); err != nil {
			return err
		}
		return nil
	}

	c.DdCreatePartitionReq = func(ctx context.Context, req *internalpb.CreatePartitionRequest) error {
		msgPack := ms.MsgPack{}
		baseMsg := ms.BaseMsg{
			Ctx:            ctx,
			BeginTimestamp: req.Base.Timestamp,
			EndTimestamp:   req.Base.Timestamp,
			HashValues:     []uint32{0},
		}
		collMsg := &ms.CreatePartitionMsg{
			BaseMsg:                baseMsg,
			CreatePartitionRequest: *req,
		}
		msgPack.Msgs = append(msgPack.Msgs, collMsg)
		if err := ddStream.Broadcast(&msgPack); err != nil {
			return err
		}
		return nil
	}

	c.DdDropPartitionReq = func(ctx context.Context, req *internalpb.DropPartitionRequest) error {
		msgPack := ms.MsgPack{}
		baseMsg := ms.BaseMsg{
			Ctx:            ctx,
			BeginTimestamp: req.Base.Timestamp,
			EndTimestamp:   req.Base.Timestamp,
			HashValues:     []uint32{0},
		}
		collMsg := &ms.DropPartitionMsg{
			BaseMsg:              baseMsg,
			DropPartitionRequest: *req,
		}
		msgPack.Msgs = append(msgPack.Msgs, collMsg)
		if err := ddStream.Broadcast(&msgPack); err != nil {
			return err
		}
		return nil
	}

	// receive time tick from msg stream
	c.ProxyTimeTickChan = make(chan typeutil.Timestamp, 1024)
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			case ttmsgs, ok := <-proxyTimeTickStream.Chan():
				if !ok {
					log.Warn("proxy time tick msg stream closed")
					return
				}
				if len(ttmsgs.Msgs) > 0 {
					for _, ttm := range ttmsgs.Msgs {
						ttmsg, ok := ttm.(*ms.TimeTickMsg)
						if !ok {
							continue
						}
						c.ProxyTimeTickChan <- ttmsg.Base.Timestamp
					}
				}
			}
		}
	}()

	//segment channel, data service create segment,or data node flush segment will put msg in this channel
	if Params.DataServiceSegmentChannel == "" {
		return fmt.Errorf("DataServiceSegmentChannel is empty")
	}
	dataServiceStream, _ := c.msFactory.NewMsgStream(c.ctx)
	dataServiceStream.AsConsumer([]string{Params.DataServiceSegmentChannel}, Params.MsgChannelSubName)
	log.Debug("master AsConsumer: " + Params.DataServiceSegmentChannel + " : " + Params.MsgChannelSubName)
	dataServiceStream.Start()
	c.DataServiceSegmentChan = make(chan *datapb.SegmentInfo, 1024)
	c.DataNodeSegmentFlushCompletedChan = make(chan typeutil.UniqueID, 1024)

	// receive segment info from msg stream
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			case segMsg, ok := <-dataServiceStream.Chan():
				if !ok {
					log.Warn("data service segment msg closed")
				}
				if len(segMsg.Msgs) > 0 {
					for _, segm := range segMsg.Msgs {
						segInfoMsg, ok := segm.(*ms.SegmentInfoMsg)
						if ok {
							c.DataServiceSegmentChan <- segInfoMsg.Segment
						} else {
							flushMsg, ok := segm.(*ms.FlushCompletedMsg)
							if ok {
								c.DataNodeSegmentFlushCompletedChan <- flushMsg.SegmentFlushCompletedMsg.SegmentID
							} else {
								log.Debug("receive unexpected msg from data service stream", zap.Stringer("segment", segInfoMsg.SegmentMsg.Segment))
							}
						}
					}
				}
			}
		}
	}()

	return nil
}

func (c *Core) SetProxyService(ctx context.Context, s types.ProxyService) error {
	rsp, err := s.GetTimeTickChannel(ctx)
	if err != nil {
		return err
	}
	Params.ProxyTimeTickChannel = rsp.Value
	log.Debug("proxy time tick", zap.String("channel name", Params.ProxyTimeTickChannel))

	c.InvalidateCollectionMetaCache = func(ctx context.Context, ts typeutil.Timestamp, dbName string, collectionName string) error {
		status, _ := s.InvalidateCollectionMetaCache(ctx, &proxypb.InvalidateCollMetaCacheRequest{
			Base: &commonpb.MsgBase{
				MsgType:   0, //TODO,MsgType
				MsgID:     0,
				Timestamp: ts,
				SourceID:  int64(Params.NodeID),
			},
			DbName:         dbName,
			CollectionName: collectionName,
		})
		if status == nil {
			return fmt.Errorf("invalidate collection metacache resp is nil")
		}
		if status.ErrorCode != commonpb.ErrorCode_Success {
			return fmt.Errorf(status.Reason)
		}
		return nil
	}
	return nil
}

func (c *Core) SetDataService(ctx context.Context, s types.DataService) error {
	rsp, err := s.GetSegmentInfoChannel(ctx)
	if err != nil {
		return err
	}
	Params.DataServiceSegmentChannel = rsp.Value
	log.Debug("data service segment", zap.String("channel name", Params.DataServiceSegmentChannel))

	c.GetBinlogFilePathsFromDataServiceReq = func(segID typeutil.UniqueID, fieldID typeutil.UniqueID) ([]string, error) {
		ts, err := c.tsoAllocator(1)
		if err != nil {
			return nil, err
		}
		binlog, err := s.GetInsertBinlogPaths(ctx, &datapb.GetInsertBinlogPathsRequest{
			Base: &commonpb.MsgBase{
				MsgType:   0, //TODO, msg type
				MsgID:     0,
				Timestamp: ts,
				SourceID:  int64(Params.NodeID),
			},
			SegmentID: segID,
		})
		if err != nil {
			return nil, err
		}
		if binlog.Status.ErrorCode != commonpb.ErrorCode_Success {
			return nil, fmt.Errorf("GetInsertBinlogPaths from data service failed, error = %s", binlog.Status.Reason)
		}
		for i := range binlog.FieldIDs {
			if binlog.FieldIDs[i] == fieldID {
				return binlog.Paths[i].Values, nil
			}
		}
		return nil, fmt.Errorf("binlog file not exist, segment id = %d, field id = %d", segID, fieldID)
	}

	c.GetNumRowsReq = func(segID typeutil.UniqueID, isFromFlushedChan bool) (int64, error) {
		ts, err := c.tsoAllocator(1)
		if err != nil {
			return 0, err
		}
		segInfo, err := s.GetSegmentInfo(ctx, &datapb.GetSegmentInfoRequest{
			Base: &commonpb.MsgBase{
				MsgType:   0, //TODO, msg type
				MsgID:     0,
				Timestamp: ts,
				SourceID:  int64(Params.NodeID),
			},
			SegmentIDs: []typeutil.UniqueID{segID},
		})
		if err != nil {
			return 0, err
		}
		if segInfo.Status.ErrorCode != commonpb.ErrorCode_Success {
			return 0, fmt.Errorf("GetSegmentInfo from data service failed, error = %s", segInfo.Status.Reason)
		}
		if len(segInfo.Infos) != 1 {
			log.Debug("get segment info empty")
			return 0, nil
		}
		if !isFromFlushedChan && segInfo.Infos[0].State != commonpb.SegmentState_Flushed {
			log.Debug("segment id not flushed", zap.Int64("segment id", segID))
			return 0, nil
		}
		return segInfo.Infos[0].NumRows, nil
	}
	return nil
}

func (c *Core) SetIndexService(s types.IndexService) error {
	c.BuildIndexReq = func(ctx context.Context, binlog []string, typeParams []*commonpb.KeyValuePair, indexParams []*commonpb.KeyValuePair, indexID typeutil.UniqueID, indexName string) (typeutil.UniqueID, error) {
		rsp, err := s.BuildIndex(ctx, &indexpb.BuildIndexRequest{
			DataPaths:   binlog,
			TypeParams:  typeParams,
			IndexParams: indexParams,
			IndexID:     indexID,
			IndexName:   indexName,
		})
		if err != nil {
			return 0, err
		}
		if rsp.Status.ErrorCode != commonpb.ErrorCode_Success {
			return 0, fmt.Errorf("BuildIndex from index service failed, error = %s", rsp.Status.Reason)
		}
		return rsp.IndexBuildID, nil
	}

	c.DropIndexReq = func(ctx context.Context, indexID typeutil.UniqueID) error {
		rsp, err := s.DropIndex(ctx, &indexpb.DropIndexRequest{
			IndexID: indexID,
		})
		if err != nil {
			return err
		}
		if rsp.ErrorCode != commonpb.ErrorCode_Success {
			return fmt.Errorf(rsp.Reason)
		}
		return nil
	}

	return nil
}

func (c *Core) SetQueryService(s types.QueryService) error {
	c.ReleaseCollection = func(ctx context.Context, ts typeutil.Timestamp, dbID typeutil.UniqueID, collectionID typeutil.UniqueID) error {
		req := &querypb.ReleaseCollectionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_ReleaseCollection,
				MsgID:     0, //TODO, msg ID
				Timestamp: ts,
				SourceID:  int64(Params.NodeID),
			},
			DbID:         dbID,
			CollectionID: collectionID,
		}
		rsp, err := s.ReleaseCollection(ctx, req)
		if err != nil {
			return err
		}
		if rsp.ErrorCode != commonpb.ErrorCode_Success {
			return fmt.Errorf("ReleaseCollection from query service failed, error = %s", rsp.Reason)
		}
		return nil
	}
	return nil
}

func (c *Core) Init() error {
	var initError error = nil
	c.initOnce.Do(func() {
		connectEtcdFn := func() error {
			if c.etcdCli, initError = clientv3.New(clientv3.Config{Endpoints: []string{Params.EtcdAddress}, DialTimeout: 5 * time.Second}); initError != nil {
				return initError
			}
			c.metaKV = etcdkv.NewEtcdKV(c.etcdCli, Params.MetaRootPath)
			if c.MetaTable, initError = NewMetaTable(c.metaKV); initError != nil {
				return initError
			}
			c.kvBase = etcdkv.NewEtcdKV(c.etcdCli, Params.KvRootPath)
			return nil
		}
		err := retry.Retry(100000, time.Millisecond*200, connectEtcdFn)
		if err != nil {
			return
		}

		idAllocator := allocator.NewGlobalIDAllocator("idTimestamp", tsoutil.NewTSOKVBase([]string{Params.EtcdAddress}, Params.KvRootPath, "gid"))
		if initError = idAllocator.Initialize(); initError != nil {
			return
		}
		c.idAllocator = func(count uint32) (typeutil.UniqueID, typeutil.UniqueID, error) {
			return idAllocator.Alloc(count)
		}
		c.idAllocatorUpdate = func() error {
			return idAllocator.UpdateID()
		}

		tsoAllocator := tso.NewGlobalTSOAllocator("timestamp", tsoutil.NewTSOKVBase([]string{Params.EtcdAddress}, Params.KvRootPath, "tso"))
		if initError = tsoAllocator.Initialize(); initError != nil {
			return
		}
		c.tsoAllocator = func(count uint32) (typeutil.Timestamp, error) {
			return tsoAllocator.Alloc(count)
		}
		c.tsoAllocatorUpdate = func() error {
			return tsoAllocator.UpdateTSO()
		}

		c.ddReqQueue = make(chan reqTask, 1024)
		c.indexTaskQueue = make(chan *CreateIndexTask, 1024)
		initError = c.setMsgStreams()
	})
	if initError == nil {
		log.Debug("Master service", zap.String("State Code", internalpb.StateCode_name[int32(internalpb.StateCode_Initializing)]))
	}
	return initError
}

func (c *Core) Start() error {
	if err := c.checkInit(); err != nil {
		return err
	}

	log.Debug("master", zap.Int64("node id", int64(Params.NodeID)))
	log.Debug("master", zap.String("dd channel name", Params.DdChannel))
	log.Debug("master", zap.String("time tick channel name", Params.TimeTickChannel))

	c.startOnce.Do(func() {
		go c.startDdScheduler()
		go c.startTimeTickLoop()
		go c.startDataServiceSegmentLoop()
		go c.startCreateIndexLoop()
		go c.startSegmentFlushCompletedLoop()
		go c.tsLoop()
		c.stateCode.Store(internalpb.StateCode_Healthy)
	})
	log.Debug("Master service", zap.String("State Code", internalpb.StateCode_name[int32(internalpb.StateCode_Healthy)]))
	return nil
}

func (c *Core) Stop() error {
	c.cancel()
	c.stateCode.Store(internalpb.StateCode_Abnormal)
	return nil
}

func (c *Core) GetComponentStates(ctx context.Context) (*internalpb.ComponentStates, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	log.Debug("GetComponentStates", zap.String("State Code", internalpb.StateCode_name[int32(code)]))

	return &internalpb.ComponentStates{
		State: &internalpb.ComponentInfo{
			NodeID:    int64(Params.NodeID),
			Role:      typeutil.MasterServiceRole,
			StateCode: code,
			ExtraInfo: nil,
		},
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		},
		SubcomponentStates: []*internalpb.ComponentInfo{
			{
				NodeID:    int64(Params.NodeID),
				Role:      typeutil.MasterServiceRole,
				StateCode: code,
				ExtraInfo: nil,
			},
		},
	}, nil
}

func (c *Core) GetTimeTickChannel(ctx context.Context) (*milvuspb.StringResponse, error) {
	return &milvuspb.StringResponse{
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		},
		Value: Params.TimeTickChannel,
	}, nil
}

func (c *Core) GetDdChannel(ctx context.Context) (*milvuspb.StringResponse, error) {
	return &milvuspb.StringResponse{
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		},
		Value: Params.DdChannel,
	}, nil
}

func (c *Core) GetStatisticsChannel(ctx context.Context) (*milvuspb.StringResponse, error) {
	return &milvuspb.StringResponse{
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		},
		Value: Params.StatisticsChannel,
	}, nil
}

func (c *Core) CreateCollection(ctx context.Context, in *milvuspb.CreateCollectionRequest) (*commonpb.Status, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
		}, nil
	}
	log.Debug("CreateCollection ", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
	t := &CreateCollectionReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("CreateCollection failed", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    "Create collection failed: " + err.Error(),
		}, nil
	}
	log.Debug("CreateCollection Success", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
	return &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}, nil
}

func (c *Core) DropCollection(ctx context.Context, in *milvuspb.DropCollectionRequest) (*commonpb.Status, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
		}, nil
	}
	log.Debug("DropCollection", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
	t := &DropCollectionReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("DropCollection Failed", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    "Drop collection failed: " + err.Error(),
		}, nil
	}
	log.Debug("DropCollection Success", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
	return &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}, nil
}

func (c *Core) HasCollection(ctx context.Context, in *milvuspb.HasCollectionRequest) (*milvuspb.BoolResponse, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &milvuspb.BoolResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
			},
			Value: false,
		}, nil
	}
	log.Debug("HasCollection", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
	t := &HasCollectionReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req:           in,
		HasCollection: false,
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("HasCollection Failed", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
		return &milvuspb.BoolResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    "Has collection failed: " + err.Error(),
			},
			Value: false,
		}, nil
	}
	log.Debug("HasCollection Success", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
	return &milvuspb.BoolResponse{
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		},
		Value: t.HasCollection,
	}, nil
}

func (c *Core) DescribeCollection(ctx context.Context, in *milvuspb.DescribeCollectionRequest) (*milvuspb.DescribeCollectionResponse, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &milvuspb.DescribeCollectionResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
			},
			Schema:       nil,
			CollectionID: 0,
		}, nil
	}
	log.Debug("DescribeCollection", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
	t := &DescribeCollectionReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
		Rsp: &milvuspb.DescribeCollectionResponse{},
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("DescribeCollection Failed", zap.String("name", in.CollectionName), zap.Error(err), zap.Int64("msgID", in.Base.MsgID))
		return &milvuspb.DescribeCollectionResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    "describe collection failed: " + err.Error(),
			},
			Schema: nil,
		}, nil
	}
	log.Debug("DescribeCollection Success", zap.String("name", in.CollectionName), zap.Int64("msgID", in.Base.MsgID))
	t.Rsp.Status = &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}
	return t.Rsp, nil
}

func (c *Core) ShowCollections(ctx context.Context, in *milvuspb.ShowCollectionsRequest) (*milvuspb.ShowCollectionsResponse, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &milvuspb.ShowCollectionsResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
			},
			CollectionNames: nil,
		}, nil
	}
	log.Debug("ShowCollections", zap.String("dbname", in.DbName), zap.Int64("msgID", in.Base.MsgID))
	t := &ShowCollectionReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
		Rsp: &milvuspb.ShowCollectionsResponse{
			CollectionNames: nil,
		},
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("ShowCollections failed", zap.String("dbname", in.DbName), zap.Int64("msgID", in.Base.MsgID))
		return &milvuspb.ShowCollectionsResponse{
			CollectionNames: nil,
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    "ShowCollections failed: " + err.Error(),
			},
		}, nil
	}
	log.Debug("ShowCollections Success", zap.String("dbname", in.DbName), zap.Strings("collection names", t.Rsp.CollectionNames), zap.Int64("msgID", in.Base.MsgID))
	t.Rsp.Status = &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}
	return t.Rsp, nil
}

func (c *Core) CreatePartition(ctx context.Context, in *milvuspb.CreatePartitionRequest) (*commonpb.Status, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
		}, nil
	}
	log.Debug("CreatePartition", zap.String("collection name", in.CollectionName), zap.String("partition name", in.PartitionName), zap.Int64("msgID", in.Base.MsgID))
	t := &CreatePartitionReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("CreatePartition Failed", zap.String("collection name", in.CollectionName), zap.String("partition name", in.PartitionName), zap.Int64("msgID", in.Base.MsgID))
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    "create partition failed: " + err.Error(),
		}, nil
	}
	log.Debug("CreatePartition Success", zap.String("collection name", in.CollectionName), zap.String("partition name", in.PartitionName), zap.Int64("msgID", in.Base.MsgID))
	return &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}, nil
}

func (c *Core) DropPartition(ctx context.Context, in *milvuspb.DropPartitionRequest) (*commonpb.Status, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
		}, nil
	}
	log.Debug("DropPartition", zap.String("collection name", in.CollectionName), zap.String("partition name", in.PartitionName), zap.Int64("msgID", in.Base.MsgID))
	t := &DropPartitionReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("DropPartition Failed", zap.String("collection name", in.CollectionName), zap.String("partition name", in.PartitionName), zap.Int64("msgID", in.Base.MsgID))
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    "DropPartition failed: " + err.Error(),
		}, nil
	}
	log.Debug("DropPartition Success", zap.String("collection name", in.CollectionName), zap.String("partition name", in.PartitionName), zap.Int64("msgID", in.Base.MsgID))
	return &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}, nil
}

func (c *Core) HasPartition(ctx context.Context, in *milvuspb.HasPartitionRequest) (*milvuspb.BoolResponse, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &milvuspb.BoolResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
			},
			Value: false,
		}, nil
	}
	log.Debug("HasPartition", zap.String("collection name", in.CollectionName), zap.String("partition name", in.PartitionName), zap.Int64("msgID", in.Base.MsgID))
	t := &HasPartitionReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req:          in,
		HasPartition: false,
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("HasPartition Failed", zap.String("collection name", in.CollectionName), zap.String("partition name", in.PartitionName), zap.Int64("msgID", in.Base.MsgID))
		return &milvuspb.BoolResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    "HasPartition failed: " + err.Error(),
			},
			Value: false,
		}, nil
	}
	log.Debug("HasPartition Success", zap.String("collection name", in.CollectionName), zap.String("partition name", in.PartitionName), zap.Int64("msgID", in.Base.MsgID))
	return &milvuspb.BoolResponse{
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		},
		Value: t.HasPartition,
	}, nil
}

func (c *Core) ShowPartitions(ctx context.Context, in *milvuspb.ShowPartitionsRequest) (*milvuspb.ShowPartitionsResponse, error) {
	log.Debug("ShowPartitionRequest received", zap.String("role", Params.RoleName), zap.Int64("msgID", in.Base.MsgID),
		zap.String("collection", in.CollectionName))
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		log.Debug("ShowPartitionRequest failed: master is not healthy", zap.String("role", Params.RoleName),
			zap.Int64("msgID", in.Base.MsgID), zap.String("state", internalpb.StateCode_name[int32(code)]))
		return &milvuspb.ShowPartitionsResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    fmt.Sprintf("master is not healthy, state code = %s", internalpb.StateCode_name[int32(code)]),
			},
			PartitionNames: nil,
			PartitionIDs:   nil,
		}, nil
	}
	t := &ShowPartitionReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
		Rsp: &milvuspb.ShowPartitionsResponse{
			PartitionNames: nil,
			Status:         nil,
		},
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("ShowPartitionsRequest failed", zap.String("role", Params.RoleName), zap.Int64("msgID", in.Base.MsgID), zap.Error(err))
		return &milvuspb.ShowPartitionsResponse{
			PartitionNames: nil,
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    err.Error(),
			},
		}, nil
	}
	log.Debug("ShowPartitions succeed", zap.String("role", Params.RoleName), zap.Int64("msgID", t.Req.Base.MsgID),
		zap.String("collection name", in.CollectionName), zap.Strings("partition names", t.Rsp.PartitionNames),
		zap.Int64s("partition ids", t.Rsp.PartitionIDs))
	t.Rsp.Status = &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}
	return t.Rsp, nil
}

func (c *Core) CreateIndex(ctx context.Context, in *milvuspb.CreateIndexRequest) (*commonpb.Status, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
		}, nil
	}
	log.Debug("CreateIndex", zap.String("collection name", in.CollectionName), zap.String("field name", in.FieldName), zap.Int64("msgID", in.Base.MsgID))
	t := &CreateIndexReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("CreateIndex Failed", zap.String("collection name", in.CollectionName), zap.String("field name", in.FieldName), zap.Int64("msgID", in.Base.MsgID))
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    "CreateIndex failed, error = " + err.Error(),
		}, nil
	}
	log.Debug("CreateIndex Success", zap.String("collection name", in.CollectionName), zap.String("field name", in.FieldName), zap.Int64("msgID", in.Base.MsgID))
	return &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}, nil
}

func (c *Core) DescribeIndex(ctx context.Context, in *milvuspb.DescribeIndexRequest) (*milvuspb.DescribeIndexResponse, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &milvuspb.DescribeIndexResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
			},
			IndexDescriptions: nil,
		}, nil
	}
	log.Debug("DescribeIndex", zap.String("collection name", in.CollectionName), zap.String("field name", in.FieldName), zap.Int64("msgID", in.Base.MsgID))
	t := &DescribeIndexReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
		Rsp: &milvuspb.DescribeIndexResponse{
			Status:            nil,
			IndexDescriptions: nil,
		},
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("DescribeIndex Failed", zap.String("collection name", in.CollectionName), zap.String("field name", in.FieldName), zap.Int64("msgID", in.Base.MsgID))
		return &milvuspb.DescribeIndexResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    "DescribeIndex failed, error = " + err.Error(),
			},
			IndexDescriptions: nil,
		}, nil
	}
	idxNames := make([]string, 0, len(t.Rsp.IndexDescriptions))
	for _, i := range t.Rsp.IndexDescriptions {
		idxNames = append(idxNames, i.IndexName)
	}
	log.Debug("DescribeIndex Success", zap.String("collection name", in.CollectionName), zap.String("field name", in.FieldName), zap.Strings("index names", idxNames), zap.Int64("msgID", in.Base.MsgID))
	if len(t.Rsp.IndexDescriptions) == 0 {
		t.Rsp.Status = &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_IndexNotExist,
			Reason:    "index not exist",
		}
	} else {
		t.Rsp.Status = &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		}
	}
	return t.Rsp, nil
}

func (c *Core) DropIndex(ctx context.Context, in *milvuspb.DropIndexRequest) (*commonpb.Status, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
		}, nil
	}
	log.Debug("DropIndex", zap.String("collection name", in.CollectionName), zap.String("field name", in.FieldName), zap.String("index name", in.IndexName), zap.Int64("msgID", in.Base.MsgID))
	t := &DropIndexReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("DropIndex Failed", zap.String("collection name", in.CollectionName), zap.String("field name", in.FieldName), zap.String("index name", in.IndexName), zap.Int64("msgID", in.Base.MsgID))
		return &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_UnexpectedError,
			Reason:    "DropIndex failed, error = " + err.Error(),
		}, nil
	}
	log.Debug("DropIndex Success", zap.String("collection name", in.CollectionName), zap.String("field name", in.FieldName), zap.String("index name", in.IndexName), zap.Int64("msgID", in.Base.MsgID))
	return &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}, nil
}

func (c *Core) DescribeSegment(ctx context.Context, in *milvuspb.DescribeSegmentRequest) (*milvuspb.DescribeSegmentResponse, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &milvuspb.DescribeSegmentResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
			},
			IndexID: 0,
		}, nil
	}
	log.Debug("DescribeSegment", zap.Int64("collection id", in.CollectionID), zap.Int64("segment id", in.SegmentID), zap.Int64("msgID", in.Base.MsgID))
	t := &DescribeSegmentReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
		Rsp: &milvuspb.DescribeSegmentResponse{
			Status:  nil,
			IndexID: 0,
		},
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("DescribeSegment Failed", zap.Int64("collection id", in.CollectionID), zap.Int64("segment id", in.SegmentID), zap.Int64("msgID", in.Base.MsgID))
		return &milvuspb.DescribeSegmentResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    "DescribeSegment failed, error = " + err.Error(),
			},
			IndexID: 0,
		}, nil
	}
	log.Debug("DescribeSegment Success", zap.Int64("collection id", in.CollectionID), zap.Int64("segment id", in.SegmentID), zap.Int64("msgID", in.Base.MsgID))
	t.Rsp.Status = &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}
	return t.Rsp, nil
}

func (c *Core) ShowSegments(ctx context.Context, in *milvuspb.ShowSegmentsRequest) (*milvuspb.ShowSegmentsResponse, error) {
	code := c.stateCode.Load().(internalpb.StateCode)
	if code != internalpb.StateCode_Healthy {
		return &milvuspb.ShowSegmentsResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    fmt.Sprintf("state code = %s", internalpb.StateCode_name[int32(code)]),
			},
			SegmentIDs: nil,
		}, nil
	}
	log.Debug("ShowSegments", zap.Int64("collection id", in.CollectionID), zap.Int64("partition id", in.PartitionID), zap.Int64("msgID", in.Base.MsgID))
	t := &ShowSegmentReqTask{
		baseReqTask: baseReqTask{
			ctx:  ctx,
			cv:   make(chan error, 1),
			core: c,
		},
		Req: in,
		Rsp: &milvuspb.ShowSegmentsResponse{
			Status:     nil,
			SegmentIDs: nil,
		},
	}
	c.ddReqQueue <- t
	err := t.WaitToFinish()
	if err != nil {
		log.Debug("ShowSegments Failed", zap.Int64("collection id", in.CollectionID), zap.Int64("partition id", in.PartitionID), zap.Int64("msgID", in.Base.MsgID))
		return &milvuspb.ShowSegmentsResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    "ShowSegments failed, error: " + err.Error(),
			},
			SegmentIDs: nil,
		}, nil
	}
	log.Debug("ShowSegments Success", zap.Int64("collection id", in.CollectionID), zap.Int64("partition id", in.PartitionID), zap.Int64s("segments ids", t.Rsp.SegmentIDs), zap.Int64("msgID", in.Base.MsgID))
	t.Rsp.Status = &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
		Reason:    "",
	}
	return t.Rsp, nil
}

func (c *Core) AllocTimestamp(ctx context.Context, in *masterpb.AllocTimestampRequest) (*masterpb.AllocTimestampResponse, error) {
	ts, err := c.tsoAllocator(in.Count)
	if err != nil {
		log.Debug("AllocTimestamp failed", zap.Int64("msgID", in.Base.MsgID), zap.Error(err))
		return &masterpb.AllocTimestampResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    "AllocTimestamp failed: " + err.Error(),
			},
			Timestamp: 0,
			Count:     0,
		}, nil
	}
	// log.Printf("AllocTimestamp : %d", ts)
	return &masterpb.AllocTimestampResponse{
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		},
		Timestamp: ts,
		Count:     in.Count,
	}, nil
}

func (c *Core) AllocID(ctx context.Context, in *masterpb.AllocIDRequest) (*masterpb.AllocIDResponse, error) {
	start, _, err := c.idAllocator(in.Count)
	if err != nil {
		log.Debug("AllocID failed", zap.Int64("msgID", in.Base.MsgID), zap.Error(err))
		return &masterpb.AllocIDResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_UnexpectedError,
				Reason:    "AllocID failed: " + err.Error(),
			},
			ID:    0,
			Count: in.Count,
		}, nil
	}
	log.Debug("AllocID", zap.Int64("id start", start), zap.Uint32("count", in.Count))
	return &masterpb.AllocIDResponse{
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		},
		ID:    start,
		Count: in.Count,
	}, nil
}
