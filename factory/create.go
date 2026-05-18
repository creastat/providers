package factory

import (
	"context"
	"fmt"

	"github.com/madmike/go-ai-providers/core"
	"github.com/madmike/go-ai-providers/protocols/cartesia_ws"
	"github.com/madmike/go-ai-providers/protocols/deepgram_ws"
	"github.com/madmike/go-ai-providers/protocols/elevenlabs_ws"
	"github.com/madmike/go-ai-providers/protocols/gemini_api"
	"github.com/madmike/go-ai-providers/protocols/minimax_ws"
	"github.com/madmike/go-ai-providers/protocols/openai_api"
	"github.com/madmike/go-ai-providers/protocols/yandex_grpc"
)

// ProtocolFactory is a function that creates a protocol instance
type ProtocolFactory func(config core.ProviderConfig) (core.Provider, error)

// ProtocolRegistry holds all available protocol factories
var ProtocolRegistry = map[string]ProtocolFactory{
	"openai_api":    openai_api.NewProtocol,
	"gemini_api":    gemini_api.NewProtocol,
	"yandex_grpc":   yandex_grpc.NewProtocol,
	"deepgram_ws":   deepgram_ws.NewProtocol,
	"cartesia_ws":   cartesia_ws.NewProtocol,
	"minimax_ws":    minimax_ws.NewProtocol,
	"elevenlabs_ws": elevenlabs_ws.NewProtocol,
}

// DBProviderConfig represents provider configuration from database
type DBProviderConfig struct {
	PresetName  string
	DisplayName string
	APIKey      string
	BaseURL     string         // Optional override
	Options     map[string]any // Provider-specific options
	Logger      any            // Logger instance
}

// CreateFromDB creates a provider instance from database configuration
func CreateFromDB(dbConfig DBProviderConfig) (core.Provider, error) {
	// Get preset
	preset, ok := Presets[dbConfig.PresetName]
	if !ok {
		return nil, fmt.Errorf("unknown preset: %s", dbConfig.PresetName)
	}

	// Get protocol factory
	protocolFactory, ok := ProtocolRegistry[preset.Protocol]
	if !ok {
		return nil, fmt.Errorf("unknown protocol: %s", preset.Protocol)
	}

	// Build provider config
	config := core.ProviderConfig{
		Name:    dbConfig.DisplayName,
		Type:    core.ProviderType(preset.Protocol),
		APIKey:  dbConfig.APIKey,
		BaseURL: dbConfig.BaseURL,
		Options: dbConfig.Options,
		Logger:  dbConfig.Logger,
	}

	// Use preset base URL if not overridden
	if config.BaseURL == "" {
		config.BaseURL = preset.BaseURL
	}

	// Validate required options
	for _, reqOpt := range preset.RequiredOptions {
		if config.Options == nil || config.Options[reqOpt] == nil {
			return nil, fmt.Errorf("required option %s not provided for preset %s", reqOpt, dbConfig.PresetName)
		}
	}

	// Create provider
	provider, err := protocolFactory(config)
	if err != nil {
		return nil, err
	}

	// Initialize provider
	if err := provider.Initialize(context.Background(), config); err != nil {
		return nil, fmt.Errorf("failed to initialize provider %s: %w", dbConfig.DisplayName, err)
	}

	return provider, nil
}

// CreateFromPreset creates a provider instance from a preset name and API key
func CreateFromPreset(presetName, displayName, apiKey string, baseURL string, options map[string]any, logger any) (core.Provider, error) {
	return CreateFromDB(DBProviderConfig{
		PresetName:  presetName,
		DisplayName: displayName,
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Options:     options,
		Logger:      logger,
	})
}

// GetPreset returns a preset by name
func GetPreset(name string) (ProviderPreset, bool) {
	preset, ok := Presets[name]
	return preset, ok
}

// ListPresets returns all available presets
func ListPresets() map[string]ProviderPreset {
	return Presets
}
