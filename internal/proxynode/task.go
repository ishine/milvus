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

package proxynode

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/milvus-io/milvus/internal/util/funcutil"

	"go.uber.org/zap"

	"github.com/golang/protobuf/proto"
	"github.com/milvus-io/milvus/internal/allocator"
	"github.com/milvus-io/milvus/internal/log"
	"github.com/milvus-io/milvus/internal/msgstream"
	"github.com/milvus-io/milvus/internal/proto/commonpb"
	"github.com/milvus-io/milvus/internal/proto/datapb"
	"github.com/milvus-io/milvus/internal/proto/indexpb"
	"github.com/milvus-io/milvus/internal/proto/internalpb"
	"github.com/milvus-io/milvus/internal/proto/milvuspb"
	"github.com/milvus-io/milvus/internal/proto/querypb"
	"github.com/milvus-io/milvus/internal/proto/schemapb"
	"github.com/milvus-io/milvus/internal/types"
	"github.com/milvus-io/milvus/internal/util/typeutil"
)

const (
	InsertTaskName                  = "InsertTask"
	CreateCollectionTaskName        = "CreateCollectionTask"
	DropCollectionTaskName          = "DropCollectionTask"
	SearchTaskName                  = "SearchTask"
	HasCollectionTaskName           = "HasCollectionTask"
	DescribeCollectionTaskName      = "DescribeCollectionTask"
	GetCollectionStatisticsTaskName = "GetCollectionStatisticsTask"
	ShowCollectionTaskName          = "ShowCollectionTask"
	CreatePartitionTaskName         = "CreatePartitionTask"
	DropPartitionTaskName           = "DropPartitionTask"
	HasPartitionTaskName            = "HasPartitionTask"
	ShowPartitionTaskName           = "ShowPartitionTask"
	CreateIndexTaskName             = "CreateIndexTask"
	DescribeIndexTaskName           = "DescribeIndexTask"
	DropIndexTaskName               = "DropIndexTask"
	GetIndexStateTaskName           = "GetIndexStateTask"
	FlushTaskName                   = "FlushTask"
	LoadCollectionTaskName          = "LoadCollectionTask"
	ReleaseCollectionTaskName       = "ReleaseCollectionTask"
	LoadPartitionTaskName           = "LoadPartitionTask"
	ReleasePartitionTaskName        = "ReleasePartitionTask"
)

type task interface {
	TraceCtx() context.Context
	ID() UniqueID       // return ReqID
	SetID(uid UniqueID) // set ReqID
	Name() string
	Type() commonpb.MsgType
	BeginTs() Timestamp
	EndTs() Timestamp
	SetTs(ts Timestamp)
	OnEnqueue() error
	PreExecute(ctx context.Context) error
	Execute(ctx context.Context) error
	PostExecute(ctx context.Context) error
	WaitToFinish() error
	Notify(err error)
}

type BaseInsertTask = msgstream.InsertMsg

type InsertTask struct {
	BaseInsertTask
	Condition
	ctx            context.Context
	dataService    types.DataService
	result         *milvuspb.InsertResponse
	rowIDAllocator *allocator.IDAllocator
}

func (it *InsertTask) TraceCtx() context.Context {
	return it.ctx
}

func (it *InsertTask) ID() UniqueID {
	return it.Base.MsgID
}

func (it *InsertTask) SetID(uid UniqueID) {
	it.Base.MsgID = uid
}

func (it *InsertTask) Name() string {
	return InsertTaskName
}

func (it *InsertTask) Type() commonpb.MsgType {
	return it.Base.MsgType
}

func (it *InsertTask) BeginTs() Timestamp {
	return it.BeginTimestamp
}

func (it *InsertTask) SetTs(ts Timestamp) {
	rowNum := len(it.RowData)
	it.Timestamps = make([]uint64, rowNum)
	for index := range it.Timestamps {
		it.Timestamps[index] = ts
	}
	it.BeginTimestamp = ts
	it.EndTimestamp = ts
}

func (it *InsertTask) EndTs() Timestamp {
	return it.EndTimestamp
}

func (it *InsertTask) OnEnqueue() error {
	it.BaseInsertTask.InsertRequest.Base = &commonpb.MsgBase{}
	return nil
}

func (it *InsertTask) PreExecute(ctx context.Context) error {
	it.Base.MsgType = commonpb.MsgType_Insert
	it.Base.SourceID = Params.ProxyID

	collectionName := it.BaseInsertTask.CollectionName
	if err := ValidateCollectionName(collectionName); err != nil {
		return err
	}
	partitionTag := it.BaseInsertTask.PartitionName
	if err := ValidatePartitionTag(partitionTag, true); err != nil {
		return err
	}

	return nil
}

func (it *InsertTask) Execute(ctx context.Context) error {
	collectionName := it.BaseInsertTask.CollectionName
	collSchema, err := globalMetaCache.GetCollectionSchema(ctx, collectionName)
	if err != nil {
		return err
	}
	autoID := collSchema.AutoID
	collID, err := globalMetaCache.GetCollectionID(ctx, collectionName)
	if err != nil {
		return err
	}
	it.CollectionID = collID
	var partitionID UniqueID
	if len(it.PartitionName) > 0 {
		partitionID, err = globalMetaCache.GetPartitionID(ctx, collectionName, it.PartitionName)
		if err != nil {
			return err
		}
	} else {
		partitionID, err = globalMetaCache.GetPartitionID(ctx, collectionName, Params.DefaultPartitionName)
		if err != nil {
			return err
		}
	}
	it.PartitionID = partitionID
	var rowIDBegin UniqueID
	var rowIDEnd UniqueID
	rowNums := len(it.BaseInsertTask.RowData)
	rowIDBegin, rowIDEnd, _ = it.rowIDAllocator.Alloc(uint32(rowNums))

	it.BaseInsertTask.RowIDs = make([]UniqueID, rowNums)
	for i := rowIDBegin; i < rowIDEnd; i++ {
		offset := i - rowIDBegin
		it.BaseInsertTask.RowIDs[offset] = i
	}

	if autoID {
		if it.HashValues == nil || len(it.HashValues) == 0 {
			it.HashValues = make([]uint32, 0)
		}
		for _, rowID := range it.RowIDs {
			hashValue, _ := typeutil.Hash32Int64(rowID)
			it.HashValues = append(it.HashValues, hashValue)
		}
	}

	var tsMsg msgstream.TsMsg = &it.BaseInsertTask
	it.BaseMsg.Ctx = ctx
	msgPack := msgstream.MsgPack{
		BeginTs: it.BeginTs(),
		EndTs:   it.EndTs(),
		Msgs:    make([]msgstream.TsMsg, 1),
	}

	it.result = &milvuspb.InsertResponse{
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
		},
		RowIDBegin: rowIDBegin,
		RowIDEnd:   rowIDEnd,
	}

	msgPack.Msgs[0] = tsMsg

	stream, err := globalInsertChannelsMap.GetInsertMsgStream(collID)
	if err != nil {
		resp, _ := it.dataService.GetInsertChannels(ctx, &datapb.GetInsertChannelsRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_Insert, // todo
				MsgID:     it.Base.MsgID,           // todo
				Timestamp: 0,                       // todo
				SourceID:  Params.ProxyID,
			},
			DbID:         0, // todo
			CollectionID: collID,
		})
		if resp == nil {
			return errors.New("get insert channels resp is nil")
		}
		if resp.Status.ErrorCode != commonpb.ErrorCode_Success {
			return errors.New(resp.Status.Reason)
		}
		err = globalInsertChannelsMap.CreateInsertMsgStream(collID, resp.Values)
		if err != nil {
			return err
		}
	}
	stream, err = globalInsertChannelsMap.GetInsertMsgStream(collID)
	if err != nil {
		it.result.Status.ErrorCode = commonpb.ErrorCode_UnexpectedError
		it.result.Status.Reason = err.Error()
		return err
	}

	err = stream.Produce(&msgPack)
	if err != nil {
		it.result.Status.ErrorCode = commonpb.ErrorCode_UnexpectedError
		it.result.Status.Reason = err.Error()
		return err
	}

	return nil
}

func (it *InsertTask) PostExecute(ctx context.Context) error {
	return nil
}

type CreateCollectionTask struct {
	Condition
	*milvuspb.CreateCollectionRequest
	ctx               context.Context
	masterService     types.MasterService
	dataServiceClient types.DataService
	result            *commonpb.Status
	schema            *schemapb.CollectionSchema
}

func (cct *CreateCollectionTask) TraceCtx() context.Context {
	return cct.ctx
}

func (cct *CreateCollectionTask) ID() UniqueID {
	return cct.Base.MsgID
}

func (cct *CreateCollectionTask) SetID(uid UniqueID) {
	cct.Base.MsgID = uid
}

