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
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/milvus-io/milvus/internal/log"
	"github.com/milvus-io/milvus/internal/util/paramtable"
)

type ParamTable struct {
	// === PRIVATE Configs ===
	dataNodeIDList []UniqueID

	paramtable.BaseTable

	// === DataNode Internal Components Configs ===
	NodeID                  UniqueID
	IP                      string
	Port                    int
	FlowGraphMaxQueueLength int32
	FlowGraphMaxParallelism int32
	FlushInsertBufferSize   int32
	FlushDdBufferSize       int32
	InsertBinlogRootPath    string
	DdlBinlogRootPath       string
	Log                     log.Config

	// === DataNode External Components Configs ===
	// --- Pulsar ---
	PulsarAddress string

	// - insert channel -
	InsertChannelNames []string

	// - dd channel -
	DDChannelNames []string

	// - seg statistics channel -
	SegmentStatisticsChannelName string

	// - timetick channel -
	TimeTickChannelName string

	// - complete flush channel -
	CompleteFlushChannelName string

	// - channel subname -
	MsgChannelSubName string

	// --- ETCD ---
	EtcdAddress         string
	MetaRootPath        string
	SegFlushMetaSubPath string
	DDLFlushMetaSubPath string

	// --- MinIO ---
	MinioAddress         string
	MinioAccessKeyID     string
	MinioSecretAccessKey string
	MinioUseSSL          bool
	MinioBucketName      string
}

var Params ParamTable
var once sync.Once

func (p *ParamTable) Init() {
	once.Do(func() {
		p.BaseTable.Init()
		err := p.LoadYaml("advanced/data_node.yaml")
		if err != nil {
			panic(err)
		}

		// === DataNode Internal Components Configs ===
		p.initNodeID()
		p.initFlowGraphMaxQueueLength()
		p.initFlowGraphMaxParallelism()
		p.initFlushInsertBufferSize()
		p.initFlushDdBufferSize()
		p.initInsertBinlogRootPath()
		p.initDdlBinlogRootPath()
		p.initLogCfg()

		// === DataNode External Components Configs ===
		// --- Pulsar ---
		p.initPulsarAddress()

		// - insert channel -
		p.initInsertChannelNames()

		// - dd channel -
		p.initDDChannelNames()

		// - channel subname -
		p.initMsgChannelSubName()

		// --- ETCD ---
		p.initEtcdAddress()
		p.initMetaRootPath()
		p.initSegFlushMetaSubPath()
		p.initDDLFlushMetaSubPath()

		// --- MinIO ---
		p.initMinioAddress()
		p.initMinioAccessKeyID()
		p.initMinioSecretAccessKey()
		p.initMinioUseSSL()
		p.initMinioBucketName()
	})
}

// ==== DataNode internal components configs ====
func (p *ParamTable) initNodeID() {
	p.dataNodeIDList = p.DataNodeIDList()
	dataNodeIDStr := os.Getenv("DATA_NODE_ID")
	if dataNodeIDStr == "" {
		if len(p.dataNodeIDList) <= 0 {
			dataNodeIDStr = "0"
		} else {
			dataNodeIDStr = strconv.Itoa(int(p.dataNodeIDList[0]))
		}
	}
	err := p.Save("_dataNodeID", dataNodeIDStr)
	if err != nil {
		panic(err)
	}

	p.NodeID = p.ParseInt64("_dataNodeID")
}

// ---- flowgraph configs ----
func (p *ParamTable) initFlowGraphMaxQueueLength() {
	p.FlowGraphMaxQueueLength = p.ParseInt32("dataNode.dataSync.flowGraph.maxQueueLength")
}

func (p *ParamTable) initFlowGraphMaxParallelism() {
	p.FlowGraphMaxParallelism = p.ParseInt32("dataNode.dataSync.flowGraph.maxParallelism")
}

// ---- flush configs ----
func (p *ParamTable) initFlushInsertBufferSize() {
	p.FlushInsertBufferSize = p.ParseInt32("datanode.flush.insertBufSize")
}

func (p *ParamTable) initFlushDdBufferSize() {
	p.FlushDdBufferSize = p.ParseInt32("datanode.flush.ddBufSize")
}

func (p *ParamTable) initInsertBinlogRootPath() {
	// GOOSE TODO: rootPath change to  TenentID
	rootPath, err := p.Load("etcd.rootPath")
	if err != nil {
		panic(err)
	}
	p.InsertBinlogRootPath = path.Join(rootPath, "insert_log")
}

