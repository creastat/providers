package cartesia_ws

import (
	"context"
	"fmt"

	"github.com/madmike/go-ai-providers/core"
)

// Protocol implements the Cartesia WebSocket protocol for TTS
type Protocol struct {
	name         string
	apiKey       string
	capabilities []core.Capability
	initialized  bool
}

// NewProtocol creates a new Cartesia WebSocket protocol instance
func NewProtocol(config core.ProviderConfig) (core.Provider, error) {
	return &Protocol{
		name:   config.Name,
		apiKey: config.APIKey,
		capabilities: []core.Capability{
			core.CapabilityTTS,
		},
		initialized: false,
	}, nil
}

func (p *Protocol) Name() string                    { return p.name }
func (p *Protocol) Type() core.ProviderType         { return "cartesia_ws" }
func (p *Protocol) Capabilities() []core.Capability { return p.capabilities }
func (p *Protocol) SupportsCapability(cap core.Capability) bool {
	for _, c := range p.capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func (p *Protocol) Initialize(ctx context.Context, config core.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	p.apiKey = config.APIKey
	p.initialized = true
	return nil
}

func (p *Protocol) Close() error {
	p.initialized = false
	return nil
}

func (p *Protocol) HealthCheck(ctx context.Context) error {
	if !p.initialized {
		return fmt.Errorf("protocol not initialized")
	}
	return nil
}
