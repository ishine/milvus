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

package grpcdatanodeclient

import (
	"context"
	"time"

	"github.com/milvus-io/milvus/internal/log"
	"github.com/milvus-io/milvus/internal/util/retry"
	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"

	"github.com/milvus-io/milvus/internal/proto/commonpb"
	"github.com/milvus-io/milvus/internal/proto/datapb"
	"github.com/milvus-io/milvus/internal/proto/internalpb"
	"github.com/milvus-io/milvus/internal/proto/milvuspb"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Client struct {
	ctx     context.Context
	grpc    datapb.DataNodeClient
	conn    *grpc.ClientConn
	address string
}

func NewClient(address string) *Client {
	return &Client{
		address: address,
		ctx:     context.Background(),
	}
}

func (c *Client) Init() error {
	tracer := opentracing.GlobalTracer()
	connectGrpcFunc := func() error {
		log.Debug("DataNode connect ", zap.String("address", c.address))
		conn, err := grpc.DialContext(c.ctx, c.address, grpc.WithInsecure(), grpc.WithBlock(),
			grpc.WithUnaryInterceptor(
				otgrpc.OpenTracingClientInterceptor(tracer)),
			grpc.WithStreamInterceptor(
				otgrpc.OpenTracingStreamClientInterceptor(tracer)))
		if err != nil {
			return err
		}
		c.conn = conn
		return nil
	}

	err := retry.Retry(100000, time.Millisecond*200, connectGrpcFunc)
	if err != nil {
		return err
	}
	c.grpc = datapb.NewDataNodeClient(c.conn)
	return nil
}

func (c *Client) Start() error {
	return nil
}

func (c *Client) Stop() error {
	return c.conn.Close()
}

func (c *Client) GetComponentStates(ctx context.Context) (*internalpb.ComponentStates, error) {
	return c.grpc.GetComponentStates(ctx, &internalpb.GetComponentStatesRequest{})
}

func (c *Client) GetStatisticsChannel(ctx context.Context) (*milvuspb.StringResponse, error) {
	return c.grpc.GetStatisticsChannel(ctx, &internalpb.GetStatisticsChannelRequest{})
}

func (c *Client) WatchDmChannels(ctx context.Context, req *datapb.WatchDmChannelsRequest) (*commonpb.Status, error) {
	return c.grpc.WatchDmChannels(ctx, req)
}

func (c *Client) FlushSegments(ctx context.Context, req *datapb.FlushSegmentsRequest) (*commonpb.Status, error) {
	return c.grpc.FlushSegments(ctx, req)
}
