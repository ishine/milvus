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

	"github.com/golang/protobuf/proto"
	"github.com/milvus-io/milvus/internal/log"
	"github.com/milvus-io/milvus/internal/proto/commonpb"
	"github.com/milvus-io/milvus/internal/proto/etcdpb"
	"github.com/milvus-io/milvus/internal/proto/internalpb"
	"github.com/milvus-io/milvus/internal/proto/milvuspb"
	"github.com/milvus-io/milvus/internal/proto/schemapb"
	"github.com/milvus-io/milvus/internal/util/typeutil"
	"go.uber.org/zap"
)

type reqTask interface {
	Ctx() context.Context
	Type() commonpb.MsgType
	Ts() (typeutil.Timestamp, error)
	IgnoreTimeStamp() bool
	Execute(ctx context.Context) error
	WaitToFinish() error
	Notify(err error)
}

type baseReqTask struct {
	ctx  context.Context
	cv   chan error
	core *Core
}

func (bt *baseReqTask) Notify(err error) {
	bt.cv <- err
}

func (bt *baseReqTask) WaitToFinish() error {
	select {
	case <-bt.core.ctx.Done():
		return fmt.Errorf("core context done, %w", bt.core.ctx.Err())
	case <-bt.ctx.Done():
		return fmt.Errorf("request context done, %w", bt.ctx.Err())
	case err, ok := <-bt.cv:
		if !ok {
			return fmt.Errorf("notify chan closed")
		}
		return err
	}
}

type CreateCollectionReqTask struct {
	baseReqTask
	Req *milvuspb.CreateCollectionRequest
}

func (t *CreateCollectionReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *CreateCollectionReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *CreateCollectionReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *CreateCollectionReqTask) IgnoreTimeStamp() bool {
	return false
}

