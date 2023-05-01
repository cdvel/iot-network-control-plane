#!/bin/bash
# scripts/proto-gen.sh

set -e

PROTO_DIR="api/proto"
OUT_DIR="."

echo "Generating protobuf code..."

for proto_file in ${PROTO_DIR}/*.proto; do
    echo "Processing ${proto_file}..."
    protoc \
        --go_out=${OUT_DIR} \
        --go_opt=paths=source_relative \
        --go-grpc_out=${OUT_DIR} \
        --go-grpc_opt=paths=source_relative \
        -I ${PROTO_DIR} \
        ${proto_file}
done

echo "Protobuf code generation complete!"