// Licensed to the LF AI & Data foundation under one
// or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership. The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package components

import (
	"context"

	"github.com/milvus-io/milvus/internal/log"
	"github.com/milvus-io/milvus/internal/proto/internalpb"

	grpcquerycoord "github.com/milvus-io/milvus/internal/distributed/querycoord"
	"github.com/milvus-io/milvus/internal/msgstream"
)

// QueryCoord implements QueryCoord grpc server
type QueryCoord struct {
	ctx context.Context
	svr *grpcquerycoord.Server
}

// NewQueryCoord creates a new QueryCoord
func NewQueryCoord(ctx context.Context, factory msgstream.Factory) (*QueryCoord, error) {
	svr, err := grpcquerycoord.NewServer(ctx, factory)
	if err != nil {
		panic(err)
	}

	return &QueryCoord{
		ctx: ctx,
		svr: svr,
	}, nil
}

// Run starts service
func (qs *QueryCoord) Run() error {
	if err := qs.svr.Run(); err != nil {
		panic(err)
	}
	log.Debug("QueryCoord successfully started")
	return nil
}

// Stop terminates service
func (qs *QueryCoord) Stop() error {
	if err := qs.svr.Stop(); err != nil {
		return err
	}
	return nil
}

// GetComponentStates returns QueryCoord's states
func (qs *QueryCoord) GetComponentStates(ctx context.Context, request *internalpb.GetComponentStatesRequest) (*internalpb.ComponentStates, error) {
	return qs.svr.GetComponentStates(ctx, request)
}