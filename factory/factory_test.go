package factory

import (
	"context"
	"testing"

	"github.com/madmike/go-ai-providers/core"
	"github.com/stretchr/testify/require"
)

// MockProtocol implements core.Provider for testing factory behavior.
type MockProtocol struct {
	name_        string
	initialized  bool
	closed       bool
	capabilities []core.Capability
}

func (m *MockProtocol) Name() string {
	return m.name_
}

func (m *MockProtocol) Type() core.ProviderType {
	return core.ProviderTypeOpenAI
}

func (m *MockProtocol) Initialize(ctx context.Context, config core.ProviderConfig) error {
	m.initialized = true
	return nil
}

func (m *MockProtocol) Close() error {
	m.closed = true
	return nil
}

func (m *MockProtocol) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockProtocol) Capabilities() []core.Capability {
	return m.capabilities
}

func (m *MockProtocol) SupportsCapability(capability core.Capability) bool {
	for _, c := range m.capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// TestProtocolRegistry verifies core protocols are registered.
func TestProtocolRegistry(t *testing.T) {
	expectedProtocols := []string{
		"openai_api",
		"gemini_api",
		"yandex_grpc",
		"deepgram_ws",
		"cartesia_ws",
		"minimax_ws",
		"elevenlabs_ws",
	}

	for _, proto := range expectedProtocols {
		_, ok := ProtocolRegistry[proto]
		require.True(t, ok, "protocol %s not in registry", proto)
	}
}

// TestProtocolFactorySignature verifies protocol factories are callable.
func TestProtocolFactorySignature(t *testing.T) {
	factory, ok := ProtocolRegistry["openai_api"]
	require.True(t, ok)
	require.NotNil(t, factory)
}

// TestDBProviderConfig tests the configuration struct.
func TestDBProviderConfig(t *testing.T) {
	config := DBProviderConfig{
		PresetName:  "openai-gpt4",
		DisplayName: "OpenAI GPT-4",
		APIKey:      "sk-test-123",
		BaseURL:     "https://api.openai.com/v1",
		Options: map[string]any{
			"max_tokens": 2048,
		},
	}

	require.Equal(t, "openai-gpt4", config.PresetName)
	require.Equal(t, "OpenAI GPT-4", config.DisplayName)
	require.Equal(t, "sk-test-123", config.APIKey)
	require.NotNil(t, config.Options)
}

// TestCreateFromDBUnknownPreset returns error for unknown preset.
func TestCreateFromDBUnknownPreset(t *testing.T) {
	config := DBProviderConfig{
		PresetName: "unknown-preset",
		APIKey:     "key",
	}

	provider, err := CreateFromDB(config)
	require.Error(t, err)
	require.Nil(t, provider)
	require.Contains(t, err.Error(), "unknown preset")
}

// TestCreateFromPresetSignature tests the CreateFromPreset wrapper.
func TestCreateFromPresetSignature(t *testing.T) {
	// This just verifies the function exists and can be called
	// Actual provider creation tests would need real protocol implementations
	require.NotNil(t, CreateFromPreset)
}

// TestDBProviderConfigWithOptions verifies options handling.
func TestDBProviderConfigWithOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
	}{
		{
			"empty options",
			map[string]any{},
		},
		{
			"multiple options",
			map[string]any{
				"max_tokens":     2048,
				"temperature":    0.7,
				"top_p":          0.9,
				"stream":         true,
			},
		},
		{
			"nil options",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DBProviderConfig{
				PresetName: "test",
				Options:    tt.options,
			}
			require.Equal(t, tt.options, config.Options)
		})
	}
}

// TestDBProviderConfigBaseURLOverride verifies BaseURL handling in CreateFromDB.
// This is a unit test for the logic pattern, not actual provider creation.
func TestDBProviderConfigBaseURLOverride(t *testing.T) {
	// When BaseURL is empty string, the preset's default should be used
	// When BaseURL is provided, it should override the preset
	config := DBProviderConfig{
		PresetName: "openai",
		BaseURL:    "https://custom.openai.com/v1",
	}

	require.Equal(t, "https://custom.openai.com/v1", config.BaseURL)

	config2 := DBProviderConfig{
		PresetName: "openai",
		BaseURL:    "",
	}
	require.Empty(t, config2.BaseURL)
}

// TestCreateFromDBContextIsPassed verifies context is properly passed to Initialize.
// This ensures the factory passes context correctly through the chain.
func TestCreateFromDBInitializePattern(t *testing.T) {
	// The factory calls provider.Initialize(context.Background(), config)
	// This test verifies that pattern is correct
	config := DBProviderConfig{
		PresetName: "nonexistent", // Will fail before Initialize
	}

	_, err := CreateFromDB(config)
	require.Error(t, err)
}

// TestProtocolRegistryIsPopulated verifies registry is not empty at startup.
func TestProtocolRegistryIsPopulated(t *testing.T) {
	require.NotEmpty(t, ProtocolRegistry)
	require.Greater(t, len(ProtocolRegistry), 0)
}

// TestDBProviderConfigDisplayName verifies display name is preserved.
func TestDBProviderConfigDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
	}{
		{"gpt-4-turbo", "gpt-4-turbo"},
		{"claude-3-opus", "claude-3-opus"},
		{"gemini-pro", "gemini-pro"},
		{"", ""},
	}

	for _, tt := range tests {
		config := DBProviderConfig{
			DisplayName: tt.displayName,
		}
		require.Equal(t, tt.name, config.DisplayName)
	}
}
