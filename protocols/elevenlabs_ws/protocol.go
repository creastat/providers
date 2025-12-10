package elevenlabs_ws

import (
	"context"
	"fmt"

	"github.com/creastat/infra/telemetry"
	"github.com/creastat/providers/core"
)

// Protocol implements the ElevenLabs WebSocket protocol
type Protocol struct {
	name         string
	apiKey       string
	capabilities []core.Capability
	initialized  bool
	logger       any // telemetry.Logger
}

// NewProtocol creates a new ElevenLabs WebSocket protocol instance
func NewProtocol(config core.ProviderConfig) (core.Provider, error) {
	return &Protocol{
		name:   config.Name,
		apiKey: config.APIKey,
		capabilities: []core.Capability{
			core.CapabilityTTS,
			core.CapabilitySTT,
		},
		initialized: false,
		logger:      config.Logger,
	}, nil
}

func (p *Protocol) Name() string                    { return p.name }
func (p *Protocol) Type() core.ProviderType         { return "elevenlabs_ws" }
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
	p.logger = config.Logger
	p.initialized = true

	// Log initialization
	if p.logger != nil {
		if logger, ok := p.logger.(telemetry.Logger); ok {
			logger.Debug("ElevenLabs provider initialized")
		}
	}

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
