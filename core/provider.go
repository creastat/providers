package core

import (
	"context"
	"time"
)

// ProviderType represents the type/category of provider
type ProviderType string

const (
	ProviderTypeOpenAI   ProviderType = "openai"
	ProviderTypeGemini   ProviderType = "gemini"
	ProviderTypeYandex   ProviderType = "yandex"
	ProviderTypeDeepgram ProviderType = "deepgram"
	ProviderTypeCartesia ProviderType = "cartesia"
	ProviderTypeMinimax  ProviderType = "minimax"
)

// Capability represents what a provider can do
type Capability string

const (
	CapabilityLLM       Capability = "llm"
	CapabilityEmbedding Capability = "embedding"
	CapabilitySTT       Capability = "stt"
	CapabilityTTS       Capability = "tts"
)

// Provider is the base interface for all AI providers
type Provider interface {
	// Identity
	Name() string
	Type() ProviderType

	// Lifecycle
	Initialize(ctx context.Context, config ProviderConfig) error
	Close() error
	HealthCheck(ctx context.Context) error

	// Capabilities
	Capabilities() []Capability
	SupportsCapability(capability Capability) bool
}

// ProviderConfig represents configuration for a provider
type ProviderConfig struct {
	Name    string
	Type    ProviderType
	APIKey  string
	BaseURL string
	Options map[string]any
	Timeout time.Duration
	Enabled bool
	Logger  any // Logger instance (telemetry.Logger)
}

// Usage represents token/resource usage for a request
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
