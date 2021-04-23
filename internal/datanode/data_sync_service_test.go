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

package datanode

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/milvus-io/milvus/internal/msgstream"
	"github.com/milvus-io/milvus/internal/proto/commonpb"
	"github.com/milvus-io/milvus/internal/proto/internalpb"
)

// NOTE: start pulsar before test
func TestDataSyncService_Start(t *testing.T) {
	const ctxTimeInMillisecond = 2000
	const closeWithDeadline = true
	var ctx context.Context

	if closeWithDeadline {
		var cancel context.CancelFunc
		d := time.Now().Add(ctxTimeInMillisecond * time.Millisecond)
		ctx, cancel = context.WithDeadline(context.Background(), d)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	// init data node
	pulsarURL := Params.PulsarAddress

	Factory := &MetaFactory{}
	collMeta := Factory.CollectionMetaFactory(UniqueID(0), "coll1")

	chanSize := 100
	flushChan := make(chan *flushMsg, chanSize)
	replica := newReplica()
	allocFactory := AllocatorFactory{}
	msFactory := msgstream.NewPmsFactory()
	m := map[string]interface{}{
		"pulsarAddress":  pulsarURL,
		"receiveBufSize": 1024,
		"pulsarBufSize":  1024}
	err := msFactory.SetParams(m)
	sync := newDataSyncService(ctx, flushChan, replica, &allocFactory, msFactory)
	sync.replica.addCollection(collMeta.ID, collMeta.Schema)
	sync.init()
	go sync.start()

	timeRange := TimeRange{
		timestampMin: 0,
		timestampMax: math.MaxUint64,
	}
	dataFactory := NewDataFactory()
	insertMessages := dataFactory.GetMsgStreamTsInsertMsgs(2)

	msgPack := msgstream.MsgPack{
		BeginTs: timeRange.timestampMin,
		EndTs:   timeRange.timestampMax,
		Msgs:    insertMessages,
	}

	// generate timeTick
	timeTickMsgPack := msgstream.MsgPack{}

	timeTickMsg := &msgstream.TimeTickMsg{
		BaseMsg: msgstream.BaseMsg{
			BeginTimestamp: Timestamp(0),
			EndTimestamp:   Timestamp(0),
			HashValues:     []uint32{0},
		},
		TimeTickMsg: internalpb.TimeTickMsg{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_TimeTick,
				MsgID:     UniqueID(0),
				Timestamp: math.MaxUint64,
				SourceID:  0,
			},
		},
	}
	timeTickMsgPack.Msgs = append(timeTickMsgPack.Msgs, timeTickMsg)

	// pulsar produce
	insertChannels := Params.InsertChannelNames
	ddChannels := Params.DDChannelNames

	assert.NoError(t, err)
	insertStream, _ := msFactory.NewMsgStream(ctx)
	insertStream.AsProducer(insertChannels)

	ddStream, _ := msFactory.NewMsgStream(ctx)
	ddStream.AsProducer(ddChannels)

	var insertMsgStream msgstream.MsgStream = insertStream
	insertMsgStream.Start()

	var ddMsgStream msgstream.MsgStream = ddStream
	ddMsgStream.Start()

	err = insertMsgStream.Produce(&msgPack)
	assert.NoError(t, err)

	err = insertMsgStream.Broadcast(&timeTickMsgPack)
	assert.NoError(t, err)
	err = ddMsgStream.Broadcast(&timeTickMsgPack)
	assert.NoError(t, err)

	// dataSync
	Params.FlushInsertBufferSize = 1
	<-sync.ctx.Done()

	sync.close()
}
