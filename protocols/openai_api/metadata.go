package openai_api

import "github.com/madmike/go-ai-providers/core"

// ProtocolMetadata contains metadata about the OpenAI API protocol
type ProtocolMetadata struct {
	Name         string
	Description  string
	Capabilities []core.Capability
	Models       map[core.Capability][]ModelInfo
}

// ModelInfo contains information about a model
type ModelInfo struct {
	ID          string
	Name        string
	ContextSize int
	Dimensions  int
	Features    []string
}

// Metadata returns the protocol metadata
func Metadata() ProtocolMetadata {
	return ProtocolMetadata{
		Name:        "openai_api",
		Description: "OpenAI-compatible HTTP API",
		Capabilities: []core.Capability{
			core.CapabilityLLM,
			core.CapabilityEmbedding,
			core.CapabilitySTT,
		},
		Models: map[core.Capability][]ModelInfo{
			core.CapabilityLLM: {
				{
					ID:          "gpt-4o",
					Name:        "GPT-4o",
					ContextSize: 128000,
					Features:    []string{"chat", "streaming", "function-calling"},
				},
				{
					ID:          "gpt-4o-mini",
					Name:        "GPT-4o Mini",
					ContextSize: 128000,
					Features:    []string{"chat", "streaming", "function-calling"},
				},
				{
					ID:          "gpt-3.5-turbo",
					Name:        "GPT-3.5 Turbo",
					ContextSize: 16385,
					Features:    []string{"chat", "streaming", "function-calling"},
				},
			},
			core.CapabilityEmbedding: {
				{
					ID:         "text-embedding-3-small",
					Name:       "Text Embedding 3 Small",
					Dimensions: 1536,
					Features:   []string{"embedding"},
				},
				{
					ID:         "text-embedding-3-large",
					Name:       "Text Embedding 3 Large",
					Dimensions: 3072,
					Features:   []string{"embedding"},
				},
			},
			core.CapabilitySTT: {
				{
					ID:       "whisper-1",
					Name:     "Whisper",
					Features: []string{"transcription", "translation"},
				},
			},
		},
	}
}
