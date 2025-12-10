# Yandex Foundation Models Proto Definitions

This directory contains the Protocol Buffer definitions for Yandex Foundation Models APIs (LLM, Embeddings, STT, TTS).

## Structure

- `text_generation/` - LLM (Text Generation) proto definitions
  - `text_common.proto` - Common message types for text generation
  - `text_generation_service.proto` - Text generation service definitions
- `embedding/` - Embedding service proto definitions
  - `embedding_service.proto` - Embedding service definitions
- `stt/` - STT (Speech-to-Text) proto definitions
- `tts/` - TTS (Text-to-Speech) proto definitions
- `generated/` - Generated Go code from proto files

## Generating Go Code

To regenerate the Go code from all proto files:

```bash
./generate.sh
```

### Prerequisites

- `protoc` - Protocol Buffers compiler
- `protoc-gen-go` - Go plugin for protoc
- `protoc-gen-go-grpc` - gRPC plugin for protoc

Install the Go plugins:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

## Usage

Import the generated types in your Go code:

```go
// For LLM
import pb "github.com/creastat/providers/protocols/yandex_grpc/proto/generated/text_generation"

// For Embeddings
import pb "github.com/creastat/providers/protocols/yandex_grpc/proto/generated/embedding"

// For STT
import pb "github.com/creastat/providers/protocols/yandex_grpc/proto/generated/stt"

// For TTS
import pb "github.com/creastat/providers/protocols/yandex_grpc/proto/generated/tts"
```

## Changes from Official Yandex Protos

1. Updated `go_package` option to use local package path
2. Removed `google/api/annotations.proto` dependency (not needed for gRPC client)
3. Removed HTTP annotations from service definitions
4. Simplified proto definitions to focus on gRPC usage
