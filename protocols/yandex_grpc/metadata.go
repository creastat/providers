package yandex_grpc

import "github.com/madmike/go-ai-providers/core"

// ProtocolMetadata contains metadata about the Yandex gRPC protocol
type ProtocolMetadata struct {
	Name         string
	Description  string
	Capabilities []core.Capability
	Models       map[core.Capability][]ModelInfo
}

// ModelInfo contains information about a model
type ModelInfo struct {
	ID       string
	Name     string
	Features []string
}

// Metadata returns the protocol metadata
func Metadata() ProtocolMetadata {
	return ProtocolMetadata{
		Name:        "yandex_grpc",
		Description: "Yandex SpeechKit gRPC API",
		Capabilities: []core.Capability{
			core.CapabilitySTT,
			core.CapabilityTTS,
		},
		Models: map[core.Capability][]ModelInfo{
			core.CapabilitySTT: {
				{
					ID:       "general",
					Name:     "General",
					Features: []string{"streaming", "word-timestamps"},
				},
				{
					ID:       "general:rc",
					Name:     "General RC",
					Features: []string{"streaming", "word-timestamps"},
				},
			},
			core.CapabilityTTS: {
				{
					ID:       "tts",
					Name:     "Text-to-Speech",
					Features: []string{"streaming", "voice-selection"},
				},
			},
		},
	}
}
