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

	grpcindexnode "github.com/milvus-io/milvus/internal/distributed/indexnode"
)

type IndexNode struct {
	svr *grpcindexnode.Server
}

func NewIndexNode(ctx context.Context) (*IndexNode, error) {
	var err error
	n := &IndexNode{}
	svr, err := grpcindexnode.NewServer(ctx)
	if err != nil {
		return nil, err
	}
	n.svr = svr
	return n, nil

}
func (n *IndexNode) Run() error {
	if err := n.svr.Run(); err != nil {
		return err
	}
	return nil
}
func (n *IndexNode) Stop() error {
	if err := n.svr.Stop(); err != nil {
		return err
	}
	return nil
}
