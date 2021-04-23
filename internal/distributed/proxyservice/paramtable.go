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

package grpcproxyservice

import (
	"sync"

	"github.com/milvus-io/milvus/internal/util/paramtable"
)

type ParamTable struct {
	paramtable.BaseTable

	ServiceAddress string
	ServicePort    int
}

var Params ParamTable
var once sync.Once

func (pt *ParamTable) Init() {
	once.Do(func() {
		pt.BaseTable.Init()
		pt.initParams()
	})
}

func (pt *ParamTable) initParams() {
	pt.initServicePort()
	pt.initServiceAddress()
}

func (pt *ParamTable) initServicePort() {
	pt.ServicePort = pt.ParseInt("proxyService.port")
}

func (pt *ParamTable) initServiceAddress() {
	ret, err := pt.Load("_PROXY_SERVICE_ADDRESS")
	if err != nil {
		panic(err)
	}
	pt.ServiceAddress = ret
}
