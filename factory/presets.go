package factory

import "github.com/creastat/providers/core"

// ProviderPreset defines a provider configuration preset
type ProviderPreset struct {
	Protocol        string
	Name            string
	BaseURL         string
	Capabilities    []core.Capability
	ModelOverrides  map[core.Capability][]string
	RequiredOptions []string
	Description     string
}

// Presets contains predefined provider configurations
var Presets = map[string]ProviderPreset{
	"openai": {
		Protocol: "openai_api",
		Name:     "OpenAI",
		BaseURL:  "https://api.openai.com/v1",
		Capabilities: []core.Capability{
			core.CapabilityLLM,
			core.CapabilityEmbedding,
			core.CapabilitySTT,
		},
	},
	"openrouter": {
		Protocol: "openai_api",
		Name:     "OpenRouter",
		BaseURL:  "https://openrouter.ai/api/v1",
		Capabilities: []core.Capability{
			core.CapabilityLLM,
		},
		ModelOverrides: map[core.Capability][]string{
			core.CapabilityLLM: {
				"openai/gpt-4o",
				"openai/gpt-4o-mini",
				"anthropic/claude-3.5-sonnet",
				"google/gemini-pro-1.5",
				"meta-llama/llama-3.1-70b-instruct",
			},
		},
	},
	"yandex-llm": {
		Protocol: "openai_api",
		Name:     "Yandex LLM",
		BaseURL:  "https://llm.api.cloud.yandex.net/v1",
		Capabilities: []core.Capability{
			core.CapabilityLLM,
			core.CapabilityEmbedding,
		},
		RequiredOptions: []string{"folder_id"},
		ModelOverrides: map[core.Capability][]string{
			core.CapabilityLLM: {
				"yandexgpt/latest",
				"yandexgpt-lite/latest",
				"yandexgpt-32k/latest",
			},
			core.CapabilityEmbedding: {
				"text-search-doc/latest",
				"text-search-query/latest",
			},
		},
	},
	"gemini": {
		Protocol: "gemini_api",
		Name:     "Google Gemini",
		Capabilities: []core.Capability{
			core.CapabilityLLM,
			core.CapabilityEmbedding,
		},
	},
	"yandex-voice": {
		Protocol: "yandex_grpc",
		Name:     "Yandex Voice",
		Capabilities: []core.Capability{
			core.CapabilitySTT,
			core.CapabilityTTS,
		},
		RequiredOptions: []string{"folder_id"},
	},
	"deepgram": {
		Protocol: "deepgram_ws",
		Name:     "Deepgram",
		Capabilities: []core.Capability{
			core.CapabilitySTT,
		},
	},
	"cartesia": {
		Protocol:     "cartesia_ws",
		BaseURL:      "",
		Capabilities: []core.Capability{core.CapabilityTTS},
		Description:  "Cartesia TTS (WebSocket)",
	},
	"minimax": {
		Protocol:     "minimax_ws",
		BaseURL:      "",
		Capabilities: []core.Capability{core.CapabilityTTS},
		Description:  "Minimax TTS (WebSocket)",
	},
	"elevenlabs": {
		Protocol: "elevenlabs_ws",
		BaseURL:  "https://api.elevenlabs.io/v1",
		Capabilities: []core.Capability{
			core.CapabilityTTS,
			core.CapabilitySTT,
		},
		Description: "ElevenLabs TTS & STT (WebSocket)",
	},
	"yandex-llm-grpc": {
		Protocol: "yandex_grpc",
		Name:     "Yandex LLM (gRPC)",
		Capabilities: []core.Capability{
			core.CapabilityLLM,
			core.CapabilityEmbedding,
		},
		RequiredOptions: []string{"folder_id", "model_uri"},
		ModelOverrides: map[core.Capability][]string{
			core.CapabilityLLM: {
				"yandexgpt/latest",
				"yandexgpt-lite/latest",
				"yandexgpt-32k/latest",
			},
			core.CapabilityEmbedding: {
				"text-search-doc/latest",
				"text-search-query/latest",
			},
		},
		Description: "Yandex Foundation Models via gRPC (LLM + Embeddings)",
	},
}
