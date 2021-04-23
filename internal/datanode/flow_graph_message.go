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
	"github.com/milvus-io/milvus/internal/msgstream"
	"github.com/milvus-io/milvus/internal/proto/internalpb"
	"github.com/milvus-io/milvus/internal/util/flowgraph"
)

type (
	Msg          = flowgraph.Msg
	MsgStreamMsg = flowgraph.MsgStreamMsg
)

type key2SegMsg struct {
	tsMessages []msgstream.TsMsg
	timeRange  TimeRange
}

type ddMsg struct {
	collectionRecords map[UniqueID][]*metaOperateRecord
	partitionRecords  map[UniqueID][]*metaOperateRecord
	flushMessages     []*flushMsg
	gcRecord          *gcRecord
	timeRange         TimeRange
}

type metaOperateRecord struct {
	createOrDrop bool // create: true, drop: false
	timestamp    Timestamp
}

type insertMsg struct {
	insertMessages []*msgstream.InsertMsg
	flushMessages  []*flushMsg
	gcRecord       *gcRecord
	timeRange      TimeRange
	startPositions []*internalpb.MsgPosition
	endPositions   []*internalpb.MsgPosition
}

type deleteMsg struct {
	deleteMessages []*msgstream.DeleteMsg
	timeRange      TimeRange
}

type gcMsg struct {
	gcRecord  *gcRecord
	timeRange TimeRange
}

type gcRecord struct {
	collections []UniqueID
}

type flushMsg struct {
	msgID        UniqueID
	timestamp    Timestamp
	segmentIDs   []UniqueID
	collectionID UniqueID
}

func (ksMsg *key2SegMsg) TimeTick() Timestamp {
	return ksMsg.timeRange.timestampMax
}

func (suMsg *ddMsg) TimeTick() Timestamp {
	return suMsg.timeRange.timestampMax
}

func (iMsg *insertMsg) TimeTick() Timestamp {
	return iMsg.timeRange.timestampMax
}

func (dMsg *deleteMsg) TimeTick() Timestamp {
	return dMsg.timeRange.timestampMax
}

func (gcMsg *gcMsg) TimeTick() Timestamp {
	return gcMsg.timeRange.timestampMax
}
