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

package components

import (
	"context"

	grpcquerynode "github.com/milvus-io/milvus/internal/distributed/querynode"
	"github.com/milvus-io/milvus/internal/msgstream"
)

type QueryNode struct {
	ctx context.Context
	svr *grpcquerynode.Server
}

func NewQueryNode(ctx context.Context, factory msgstream.Factory) (*QueryNode, error) {

	svr, err := grpcquerynode.NewServer(ctx, factory)
	if err != nil {
		return nil, err
	}

	return &QueryNode{
		ctx: ctx,
		svr: svr,
	}, nil

}

func (q *QueryNode) Run() error {
	if err := q.svr.Run(); err != nil {
		panic(err)
	}
	return nil
}

func (q *QueryNode) Stop() error {
	if err := q.svr.Stop(); err != nil {
		return err
	}
	return nil
}
