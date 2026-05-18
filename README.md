# Providers Library

The `providers` library is a unified, protocol-based abstraction layer for integrating AI capabilities (LLM, Embedding, STT, TTS) into services. It decouples the *implementation* (Protocol) from the *configuration* (Provider), allowing for flexible, runtime-configurable AI services.

This library is part of the `services/libraries` collection of reusable Go packages.

## 🌟 Key Features

*   **Unified Interfaces**: Standardized `LLMProvider`, `EmbeddingProvider`, `STTProvider`, and `TTSProvider` interfaces.
*   **Protocol-Based**: Implementations are defined by protocols (e.g., `openai_api`, `yandex_grpc`), not just provider names.
*   **Factory Pattern**: Easy instantiation via `factory` package using Presets or Database configuration.
*   **Streaming Support**: First-class support for streaming responses (LLM chunks, Audio streams).
*   **Zero Business Logic**: Pure integration layer; tenancy and routing are handled by consumer services.

## 🏗️ Architecture

### Core Concepts

1.  **Protocol**: The technical implementation of an API (e.g., "How to talk to OpenAI", "How to stream audio to Yandex").
    *   Located in `pkg/providers/protocols/`
2.  **Provider**: A configured instance of a Protocol (e.g., "OpenAI with Key X", "Yandex Voice with Folder Y").
3.  **Preset**: A predefined configuration template (e.g., "yandex-voice" uses `yandex_grpc` protocol).
    *   Located in `pkg/providers/factory/presets.go`

### Directory Structure

```
services/libraries/providers/
├── core/                    # Base interfaces and types
│   ├── provider.go         # Base Provider interface
│   ├── llm.go              # LLM capability interfaces
│   ├── stt.go              # STT capability interfaces
│   └── ...
├── protocols/               # Concrete implementations
│   ├── openai_api/         # HTTP-based (OpenAI, OpenRouter, Yandex LLM)
│   ├── yandex_grpc/        # gRPC-based (Yandex STT/TTS)
│   ├── deepgram_ws/        # WebSocket-based (Deepgram STT)
│   └── ...
├── factory/                 # Instantiation logic
│   ├── create.go           # Factory functions
│   └── presets.go          # Predefined configurations
├── go.mod                   # Module definition
└── README.md               # This file
```

## 🚀 Quick Start

### 1. Import the Library

```go
import (
    "github.com/madmike/go-ai-providers/core"
    "github.com/madmike/go-ai-providers/factory"
)
```

### 2. Create a Provider

**Option A: From a Preset** (Easiest)

```go
// Create an OpenAI provider
provider, err := factory.CreateFromPreset(
    "openai",           // Preset name
    "My OpenAI",        // Display name
    "sk-...",           // API Key
    nil,                // Extra options (optional)
)
```

**Option B: From DB Configuration** (Dynamic)

```go
// Configuration usually comes from a database
config := factory.DBProviderConfig{
    PresetName:  "yandex-voice",
    DisplayName: "Production Yandex",
    APIKey:      "AQVN...",
    Options: map[string]any{
        "folder_id": "b1g...",
    },
}

provider, err := factory.CreateFromDB(config)
```

### 3. Use Capabilities

Type-assert the provider to the capability interface you need.

**LLM (Chat)**
```go
if llm, ok := provider.(core.LLMProvider); ok {
    resp, err := llm.ChatCompletion(ctx, core.ChatRequest{
        Model: "gpt-4",
        Messages: []core.Message{
            {Role: "user", Content: "Hello!"},
        },
    })
    fmt.Println(resp.Content)
}
```

**STT (Streaming)**
```go
if stt, ok := provider.(core.STTProvider); ok {
    stream, err := stt.StreamTranscribe(ctx, core.STTRequest{
        Model: "general",
        SampleRate: 16000,
    })
    
    // Send audio
    go stream.Send(ctx, audioBytes)
    
    // Receive text
    for {
        chunk, _ := stream.Receive(ctx)
        if chunk.Done { break }
        fmt.Print(chunk.Text)
    }
}
```

## 🔌 Supported Protocols & Presets

| Preset Name | Protocol | Capabilities | Description |
| :--- | :--- | :--- | :--- |
| `openai` | `openai_api` | LLM, Embedding, STT | Official OpenAI API |
| `openrouter` | `openai_api` | LLM | OpenRouter Aggregator |
| `yandex-llm` | `openai_api` | LLM, Embedding | Yandex Foundation Models |
| `yandex-voice` | `yandex_grpc` | STT, TTS | Yandex SpeechKit (High Perf) |
| `deepgram` | `deepgram_ws` | STT | Deepgram Nova-2 |
| `cartesia` | `cartesia_ws` | TTS | Cartesia Sonic |
| `minimax` | `minimax_ws` | TTS | Minimax Speech-01 |
| `gemini` | `gemini_api` | LLM, Embedding | Google Gemini |

## 🛠️ Integration Guide

### For Services (`agent-runtime`, `ingestion-service`)

1.  **Add Dependency**: Add `github.com/madmike/go-ai-providers` to your service's `go.mod`
2.  **Remove Local Providers**: Delete legacy provider implementations in your service.
3.  **Update Config**: Ensure your database `providers` table matches the `DBProviderConfig` structure (needs `preset_name`, `api_key`, `options` JSON).
4.  **Switch to Factory**: Replace manual struct initialization with `factory.CreateFromDB`.
5.  **Use Core Interfaces**: Update your domain logic to depend on `core.LLMProvider`, `core.STTProvider`, etc.

### Adding a New Protocol

1.  Create a new directory in `protocols/<name>`.
2.  Implement `core.Provider` and capability interfaces (`core.LLMProvider`, etc.).
3.  Register the protocol in `factory/create.go`.
4.  Add a default preset in `factory/presets.go`.
5.  Submit a PR to the providers library.
