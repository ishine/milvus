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

	"github.com/milvus-io/milvus/internal/types"

	"github.com/milvus-io/milvus/internal/proto/commonpb"
	"github.com/milvus-io/milvus/internal/proto/masterpb"
)

type allocatorInterface interface {
	allocID() (UniqueID, error)
}
type allocator struct {
	masterService types.MasterService
}

func newAllocator(s types.MasterService) *allocator {
	return &allocator{
		masterService: s,
	}
}

func (alloc *allocator) allocID() (UniqueID, error) {
	ctx := context.TODO()
	resp, err := alloc.masterService.AllocID(ctx, &masterpb.AllocIDRequest{
		Base: &commonpb.MsgBase{
			MsgType:   commonpb.MsgType_RequestID,
			MsgID:     1, // GOOSE TODO
			Timestamp: 0, // GOOSE TODO
			SourceID:  Params.NodeID,
		},
		Count: 1,
	})
	if err != nil {
		return 0, err
	}
	return resp.ID, nil
}
