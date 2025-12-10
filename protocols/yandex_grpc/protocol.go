package yandex_grpc

import (
	"context"
	"fmt"

	"github.com/creastat/providers/core"
)

// Protocol implements the Yandex gRPC protocol for voice services
type Protocol struct {
	name         string
	apiKey       string
	folderID     string
	capabilities []core.Capability
	initialized  bool
}

// NewProtocol creates a new Yandex gRPC protocol instance
func NewProtocol(config core.ProviderConfig) (core.Provider, error) {
	// Determine capabilities based on model URI or options
	capabilities := []core.Capability{
		core.CapabilitySTT,
		core.CapabilityTTS,
	}

	// If model_uri is provided in options, add LLM and Embedding capabilities
	if config.Options != nil {
		if _, ok := config.Options["model_uri"]; ok {
			capabilities = append(capabilities, core.CapabilityLLM, core.CapabilityEmbedding)
		}
	}

	return &Protocol{
		name:         config.Name,
		apiKey:       config.APIKey,
		capabilities: capabilities,
		initialized:  false,
	}, nil
}

// Name returns the provider name
func (p *Protocol) Name() string {
	return p.name
}

// Type returns the provider type
func (p *Protocol) Type() core.ProviderType {
	return "yandex_grpc"
}

// Initialize initializes the protocol
func (p *Protocol) Initialize(ctx context.Context, config core.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	// Extract folder ID from options
	if config.Options != nil {
		if folderID, ok := config.Options["folder_id"].(string); ok {
			p.folderID = folderID
		}
	}

	if p.folderID == "" {
		return fmt.Errorf("folder_id is required in options")
	}

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
