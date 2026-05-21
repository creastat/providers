package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// MockProvider implements Provider for testing.
type MockProvider struct {
	name_        string
	type_        ProviderType
	capabilities []Capability
	initialized  bool
	closed       bool
	healthErr    error
}

func (m *MockProvider) Name() string {
	return m.name_
}

func (m *MockProvider) Type() ProviderType {
	return m.type_
}

func (m *MockProvider) Initialize(ctx context.Context, config ProviderConfig) error {
	m.initialized = true
	return nil
}

func (m *MockProvider) Close() error {
	m.closed = true
	return nil
}

func (m *MockProvider) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

func (m *MockProvider) Capabilities() []Capability {
	return m.capabilities
}

func (m *MockProvider) SupportsCapability(capability Capability) bool {
	for _, c := range m.capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// TestNewRegistry creates and returns a new registry.
func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	require.NotNil(t, registry)

	// Registry should start empty
	providers := registry.List()
	require.Equal(t, 0, len(providers))
}

// TestRegistryRegister adds a provider to the registry.
func TestRegistryRegister(t *testing.T) {
	registry := NewRegistry()
	provider := &MockProvider{
		name_: "openai",
		type_: ProviderTypeOpenAI,
	}

	err := registry.Register(provider)
	require.NoError(t, err)

	// Verify it was added
	providers := registry.List()
	require.Equal(t, 1, len(providers))
	require.Equal(t, "openai", providers[0].Name())
}