func (t *CreateCollectionReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_CreateCollection {
		return fmt.Errorf("create collection, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	var schema schemapb.CollectionSchema
	err := proto.Unmarshal(t.Req.Schema, &schema)
	if err != nil {
		return err
	}

	if t.Req.CollectionName != schema.Name {
		return fmt.Errorf("collection name = %s, schema.Name=%s", t.Req.CollectionName, schema.Name)
	}

	for idx, field := range schema.Fields {
		field.FieldID = int64(idx + StartOfUserFieldID)
	}
	rowIDField := &schemapb.FieldSchema{
		FieldID:      int64(RowIDField),
		Name:         RowIDFieldName,
		IsPrimaryKey: false,
		Description:  "row id",
		DataType:     schemapb.DataType_Int64,
	}
	timeStampField := &schemapb.FieldSchema{
		FieldID:      int64(TimeStampField),
		Name:         TimeStampFieldName,
		IsPrimaryKey: false,
		Description:  "time stamp",
		DataType:     schemapb.DataType_Int64,
	}
	schema.Fields = append(schema.Fields, rowIDField, timeStampField)

	collID, _, err := t.core.idAllocator(1)
	if err != nil {
		return err
	}
	collTs, err := t.Ts()
	if err != nil {
		return err
	}
	partitionID, _, err := t.core.idAllocator(1)
	if err != nil {
		return err
	}
	collMeta := etcdpb.CollectionInfo{
		ID:           collID,
		Schema:       &schema,
		CreateTime:   collTs,
		PartitionIDs: make([]typeutil.UniqueID, 0, 16),
		FieldIndexes: make([]*etcdpb.FieldIndexInfo, 0, 16),
	}
	partMeta := etcdpb.PartitionInfo{
		PartitionName: Params.DefaultPartitionName,
		PartitionID:   partitionID,
		SegmentIDs:    make([]typeutil.UniqueID, 0, 16),
	}
	idxInfo := make([]*etcdpb.IndexInfo, 0, 16)
	/////////////////////// ignore index param from create_collection /////////////////////////
	//for _, field := range schema.Fields {
	//	if field.DataType == schemapb.DataType_VectorFloat || field.DataType == schemapb.DataType_VectorBinary {
	//		if len(field.IndexParams) > 0 {
	//			idxID, err := t.core.idAllocator.AllocOne()
	//			if err != nil {
	//				return err
	//			}
	//			filedIdx := &etcdpb.FieldIndexInfo{
	//				FiledID: field.FieldID,
	//				IndexID: idxID,
	//			}
	//			idx := &etcdpb.IndexInfo{
	//				IndexName:   fmt.Sprintf("%s_index_%d", collMeta.Schema.Name, field.FieldID),
	//				IndexID:     idxID,
	//				IndexParams: field.IndexParams,
	//			}
	//			idxInfo = append(idxInfo, idx)
	//			collMeta.FieldIndexes = append(collMeta.FieldIndexes, filedIdx)
	//		}
	//	}
	//}

	err = t.core.MetaTable.AddCollection(&collMeta, &partMeta, idxInfo)
	if err != nil {
		return err
	}
	schemaBytes, err := proto.Marshal(&schema)
	if err != nil {
		return err
	}

	ddReq := internalpb.CreateCollectionRequest{
		Base:           t.Req.Base,
		DbName:         t.Req.DbName,
		CollectionName: t.Req.CollectionName,
		DbID:           0, //TODO,not used
		CollectionID:   collID,
		Schema:         schemaBytes,
	}

	err = t.core.DdCreateCollectionReq(ctx, &ddReq)
	if err != nil {
		return err
	}

	ddPart := internalpb.CreatePartitionRequest{
		Base: &commonpb.MsgBase{
			MsgType:   commonpb.MsgType_CreatePartition,
			MsgID:     t.Req.Base.MsgID, //TODO, msg id
			Timestamp: t.Req.Base.Timestamp + 1,
			SourceID:  t.Req.Base.SourceID,
		},
		DbName:         t.Req.DbName,
		CollectionName: t.Req.CollectionName,
		PartitionName:  Params.DefaultPartitionName,
		DbID:           0, //TODO, not used
		CollectionID:   collMeta.ID,
		PartitionID:    partMeta.PartitionID,
	}

	err = t.core.DdCreatePartitionReq(ctx, &ddPart)
	if err != nil {
		return err
	}

	return nil
}

type DropCollectionReqTask struct {
	baseReqTask
	Req *milvuspb.DropCollectionRequest
}

func (t *DropCollectionReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *DropCollectionReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *DropCollectionReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *DropCollectionReqTask) IgnoreTimeStamp() bool {
	return false
}

func (t *DropCollectionReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_DropCollection {
		return fmt.Errorf("drop collection, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}

	collMeta, err := t.core.MetaTable.GetCollectionByName(t.Req.CollectionName)
	if err != nil {
		return err
	}
	if err = t.core.InvalidateCollectionMetaCache(ctx, t.Req.Base.Timestamp, t.Req.DbName, t.Req.CollectionName); err != nil {
		return err
	}

	err = t.core.MetaTable.DeleteCollection(collMeta.ID)
	if err != nil {
		return err
	}

	//data service should drop segments , which belong to this collection, from the segment manager

	ddReq := internalpb.DropCollectionRequest{
		Base:           t.Req.Base,
		DbName:         t.Req.DbName,
		CollectionName: t.Req.CollectionName,
		DbID:           0, //not used
		CollectionID:   collMeta.ID,
	}

	err = t.core.DdDropCollectionReq(ctx, &ddReq)
	if err != nil {
		return err
	}

	//notify query service to release collection
	go func() {
		if err = t.core.ReleaseCollection(t.core.ctx, t.Req.Base.Timestamp, 0, collMeta.ID); err != nil {
			log.Warn("ReleaseCollection failed", zap.String("error", err.Error()))
		}
	}()

	return nil
}

type HasCollectionReqTask struct {
	baseReqTask
	Req           *milvuspb.HasCollectionRequest
	HasCollection bool
}

func (t *HasCollectionReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *HasCollectionReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *HasCollectionReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *HasCollectionReqTask) IgnoreTimeStamp() bool {
	return true
}

func (t *HasCollectionReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_HasCollection {
		return fmt.Errorf("has collection, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	_, err := t.core.MetaTable.GetCollectionByName(t.Req.CollectionName)
	if err == nil {
		t.HasCollection = true
	} else {
		t.HasCollection = false
	}
	return nil
}

type DescribeCollectionReqTask struct {
	baseReqTask
	Req *milvuspb.DescribeCollectionRequest
	Rsp *milvuspb.DescribeCollectionResponse
}

func (t *DescribeCollectionReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *DescribeCollectionReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *DescribeCollectionReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *DescribeCollectionReqTask) IgnoreTimeStamp() bool {
	return true
}

func (t *DescribeCollectionReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_DescribeCollection {
		return fmt.Errorf("describe collection, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	var coll *etcdpb.CollectionInfo
	var err error

	if t.Req.CollectionName != "" {
		coll, err = t.core.MetaTable.GetCollectionByName(t.Req.CollectionName)
		if err != nil {
			return err
		}
	} else {
		coll, err = t.core.MetaTable.GetCollectionByID(t.Req.CollectionID)
		if err != nil {
			return err
		}
	}

	t.Rsp.Schema = proto.Clone(coll.Schema).(*schemapb.CollectionSchema)
	t.Rsp.CollectionID = coll.ID
	var newField []*schemapb.FieldSchema
	for _, field := range t.Rsp.Schema.Fields {
		if field.FieldID >= StartOfUserFieldID {
			newField = append(newField, field)
		}
	}
	t.Rsp.Schema.Fields = newField
	return nil
}

type ShowCollectionReqTask struct {
	baseReqTask
	Req *milvuspb.ShowCollectionsRequest
	Rsp *milvuspb.ShowCollectionsResponse
}

func (t *ShowCollectionReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *ShowCollectionReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *ShowCollectionReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *ShowCollectionReqTask) IgnoreTimeStamp() bool {
	return true
}

func (t *ShowCollectionReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_ShowCollections {
		return fmt.Errorf("show collection, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	coll, err := t.core.MetaTable.ListCollections()
	if err != nil {
		return err
	}
	t.Rsp.CollectionNames = coll
	return nil
}

type CreatePartitionReqTask struct {
	baseReqTask
	Req *milvuspb.CreatePartitionRequest
}

func (t *CreatePartitionReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *CreatePartitionReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *CreatePartitionReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *CreatePartitionReqTask) IgnoreTimeStamp() bool {
	return false
}

func (t *CreatePartitionReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_CreatePartition {
		return fmt.Errorf("create partition, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	collMeta, err := t.core.MetaTable.GetCollectionByName(t.Req.CollectionName)
	if err != nil {
		return err
	}
	partitionID, _, err := t.core.idAllocator(1)
	if err != nil {
		return err
	}
	err = t.core.MetaTable.AddPartition(collMeta.ID, t.Req.PartitionName, partitionID)
	if err != nil {
		return err
	}

	ddReq := internalpb.CreatePartitionRequest{
		Base:           t.Req.Base,
		DbName:         t.Req.DbName,
		CollectionName: t.Req.CollectionName,
		PartitionName:  t.Req.PartitionName,
		DbID:           0, // todo, not used
		CollectionID:   collMeta.ID,
		PartitionID:    partitionID,
	}

	err = t.core.DdCreatePartitionReq(ctx, &ddReq)
	if err != nil {
		return err
	}

	// error doesn't matter here
	_ = t.core.InvalidateCollectionMetaCache(ctx, t.Req.Base.Timestamp, t.Req.DbName, t.Req.CollectionName)

	return nil
}

type DropPartitionReqTask struct {
	baseReqTask
	Req *milvuspb.DropPartitionRequest
}

func (t *DropPartitionReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *DropPartitionReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *DropPartitionReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *DropPartitionReqTask) IgnoreTimeStamp() bool {
	return false
}

func (t *DropPartitionReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_DropPartition {
		return fmt.Errorf("drop partition, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	coll, err := t.core.MetaTable.GetCollectionByName(t.Req.CollectionName)
	if err != nil {
		return err
	}
	partID, err := t.core.MetaTable.DeletePartition(coll.ID, t.Req.PartitionName)
	if err != nil {
		return err
	}

	ddReq := internalpb.DropPartitionRequest{
		Base:           t.Req.Base,
		DbName:         t.Req.DbName,
		CollectionName: t.Req.CollectionName,
		PartitionName:  t.Req.PartitionName,
		DbID:           0, //todo,not used
		CollectionID:   coll.ID,
		PartitionID:    partID,
	}

	err = t.core.DdDropPartitionReq(ctx, &ddReq)
	if err != nil {
		return err
	}

	// error doesn't matter here
	_ = t.core.InvalidateCollectionMetaCache(ctx, t.Req.Base.Timestamp, t.Req.DbName, t.Req.CollectionName)
	return nil
}

type HasPartitionReqTask struct {
	baseReqTask
	Req          *milvuspb.HasPartitionRequest
	HasPartition bool
}

func (t *HasPartitionReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *HasPartitionReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *HasPartitionReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *HasPartitionReqTask) IgnoreTimeStamp() bool {
	return true
}

func (t *HasPartitionReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_HasPartition {
		return fmt.Errorf("has partition, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	coll, err := t.core.MetaTable.GetCollectionByName(t.Req.CollectionName)
	if err != nil {
		return err
	}
	t.HasPartition = t.core.MetaTable.HasPartition(coll.ID, t.Req.PartitionName)
	return nil
}

type ShowPartitionReqTask struct {
	baseReqTask
	Req *milvuspb.ShowPartitionsRequest
	Rsp *milvuspb.ShowPartitionsResponse
}

func (t *ShowPartitionReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *ShowPartitionReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *ShowPartitionReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *ShowPartitionReqTask) IgnoreTimeStamp() bool {
	return true
}

func (t *ShowPartitionReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_ShowPartitions {
		return fmt.Errorf("show partition, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	var coll *etcdpb.CollectionInfo
	var err error
	if t.Req.CollectionName == "" {
		coll, err = t.core.MetaTable.GetCollectionByID(t.Req.CollectionID)
	} else {
		coll, err = t.core.MetaTable.GetCollectionByName(t.Req.CollectionName)
	}
	if err != nil {
		return err
	}
	for _, partID := range coll.PartitionIDs {
		partMeta, err := t.core.MetaTable.GetPartitionByID(partID)
		if err != nil {
			return err
		}
		t.Rsp.PartitionIDs = append(t.Rsp.PartitionIDs, partMeta.PartitionID)
		t.Rsp.PartitionNames = append(t.Rsp.PartitionNames, partMeta.PartitionName)
	}
	return nil
}

type DescribeSegmentReqTask struct {
	baseReqTask
	Req *milvuspb.DescribeSegmentRequest
	Rsp *milvuspb.DescribeSegmentResponse //TODO,return repeated segment id in the future
}

func (t *DescribeSegmentReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *DescribeSegmentReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *DescribeSegmentReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *DescribeSegmentReqTask) IgnoreTimeStamp() bool {
	return true
}

func (t *DescribeSegmentReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_DescribeSegment {
		return fmt.Errorf("describe segment, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	coll, err := t.core.MetaTable.GetCollectionByID(t.Req.CollectionID)
	if err != nil {
		return err
	}
	exist := false
	for _, partID := range coll.PartitionIDs {
		if exist {
			break
		}
		partMeta, err := t.core.MetaTable.GetPartitionByID(partID)
		if err != nil {
			return err
		}
		for _, e := range partMeta.SegmentIDs {
			if e == t.Req.SegmentID {
				exist = true
				break
			}
		}
	}
	if !exist {
		return fmt.Errorf("segment id %d not belong to collection id %d", t.Req.SegmentID, t.Req.CollectionID)
	}
	//TODO, get filed_id and index_name from request
	segIdxInfo, err := t.core.MetaTable.GetSegmentIndexInfoByID(t.Req.SegmentID, -1, "")
	if err != nil {
		return err
	}
	t.Rsp.IndexID = segIdxInfo.IndexID
	t.Rsp.BuildID = segIdxInfo.BuildID
	t.Rsp.EnableIndex = segIdxInfo.EnableIndex
	return nil
}

type ShowSegmentReqTask struct {
	baseReqTask
	Req *milvuspb.ShowSegmentsRequest
	Rsp *milvuspb.ShowSegmentsResponse
}

func (t *ShowSegmentReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *ShowSegmentReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *ShowSegmentReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *ShowSegmentReqTask) IgnoreTimeStamp() bool {
	return true
}

func (t *ShowSegmentReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_ShowSegments {
		return fmt.Errorf("show segments, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	coll, err := t.core.MetaTable.GetCollectionByID(t.Req.CollectionID)
	if err != nil {
		return err
	}
	exist := false
	for _, partID := range coll.PartitionIDs {
		if partID == t.Req.PartitionID {
			exist = true
			break
		}
	}
	if !exist {
		return fmt.Errorf("partition id = %d not belong to collection id = %d", t.Req.PartitionID, t.Req.CollectionID)
	}
	partMeta, err := t.core.MetaTable.GetPartitionByID(t.Req.PartitionID)
	if err != nil {
		return err
	}
	t.Rsp.SegmentIDs = append(t.Rsp.SegmentIDs, partMeta.SegmentIDs...)
	return nil
}

type CreateIndexReqTask struct {
	baseReqTask
	Req *milvuspb.CreateIndexRequest
}

func (t *CreateIndexReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *CreateIndexReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *CreateIndexReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *CreateIndexReqTask) IgnoreTimeStamp() bool {
	return false
}

func (t *CreateIndexReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_CreateIndex {
		return fmt.Errorf("create index, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	indexName := Params.DefaultIndexName //TODO, get name from request
	indexID, _, err := t.core.idAllocator(1)
	if err != nil {
		return err
	}
	idxInfo := &etcdpb.IndexInfo{
		IndexName:   indexName,
		IndexID:     indexID,
		IndexParams: t.Req.ExtraParams,
	}
	segIDs, field, err := t.core.MetaTable.GetNotIndexedSegments(t.Req.CollectionName, t.Req.FieldName, idxInfo)
	if err != nil {
		return err
	}
	if field.DataType != schemapb.DataType_FloatVector && field.DataType != schemapb.DataType_BinaryVector {
		return fmt.Errorf("field name = %s, data type = %s", t.Req.FieldName, schemapb.DataType_name[int32(field.DataType)])
	}
	for _, seg := range segIDs {
		task := CreateIndexTask{
			ctx:               t.core.ctx,
			core:              t.core,
			segmentID:         seg,
			indexName:         idxInfo.IndexName,
			indexID:           idxInfo.IndexID,
			fieldSchema:       &field,
			indexParams:       t.Req.ExtraParams,
			isFromFlushedChan: false,
		}
		t.core.indexTaskQueue <- &task
		fmt.Println("create index task enqueue, segID = ", seg)
	}
	return nil
}

type DescribeIndexReqTask struct {
	baseReqTask
	Req *milvuspb.DescribeIndexRequest
	Rsp *milvuspb.DescribeIndexResponse
}

func (t *DescribeIndexReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *DescribeIndexReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *DescribeIndexReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *DescribeIndexReqTask) IgnoreTimeStamp() bool {
	return true
}

func (t *DescribeIndexReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_DescribeIndex {
		return fmt.Errorf("describe index, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	idx, err := t.core.MetaTable.GetIndexByName(t.Req.CollectionName, t.Req.FieldName, t.Req.IndexName)
	if err != nil {
		return err
	}
	for _, i := range idx {
		desc := &milvuspb.IndexDescription{
			IndexName: i.IndexName,
			Params:    i.IndexParams,
			IndexID:   i.IndexID,
		}
		t.Rsp.IndexDescriptions = append(t.Rsp.IndexDescriptions, desc)
	}
	return nil
}

type DropIndexReqTask struct {
	baseReqTask
	Req *milvuspb.DropIndexRequest
}

func (t *DropIndexReqTask) Ctx() context.Context {
	return t.ctx
}

func (t *DropIndexReqTask) Type() commonpb.MsgType {
	return t.Req.Base.MsgType
}

func (t *DropIndexReqTask) Ts() (typeutil.Timestamp, error) {
	return t.Req.Base.Timestamp, nil
}

func (t *DropIndexReqTask) IgnoreTimeStamp() bool {
	return false
}

func (t *DropIndexReqTask) Execute(ctx context.Context) error {
	if t.Type() != commonpb.MsgType_DropIndex {
		return fmt.Errorf("drop index, msg type = %s", commonpb.MsgType_name[int32(t.Type())])
	}
	info, err := t.core.MetaTable.GetIndexByName(t.Req.CollectionName, t.Req.FieldName, t.Req.IndexName)
	if err != nil {
		log.Warn("GetIndexByName failed,", zap.String("collection name", t.Req.CollectionName), zap.String("field name", t.Req.FieldName), zap.String("index name", t.Req.IndexName), zap.Error(err))
		return err
	}
	if len(info) == 0 {
		return nil
	}
	if len(info) != 1 {
		return fmt.Errorf("len(index) = %d", len(info))
	}
	err = t.core.DropIndexReq(ctx, info[0].IndexID)
	if err != nil {
		return err
	}
	_, _, err = t.core.MetaTable.DropIndex(t.Req.CollectionName, t.Req.FieldName, t.Req.IndexName)
	return err
}

type CreateIndexTask struct {
	ctx               context.Context
	core              *Core
	segmentID         typeutil.UniqueID
	indexName         string
	indexID           typeutil.UniqueID
	fieldSchema       *schemapb.FieldSchema
	indexParams       []*commonpb.KeyValuePair
	isFromFlushedChan bool
}

func (t *CreateIndexTask) BuildIndex() error {
	if t.core.MetaTable.IsSegmentIndexed(t.segmentID, t.fieldSchema, t.indexParams) {
		return nil
	}
	rows, err := t.core.GetNumRowsReq(t.segmentID, t.isFromFlushedChan)
	if err != nil {
		return err
	}
	var bldID typeutil.UniqueID = 0
	enableIdx := false
	if rows < Params.MinSegmentSizeToEnableIndex {
		log.Debug("num of is less than MinSegmentSizeToEnableIndex", zap.Int64("num rows", rows))
	} else {
		binlogs, err := t.core.GetBinlogFilePathsFromDataServiceReq(t.segmentID, t.fieldSchema.FieldID)
		if err != nil {
			return err
		}
		bldID, err = t.core.BuildIndexReq(t.ctx, binlogs, t.fieldSchema.TypeParams, t.indexParams, t.indexID, t.indexName)
		if err != nil {
			return err
		}
		enableIdx = true
	}
	seg := etcdpb.SegmentIndexInfo{
		SegmentID:   t.segmentID,
		FieldID:     t.fieldSchema.FieldID,
		IndexID:     t.indexID,
		BuildID:     bldID,
		EnableIndex: enableIdx,
	}
	err = t.core.MetaTable.AddIndex(&seg)
	return err
}