func (p *ParamTable) initDdlBinlogRootPath() {
	// GOOSE TODO: rootPath change to  TenentID
	rootPath, err := p.Load("etcd.rootPath")
	if err != nil {
		panic(err)
	}
	p.DdlBinlogRootPath = path.Join(rootPath, "data_definition_log")
}

// ---- Pulsar ----
func (p *ParamTable) initPulsarAddress() {
	url, err := p.Load("_PulsarAddress")
	if err != nil {
		panic(err)
	}
	p.PulsarAddress = url
}

// - insert channel -
func (p *ParamTable) initInsertChannelNames() {
	p.InsertChannelNames = make([]string, 0)
}

func (p *ParamTable) initDDChannelNames() {
	p.DDChannelNames = make([]string, 0)
}

// - msg channel subname -
func (p *ParamTable) initMsgChannelSubName() {
	name, err := p.Load("msgChannel.subNamePrefix.dataNodeSubNamePrefix")
	if err != nil {
		panic(err)
	}
	p.MsgChannelSubName = name + "-" + strconv.FormatInt(p.NodeID, 10)
}

// --- ETCD ---
func (p *ParamTable) initEtcdAddress() {
	addr, err := p.Load("_EtcdAddress")
	if err != nil {
		panic(err)
	}
	p.EtcdAddress = addr
}

func (p *ParamTable) initMetaRootPath() {
	rootPath, err := p.Load("etcd.rootPath")
	if err != nil {
		panic(err)
	}
	subPath, err := p.Load("etcd.metaSubPath")
	if err != nil {
		panic(err)
	}
	p.MetaRootPath = path.Join(rootPath, subPath)
}

func (p *ParamTable) initSegFlushMetaSubPath() {
	subPath, err := p.Load("etcd.segFlushMetaSubPath")
	if err != nil {
		panic(err)
	}
	p.SegFlushMetaSubPath = subPath
}

func (p *ParamTable) initDDLFlushMetaSubPath() {
	subPath, err := p.Load("etcd.ddlFlushMetaSubPath")
	if err != nil {
		panic(err)
	}
	p.DDLFlushMetaSubPath = subPath
}

func (p *ParamTable) initMinioAddress() {
	endpoint, err := p.Load("_MinioAddress")
	if err != nil {
		panic(err)
	}
	p.MinioAddress = endpoint
}

func (p *ParamTable) initMinioAccessKeyID() {
	keyID, err := p.Load("minio.accessKeyID")
	if err != nil {
		panic(err)
	}
	p.MinioAccessKeyID = keyID
}

func (p *ParamTable) initMinioSecretAccessKey() {
	key, err := p.Load("minio.secretAccessKey")
	if err != nil {
		panic(err)
	}
	p.MinioSecretAccessKey = key
}

func (p *ParamTable) initMinioUseSSL() {
	usessl, err := p.Load("minio.useSSL")
	if err != nil {
		panic(err)
	}
	p.MinioUseSSL, _ = strconv.ParseBool(usessl)
}

func (p *ParamTable) initMinioBucketName() {
	bucketName, err := p.Load("minio.bucketName")
	if err != nil {
		panic(err)
	}
	p.MinioBucketName = bucketName
}

func (p *ParamTable) sliceIndex() int {
	dataNodeID := p.NodeID
	dataNodeIDList := p.dataNodeIDList
	for i := 0; i < len(dataNodeIDList); i++ {
		if dataNodeID == dataNodeIDList[i] {
			return i
		}
	}
	return -1
}

func (p *ParamTable) initLogCfg() {
	p.Log = log.Config{}
	format, err := p.Load("log.format")
	if err != nil {
		panic(err)
	}
	p.Log.Format = format
	level, err := p.Load("log.level")
	if err != nil {
		panic(err)
	}
	p.Log.Level = level
	devStr, err := p.Load("log.dev")
	if err != nil {
		panic(err)
	}
	dev, err := strconv.ParseBool(devStr)
	if err != nil {
		panic(err)
	}
	p.Log.Development = dev
	p.Log.File.MaxSize = p.ParseInt("log.file.maxSize")
	p.Log.File.MaxBackups = p.ParseInt("log.file.maxBackups")
	p.Log.File.MaxDays = p.ParseInt("log.file.maxAge")
	rootPath, err := p.Load("log.file.rootPath")
	if err != nil {
		panic(err)
	}
	if len(rootPath) != 0 {
		p.Log.File.Filename = path.Join(rootPath, "datanode-"+strconv.FormatInt(p.NodeID, 10)+".log")
	} else {
		p.Log.File.Filename = ""
	}
}