// TestRegistryRegisterDuplicate verifies that duplicate registrations are rejected.
func TestRegistryRegisterDuplicate(t *testing.T) {
	registry := NewRegistry()
	provider1 := &MockProvider{name_: "openai"}

	err := registry.Register(provider1)
	require.NoError(t, err)

	provider2 := &MockProvider{name_: "openai"}
	err = registry.Register(provider2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered")
}

// TestRegistryGet retrieves a provider by name.
func TestRegistryGet(t *testing.T) {
	registry := NewRegistry()
	provider := &MockProvider{
		name_: "openai",
		type_: ProviderTypeOpenAI,
	}
	err := registry.Register(provider)
	require.NoError(t, err)

	retrieved, err := registry.Get("openai")
	require.NoError(t, err)
	require.Equal(t, "openai", retrieved.Name())
}

// TestRegistryGetNotFound returns error for missing provider.
func TestRegistryGetNotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Get("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// TestRegistryList returns all registered providers.
func TestRegistryList(t *testing.T) {
	registry := NewRegistry()

	providers := []struct{ name string }{
		{"openai"},
		{"gemini"},
		{"anthropic"},
	}

	for _, p := range providers {
		err := registry.Register(&MockProvider{name_: p.name})
		require.NoError(t, err)
	}

	list := registry.List()
	require.Equal(t, 3, len(list))
}

// TestRegistryUnregister removes a provider from the registry.
func TestRegistryUnregister(t *testing.T) {
	registry := NewRegistry()
	provider := &MockProvider{name_: "openai"}

	err := registry.Register(provider)
	require.NoError(t, err)

	require.Equal(t, 1, len(registry.List()))

	err = registry.Unregister("openai")
	require.NoError(t, err)

	require.Equal(t, 0, len(registry.List()))
}

// TestRegistryUnregisterNotFound returns error when unregistering non-existent provider.
func TestRegistryUnregisterNotFound(t *testing.T) {
	registry := NewRegistry()

	err := registry.Unregister("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// TestRegistryConcurrentRegister tests thread-safe registration.
func TestRegistryConcurrentRegister(t *testing.T) {
	registry := NewRegistry()

	// Register multiple providers concurrently
	done := make(chan bool, 3)

	for i := 0; i < 3; i++ {
		go func(idx int) {
			provider := &MockProvider{name_: fmt.Sprintf("provider-%d", idx)}
			err := registry.Register(provider)
			require.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	require.Equal(t, 3, len(registry.List()))
}

// TestProviderTypeConstants verifies all provider types are defined.
func TestProviderTypeConstants(t *testing.T) {
	types := []ProviderType{
		ProviderTypeOpenAI,
		ProviderTypeGemini,
		ProviderTypeYandex,
		ProviderTypeDeepgram,
		ProviderTypeCartesia,
		ProviderTypeMinimax,
	}

	for _, pt := range types {
		require.NotEmpty(t, pt)
	}
}

// TestCapabilityConstants verifies all capabilities are defined.
func TestCapabilityConstants(t *testing.T) {
	caps := []Capability{
		CapabilityLLM,
		CapabilityEmbedding,
		CapabilitySTT,
		CapabilityTTS,
	}

	for _, c := range caps {
		require.NotEmpty(t, c)
	}
}

// TestMockProviderSupportsCapability tests capability checking.
func TestMockProviderSupportsCapability(t *testing.T) {
	provider := &MockProvider{
		name_: "openai",
		capabilities: []Capability{
			CapabilityLLM,
			CapabilityEmbedding,
		},
	}

	require.True(t, provider.SupportsCapability(CapabilityLLM))
	require.True(t, provider.SupportsCapability(CapabilityEmbedding))
	require.False(t, provider.SupportsCapability(CapabilitySTT))
	require.False(t, provider.SupportsCapability(CapabilityTTS))
}

// TestMockProviderLifecycle tests initialization and closure.
func TestMockProviderLifecycle(t *testing.T) {
	provider := &MockProvider{name_: "test"}

	require.False(t, provider.initialized)
	require.False(t, provider.closed)

	err := provider.Initialize(context.Background(), ProviderConfig{})
	require.NoError(t, err)
	require.True(t, provider.initialized)

	err = provider.Close()
	require.NoError(t, err)
	require.True(t, provider.closed)
}

// TestUsageStruct tests Usage data structure.
func TestUsageStruct(t *testing.T) {
	usage := Usage{
		InputTokens:       100,
		OutputTokens:      50,
		CachedInputTokens: 25,
		TotalTokens:       175,
	}

	require.Equal(t, 100, usage.InputTokens)
	require.Equal(t, 50, usage.OutputTokens)
	require.Equal(t, 25, usage.CachedInputTokens)
	require.Equal(t, 175, usage.TotalTokens)
}

// TestMessageStruct tests Message data structure.
func TestMessageStruct(t *testing.T) {
	message := Message{
		Role:    "assistant",
		Content: "Hello",
		ToolCalls: []ToolCall{
			{
				ID:    "call-1",
				Name:  "get_weather",
				Input: `{"location": "NYC"}`,
			},
		},
	}

	require.Equal(t, "assistant", message.Role)
	require.Equal(t, "Hello", message.Content)
	require.Equal(t, 1, len(message.ToolCalls))
	require.Equal(t, "call-1", message.ToolCalls[0].ID)
}

// TestToolStruct tests Tool data structure.
func TestToolStruct(t *testing.T) {
	tool := Tool{
		Name:        "get_weather",
		Description: "Get weather for a location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	require.Equal(t, "get_weather", tool.Name)
	require.NotNil(t, tool.InputSchema)
}

// TestProviderConfigStruct tests ProviderConfig data structure.
func TestProviderConfigStruct(t *testing.T) {
	config := ProviderConfig{
		Name:    "openai-gpt4",
		Type:    ProviderTypeOpenAI,
		APIKey:  "sk-test-123",
		BaseURL: "https://api.openai.com/v1",
		Options: map[string]any{
			"max_tokens": 2048,
		},
		Enabled: true,
	}

	require.Equal(t, "openai-gpt4", config.Name)
	require.Equal(t, ProviderTypeOpenAI, config.Type)
	require.Equal(t, "sk-test-123", config.APIKey)
	require.True(t, config.Enabled)
}