func (cct *CreateCollectionTask) Name() string {
	return CreateCollectionTaskName
}

func (cct *CreateCollectionTask) Type() commonpb.MsgType {
	return cct.Base.MsgType
}

func (cct *CreateCollectionTask) BeginTs() Timestamp {
	return cct.Base.Timestamp
}

func (cct *CreateCollectionTask) EndTs() Timestamp {
	return cct.Base.Timestamp
}

func (cct *CreateCollectionTask) SetTs(ts Timestamp) {
	cct.Base.Timestamp = ts
}

func (cct *CreateCollectionTask) OnEnqueue() error {
	cct.Base = &commonpb.MsgBase{}
	return nil
}

func (cct *CreateCollectionTask) PreExecute(ctx context.Context) error {
	cct.Base.MsgType = commonpb.MsgType_CreateCollection
	cct.Base.SourceID = Params.ProxyID

	cct.schema = &schemapb.CollectionSchema{}
	err := proto.Unmarshal(cct.Schema, cct.schema)
	if err != nil {
		return err
	}

	if int64(len(cct.schema.Fields)) > Params.MaxFieldNum {
		return fmt.Errorf("maximum field's number should be limited to %d", Params.MaxFieldNum)
	}

	// validate collection name
	if err := ValidateCollectionName(cct.schema.Name); err != nil {
		return err
	}

	if err := ValidateDuplicatedFieldName(cct.schema.Fields); err != nil {
		return err
	}

	if err := ValidatePrimaryKey(cct.schema); err != nil {
		return err
	}

	// validate field name
	for _, field := range cct.schema.Fields {
		if err := ValidateFieldName(field.Name); err != nil {
			return err
		}
		if field.DataType == schemapb.DataType_FloatVector || field.DataType == schemapb.DataType_BinaryVector {
			exist := false
			var dim int64 = 0
			for _, param := range field.TypeParams {
				if param.Key == "dim" {
					exist = true
					tmp, err := strconv.ParseInt(param.Value, 10, 64)
					if err != nil {
						return err
					}
					dim = tmp
					break
				}
			}
			if !exist {
				return errors.New("dimension is not defined in field type params")
			}
			if field.DataType == schemapb.DataType_FloatVector {
				if err := ValidateDimension(dim, false); err != nil {
					return err
				}
			} else {
				if err := ValidateDimension(dim, true); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (cct *CreateCollectionTask) Execute(ctx context.Context) error {
	var err error
	cct.result, err = cct.masterService.CreateCollection(ctx, cct.CreateCollectionRequest)
	if err != nil {
		return err
	}
	if cct.result.ErrorCode == commonpb.ErrorCode_Success {
		collID, err := globalMetaCache.GetCollectionID(ctx, cct.CollectionName)
		if err != nil {
			return err
		}
		resp, _ := cct.dataServiceClient.GetInsertChannels(ctx, &datapb.GetInsertChannelsRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_Insert, // todo
				MsgID:     cct.Base.MsgID,          // todo
				Timestamp: 0,                       // todo
				SourceID:  Params.ProxyID,
			},
			DbID:         0, // todo
			CollectionID: collID,
		})
		if resp == nil {
			return errors.New("get insert channels resp is nil")
		}
		if resp.Status.ErrorCode != commonpb.ErrorCode_Success {
			return errors.New(resp.Status.Reason)
		}
		err = globalInsertChannelsMap.CreateInsertMsgStream(collID, resp.Values)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cct *CreateCollectionTask) PostExecute(ctx context.Context) error {
	return nil
}

type DropCollectionTask struct {
	Condition
	*milvuspb.DropCollectionRequest
	ctx           context.Context
	masterService types.MasterService
	result        *commonpb.Status
}

func (dct *DropCollectionTask) TraceCtx() context.Context {
	return dct.ctx
}

func (dct *DropCollectionTask) ID() UniqueID {
	return dct.Base.MsgID
}

func (dct *DropCollectionTask) SetID(uid UniqueID) {
	dct.Base.MsgID = uid
}

func (dct *DropCollectionTask) Name() string {
	return DropCollectionTaskName
}

func (dct *DropCollectionTask) Type() commonpb.MsgType {
	return dct.Base.MsgType
}

func (dct *DropCollectionTask) BeginTs() Timestamp {
	return dct.Base.Timestamp
}

func (dct *DropCollectionTask) EndTs() Timestamp {
	return dct.Base.Timestamp
}

func (dct *DropCollectionTask) SetTs(ts Timestamp) {
	dct.Base.Timestamp = ts
}

func (dct *DropCollectionTask) OnEnqueue() error {
	dct.Base = &commonpb.MsgBase{}
	return nil
}

func (dct *DropCollectionTask) PreExecute(ctx context.Context) error {
	dct.Base.MsgType = commonpb.MsgType_DropCollection
	dct.Base.SourceID = Params.ProxyID

	if err := ValidateCollectionName(dct.CollectionName); err != nil {
		return err
	}
	return nil
}

func (dct *DropCollectionTask) Execute(ctx context.Context) error {
	collID, err := globalMetaCache.GetCollectionID(ctx, dct.CollectionName)
	if err != nil {
		return err
	}

	dct.result, err = dct.masterService.DropCollection(ctx, dct.DropCollectionRequest)
	if err != nil {
		return err
	}

	err = globalInsertChannelsMap.CloseInsertMsgStream(collID)
	if err != nil {
		return err
	}

	return nil
}

func (dct *DropCollectionTask) PostExecute(ctx context.Context) error {
	globalMetaCache.RemoveCollection(ctx, dct.CollectionName)
	return nil
}

type SearchTask struct {
	Condition
	*internalpb.SearchRequest
	ctx            context.Context
	queryMsgStream msgstream.MsgStream
	resultBuf      chan []*internalpb.SearchResults
	result         *milvuspb.SearchResults
	query          *milvuspb.SearchRequest
}

func (st *SearchTask) TraceCtx() context.Context {
	return st.ctx
}

func (st *SearchTask) ID() UniqueID {
	return st.Base.MsgID
}

func (st *SearchTask) SetID(uid UniqueID) {
	st.Base.MsgID = uid
}

func (st *SearchTask) Name() string {
	return SearchTaskName
}

func (st *SearchTask) Type() commonpb.MsgType {
	return st.Base.MsgType
}

func (st *SearchTask) BeginTs() Timestamp {
	return st.Base.Timestamp
}

func (st *SearchTask) EndTs() Timestamp {
	return st.Base.Timestamp
}

func (st *SearchTask) SetTs(ts Timestamp) {
	st.Base.Timestamp = ts
}

func (st *SearchTask) OnEnqueue() error {
	st.Base = &commonpb.MsgBase{}
	return nil
}

func (st *SearchTask) PreExecute(ctx context.Context) error {
	st.Base.MsgType = commonpb.MsgType_Search
	st.Base.SourceID = Params.ProxyID

	collectionName := st.query.CollectionName
	_, err := globalMetaCache.GetCollectionID(ctx, collectionName)
	if err != nil { // err is not nil if collection not exists
		return err
	}

	if err := ValidateCollectionName(st.query.CollectionName); err != nil {
		return err
	}

	for _, tag := range st.query.PartitionNames {
		if err := ValidatePartitionTag(tag, false); err != nil {
			return err
		}
	}
	st.Base.MsgType = commonpb.MsgType_Search
	queryBytes, err := proto.Marshal(st.query)
	if err != nil {
		return err
	}
	st.Query = &commonpb.Blob{
		Value: queryBytes,
	}

	st.ResultChannelID = Params.SearchResultChannelNames[0]
	st.DbID = 0 // todo
	collectionID, err := globalMetaCache.GetCollectionID(ctx, collectionName)
	if err != nil { // err is not nil if collection not exists
		return err
	}
	st.CollectionID = collectionID
	st.PartitionIDs = make([]UniqueID, 0)

	partitionsMap, err := globalMetaCache.GetPartitions(ctx, collectionName)
	if err != nil {
		return err
	}

	partitionsRecord := make(map[UniqueID]bool)
	for _, partitionName := range st.query.PartitionNames {
		pattern := fmt.Sprintf("^%s$", partitionName)
		re, err := regexp.Compile(pattern)
		if err != nil {
			return errors.New("invalid partition names")
		}
		found := false
		for name, pID := range partitionsMap {
			if re.MatchString(name) {
				if _, exist := partitionsRecord[pID]; !exist {
					st.PartitionIDs = append(st.PartitionIDs, pID)
					partitionsRecord[pID] = true
				}
				found = true
			}
		}
		if !found {
			errMsg := fmt.Sprintf("PartitonName: %s not found", partitionName)
			return errors.New(errMsg)
		}
	}

	st.Dsl = st.query.Dsl
	st.PlaceholderGroup = st.query.PlaceholderGroup

	return nil
}

func (st *SearchTask) Execute(ctx context.Context) error {
	var tsMsg msgstream.TsMsg = &msgstream.SearchMsg{
		SearchRequest: *st.SearchRequest,
		BaseMsg: msgstream.BaseMsg{
			Ctx:            ctx,
			HashValues:     []uint32{uint32(Params.ProxyID)},
			BeginTimestamp: st.Base.Timestamp,
			EndTimestamp:   st.Base.Timestamp,
		},
	}
	msgPack := msgstream.MsgPack{
		BeginTs: st.Base.Timestamp,
		EndTs:   st.Base.Timestamp,
		Msgs:    make([]msgstream.TsMsg, 1),
	}
	msgPack.Msgs[0] = tsMsg
	err := st.queryMsgStream.Produce(&msgPack)
	log.Debug("proxynode", zap.Int("length of searchMsg", len(msgPack.Msgs)))
	if err != nil {
		log.Debug("proxynode", zap.String("send search request failed", err.Error()))
	}
	return err
}

// TODO: add benchmark to compare with serial implementation
func decodeSearchResultsParallel(searchResults []*internalpb.SearchResults, maxParallel int) ([][]*milvuspb.Hits, error) {
	log.Debug("decodeSearchResultsParallel", zap.Any("NumOfGoRoutines", maxParallel))

	hits := make([][]*milvuspb.Hits, 0)
	// necessary to parallel this?
	for _, partialSearchResult := range searchResults {
		if partialSearchResult.Hits == nil || len(partialSearchResult.Hits) <= 0 {
			continue
		}

		nq := len(partialSearchResult.Hits)
		partialHits := make([]*milvuspb.Hits, nq)

		f := func(idx int) error {
			partialHit := &milvuspb.Hits{}

			err := proto.Unmarshal(partialSearchResult.Hits[idx], partialHit)
			if err != nil {
				return err
			}

			partialHits[idx] = partialHit

			return nil
		}

		err := funcutil.ProcessFuncParallel(nq, maxParallel, f, "decodePartialSearchResult")

		if err != nil {
			return nil, err
		}

		hits = append(hits, partialHits)
	}

	return hits, nil
}

func decodeSearchResultsSerial(searchResults []*internalpb.SearchResults) ([][]*milvuspb.Hits, error) {
	return decodeSearchResultsParallel(searchResults, 1)
}

// TODO: add benchmark to compare with serial implementation
func decodeSearchResultsParallelByNq(searchResults []*internalpb.SearchResults) ([][]*milvuspb.Hits, error) {
	if len(searchResults) <= 0 {
		return nil, errors.New("no need to decode empty search results")
	}
	nq := len(searchResults[0].Hits)
	return decodeSearchResultsParallel(searchResults, nq)
}

// TODO: add benchmark to compare with serial implementation
func decodeSearchResultsParallelByCPU(searchResults []*internalpb.SearchResults) ([][]*milvuspb.Hits, error) {
	return decodeSearchResultsParallel(searchResults, runtime.NumCPU())
}

func decodeSearchResults(searchResults []*internalpb.SearchResults) ([][]*milvuspb.Hits, error) {
	t := time.Now()
	defer func() {
		log.Debug("decodeSearchResults", zap.Any("time cost", time.Since(t)))
	}()
	return decodeSearchResultsParallelByCPU(searchResults)
}

func reduceSearchResultsParallel(hits [][]*milvuspb.Hits, nq, availableQueryNodeNum, topk int, metricType string, maxParallel int) *milvuspb.SearchResults {
	log.Debug("reduceSearchResultsParallel", zap.Any("NumOfGoRoutines", maxParallel))

	ret := &milvuspb.SearchResults{
		Status: &commonpb.Status{
			ErrorCode: 0,
		},
		Hits: make([][]byte, nq),
	}

	const minFloat32 = -1 * float32(math.MaxFloat32)

	f := func(idx int) error {
		locs := make([]int, availableQueryNodeNum)
		reducedHits := &milvuspb.Hits{
			IDs:     make([]int64, 0),
			RowData: make([][]byte, 0),
			Scores:  make([]float32, 0),
		}

		for j := 0; j < topk; j++ {
			valid := false
			choice, maxDistance := 0, minFloat32
			for q, loc := range locs { // query num, the number of ways to merge
				if loc >= len(hits[q][idx].IDs) {
					continue
				}
				distance := hits[q][idx].Scores[loc]
				if distance > maxDistance || (math.Abs(float64(distance-maxDistance)) < math.SmallestNonzeroFloat32 && choice != q) {
					choice = q
					maxDistance = distance
					valid = true
				}
			}
			if !valid {
				break
			}
			choiceOffset := locs[choice]
			// check if distance is valid, `invalid` here means very very big,
			// in this process, distance here is the smallest, so the rest of distance are all invalid
			if hits[choice][idx].Scores[choiceOffset] <= minFloat32 {
				break
			}
			reducedHits.IDs = append(reducedHits.IDs, hits[choice][idx].IDs[choiceOffset])
			if hits[choice][idx].RowData != nil && len(hits[choice][idx].RowData) > 0 {
				reducedHits.RowData = append(reducedHits.RowData, hits[choice][idx].RowData[choiceOffset])
			}
			reducedHits.Scores = append(reducedHits.Scores, hits[choice][idx].Scores[choiceOffset])
			locs[choice]++
		}

		if metricType != "IP" {
			for k := range reducedHits.Scores {
				reducedHits.Scores[k] *= -1
			}
		}

		reducedHitsBs, err := proto.Marshal(reducedHits)
		if err != nil {
			return err
		}

		ret.Hits[idx] = reducedHitsBs

		return nil
	}

	err := funcutil.ProcessFuncParallel(nq, maxParallel, f, "reduceSearchResults")
	if err != nil {
		return nil
	}

	return ret
}

func reduceSearchResultsSerial(hits [][]*milvuspb.Hits, nq, availableQueryNodeNum, topk int, metricType string) *milvuspb.SearchResults {
	return reduceSearchResultsParallel(hits, nq, availableQueryNodeNum, topk, metricType, 1)
}

// TODO: add benchmark to compare with serial implementation
func reduceSearchResultsParallelByNq(hits [][]*milvuspb.Hits, nq, availableQueryNodeNum, topk int, metricType string) *milvuspb.SearchResults {
	return reduceSearchResultsParallel(hits, nq, availableQueryNodeNum, topk, metricType, nq)
}

// TODO: add benchmark to compare with serial implementation
func reduceSearchResultsParallelByCPU(hits [][]*milvuspb.Hits, nq, availableQueryNodeNum, topk int, metricType string) *milvuspb.SearchResults {
	return reduceSearchResultsParallel(hits, nq, availableQueryNodeNum, topk, metricType, runtime.NumCPU())
}

func reduceSearchResults(hits [][]*milvuspb.Hits, nq, availableQueryNodeNum, topk int, metricType string) *milvuspb.SearchResults {
	t := time.Now()
	defer func() {
		log.Debug("reduceSearchResults", zap.Any("time cost", time.Since(t)))
	}()
	return reduceSearchResultsParallelByCPU(hits, nq, availableQueryNodeNum, topk, metricType)
}

func printSearchResult(partialSearchResult *internalpb.SearchResults) {
	for i := 0; i < len(partialSearchResult.Hits); i++ {
		testHits := milvuspb.Hits{}
		err := proto.Unmarshal(partialSearchResult.Hits[i], &testHits)
		if err != nil {
			panic(err)
		}
		fmt.Println(testHits.IDs)
		fmt.Println(testHits.Scores)
	}
}

func (st *SearchTask) PostExecute(ctx context.Context) error {
	t0 := time.Now()
	defer func() {
		log.Debug("WaitAndPostExecute", zap.Any("time cost", time.Since(t0)))
	}()
	for {
		select {
		case <-st.TraceCtx().Done():
			log.Debug("proxynode", zap.Int64("SearchTask: wait to finish failed, timeout!, taskID:", st.ID()))
			return fmt.Errorf("SearchTask:wait to finish failed, timeout: %d", st.ID())
		case searchResults := <-st.resultBuf:
			// fmt.Println("searchResults: ", searchResults)
			filterSearchResult := make([]*internalpb.SearchResults, 0)
			var filterReason string
			for _, partialSearchResult := range searchResults {
				if partialSearchResult.Status.ErrorCode == commonpb.ErrorCode_Success {
					filterSearchResult = append(filterSearchResult, partialSearchResult)
					// For debugging, please don't delete.
					// printSearchResult(partialSearchResult)
				} else {
					filterReason += partialSearchResult.Status.Reason + "\n"
				}
			}

			availableQueryNodeNum := len(filterSearchResult)
			if availableQueryNodeNum <= 0 {
				st.result = &milvuspb.SearchResults{
					Status: &commonpb.Status{
						ErrorCode: commonpb.ErrorCode_UnexpectedError,
						Reason:    filterReason,
					},
				}
				return errors.New(filterReason)
			}

			availableQueryNodeNum = 0
			for _, partialSearchResult := range filterSearchResult {
				if partialSearchResult.Hits == nil || len(partialSearchResult.Hits) <= 0 {
					filterReason += "nq is zero\n"
					continue
				}
				availableQueryNodeNum++
			}

			if availableQueryNodeNum <= 0 {
				st.result = &milvuspb.SearchResults{
					Status: &commonpb.Status{
						ErrorCode: commonpb.ErrorCode_Success,
						Reason:    filterReason,
					},
				}
				return nil
			}

			hits, err := decodeSearchResults(filterSearchResult)
			if err != nil {
				return err
			}

			nq := len(hits[0])
			if nq <= 0 {
				st.result = &milvuspb.SearchResults{
					Status: &commonpb.Status{
						ErrorCode: commonpb.ErrorCode_Success,
						Reason:    filterReason,
					},
				}
				return nil
			}

			topk := 0
			for _, hit := range hits {
				topk = getMax(topk, len(hit[0].IDs))
			}

			st.result = reduceSearchResults(hits, nq, availableQueryNodeNum, topk, searchResults[0].MetricType)

			return nil
		}
	}
}

type HasCollectionTask struct {
	Condition
	*milvuspb.HasCollectionRequest
	ctx           context.Context
	masterService types.MasterService
	result        *milvuspb.BoolResponse
}

func (hct *HasCollectionTask) TraceCtx() context.Context {
	return hct.ctx
}

func (hct *HasCollectionTask) ID() UniqueID {
	return hct.Base.MsgID
}

func (hct *HasCollectionTask) SetID(uid UniqueID) {
	hct.Base.MsgID = uid
}

func (hct *HasCollectionTask) Name() string {
	return HasCollectionTaskName
}

func (hct *HasCollectionTask) Type() commonpb.MsgType {
	return hct.Base.MsgType
}

func (hct *HasCollectionTask) BeginTs() Timestamp {
	return hct.Base.Timestamp
}

func (hct *HasCollectionTask) EndTs() Timestamp {
	return hct.Base.Timestamp
}

func (hct *HasCollectionTask) SetTs(ts Timestamp) {
	hct.Base.Timestamp = ts
}

func (hct *HasCollectionTask) OnEnqueue() error {
	hct.Base = &commonpb.MsgBase{}
	return nil
}

func (hct *HasCollectionTask) PreExecute(ctx context.Context) error {
	hct.Base.MsgType = commonpb.MsgType_HasCollection
	hct.Base.SourceID = Params.ProxyID

	if err := ValidateCollectionName(hct.CollectionName); err != nil {
		return err
	}
	return nil
}

func (hct *HasCollectionTask) Execute(ctx context.Context) error {
	var err error
	hct.result, err = hct.masterService.HasCollection(ctx, hct.HasCollectionRequest)
	if hct.result == nil {
		return errors.New("has collection resp is nil")
	}
	if hct.result.Status.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(hct.result.Status.Reason)
	}
	return err
}

func (hct *HasCollectionTask) PostExecute(ctx context.Context) error {
	return nil
}

type DescribeCollectionTask struct {
	Condition
	*milvuspb.DescribeCollectionRequest
	ctx           context.Context
	masterService types.MasterService
	result        *milvuspb.DescribeCollectionResponse
}

func (dct *DescribeCollectionTask) TraceCtx() context.Context {
	return dct.ctx
}

func (dct *DescribeCollectionTask) ID() UniqueID {
	return dct.Base.MsgID
}

func (dct *DescribeCollectionTask) SetID(uid UniqueID) {
	dct.Base.MsgID = uid
}

func (dct *DescribeCollectionTask) Name() string {
	return DescribeCollectionTaskName
}

func (dct *DescribeCollectionTask) Type() commonpb.MsgType {
	return dct.Base.MsgType
}

func (dct *DescribeCollectionTask) BeginTs() Timestamp {
	return dct.Base.Timestamp
}

func (dct *DescribeCollectionTask) EndTs() Timestamp {
	return dct.Base.Timestamp
}

func (dct *DescribeCollectionTask) SetTs(ts Timestamp) {
	dct.Base.Timestamp = ts
}

func (dct *DescribeCollectionTask) OnEnqueue() error {
	dct.Base = &commonpb.MsgBase{}
	return nil
}

func (dct *DescribeCollectionTask) PreExecute(ctx context.Context) error {
	dct.Base.MsgType = commonpb.MsgType_DescribeCollection
	dct.Base.SourceID = Params.ProxyID

	if err := ValidateCollectionName(dct.CollectionName); err != nil {
		return err
	}
	return nil
}

func (dct *DescribeCollectionTask) Execute(ctx context.Context) error {
	var err error
	dct.result, err = dct.masterService.DescribeCollection(ctx, dct.DescribeCollectionRequest)
	if dct.result == nil {
		return errors.New("has collection resp is nil")
	}
	if dct.result.Status.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(dct.result.Status.Reason)
	}
	return err
}

func (dct *DescribeCollectionTask) PostExecute(ctx context.Context) error {
	return nil
}

type GetCollectionsStatisticsTask struct {
	Condition
	*milvuspb.GetCollectionStatisticsRequest
	ctx         context.Context
	dataService types.DataService
	result      *milvuspb.GetCollectionStatisticsResponse
}

func (g *GetCollectionsStatisticsTask) TraceCtx() context.Context {
	return g.ctx
}

func (g *GetCollectionsStatisticsTask) ID() UniqueID {
	return g.Base.MsgID
}

func (g *GetCollectionsStatisticsTask) SetID(uid UniqueID) {
	g.Base.MsgID = uid
}

func (g *GetCollectionsStatisticsTask) Name() string {
	return GetCollectionStatisticsTaskName
}

func (g *GetCollectionsStatisticsTask) Type() commonpb.MsgType {
	return g.Base.MsgType
}

func (g *GetCollectionsStatisticsTask) BeginTs() Timestamp {
	return g.Base.Timestamp
}

func (g *GetCollectionsStatisticsTask) EndTs() Timestamp {
	return g.Base.Timestamp
}

func (g *GetCollectionsStatisticsTask) SetTs(ts Timestamp) {
	g.Base.Timestamp = ts
}

func (g *GetCollectionsStatisticsTask) OnEnqueue() error {
	g.Base = &commonpb.MsgBase{}
	return nil
}

func (g *GetCollectionsStatisticsTask) PreExecute(ctx context.Context) error {
	g.Base.MsgType = commonpb.MsgType_GetCollectionStatistics
	g.Base.SourceID = Params.ProxyID
	return nil
}

func (g *GetCollectionsStatisticsTask) Execute(ctx context.Context) error {
	collID, err := globalMetaCache.GetCollectionID(ctx, g.CollectionName)
	if err != nil {
		return err
	}
	req := &datapb.GetCollectionStatisticsRequest{
		Base: &commonpb.MsgBase{
			MsgType:   commonpb.MsgType_GetCollectionStatistics,
			MsgID:     g.Base.MsgID,
			Timestamp: g.Base.Timestamp,
			SourceID:  g.Base.SourceID,
		},
		CollectionID: collID,
	}

	result, _ := g.dataService.GetCollectionStatistics(ctx, req)
	if result == nil {
		return errors.New("get collection statistics resp is nil")
	}
	if result.Status.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(result.Status.Reason)
	}
	g.result = &milvuspb.GetCollectionStatisticsResponse{
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		},
		Stats: result.Stats,
	}
	return nil
}

