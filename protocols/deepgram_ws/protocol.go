package deepgram_ws

import (
	"context"
	"fmt"

	"github.com/creastat/infra/telemetry"
	"github.com/creastat/providers/core"
)

// Protocol implements the Deepgram WebSocket protocol for STT
type Protocol struct {
	name         string
	apiKey       string
	capabilities []core.Capability
	initialized  bool
	logger       telemetry.Logger
}

// NewProtocol creates a new Deepgram WebSocket protocol instance
func NewProtocol(config core.ProviderConfig) (core.Provider, error) {
	var logger telemetry.Logger
	if config.Logger != nil {
		// Use provided logger
		logger = config.Logger.(telemetry.Logger)
	} else {
		// Create logger with console format to match service
		logger = telemetry.New(telemetry.Config{
			Level:  "debug",
			Format: "console",
		})
	}
	return &Protocol{
		name:   config.Name,
		apiKey: config.APIKey,
		capabilities: []core.Capability{
			core.CapabilitySTT,
		},
		initialized: false,
		logger:      logger,
	}, nil
}

func (p *Protocol) Name() string                    { return p.name }
func (p *Protocol) Type() core.ProviderType         { return "deepgram_ws" }
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
