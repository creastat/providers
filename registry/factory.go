package registry

import (
	"fmt"
	"strings"
	"sync"

	providercore "github.com/madmike/go-ai-providers/core"
	"github.com/madmike/go-ai-providers/factory"
)

// ProviderFactory is a cached factory for instantiating AI providers
type ProviderFactory struct {
	mu        sync.RWMutex
	providers map[string]providercore.LLMProvider
}

// NewProviderFactory creates a new registry
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		providers: make(map[string]providercore.LLMProvider),
	}
}

// GetOrCreateProvider parses a connection string (e.g., "openai://apikey") and returns a cached or new LLMProvider
func (f *ProviderFactory) GetOrCreateProvider(connString string) (providercore.LLMProvider, error) {
	if connString == "" {
		return nil, fmt.Errorf("connection string cannot be empty")
	}

	f.mu.RLock()
	if p, ok := f.providers[connString]; ok {
		f.mu.RUnlock()
		return p, nil
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()

	// Double check
	if p, ok := f.providers[connString]; ok {
		return p, nil
	}

	// Determine provider type and base URL from conn string
	var providerType string
	var apiKey string

	parts := strings.SplitN(connString, "://", 2)
	if len(parts) == 2 {
		providerType = parts[0]
		apiKey = parts[1]
	} else {
		// Fallback assumption
		providerType = "openai"
		apiKey = connString
	}

	// Remove generic host/path parts if they exist inside the key part (naive check)
	if idx := strings.Index(apiKey, "@"); idx != -1 {
		// handle proxy/host based API keys like deepseek://key@api.deepseek.com
		apiKey = apiKey[:idx]
	}

	var p providercore.Provider
	var err error

	switch providerType {
	case "anthropic":
		p, err = factory.CreateFromPreset("anthropic", "Anthropic", apiKey, "", nil, nil)
	case "gemini":
		p, err = factory.CreateFromPreset("gemini", "Gemini", apiKey, "", nil, nil)
	case "deepseek":
		p, err = factory.CreateFromPreset("openai", "Deepseek", apiKey, "https://api.deepseek.com/v1", nil, nil)
	case "openrouter":
		p, err = factory.CreateFromPreset("openrouter", "OpenRouter", apiKey, "", nil, nil)
	default:
		p, err = factory.CreateFromPreset("openai", "OpenAI", apiKey, "", nil, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create provider %s: %w", providerType, err)
	}

	llm, ok := p.(providercore.LLMProvider)
	if !ok {
		return nil, fmt.Errorf("provider %s does not implement LLMProvider", providerType)
	}

	f.providers[connString] = llm
	return llm, nil
}