func (g *GetCollectionsStatisticsTask) PostExecute(ctx context.Context) error {
	return nil
}

type ShowCollectionsTask struct {
	Condition
	*milvuspb.ShowCollectionsRequest
	ctx           context.Context
	masterService types.MasterService
	result        *milvuspb.ShowCollectionsResponse
}

func (sct *ShowCollectionsTask) TraceCtx() context.Context {
	return sct.ctx
}

func (sct *ShowCollectionsTask) ID() UniqueID {
	return sct.Base.MsgID
}

func (sct *ShowCollectionsTask) SetID(uid UniqueID) {
	sct.Base.MsgID = uid
}

func (sct *ShowCollectionsTask) Name() string {
	return ShowCollectionTaskName
}

func (sct *ShowCollectionsTask) Type() commonpb.MsgType {
	return sct.Base.MsgType
}

func (sct *ShowCollectionsTask) BeginTs() Timestamp {
	return sct.Base.Timestamp
}

func (sct *ShowCollectionsTask) EndTs() Timestamp {
	return sct.Base.Timestamp
}

func (sct *ShowCollectionsTask) SetTs(ts Timestamp) {
	sct.Base.Timestamp = ts
}

func (sct *ShowCollectionsTask) OnEnqueue() error {
	sct.Base = &commonpb.MsgBase{}
	return nil
}

