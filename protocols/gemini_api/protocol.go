package gemini_api

import (
	"context"
	"fmt"

	"github.com/madmike/go-ai-providers/core"
	"google.golang.org/genai"
)

// Protocol implements the Google Gemini API protocol
type Protocol struct {
	name         string
	apiKey       string
	client       *genai.Client
	capabilities []core.Capability
	initialized  bool
}

// NewProtocol creates a new Gemini API protocol instance
func NewProtocol(config core.ProviderConfig) (core.Provider, error) {
	return &Protocol{
		name:   config.Name,
		apiKey: config.APIKey,
		capabilities: []core.Capability{
			core.CapabilityLLM,
			core.CapabilityEmbedding,
		},
		initialized: false,
	}, nil
}

// Name returns the provider name
func (p *Protocol) Name() string {
	return p.name
}

// Type returns the provider type
func (p *Protocol) Type() core.ProviderType {
	return "gemini_api"
}

// Initialize initializes the protocol
func (p *Protocol) Initialize(ctx context.Context, config core.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: config.APIKey,
	})
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}

	p.client = client
	p.apiKey = config.APIKey
	p.initialized = true
	return nil
}

// Close closes the protocol
func (p *Protocol) Close() error {
	p.initialized = false
	return nil
}

// HealthCheck performs a health check
func (p *Protocol) HealthCheck(ctx context.Context) error {
	if !p.initialized {
		return fmt.Errorf("protocol not initialized")
	}
	return nil
}

// Capabilities returns the list of capabilities
func (p *Protocol) Capabilities() []core.Capability {
	return p.capabilities
}

// SupportsCapability checks if the protocol supports a capability
func (p *Protocol) SupportsCapability(capability core.Capability) bool {
	for _, cap := range p.capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}
