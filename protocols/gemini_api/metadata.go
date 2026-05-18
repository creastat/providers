package gemini_api

import "github.com/madmike/go-ai-providers/core"

// ProtocolMetadata contains metadata about the Gemini API protocol
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
		Name:        "gemini_api",
		Description: "Google Gemini API",
		Capabilities: []core.Capability{
			core.CapabilityLLM,
			core.CapabilityEmbedding,
		},
		Models: map[core.Capability][]ModelInfo{
			core.CapabilityLLM: {
				{
					ID:          "gemini-2.0-flash-exp",
					Name:        "Gemini 2.0 Flash Experimental",
					ContextSize: 1048576,
					Features:    []string{"chat", "streaming"},
				},
				{
					ID:          "gemini-1.5-flash",
					Name:        "Gemini 1.5 Flash",
					ContextSize: 1048576,
					Features:    []string{"chat", "streaming"},
				},
				{
					ID:          "gemini-1.5-pro",
					Name:        "Gemini 1.5 Pro",
					ContextSize: 2097152,
					Features:    []string{"chat", "streaming"},
				},
			},
			core.CapabilityEmbedding: {
				{
					ID:         "text-embedding-004",
					Name:       "Text Embedding 004",
					Dimensions: 768,
					Features:   []string{"embedding"},
				},
			},
		},
	}
}