func (sct *ShowCollectionsTask) PreExecute(ctx context.Context) error {
	sct.Base.MsgType = commonpb.MsgType_ShowCollections
	sct.Base.SourceID = Params.ProxyID

	return nil
}

func (sct *ShowCollectionsTask) Execute(ctx context.Context) error {
	var err error
	sct.result, err = sct.masterService.ShowCollections(ctx, sct.ShowCollectionsRequest)
	if sct.result == nil {
		return errors.New("get collection statistics resp is nil")
	}
	if sct.result.Status.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(sct.result.Status.Reason)
	}
	return err
}

func (sct *ShowCollectionsTask) PostExecute(ctx context.Context) error {
	return nil
}

type CreatePartitionTask struct {
	Condition
	*milvuspb.CreatePartitionRequest
	ctx           context.Context
	masterService types.MasterService
	result        *commonpb.Status
}

func (cpt *CreatePartitionTask) TraceCtx() context.Context {
	return cpt.ctx
}

func (cpt *CreatePartitionTask) ID() UniqueID {
	return cpt.Base.MsgID
}

func (cpt *CreatePartitionTask) SetID(uid UniqueID) {
	cpt.Base.MsgID = uid
}

func (cpt *CreatePartitionTask) Name() string {
	return CreatePartitionTaskName
}

