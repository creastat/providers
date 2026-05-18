package factory

import "github.com/madmike/go-ai-providers/core"

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
	"groq": {
		Protocol: "openai_api",
		Name:     "Groq",
		BaseURL:  "https://api.groq.com/openai/v1",
		Capabilities: []core.Capability{
			core.CapabilityLLM,
		},
		ModelOverrides: map[core.Capability][]string{
			core.CapabilityLLM: {
				// Llama 4 + low-latency inference on Groq LPU.
				// Primary LLM target for VOIP agent (sub-second TTFT).
				"meta-llama/llama-4-scout-17b-16e-instruct",
				"meta-llama/llama-4-maverick-17b-128e-instruct",
				"llama-3.3-70b-versatile",
				"llama-3.1-8b-instant",
				"mixtral-8x7b-32768",
			},
		},
		Description: "Groq LPU — ultra-low-latency Llama 4 / Mixtral inference. Primary LLM for VOIP agent.",
	},
	"fireworks": {
		Protocol: "openai_api",
		Name:     "Fireworks AI",
		BaseURL:  "https://api.fireworks.ai/inference/v1",
		Capabilities: []core.Capability{
			core.CapabilityLLM,
			core.CapabilityEmbedding,
		},
		ModelOverrides: map[core.Capability][]string{
			core.CapabilityLLM: {
				"accounts/fireworks/models/llama4-scout-instruct-basic",
				"accounts/fireworks/models/llama4-maverick-instruct-basic",
				"accounts/fireworks/models/llama-v3p3-70b-instruct",
				"accounts/fireworks/models/mixtral-8x22b-instruct",
			},
		},
		Description: "Fireworks AI — fast OpenAI-compatible inference. Fallback LLM for VOIP agent.",
	},
	"openrouter": {
		Protocol: "openai_api",
		Name:     "OpenRouter",
		BaseURL:  "https://openrouter.ai/api/v1",
		Capabilities: []core.Capability{
			core.CapabilityLLM,
			core.CapabilityEmbedding,
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
			core.CapabilityTTS,
		},
	},
	"cartesia": {
		Protocol:     "cartesia_ws",
		Name:         "Cartesia Sonic",
		BaseURL:      "",
		Capabilities: []core.Capability{core.CapabilityTTS},
		ModelOverrides: map[core.Capability][]string{
			core.CapabilityTTS: {
				"sonic-3",
				"sonic-english",
				"sonic-multilingual",
			},
		},
		Description: "Cartesia Sonic — ultra-low-latency TTS (WebSocket). Primary TTS for VOIP agent.",
	},
	"minimax": {
		Protocol:     "minimax_ws",
		BaseURL:      "",
		Capabilities: []core.Capability{core.CapabilityTTS},
		Description:  "Minimax TTS (WebSocket)",
	},
	"elevenlabs": {
		Protocol: "elevenlabs_ws",
		Name:     "ElevenLabs",
		BaseURL:  "https://api.elevenlabs.io/v1",
		Capabilities: []core.Capability{
			core.CapabilityTTS,
			core.CapabilitySTT,
		},
		ModelOverrides: map[core.Capability][]string{
			core.CapabilityTTS: {
				"eleven_flash_v2_5", // fastest, <75ms TTFB — preferred for VOIP fallback
				"eleven_turbo_v2_5",
				"eleven_multilingual_v2",
			},
		},
		Description: "ElevenLabs TTS & STT (WebSocket). Fallback TTS for VOIP agent when Cartesia is unavailable.",
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
