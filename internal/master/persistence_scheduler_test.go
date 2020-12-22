package master

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	etcdkv "github.com/zilliztech/milvus-distributed/internal/kv/etcd"
	"github.com/zilliztech/milvus-distributed/internal/proto/indexbuilderpb"
	"go.etcd.io/etcd/clientv3"
)

func TestPersistenceScheduler(t *testing.T) {
	//Init environment Params
	Init()

	ctx := context.Background()

	//Init client, use Mock instead
	flushClient := &MockWriteNodeClient{}
	buildIndexClient := &MockBuildIndexClient{}
	loadIndexClient := &MockLoadIndexClient{}

	etcdAddr := Params.EtcdAddress
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{etcdAddr}})
	assert.Nil(t, err)
	etcdKV := etcdkv.NewEtcdKV(cli, "/etcd/test/root")

	_, err = cli.Delete(context.TODO(), "/etcd/test/root", clientv3.WithPrefix())
	assert.Nil(t, err)

	meta, err := NewMetaTable(etcdKV)
	assert.Nil(t, err)
	defer meta.client.Close()

	//Init scheduler
	indexLoadSch := NewIndexLoadScheduler(ctx, loadIndexClient, meta)
	indexBuildSch := NewIndexBuildScheduler(ctx, buildIndexClient, meta, indexLoadSch)
	flushSch := NewFlushScheduler(ctx, flushClient, meta, indexBuildSch)

	//scheduler start
	err = indexLoadSch.Start()
	assert.Nil(t, err)
	defer indexLoadSch.Close()

	err = indexBuildSch.Start()
	assert.Nil(t, err)
	defer indexBuildSch.Close()

	err = flushSch.Start()
	assert.Nil(t, err)
	defer flushSch.Close()

	//start from flush scheduler
	err = flushSch.Enqueue(UniqueID(1))
	assert.Nil(t, err)
	//wait flush segment request sent to write node
	time.Sleep(100 * time.Millisecond)
	segDes, err := flushClient.DescribeSegment(UniqueID(1))
	assert.Nil(t, err)
	assert.Equal(t, false, segDes.IsClosed)

	//wait flush to finish
	time.Sleep(3 * time.Second)

	segDes, err = flushClient.DescribeSegment(UniqueID(1))
	assert.Nil(t, err)
	assert.Equal(t, UniqueID(1), segDes.SegmentID)
	assert.Equal(t, true, segDes.IsClosed)

	//wait flush segment request sent to build index node
	time.Sleep(100 * time.Microsecond)
	idxDes, err := buildIndexClient.DescribeIndex(UniqueID(1))
	assert.Nil(t, err)
	assert.Equal(t, indexbuilderpb.IndexStatus_INPROGRESS, idxDes.Status)

	//wait build index to finish
	time.Sleep(3 * time.Second)

	idxDes, err = buildIndexClient.DescribeIndex(UniqueID(1))
	assert.Nil(t, err)
	assert.Equal(t, indexbuilderpb.IndexStatus_FINISHED, idxDes.Status)

}