func (cpt *CreatePartitionTask) Type() commonpb.MsgType {
	return cpt.Base.MsgType
}

func (cpt *CreatePartitionTask) BeginTs() Timestamp {
	return cpt.Base.Timestamp
}

func (cpt *CreatePartitionTask) EndTs() Timestamp {
	return cpt.Base.Timestamp
}

func (cpt *CreatePartitionTask) SetTs(ts Timestamp) {
	cpt.Base.Timestamp = ts
}

func (cpt *CreatePartitionTask) OnEnqueue() error {
	cpt.Base = &commonpb.MsgBase{}
	return nil
}

func (cpt *CreatePartitionTask) PreExecute(ctx context.Context) error {
	cpt.Base.MsgType = commonpb.MsgType_CreatePartition
	cpt.Base.SourceID = Params.ProxyID

	collName, partitionTag := cpt.CollectionName, cpt.PartitionName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	if err := ValidatePartitionTag(partitionTag, true); err != nil {
		return err
	}

	return nil
}

func (cpt *CreatePartitionTask) Execute(ctx context.Context) (err error) {
	cpt.result, err = cpt.masterService.CreatePartition(ctx, cpt.CreatePartitionRequest)
	if cpt.result == nil {
		return errors.New("get collection statistics resp is nil")
	}
	if cpt.result.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(cpt.result.Reason)
	}
	return err
}

func (cpt *CreatePartitionTask) PostExecute(ctx context.Context) error {
	return nil
}

type DropPartitionTask struct {
	Condition
	*milvuspb.DropPartitionRequest
	ctx           context.Context
	masterService types.MasterService
	result        *commonpb.Status
}

func (dpt *DropPartitionTask) TraceCtx() context.Context {
	return dpt.ctx
}

func (dpt *DropPartitionTask) ID() UniqueID {
	return dpt.Base.MsgID
}

func (dpt *DropPartitionTask) SetID(uid UniqueID) {
	dpt.Base.MsgID = uid
}

func (dpt *DropPartitionTask) Name() string {
	return DropPartitionTaskName
}

func (dpt *DropPartitionTask) Type() commonpb.MsgType {
	return dpt.Base.MsgType
}

func (dpt *DropPartitionTask) BeginTs() Timestamp {
	return dpt.Base.Timestamp
}

func (dpt *DropPartitionTask) EndTs() Timestamp {
	return dpt.Base.Timestamp
}

func (dpt *DropPartitionTask) SetTs(ts Timestamp) {
	dpt.Base.Timestamp = ts
}

func (dpt *DropPartitionTask) OnEnqueue() error {
	dpt.Base = &commonpb.MsgBase{}
	return nil
}

func (dpt *DropPartitionTask) PreExecute(ctx context.Context) error {
	dpt.Base.MsgType = commonpb.MsgType_DropPartition
	dpt.Base.SourceID = Params.ProxyID

	collName, partitionTag := dpt.CollectionName, dpt.PartitionName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	if err := ValidatePartitionTag(partitionTag, true); err != nil {
		return err
	}

	return nil
}

func (dpt *DropPartitionTask) Execute(ctx context.Context) (err error) {
	dpt.result, err = dpt.masterService.DropPartition(ctx, dpt.DropPartitionRequest)
	if dpt.result == nil {
		return errors.New("get collection statistics resp is nil")
	}
	if dpt.result.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(dpt.result.Reason)
	}
	return err
}

func (dpt *DropPartitionTask) PostExecute(ctx context.Context) error {
	return nil
}

type HasPartitionTask struct {
	Condition
	*milvuspb.HasPartitionRequest
	ctx           context.Context
	masterService types.MasterService
	result        *milvuspb.BoolResponse
}

func (hpt *HasPartitionTask) TraceCtx() context.Context {
	return hpt.ctx
}

func (hpt *HasPartitionTask) ID() UniqueID {
	return hpt.Base.MsgID
}

func (hpt *HasPartitionTask) SetID(uid UniqueID) {
	hpt.Base.MsgID = uid
}

func (hpt *HasPartitionTask) Name() string {
	return HasPartitionTaskName
}

func (hpt *HasPartitionTask) Type() commonpb.MsgType {
	return hpt.Base.MsgType
}

func (hpt *HasPartitionTask) BeginTs() Timestamp {
	return hpt.Base.Timestamp
}

func (hpt *HasPartitionTask) EndTs() Timestamp {
	return hpt.Base.Timestamp
}

func (hpt *HasPartitionTask) SetTs(ts Timestamp) {
	hpt.Base.Timestamp = ts
}

func (hpt *HasPartitionTask) OnEnqueue() error {
	hpt.Base = &commonpb.MsgBase{}
	return nil
}

func (hpt *HasPartitionTask) PreExecute(ctx context.Context) error {
	hpt.Base.MsgType = commonpb.MsgType_HasPartition
	hpt.Base.SourceID = Params.ProxyID

	collName, partitionTag := hpt.CollectionName, hpt.PartitionName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	if err := ValidatePartitionTag(partitionTag, true); err != nil {
		return err
	}
	return nil
}

func (hpt *HasPartitionTask) Execute(ctx context.Context) (err error) {
	hpt.result, err = hpt.masterService.HasPartition(ctx, hpt.HasPartitionRequest)
	if hpt.result == nil {
		return errors.New("get collection statistics resp is nil")
	}
	if hpt.result.Status.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(hpt.result.Status.Reason)
	}
	return err
}

func (hpt *HasPartitionTask) PostExecute(ctx context.Context) error {
	return nil
}

type ShowPartitionsTask struct {
	Condition
	*milvuspb.ShowPartitionsRequest
	ctx           context.Context
	masterService types.MasterService
	result        *milvuspb.ShowPartitionsResponse
}

func (spt *ShowPartitionsTask) TraceCtx() context.Context {
	return spt.ctx
}

func (spt *ShowPartitionsTask) ID() UniqueID {
	return spt.Base.MsgID
}

func (spt *ShowPartitionsTask) SetID(uid UniqueID) {
	spt.Base.MsgID = uid
}

func (spt *ShowPartitionsTask) Name() string {
	return ShowPartitionTaskName
}

func (spt *ShowPartitionsTask) Type() commonpb.MsgType {
	return spt.Base.MsgType
}

func (spt *ShowPartitionsTask) BeginTs() Timestamp {
	return spt.Base.Timestamp
}

func (spt *ShowPartitionsTask) EndTs() Timestamp {
	return spt.Base.Timestamp
}

func (spt *ShowPartitionsTask) SetTs(ts Timestamp) {
	spt.Base.Timestamp = ts
}

func (spt *ShowPartitionsTask) OnEnqueue() error {
	spt.Base = &commonpb.MsgBase{}
	return nil
}

func (spt *ShowPartitionsTask) PreExecute(ctx context.Context) error {
	spt.Base.MsgType = commonpb.MsgType_ShowPartitions
	spt.Base.SourceID = Params.ProxyID

	if err := ValidateCollectionName(spt.CollectionName); err != nil {
		return err
	}
	return nil
}

func (spt *ShowPartitionsTask) Execute(ctx context.Context) error {
	var err error
	spt.result, err = spt.masterService.ShowPartitions(ctx, spt.ShowPartitionsRequest)
	if spt.result == nil {
		return errors.New("get collection statistics resp is nil")
	}
	if spt.result.Status.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(spt.result.Status.Reason)
	}
	return err
}

