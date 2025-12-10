#!/bin/bash

# Generate Go code from all Yandex gRPC proto files

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/generated"

# Find protoc-gen-go and protoc-gen-go-grpc
PROTOC_GEN_GO="${HOME}/go/bin/protoc-gen-go"
PROTOC_GEN_GO_GRPC="${HOME}/go/bin/protoc-gen-go-grpc"

# Check if plugins exist
if [ ! -f "$PROTOC_GEN_GO" ]; then
  echo "Error: protoc-gen-go not found at $PROTOC_GEN_GO"
  echo "Install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
  exit 1
fi

if [ ! -f "$PROTOC_GEN_GO_GRPC" ]; then
  echo "Error: protoc-gen-go-grpc not found at $PROTOC_GEN_GO_GRPC"
  echo "Install with: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
  exit 1
fi

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Generate Go code for text_generation protos
echo "Generating Go code for text_generation protos..."
protoc \
  --plugin=protoc-gen-go="$PROTOC_GEN_GO" \
  --plugin=protoc-gen-go-grpc="$PROTOC_GEN_GO_GRPC" \
  --go_out="$OUTPUT_DIR" \
  --go-grpc_out="$OUTPUT_DIR" \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  -I"$SCRIPT_DIR" \
  "$SCRIPT_DIR"/text_generation/*.proto

echo "✓ Generated Go code for text_generation protos"

# Generate Go code for embedding protos
echo "Generating Go code for embedding protos..."
protoc \
  --plugin=protoc-gen-go="$PROTOC_GEN_GO" \
  --plugin=protoc-gen-go-grpc="$PROTOC_GEN_GO_GRPC" \
  --go_out="$OUTPUT_DIR" \
  --go-grpc_out="$OUTPUT_DIR" \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  -I"$SCRIPT_DIR" \
  "$SCRIPT_DIR"/embedding/*.proto

echo "✓ Generated Go code for embedding protos"

# Generate Go code for existing STT/TTS protos if they exist
if [ -d "$SCRIPT_DIR/stt" ]; then
  echo "Generating Go code for STT protos..."
  protoc \
    --plugin=protoc-gen-go="$PROTOC_GEN_GO" \
    --plugin=protoc-gen-go-grpc="$PROTOC_GEN_GO_GRPC" \
    --go_out="$OUTPUT_DIR" \
    --go-grpc_out="$OUTPUT_DIR" \
    --go_opt=paths=source_relative \
    --go-grpc_opt=paths=source_relative \
    -I"$SCRIPT_DIR" \
    "$SCRIPT_DIR"/stt/*.proto
  echo "✓ Generated Go code for STT protos"
fi

if [ -d "$SCRIPT_DIR/tts" ]; then
  echo "Generating Go code for TTS protos..."
  protoc \
    --plugin=protoc-gen-go="$PROTOC_GEN_GO" \
    --plugin=protoc-gen-go-grpc="$PROTOC_GEN_GO_GRPC" \
    --go_out="$OUTPUT_DIR" \
    --go-grpc_out="$OUTPUT_DIR" \
    --go_opt=paths=source_relative \
    --go-grpc_opt=paths=source_relative \
    -I"$SCRIPT_DIR" \
    "$SCRIPT_DIR"/tts/*.proto
  echo "✓ Generated Go code for TTS protos"
fi

echo ""
echo "✓ All proto files generated successfully!"
