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

package grpcmasterservice

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	grpcmasterserviceclient "github.com/milvus-io/milvus/internal/distributed/masterservice/client"

	"github.com/golang/protobuf/proto"
	cms "github.com/milvus-io/milvus/internal/masterservice"
	"github.com/milvus-io/milvus/internal/msgstream"
	"github.com/milvus-io/milvus/internal/proto/commonpb"
	"github.com/milvus-io/milvus/internal/proto/datapb"
	"github.com/milvus-io/milvus/internal/proto/internalpb"
	"github.com/milvus-io/milvus/internal/proto/milvuspb"
	"github.com/milvus-io/milvus/internal/proto/schemapb"
	"github.com/milvus-io/milvus/internal/util/typeutil"
	"github.com/stretchr/testify/assert"
)

func TestGrpcService(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	randVal := rand.Int()

	Params.Init()
	Params.Port = (randVal % 100) + 10000
	parts := strings.Split(Params.Address, ":")
	if len(parts) == 2 {
		Params.Address = parts[0] + ":" + strconv.Itoa(Params.Port)
		t.Log("newParams.Address:", Params.Address)
	}

	ctx := context.Background()
	msFactory := msgstream.NewPmsFactory()
	svr, err := NewServer(ctx, msFactory)
	assert.Nil(t, err)
	svr.connectQueryService = false
	svr.connectProxyService = false
	svr.connectIndexService = false
	svr.connectDataService = false

	cms.Params.Init()
	cms.Params.MetaRootPath = fmt.Sprintf("/%d/test/meta", randVal)
	cms.Params.KvRootPath = fmt.Sprintf("/%d/test/kv", randVal)
	cms.Params.ProxyTimeTickChannel = fmt.Sprintf("proxyTimeTick%d", randVal)
	cms.Params.MsgChannelSubName = fmt.Sprintf("msgChannel%d", randVal)
	cms.Params.TimeTickChannel = fmt.Sprintf("timeTick%d", randVal)
	cms.Params.DdChannel = fmt.Sprintf("ddChannel%d", randVal)
	cms.Params.StatisticsChannel = fmt.Sprintf("stateChannel%d", randVal)
	cms.Params.DataServiceSegmentChannel = fmt.Sprintf("segmentChannel%d", randVal)

	cms.Params.MaxPartitionNum = 64
	cms.Params.DefaultPartitionName = "_default"
	cms.Params.DefaultIndexName = "_default"

	t.Logf("master service port = %d", Params.Port)

	err = svr.startGrpc()
	assert.Nil(t, err)
	svr.masterService.UpdateStateCode(internalpb.StateCode_Initializing)

	core := svr.masterService
	err = core.Init()
	assert.Nil(t, err)

	core.ProxyTimeTickChan = make(chan typeutil.Timestamp, 8)
	core.DataNodeSegmentFlushCompletedChan = make(chan typeutil.UniqueID, 8)

	timeTickArray := make([]typeutil.Timestamp, 0, 16)
	core.SendTimeTick = func(ts typeutil.Timestamp) error {
		t.Logf("send time tick %d", ts)
		timeTickArray = append(timeTickArray, ts)
		return nil
	}
	createCollectionArray := make([]*internalpb.CreateCollectionRequest, 0, 16)
	core.DdCreateCollectionReq = func(ctx context.Context, req *internalpb.CreateCollectionRequest) error {
		t.Logf("Create Colllection %s", req.CollectionName)
		createCollectionArray = append(createCollectionArray, req)
		return nil
	}

	dropCollectionArray := make([]*internalpb.DropCollectionRequest, 0, 16)
	core.DdDropCollectionReq = func(ctx context.Context, req *internalpb.DropCollectionRequest) error {
		t.Logf("Drop Collection %s", req.CollectionName)
		dropCollectionArray = append(dropCollectionArray, req)
		return nil
	}

	createPartitionArray := make([]*internalpb.CreatePartitionRequest, 0, 16)
	core.DdCreatePartitionReq = func(ctx context.Context, req *internalpb.CreatePartitionRequest) error {
		t.Logf("Create Partition %s", req.PartitionName)
		createPartitionArray = append(createPartitionArray, req)
		return nil
	}

	dropPartitionArray := make([]*internalpb.DropPartitionRequest, 0, 16)
	core.DdDropPartitionReq = func(ctx context.Context, req *internalpb.DropPartitionRequest) error {
		t.Logf("Drop Partition %s", req.PartitionName)
		dropPartitionArray = append(dropPartitionArray, req)
		return nil
	}

	core.DataServiceSegmentChan = make(chan *datapb.SegmentInfo, 1024)

	core.GetBinlogFilePathsFromDataServiceReq = func(segID typeutil.UniqueID, fieldID typeutil.UniqueID) ([]string, error) {
		return []string{"file1", "file2", "file3"}, nil
	}
	core.GetNumRowsReq = func(segID typeutil.UniqueID, isFromFlushedChan bool) (int64, error) {
		return cms.Params.MinSegmentSizeToEnableIndex, nil
	}

	var binlogLock sync.Mutex
	binlogPathArray := make([]string, 0, 16)
	core.BuildIndexReq = func(ctx context.Context, binlog []string, typeParams []*commonpb.KeyValuePair, indexParams []*commonpb.KeyValuePair, indexID typeutil.UniqueID, indexName string) (typeutil.UniqueID, error) {
		binlogLock.Lock()
		defer binlogLock.Unlock()
		binlogPathArray = append(binlogPathArray, binlog...)
		return 2000, nil
	}

	var dropIDLock sync.Mutex
	dropID := make([]typeutil.UniqueID, 0, 16)
	core.DropIndexReq = func(ctx context.Context, indexID typeutil.UniqueID) error {
		dropIDLock.Lock()
		defer dropIDLock.Unlock()
		dropID = append(dropID, indexID)
		return nil
	}

	collectionMetaCache := make([]string, 0, 16)
	core.InvalidateCollectionMetaCache = func(ctx context.Context, ts typeutil.Timestamp, dbName string, collectionName string) error {
		collectionMetaCache = append(collectionMetaCache, collectionName)
		return nil
	}

	core.ReleaseCollection = func(ctx context.Context, ts typeutil.Timestamp, dbID typeutil.UniqueID, collectionID typeutil.UniqueID) error {
		return nil
	}

	err = svr.start()
	assert.Nil(t, err)

	svr.masterService.UpdateStateCode(internalpb.StateCode_Healthy)

	cli, err := grpcmasterserviceclient.NewClient(Params.Address, 3*time.Second)
	assert.Nil(t, err)

	err = cli.Init()
	assert.Nil(t, err)

	err = cli.Start()
	assert.Nil(t, err)

	t.Run("create collection", func(t *testing.T) {
		schema := schemapb.CollectionSchema{
			Name:        "testColl",
			Description: "testColl",
			AutoID:      true,
			Fields: []*schemapb.FieldSchema{
				{
					FieldID:      100,
					Name:         "vector",
					IsPrimaryKey: false,
					Description:  "vector",
					DataType:     schemapb.DataType_FloatVector,
					TypeParams:   nil,
					IndexParams: []*commonpb.KeyValuePair{
						{
							Key:   "ik1",
							Value: "iv1",
						},
					},
				},
			},
		}

		sbf, err := proto.Marshal(&schema)
		assert.Nil(t, err)

		req := &milvuspb.CreateCollectionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_CreateCollection,
				MsgID:     100,
				Timestamp: 100,
				SourceID:  100,
			},
			DbName:         "testDb",
			CollectionName: "testColl",
			Schema:         sbf,
		}

		status, err := cli.CreateCollection(ctx, req)
		assert.Nil(t, err)

		assert.Equal(t, len(createCollectionArray), 1)
		assert.Equal(t, status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, createCollectionArray[0].Base.MsgType, commonpb.MsgType_CreateCollection)
		assert.Equal(t, createCollectionArray[0].CollectionName, "testColl")

		req.Base.MsgID = 101
		req.Base.Timestamp = 101
		req.Base.SourceID = 101
		status, err = cli.CreateCollection(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, status.ErrorCode, commonpb.ErrorCode_UnexpectedError)

		req.Base.MsgID = 102
		req.Base.Timestamp = 102
		req.Base.SourceID = 102
		req.CollectionName = "testColl-again"
		status, err = cli.CreateCollection(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, status.ErrorCode, commonpb.ErrorCode_UnexpectedError)

		schema.Name = req.CollectionName
		sbf, err = proto.Marshal(&schema)
		assert.Nil(t, err)
		req.Schema = sbf
		req.Base.MsgID = 103
		req.Base.Timestamp = 103
		req.Base.SourceID = 103
		status, err = cli.CreateCollection(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, len(createCollectionArray), 2)
		assert.Equal(t, createCollectionArray[1].Base.MsgType, commonpb.MsgType_CreateCollection)
		assert.Equal(t, createCollectionArray[1].CollectionName, "testColl-again")

		//time stamp go back
		schema.Name = "testColl-goback"
		sbf, err = proto.Marshal(&schema)
		assert.Nil(t, err)
		req.CollectionName = schema.Name
		req.Schema = sbf
		req.Base.MsgID = 103
		req.Base.Timestamp = 103
		req.Base.SourceID = 103
		status, err = cli.CreateCollection(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, status.ErrorCode, commonpb.ErrorCode_UnexpectedError)
		matched, err := regexp.MatchString("input timestamp = [0-9]+, last dd time stamp = [0-9]+", status.Reason)
		assert.Nil(t, err)
		assert.True(t, matched)
	})

	t.Run("has collection", func(t *testing.T) {
		req := &milvuspb.HasCollectionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_HasCollection,
				MsgID:     110,
				Timestamp: 110,
				SourceID:  110,
			},
			DbName:         "testDb",
			CollectionName: "testColl",
		}
		rsp, err := cli.HasCollection(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, rsp.Value, true)

		req = &milvuspb.HasCollectionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_HasCollection,
				MsgID:     111,
				Timestamp: 111,
				SourceID:  111,
			},
			DbName:         "testDb",
			CollectionName: "testColl2",
		}
		rsp, err = cli.HasCollection(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, rsp.Value, false)

		// test time stamp go back
		req = &milvuspb.HasCollectionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_HasCollection,
				MsgID:     111,
				Timestamp: 111,
				SourceID:  111,
			},
			DbName:         "testDb",
			CollectionName: "testColl2",
		}
		rsp, err = cli.HasCollection(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, rsp.Value, false)
	})

	t.Run("describe collection", func(t *testing.T) {
		collMeta, err := core.MetaTable.GetCollectionByName("testColl")
		assert.Nil(t, err)
		req := &milvuspb.DescribeCollectionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_DescribeCollection,
				MsgID:     120,
				Timestamp: 120,
				SourceID:  120,
			},
			DbName:         "testDb",
			CollectionName: "testColl",
		}
		rsp, err := cli.DescribeCollection(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, rsp.Schema.Name, "testColl")
		assert.Equal(t, rsp.CollectionID, collMeta.ID)
	})

	t.Run("show collection", func(t *testing.T) {
		req := &milvuspb.ShowCollectionsRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_ShowCollections,
				MsgID:     130,
				Timestamp: 130,
				SourceID:  130,
			},
			DbName: "testDb",
		}
		rsp, err := cli.ShowCollections(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		assert.ElementsMatch(t, rsp.CollectionNames, []string{"testColl", "testColl-again"})
		assert.Equal(t, len(rsp.CollectionNames), 2)
	})

	t.Run("create partition", func(t *testing.T) {
		req := &milvuspb.CreatePartitionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_CreatePartition,
				MsgID:     140,
				Timestamp: 140,
				SourceID:  140,
			},
			DbName:         "testDb",
			CollectionName: "testColl",
			PartitionName:  "testPartition",
		}
		status, err := cli.CreatePartition(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, status.ErrorCode, commonpb.ErrorCode_Success)
		collMeta, err := core.MetaTable.GetCollectionByName("testColl")
		assert.Nil(t, err)
		assert.Equal(t, len(collMeta.PartitionIDs), 2)
		partMeta, err := core.MetaTable.GetPartitionByID(collMeta.PartitionIDs[1])
		assert.Nil(t, err)
		assert.Equal(t, partMeta.PartitionName, "testPartition")

		assert.Equal(t, 1, len(collectionMetaCache))
	})

	t.Run("has partition", func(t *testing.T) {
		req := &milvuspb.HasPartitionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_HasPartition,
				MsgID:     150,
				Timestamp: 150,
				SourceID:  150,
			},
			DbName:         "testDb",
			CollectionName: "testColl",
			PartitionName:  "testPartition",
		}
		rsp, err := cli.HasPartition(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, rsp.Value, true)
	})

	t.Run("show partition", func(t *testing.T) {
		coll, err := core.MetaTable.GetCollectionByName("testColl")
		assert.Nil(t, err)
		req := &milvuspb.ShowPartitionsRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_ShowPartitions,
				MsgID:     160,
				Timestamp: 160,
				SourceID:  160,
			},
			DbName:         "testDb",
			CollectionName: "testColl",
			CollectionID:   coll.ID,
		}
		rsp, err := cli.ShowPartitions(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, len(rsp.PartitionNames), 2)
		assert.Equal(t, len(rsp.PartitionIDs), 2)
	})

	t.Run("show segment", func(t *testing.T) {
		coll, err := core.MetaTable.GetCollectionByName("testColl")
		assert.Nil(t, err)
		partID := coll.PartitionIDs[1]
		part, err := core.MetaTable.GetPartitionByID(partID)
		assert.Nil(t, err)
		assert.Zero(t, len(part.SegmentIDs))
		seg := &datapb.SegmentInfo{
			ID:           1000,
			CollectionID: coll.ID,
			PartitionID:  part.PartitionID,
		}
		core.DataServiceSegmentChan <- seg
		time.Sleep(time.Millisecond * 100)
		part, err = core.MetaTable.GetPartitionByID(partID)
		assert.Nil(t, err)
		assert.Equal(t, len(part.SegmentIDs), 1)

		req := &milvuspb.ShowSegmentsRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_ShowSegments,
				MsgID:     170,
				Timestamp: 170,
				SourceID:  170,
			},
			CollectionID: coll.ID,
			PartitionID:  partID,
		}
		rsp, err := cli.ShowSegments(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, rsp.SegmentIDs[0], int64(1000))
		assert.Equal(t, len(rsp.SegmentIDs), 1)
	})

	t.Run("create index", func(t *testing.T) {
		req := &milvuspb.CreateIndexRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_CreateIndex,
				MsgID:     180,
				Timestamp: 180,
				SourceID:  180,
			},
			DbName:         "",
			CollectionName: "testColl",
			FieldName:      "vector",
			ExtraParams: []*commonpb.KeyValuePair{
				{
					Key:   "ik1",
					Value: "iv1",
				},
			},
		}
		collMeta, err := core.MetaTable.GetCollectionByName("testColl")
		assert.Nil(t, err)
		assert.Equal(t, len(collMeta.FieldIndexes), 0)
		rsp, err := cli.CreateIndex(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.ErrorCode, commonpb.ErrorCode_Success)
		collMeta, err = core.MetaTable.GetCollectionByName("testColl")
		assert.Nil(t, err)
		assert.Equal(t, len(collMeta.FieldIndexes), 1)

		binlogLock.Lock()
		defer binlogLock.Unlock()
		assert.Equal(t, 3, len(binlogPathArray))
		assert.ElementsMatch(t, binlogPathArray, []string{"file1", "file2", "file3"})

		req.FieldName = "no field"
		rsp, err = cli.CreateIndex(ctx, req)
		assert.Nil(t, err)
		assert.NotEqual(t, rsp.ErrorCode, commonpb.ErrorCode_Success)
	})

	t.Run("describe segment", func(t *testing.T) {
		coll, err := core.MetaTable.GetCollectionByName("testColl")
		assert.Nil(t, err)

		req := &milvuspb.DescribeSegmentRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_DescribeSegment,
				MsgID:     190,
				Timestamp: 190,
				SourceID:  190,
			},
			CollectionID: coll.ID,
			SegmentID:    1000,
		}
		rsp, err := cli.DescribeSegment(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		t.Logf("index id = %d", rsp.IndexID)
	})

	t.Run("describe index", func(t *testing.T) {
		req := &milvuspb.DescribeIndexRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_DescribeIndex,
				MsgID:     200,
				Timestamp: 200,
				SourceID:  200,
			},
			DbName:         "",
			CollectionName: "testColl",
			FieldName:      "vector",
			IndexName:      "",
		}
		rsp, err := cli.DescribeIndex(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, len(rsp.IndexDescriptions), 1)
		assert.Equal(t, rsp.IndexDescriptions[0].IndexName, cms.Params.DefaultIndexName)
	})

	t.Run("flush segment", func(t *testing.T) {
		coll, err := core.MetaTable.GetCollectionByName("testColl")
		assert.Nil(t, err)
		partID := coll.PartitionIDs[1]
		part, err := core.MetaTable.GetPartitionByID(partID)
		assert.Nil(t, err)
		assert.Equal(t, len(part.SegmentIDs), 1)
		seg := &datapb.SegmentInfo{
			ID:           1001,
			CollectionID: coll.ID,
			PartitionID:  part.PartitionID,
		}
		core.DataServiceSegmentChan <- seg
		time.Sleep(time.Millisecond * 100)
		part, err = core.MetaTable.GetPartitionByID(partID)
		assert.Nil(t, err)
		assert.Equal(t, len(part.SegmentIDs), 2)
		core.DataNodeSegmentFlushCompletedChan <- 1001
		time.Sleep(time.Millisecond * 100)

		req := &milvuspb.DescribeIndexRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_DescribeIndex,
				MsgID:     210,
				Timestamp: 210,
				SourceID:  210,
			},
			DbName:         "",
			CollectionName: "testColl",
			FieldName:      "vector",
			IndexName:      "",
		}
		rsp, err := cli.DescribeIndex(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.Status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, len(rsp.IndexDescriptions), 1)
		assert.Equal(t, rsp.IndexDescriptions[0].IndexName, cms.Params.DefaultIndexName)

	})

	t.Run("drop index", func(t *testing.T) {
		req := &milvuspb.DropIndexRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_DropIndex,
				MsgID:     215,
				Timestamp: 215,
				SourceID:  215,
			},
			DbName:         "",
			CollectionName: "testColl",
			FieldName:      "vector",
			IndexName:      cms.Params.DefaultIndexName,
		}
		idx, err := core.MetaTable.GetIndexByName("testColl", "vector", cms.Params.DefaultIndexName)
		assert.Nil(t, err)
		assert.Equal(t, len(idx), 1)
		rsp, err := cli.DropIndex(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, rsp.ErrorCode, commonpb.ErrorCode_Success)

		dropIDLock.Lock()
		assert.Equal(t, len(dropID), 1)
		assert.Equal(t, dropID[0], idx[0].IndexID)
		dropIDLock.Unlock()

	})

	t.Run("drop partition", func(t *testing.T) {
		req := &milvuspb.DropPartitionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_DropPartition,
				MsgID:     220,
				Timestamp: 220,
				SourceID:  220,
			},
			DbName:         "testDb",
			CollectionName: "testColl",
			PartitionName:  "testPartition",
		}
		status, err := cli.DropPartition(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, status.ErrorCode, commonpb.ErrorCode_Success)
		collMeta, err := core.MetaTable.GetCollectionByName("testColl")
		assert.Nil(t, err)
		assert.Equal(t, len(collMeta.PartitionIDs), 1)
		partMeta, err := core.MetaTable.GetPartitionByID(collMeta.PartitionIDs[0])
		assert.Nil(t, err)
		assert.Equal(t, partMeta.PartitionName, cms.Params.DefaultPartitionName)
		assert.Equal(t, 2, len(collectionMetaCache))
	})

	t.Run("drop collection", func(t *testing.T) {
		req := &milvuspb.DropCollectionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_DropCollection,
				MsgID:     230,
				Timestamp: 230,
				SourceID:  230,
			},
			DbName:         "testDb",
			CollectionName: "testColl",
		}

		status, err := cli.DropCollection(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, len(dropCollectionArray), 1)
		assert.Equal(t, dropCollectionArray[0].Base.MsgType, commonpb.MsgType_DropCollection)
		assert.Equal(t, status.ErrorCode, commonpb.ErrorCode_Success)
		assert.Equal(t, dropCollectionArray[0].CollectionName, "testColl")
		assert.Equal(t, len(collectionMetaCache), 3)
		assert.Equal(t, collectionMetaCache[0], "testColl")

		req = &milvuspb.DropCollectionRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_DropCollection,
				MsgID:     231,
				Timestamp: 231,
				SourceID:  231,
			},
			DbName:         "testDb",
			CollectionName: "testColl",
		}
		status, err = cli.DropCollection(ctx, req)
		assert.Nil(t, err)
		assert.Equal(t, len(dropCollectionArray), 1)
		assert.Equal(t, status.ErrorCode, commonpb.ErrorCode_UnexpectedError)
	})

	err = cli.Stop()
	assert.Nil(t, err)

	err = svr.Stop()
	assert.Nil(t, err)
}