func (spt *ShowPartitionsTask) PostExecute(ctx context.Context) error {
	return nil
}

type CreateIndexTask struct {
	Condition
	*milvuspb.CreateIndexRequest
	ctx           context.Context
	masterService types.MasterService
	result        *commonpb.Status
}

func (cit *CreateIndexTask) TraceCtx() context.Context {
	return cit.ctx
}

func (cit *CreateIndexTask) ID() UniqueID {
	return cit.Base.MsgID
}

func (cit *CreateIndexTask) SetID(uid UniqueID) {
	cit.Base.MsgID = uid
}

func (cit *CreateIndexTask) Name() string {
	return CreateIndexTaskName
}

func (cit *CreateIndexTask) Type() commonpb.MsgType {
	return cit.Base.MsgType
}

func (cit *CreateIndexTask) BeginTs() Timestamp {
	return cit.Base.Timestamp
}

func (cit *CreateIndexTask) EndTs() Timestamp {
	return cit.Base.Timestamp
}

func (cit *CreateIndexTask) SetTs(ts Timestamp) {
	cit.Base.Timestamp = ts
}

func (cit *CreateIndexTask) OnEnqueue() error {
	cit.Base = &commonpb.MsgBase{}
	return nil
}

func (cit *CreateIndexTask) PreExecute(ctx context.Context) error {
	cit.Base.MsgType = commonpb.MsgType_CreateIndex
	cit.Base.SourceID = Params.ProxyID

	collName, fieldName := cit.CollectionName, cit.FieldName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	if err := ValidateFieldName(fieldName); err != nil {
		return err
	}

	return nil
}

func (cit *CreateIndexTask) Execute(ctx context.Context) error {
	var err error
	cit.result, err = cit.masterService.CreateIndex(ctx, cit.CreateIndexRequest)
	if cit.result == nil {
		return errors.New("get collection statistics resp is nil")
	}
	if cit.result.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(cit.result.Reason)
	}
	return err
}

func (cit *CreateIndexTask) PostExecute(ctx context.Context) error {
	return nil
}

type DescribeIndexTask struct {
	Condition
	*milvuspb.DescribeIndexRequest
	ctx           context.Context
	masterService types.MasterService
	result        *milvuspb.DescribeIndexResponse
}

func (dit *DescribeIndexTask) TraceCtx() context.Context {
	return dit.ctx
}

func (dit *DescribeIndexTask) ID() UniqueID {
	return dit.Base.MsgID
}

func (dit *DescribeIndexTask) SetID(uid UniqueID) {
	dit.Base.MsgID = uid
}

func (dit *DescribeIndexTask) Name() string {
	return DescribeIndexTaskName
}

func (dit *DescribeIndexTask) Type() commonpb.MsgType {
	return dit.Base.MsgType
}

func (dit *DescribeIndexTask) BeginTs() Timestamp {
	return dit.Base.Timestamp
}

func (dit *DescribeIndexTask) EndTs() Timestamp {
	return dit.Base.Timestamp
}

func (dit *DescribeIndexTask) SetTs(ts Timestamp) {
	dit.Base.Timestamp = ts
}

func (dit *DescribeIndexTask) OnEnqueue() error {
	dit.Base = &commonpb.MsgBase{}
	return nil
}

func (dit *DescribeIndexTask) PreExecute(ctx context.Context) error {
	dit.Base.MsgType = commonpb.MsgType_DescribeIndex
	dit.Base.SourceID = Params.ProxyID

	collName, fieldName := dit.CollectionName, dit.FieldName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	if err := ValidateFieldName(fieldName); err != nil {
		return err
	}

	// only support default index name for now. @2021.02.18
	if dit.IndexName == "" {
		dit.IndexName = Params.DefaultIndexName
	}

	return nil
}

func (dit *DescribeIndexTask) Execute(ctx context.Context) error {
	var err error
	dit.result, err = dit.masterService.DescribeIndex(ctx, dit.DescribeIndexRequest)
	if dit.result == nil {
		return errors.New("get collection statistics resp is nil")
	}
	if dit.result.Status.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(dit.result.Status.Reason)
	}
	return err
}

func (dit *DescribeIndexTask) PostExecute(ctx context.Context) error {
	return nil
}

type DropIndexTask struct {
	Condition
	ctx context.Context
	*milvuspb.DropIndexRequest
	masterService types.MasterService
	result        *commonpb.Status
}

func (dit *DropIndexTask) TraceCtx() context.Context {
	return dit.ctx
}

func (dit *DropIndexTask) ID() UniqueID {
	return dit.Base.MsgID
}

func (dit *DropIndexTask) SetID(uid UniqueID) {
	dit.Base.MsgID = uid
}

func (dit *DropIndexTask) Name() string {
	return DropIndexTaskName
}

func (dit *DropIndexTask) Type() commonpb.MsgType {
	return dit.Base.MsgType
}

func (dit *DropIndexTask) BeginTs() Timestamp {
	return dit.Base.Timestamp
}

func (dit *DropIndexTask) EndTs() Timestamp {
	return dit.Base.Timestamp
}

func (dit *DropIndexTask) SetTs(ts Timestamp) {
	dit.Base.Timestamp = ts
}

func (dit *DropIndexTask) OnEnqueue() error {
	dit.Base = &commonpb.MsgBase{}
	return nil
}

func (dit *DropIndexTask) PreExecute(ctx context.Context) error {
	dit.Base.MsgType = commonpb.MsgType_DropIndex
	dit.Base.SourceID = Params.ProxyID

	collName, fieldName := dit.CollectionName, dit.FieldName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	if err := ValidateFieldName(fieldName); err != nil {
		return err
	}

	return nil
}

func (dit *DropIndexTask) Execute(ctx context.Context) error {
	var err error
	dit.result, err = dit.masterService.DropIndex(ctx, dit.DropIndexRequest)
	if dit.result == nil {
		return errors.New("drop index resp is nil")
	}
	if dit.result.ErrorCode != commonpb.ErrorCode_Success {
		return errors.New(dit.result.Reason)
	}
	return err
}

func (dit *DropIndexTask) PostExecute(ctx context.Context) error {
	return nil
}

type GetIndexStateTask struct {
	Condition
	*milvuspb.GetIndexStateRequest
	ctx           context.Context
	indexService  types.IndexService
	masterService types.MasterService
	result        *milvuspb.GetIndexStateResponse
}

func (gist *GetIndexStateTask) TraceCtx() context.Context {
	return gist.ctx
}

func (gist *GetIndexStateTask) ID() UniqueID {
	return gist.Base.MsgID
}

func (gist *GetIndexStateTask) SetID(uid UniqueID) {
	gist.Base.MsgID = uid
}

func (gist *GetIndexStateTask) Name() string {
	return GetIndexStateTaskName
}

func (gist *GetIndexStateTask) Type() commonpb.MsgType {
	return gist.Base.MsgType
}

func (gist *GetIndexStateTask) BeginTs() Timestamp {
	return gist.Base.Timestamp
}

func (gist *GetIndexStateTask) EndTs() Timestamp {
	return gist.Base.Timestamp
}

func (gist *GetIndexStateTask) SetTs(ts Timestamp) {
	gist.Base.Timestamp = ts
}

func (gist *GetIndexStateTask) OnEnqueue() error {
	gist.Base = &commonpb.MsgBase{}
	return nil
}

func (gist *GetIndexStateTask) PreExecute(ctx context.Context) error {
	gist.Base.MsgType = commonpb.MsgType_GetIndexState
	gist.Base.SourceID = Params.ProxyID

	collName, fieldName := gist.CollectionName, gist.FieldName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	if err := ValidateFieldName(fieldName); err != nil {
		return err
	}

	return nil
}

