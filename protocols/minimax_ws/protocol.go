package minimax_ws

import (
	"context"
	"fmt"
	"slices"

	"github.com/creastat/providers/core"
)

// Protocol implements the Minimax WebSocket protocol for TTS
type Protocol struct {
	name         string
	apiKey       string
	capabilities []core.Capability
	initialized  bool
	logger       any // telemetry.Logger
}

// NewProtocol creates a new Minimax WebSocket protocol instance
func NewProtocol(config core.ProviderConfig) (core.Provider, error) {
	return &Protocol{
		name:   config.Name,
		apiKey: config.APIKey,
		capabilities: []core.Capability{
			core.CapabilityTTS,
		},
		initialized: false,
		logger:      config.Logger,
	}, nil
}

func (p *Protocol) Name() string                    { return p.name }
func (p *Protocol) Type() core.ProviderType         { return "minimax_ws" }
func (p *Protocol) Capabilities() []core.Capability { return p.capabilities }
func (p *Protocol) SupportsCapability(cap core.Capability) bool {
	return slices.Contains(p.capabilities, cap)
}

func (p *Protocol) Initialize(ctx context.Context, config core.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	p.apiKey = config.APIKey
	p.logger = config.Logger
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
