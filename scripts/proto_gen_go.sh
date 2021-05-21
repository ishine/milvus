#!/usr/bin/env bash
SCRIPTS_DIR=$(dirname "$0")

PROTO_DIR=$SCRIPTS_DIR/../internal/proto/

PROGRAM=$(basename "$0")
GOPATH=$(go env GOPATH)

if [ -z $GOPATH ]; then
    printf "Error: the environment variable GOPATH is not set, please set it before running %s\n" $PROGRAM > /dev/stderr
    exit 1
fi

export PATH=${GOPATH}/bin:$PATH
echo `which protoc-gen-go`

# official go code ship with the crate, so we need to generate it manually.
pushd ${PROTO_DIR}

mkdir -p commonpb
mkdir -p schemapb
mkdir -p etcdpb
mkdir -p indexcgopb

mkdir -p internalpb
mkdir -p milvuspb
mkdir -p masterpb


mkdir -p milvuspb
mkdir -p proxypb

mkdir -p indexpb
mkdir -p datapb
mkdir -p querypb
mkdir -p planpb

${protoc} --go_out=plugins=grpc,paths=source_relative:./commonpb common.proto
${protoc} --go_out=plugins=grpc,paths=source_relative:./schemapb schema.proto
${protoc} --go_out=plugins=grpc,paths=source_relative:./etcdpb etcd_meta.proto
${protoc} --go_out=plugins=grpc,paths=source_relative:./indexcgopb index_cgo_msg.proto

#${protoc} --go_out=plugins=grpc,paths=source_relative:./internalpb internal_msg.proto

${protoc} --go_out=plugins=grpc,paths=source_relative:./masterpb master.proto

${protoc} --go_out=plugins=grpc,paths=source_relative:./internalpb internal.proto
${protoc} --go_out=plugins=grpc,paths=source_relative:./milvuspb milvus.proto
${protoc} --go_out=plugins=grpc,paths=source_relative:./proxypb proxy_service.proto
${protoc} --go_out=plugins=grpc,paths=source_relative:./indexpb index_service.proto
${protoc} --go_out=plugins=grpc,paths=source_relative:./datapb data_service.proto
${protoc} --go_out=plugins=grpc,paths=source_relative:./querypb query_service.proto
${protoc} --go_out=plugins=grpc,paths=source_relative:./planpb plan.proto

popd