func (gist *GetIndexStateTask) Execute(ctx context.Context) error {
	collectionName := gist.CollectionName
	collectionID, err := globalMetaCache.GetCollectionID(ctx, collectionName)
	if err != nil { // err is not nil if collection not exists
		return err
	}

	showPartitionRequest := &milvuspb.ShowPartitionsRequest{
		Base: &commonpb.MsgBase{
			MsgType:   commonpb.MsgType_ShowPartitions,
			MsgID:     gist.Base.MsgID,
			Timestamp: gist.Base.Timestamp,
			SourceID:  Params.ProxyID,
		},
		DbName:         gist.DbName,
		CollectionName: collectionName,
		CollectionID:   collectionID,
	}
	partitions, err := gist.masterService.ShowPartitions(ctx, showPartitionRequest)
	if err != nil {
		return err
	}

	if gist.IndexName == "" {
		gist.IndexName = Params.DefaultIndexName
	}

	describeIndexReq := milvuspb.DescribeIndexRequest{
		Base: &commonpb.MsgBase{
			MsgType:   commonpb.MsgType_DescribeIndex,
			MsgID:     gist.Base.MsgID,
			Timestamp: gist.Base.Timestamp,
			SourceID:  Params.ProxyID,
		},
		DbName:         gist.DbName,
		CollectionName: gist.CollectionName,
		FieldName:      gist.FieldName,
		IndexName:      gist.IndexName,
	}

	indexDescriptionResp, err2 := gist.masterService.DescribeIndex(ctx, &describeIndexReq)
	if err2 != nil {
		return err2
	}

	matchIndexID := int64(-1)
	foundIndexID := false
	for _, desc := range indexDescriptionResp.IndexDescriptions {
		if desc.IndexName == gist.IndexName {
			matchIndexID = desc.IndexID
			foundIndexID = true
			break
		}
	}
	if !foundIndexID {
		return errors.New(fmt.Sprint("Can't found IndexID for indexName", gist.IndexName))
	}

	var allSegmentIDs []UniqueID
	for _, partitionID := range partitions.PartitionIDs {
		showSegmentsRequest := &milvuspb.ShowSegmentsRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_ShowSegments,
				MsgID:     gist.Base.MsgID,
				Timestamp: gist.Base.Timestamp,
				SourceID:  Params.ProxyID,
			},
			CollectionID: collectionID,
			PartitionID:  partitionID,
		}
		segments, err := gist.masterService.ShowSegments(ctx, showSegmentsRequest)
		if err != nil {
			return err
		}
		if segments.Status.ErrorCode != commonpb.ErrorCode_Success {
			return errors.New(segments.Status.Reason)
		}
		allSegmentIDs = append(allSegmentIDs, segments.SegmentIDs...)
	}

	getIndexStatesRequest := &indexpb.GetIndexStatesRequest{
		IndexBuildIDs: make([]UniqueID, 0),
	}
	enableIndexBitMap := make([]bool, 0)
	indexBuildIDs := make([]UniqueID, 0)

	for _, segmentID := range allSegmentIDs {
		describeSegmentRequest := &milvuspb.DescribeSegmentRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_DescribeSegment,
				MsgID:     gist.Base.MsgID,
				Timestamp: gist.Base.Timestamp,
				SourceID:  Params.ProxyID,
			},
			CollectionID: collectionID,
			SegmentID:    segmentID,
		}
		segmentDesc, err := gist.masterService.DescribeSegment(ctx, describeSegmentRequest)
		if err != nil {
			return err
		}
		if segmentDesc.IndexID == matchIndexID {
			indexBuildIDs = append(indexBuildIDs, segmentDesc.BuildID)
			if segmentDesc.EnableIndex {
				enableIndexBitMap = append(enableIndexBitMap, true)
			} else {
				enableIndexBitMap = append(enableIndexBitMap, false)
			}
		}
	}

	log.Debug("proxynode", zap.Int("GetIndexState:: len of allSegmentIDs", len(allSegmentIDs)))
	log.Debug("proxynode", zap.Int("GetIndexState:: len of IndexBuildIDs", len(indexBuildIDs)))
	if len(allSegmentIDs) != len(indexBuildIDs) {
		gist.result = &milvuspb.GetIndexStateResponse{
			Status: &commonpb.Status{
				ErrorCode: commonpb.ErrorCode_Success,
				Reason:    "",
			},
			State: commonpb.IndexState_InProgress,
		}
		return err
	}

	for idx, enableIndex := range enableIndexBitMap {
		if enableIndex {
			getIndexStatesRequest.IndexBuildIDs = append(getIndexStatesRequest.IndexBuildIDs, indexBuildIDs[idx])
		}
	}
	states, err := gist.indexService.GetIndexStates(ctx, getIndexStatesRequest)
	if err != nil {
		return err
	}

	if states.Status.ErrorCode != commonpb.ErrorCode_Success {
		gist.result = &milvuspb.GetIndexStateResponse{
			Status: states.Status,
			State:  commonpb.IndexState_Failed,
		}

		return nil
	}

	for _, state := range states.States {
		if state.State != commonpb.IndexState_Finished {
			gist.result = &milvuspb.GetIndexStateResponse{
				Status: states.Status,
				State:  state.State,
			}

			return nil
		}
	}

	gist.result = &milvuspb.GetIndexStateResponse{
		Status: &commonpb.Status{
			ErrorCode: commonpb.ErrorCode_Success,
			Reason:    "",
		},
		State: commonpb.IndexState_Finished,
	}

	return nil
}

func (gist *GetIndexStateTask) PostExecute(ctx context.Context) error {
	return nil
}

type FlushTask struct {
	Condition
	*milvuspb.FlushRequest
	ctx         context.Context
	dataService types.DataService
	result      *commonpb.Status
}

func (ft *FlushTask) TraceCtx() context.Context {
	return ft.ctx
}

func (ft *FlushTask) ID() UniqueID {
	return ft.Base.MsgID
}

func (ft *FlushTask) SetID(uid UniqueID) {
	ft.Base.MsgID = uid
}

func (ft *FlushTask) Name() string {
	return FlushTaskName
}

func (ft *FlushTask) Type() commonpb.MsgType {
	return ft.Base.MsgType
}

func (ft *FlushTask) BeginTs() Timestamp {
	return ft.Base.Timestamp
}

func (ft *FlushTask) EndTs() Timestamp {
	return ft.Base.Timestamp
}

func (ft *FlushTask) SetTs(ts Timestamp) {
	ft.Base.Timestamp = ts
}

func (ft *FlushTask) OnEnqueue() error {
	ft.Base = &commonpb.MsgBase{}
	return nil
}

func (ft *FlushTask) PreExecute(ctx context.Context) error {
	ft.Base.MsgType = commonpb.MsgType_Flush
	ft.Base.SourceID = Params.ProxyID
	return nil
}

func (ft *FlushTask) Execute(ctx context.Context) error {
	for _, collName := range ft.CollectionNames {
		collID, err := globalMetaCache.GetCollectionID(ctx, collName)
		if err != nil {
			return err
		}
		flushReq := &datapb.FlushRequest{
			Base: &commonpb.MsgBase{
				MsgType:   commonpb.MsgType_Flush,
				MsgID:     ft.Base.MsgID,
				Timestamp: ft.Base.Timestamp,
				SourceID:  ft.Base.SourceID,
			},
			DbID:         0,
			CollectionID: collID,
		}
		var status *commonpb.Status
		status, _ = ft.dataService.Flush(ctx, flushReq)
		if status == nil {
			return errors.New("flush resp is nil")
		}
		if status.ErrorCode != commonpb.ErrorCode_Success {
			return errors.New(status.Reason)
		}
	}
	ft.result = &commonpb.Status{
		ErrorCode: commonpb.ErrorCode_Success,
	}
	return nil
}

func (ft *FlushTask) PostExecute(ctx context.Context) error {
	return nil
}

type LoadCollectionTask struct {
	Condition
	*milvuspb.LoadCollectionRequest
	ctx          context.Context
	queryService types.QueryService
	result       *commonpb.Status
}

func (lct *LoadCollectionTask) TraceCtx() context.Context {
	return lct.ctx
}

func (lct *LoadCollectionTask) ID() UniqueID {
	return lct.Base.MsgID
}

func (lct *LoadCollectionTask) SetID(uid UniqueID) {
	lct.Base.MsgID = uid
}

func (lct *LoadCollectionTask) Name() string {
	return LoadCollectionTaskName
}

func (lct *LoadCollectionTask) Type() commonpb.MsgType {
	return lct.Base.MsgType
}

func (lct *LoadCollectionTask) BeginTs() Timestamp {
	return lct.Base.Timestamp
}

func (lct *LoadCollectionTask) EndTs() Timestamp {
	return lct.Base.Timestamp
}

func (lct *LoadCollectionTask) SetTs(ts Timestamp) {
	lct.Base.Timestamp = ts
}

func (lct *LoadCollectionTask) OnEnqueue() error {
	lct.Base = &commonpb.MsgBase{}
	return nil
}

func (lct *LoadCollectionTask) PreExecute(ctx context.Context) error {
	log.Debug("LoadCollectionTask PreExecute", zap.String("role", Params.RoleName), zap.Int64("msgID", lct.Base.MsgID))
	lct.Base.MsgType = commonpb.MsgType_LoadCollection
	lct.Base.SourceID = Params.ProxyID

	collName := lct.CollectionName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	return nil
}

