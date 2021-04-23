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
	"path"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/milvus-io/milvus/internal/kv"
	"github.com/milvus-io/milvus/internal/proto/datapb"
)

// ddl binlog meta key:
//   ${prefix}/${collectionID}/${idx}
// segment binlog meta key:
//   ${prefix}/${segmentID}/${fieldID}/${idx}
type binlogMeta struct {
	client      kv.TxnKV // etcd kv
	idAllocator allocatorInterface
}

func NewBinlogMeta(kv kv.TxnKV, idAllocator allocatorInterface) (*binlogMeta, error) {
	mt := &binlogMeta{
		client:      kv,
		idAllocator: idAllocator,
	}
	return mt, nil
}

// if alloc is true, the returned keys will have a generated-unique ID at the end.
// if alloc is false, the returned keys will only consist of provided ids.
func (bm *binlogMeta) genKey(alloc bool, ids ...UniqueID) (key string, err error) {
	if alloc {
		idx, err := bm.idAllocator.allocID()
		if err != nil {
			return "", err
		}
		ids = append(ids, idx)
	}

	idStr := make([]string, len(ids))
	for _, id := range ids {
		idStr = append(idStr, strconv.FormatInt(id, 10))
	}

	key = path.Join(idStr...)
	return
}

func (bm *binlogMeta) SaveSegmentBinlogMetaTxn(segmentID UniqueID, field2Path map[UniqueID]string) error {

	kvs := make(map[string]string, len(field2Path))
	for fieldID, p := range field2Path {
		key, err := bm.genKey(true, segmentID, fieldID)
		if err != nil {
			return err
		}

		v := proto.MarshalTextString(&datapb.SegmentFieldBinlogMeta{
			FieldID:    fieldID,
			BinlogPath: p,
		})
		kvs[path.Join(Params.SegFlushMetaSubPath, key)] = v
	}
	return bm.client.MultiSave(kvs)
}

func (bm *binlogMeta) getFieldBinlogMeta(segmentID UniqueID,
	fieldID UniqueID) (metas []*datapb.SegmentFieldBinlogMeta, err error) {

	prefix, err := bm.genKey(false, segmentID, fieldID)
	if err != nil {
		return nil, err
	}

	_, vs, err := bm.client.LoadWithPrefix(path.Join(Params.SegFlushMetaSubPath, prefix))
	if err != nil {
		return nil, err
	}

	for _, blob := range vs {
		m := &datapb.SegmentFieldBinlogMeta{}
		if err = proto.UnmarshalText(blob, m); err != nil {
			return nil, err
		}

		metas = append(metas, m)
	}

	return
}

func (bm *binlogMeta) getSegmentBinlogMeta(segmentID UniqueID) (metas []*datapb.SegmentFieldBinlogMeta, err error) {

	prefix, err := bm.genKey(false, segmentID)
	if err != nil {
		return nil, err
	}

	_, vs, err := bm.client.LoadWithPrefix(path.Join(Params.SegFlushMetaSubPath, prefix))
	if err != nil {
		return nil, err
	}

	for _, blob := range vs {
		m := &datapb.SegmentFieldBinlogMeta{}
		if err = proto.UnmarshalText(blob, m); err != nil {
			return nil, err
		}

		metas = append(metas, m)
	}
	return
}

// ddl binlog meta key:
//   ${prefix}/${collectionID}/${idx}
// --- DDL ---
func (bm *binlogMeta) SaveDDLBinlogMetaTxn(collID UniqueID, tsPath string, ddlPath string) error {

	k, err := bm.genKey(true, collID)
	if err != nil {
		return err
	}
	v := proto.MarshalTextString(&datapb.DDLBinlogMeta{
		DdlBinlogPath: ddlPath,
		TsBinlogPath:  tsPath,
	})

	return bm.client.Save(path.Join(Params.DDLFlushMetaSubPath, k), v)
}

func (bm *binlogMeta) getDDLBinlogMete(collID UniqueID) (metas []*datapb.DDLBinlogMeta, err error) {
	prefix, err := bm.genKey(false, collID)
	if err != nil {
		return nil, err
	}

	_, vs, err := bm.client.LoadWithPrefix(path.Join(Params.DDLFlushMetaSubPath, prefix))
	if err != nil {
		return nil, err
	}

	for _, blob := range vs {
		m := &datapb.DDLBinlogMeta{}
		if err = proto.UnmarshalText(blob, m); err != nil {
			return nil, err
		}

		metas = append(metas, m)
	}
	return
}