func (lct *LoadCollectionTask) Execute(ctx context.Context) (err error) {
	log.Debug("LoadCollectionTask Execute", zap.String("role", Params.RoleName), zap.Int64("msgID", lct.Base.MsgID))
	collID, err := globalMetaCache.GetCollectionID(ctx, lct.CollectionName)
	if err != nil {
		return err
	}
	collSchema, err := globalMetaCache.GetCollectionSchema(ctx, lct.CollectionName)
	if err != nil {
		return err
	}

	request := &querypb.LoadCollectionRequest{
		Base: &commonpb.MsgBase{
			MsgType:   commonpb.MsgType_LoadCollection,
			MsgID:     lct.Base.MsgID,
			Timestamp: lct.Base.Timestamp,
			SourceID:  lct.Base.SourceID,
		},
		DbID:         0,
		CollectionID: collID,
		Schema:       collSchema,
	}
	log.Debug("send LoadCollectionRequest to query service", zap.String("role", Params.RoleName), zap.Int64("msgID", request.Base.MsgID), zap.Int64("collectionID", request.CollectionID),
		zap.Any("schema", request.Schema))
	lct.result, err = lct.queryService.LoadCollection(ctx, request)
	if err != nil {
		return fmt.Errorf("call query service LoadCollection: %s", err)
	}
	return nil
}

func (lct *LoadCollectionTask) PostExecute(ctx context.Context) error {
	log.Debug("LoadCollectionTask PostExecute", zap.String("role", Params.RoleName), zap.Int64("msgID", lct.Base.MsgID))
	return nil
}

type ReleaseCollectionTask struct {
	Condition
	*milvuspb.ReleaseCollectionRequest
	ctx          context.Context
	queryService types.QueryService
	result       *commonpb.Status
}

func (rct *ReleaseCollectionTask) TraceCtx() context.Context {
	return rct.ctx
}

func (rct *ReleaseCollectionTask) ID() UniqueID {
	return rct.Base.MsgID
}

func (rct *ReleaseCollectionTask) SetID(uid UniqueID) {
	rct.Base.MsgID = uid
}

func (rct *ReleaseCollectionTask) Name() string {
	return ReleaseCollectionTaskName
}

func (rct *ReleaseCollectionTask) Type() commonpb.MsgType {
	return rct.Base.MsgType
}

func (rct *ReleaseCollectionTask) BeginTs() Timestamp {
	return rct.Base.Timestamp
}

func (rct *ReleaseCollectionTask) EndTs() Timestamp {
	return rct.Base.Timestamp
}

func (rct *ReleaseCollectionTask) SetTs(ts Timestamp) {
	rct.Base.Timestamp = ts
}

func (rct *ReleaseCollectionTask) OnEnqueue() error {
	rct.Base = &commonpb.MsgBase{}
	return nil
}

func (rct *ReleaseCollectionTask) PreExecute(ctx context.Context) error {
	rct.Base.MsgType = commonpb.MsgType_ReleaseCollection
	rct.Base.SourceID = Params.ProxyID

	collName := rct.CollectionName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	return nil
}

func (rct *ReleaseCollectionTask) Execute(ctx context.Context) (err error) {
	collID, err := globalMetaCache.GetCollectionID(ctx, rct.CollectionName)
	if err != nil {
		return err
	}
	request := &querypb.ReleaseCollectionRequest{
		Base: &commonpb.MsgBase{
			MsgType:   commonpb.MsgType_ReleaseCollection,
			MsgID:     rct.Base.MsgID,
			Timestamp: rct.Base.Timestamp,
			SourceID:  rct.Base.SourceID,
		},
		DbID:         0,
		CollectionID: collID,
	}
	rct.result, err = rct.queryService.ReleaseCollection(ctx, request)
	return err
}

func (rct *ReleaseCollectionTask) PostExecute(ctx context.Context) error {
	return nil
}

type LoadPartitionTask struct {
	Condition
	*milvuspb.LoadPartitionsRequest
	ctx          context.Context
	queryService types.QueryService
	result       *commonpb.Status
}

func (lpt *LoadPartitionTask) TraceCtx() context.Context {
	return lpt.ctx
}

func (lpt *LoadPartitionTask) ID() UniqueID {
	return lpt.Base.MsgID
}

func (lpt *LoadPartitionTask) SetID(uid UniqueID) {
	lpt.Base.MsgID = uid
}

func (lpt *LoadPartitionTask) Name() string {
	return LoadPartitionTaskName
}

func (lpt *LoadPartitionTask) Type() commonpb.MsgType {
	return lpt.Base.MsgType
}

func (lpt *LoadPartitionTask) BeginTs() Timestamp {
	return lpt.Base.Timestamp
}

func (lpt *LoadPartitionTask) EndTs() Timestamp {
	return lpt.Base.Timestamp
}

func (lpt *LoadPartitionTask) SetTs(ts Timestamp) {
	lpt.Base.Timestamp = ts
}

func (lpt *LoadPartitionTask) OnEnqueue() error {
	lpt.Base = &commonpb.MsgBase{}
	return nil
}

func (lpt *LoadPartitionTask) PreExecute(ctx context.Context) error {
	lpt.Base.MsgType = commonpb.MsgType_LoadPartitions
	lpt.Base.SourceID = Params.ProxyID

	collName := lpt.CollectionName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	return nil
}

func (lpt *LoadPartitionTask) Execute(ctx context.Context) error {
	var partitionIDs []int64
	collID, err := globalMetaCache.GetCollectionID(ctx, lpt.CollectionName)
	if err != nil {
		return err
	}
	collSchema, err := globalMetaCache.GetCollectionSchema(ctx, lpt.CollectionName)
	if err != nil {
		return err
	}
	for _, partitionName := range lpt.PartitionNames {
		partitionID, err := globalMetaCache.GetPartitionID(ctx, lpt.CollectionName, partitionName)
		if err != nil {
			return err
		}
		partitionIDs = append(partitionIDs, partitionID)
	}
	request := &querypb.LoadPartitionsRequest{
		Base: &commonpb.MsgBase{
			MsgType:   commonpb.MsgType_LoadPartitions,
			MsgID:     lpt.Base.MsgID,
			Timestamp: lpt.Base.Timestamp,
			SourceID:  lpt.Base.SourceID,
		},
		DbID:         0,
		CollectionID: collID,
		PartitionIDs: partitionIDs,
		Schema:       collSchema,
	}
	lpt.result, err = lpt.queryService.LoadPartitions(ctx, request)
	return err
}

func (lpt *LoadPartitionTask) PostExecute(ctx context.Context) error {
	return nil
}

type ReleasePartitionTask struct {
	Condition
	*milvuspb.ReleasePartitionsRequest
	ctx          context.Context
	queryService types.QueryService
	result       *commonpb.Status
}

func (rpt *ReleasePartitionTask) TraceCtx() context.Context {
	return rpt.ctx
}

func (rpt *ReleasePartitionTask) ID() UniqueID {
	return rpt.Base.MsgID
}

func (rpt *ReleasePartitionTask) SetID(uid UniqueID) {
	rpt.Base.MsgID = uid
}

func (rpt *ReleasePartitionTask) Type() commonpb.MsgType {
	return rpt.Base.MsgType
}

func (rpt *ReleasePartitionTask) Name() string {
	return ReleasePartitionTaskName
}

func (rpt *ReleasePartitionTask) BeginTs() Timestamp {
	return rpt.Base.Timestamp
}

func (rpt *ReleasePartitionTask) EndTs() Timestamp {
	return rpt.Base.Timestamp
}

func (rpt *ReleasePartitionTask) SetTs(ts Timestamp) {
	rpt.Base.Timestamp = ts
}

func (rpt *ReleasePartitionTask) OnEnqueue() error {
	rpt.Base = &commonpb.MsgBase{}
	return nil
}

func (rpt *ReleasePartitionTask) PreExecute(ctx context.Context) error {
	rpt.Base.MsgType = commonpb.MsgType_ReleasePartitions
	rpt.Base.SourceID = Params.ProxyID

	collName := rpt.CollectionName

	if err := ValidateCollectionName(collName); err != nil {
		return err
	}

	return nil
}

func (rpt *ReleasePartitionTask) Execute(ctx context.Context) (err error) {
	var partitionIDs []int64
	collID, err := globalMetaCache.GetCollectionID(ctx, rpt.CollectionName)
	if err != nil {
		return err
	}
	for _, partitionName := range rpt.PartitionNames {
		partitionID, err := globalMetaCache.GetPartitionID(ctx, rpt.CollectionName, partitionName)
		if err != nil {
			return err
		}
		partitionIDs = append(partitionIDs, partitionID)
	}
	request := &querypb.ReleasePartitionsRequest{
		Base: &commonpb.MsgBase{
			MsgType:   commonpb.MsgType_ReleasePartitions,
			MsgID:     rpt.Base.MsgID,
			Timestamp: rpt.Base.Timestamp,
			SourceID:  rpt.Base.SourceID,
		},
		DbID:         0,
		CollectionID: collID,
		PartitionIDs: partitionIDs,
	}
	rpt.result, err = rpt.queryService.ReleasePartitions(ctx, request)
	return err
}

func (rpt *ReleasePartitionTask) PostExecute(ctx context.Context) error {
	return nil
}
