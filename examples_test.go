package providers_test

import (
	"context"
	"fmt"
	"log"

	"github.com/madmike/go-ai-providers/core"
	"github.com/madmike/go-ai-providers/factory"
)

// Example_basicUsage demonstrates basic provider usage
func Example_basicUsage() {
	ctx := context.Background()

	// Create provider from preset
	provider, err := factory.CreateFromPreset("openai", "OpenAI Main", "sk-...", "", nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize
	err = provider.Initialize(ctx, core.ProviderConfig{
		Name:   "OpenAI Main",
		APIKey: "sk-...",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	// Use LLM capability
	llmProvider := provider.(core.LLMProvider)
	response, err := llmProvider.ChatCompletion(ctx, core.ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []core.Message{
			{Role: "user", Content: "Hello!"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(response.Content)
}

// Example_streaming demonstrates streaming usage
func Example_streaming() {
	ctx := context.Background()

	provider, _ := factory.CreateFromPreset("openai", "OpenAI", "sk-...", "", nil, nil)
	provider.Initialize(ctx, core.ProviderConfig{APIKey: "sk-..."})
	defer provider.Close()

	llmProvider := provider.(core.LLMProvider)
	stream, err := llmProvider.StreamChatCompletion(ctx, core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: "user", Content: "Tell me a story"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	for {
		chunk, err := stream.Receive(ctx)
		if err != nil {
			log.Fatal(err)
		}
		if chunk.Done {
			break
		}
		fmt.Print(chunk.Content)
	}
}

// Example_multipleProviders demonstrates using multiple providers
func Example_multipleProviders() {
	ctx := context.Background()

	// Create registry
	registry := core.NewRegistry()

	// Register multiple providers
	openai, _ := factory.CreateFromPreset("openai", "OpenAI", "sk-...", "", nil, nil)
	openai.Initialize(ctx, core.ProviderConfig{APIKey: "sk-..."})
	registry.Register(openai)

	yandex, _ := factory.CreateFromPreset("yandex-llm", "Yandex", "AQVN...", "", map[string]any{
		"folder_id": "b1g...",
	}, nil)
	yandex.Initialize(ctx, core.ProviderConfig{
		APIKey:  "AQVN...",
		Options: map[string]any{"folder_id": "b1g..."},
	})
	registry.Register(yandex)

	// Use different providers
	openaiProvider, _ := registry.Get("OpenAI")
	yandexProvider, _ := registry.Get("Yandex")

	// Both implement LLMProvider
	llm1 := openaiProvider.(core.LLMProvider)
	llm2 := yandexProvider.(core.LLMProvider)

	// Use them
	resp1, _ := llm1.ChatCompletion(ctx, core.ChatRequest{Model: "gpt-4o-mini", Messages: []core.Message{{Role: "user", Content: "Hi"}}})
	resp2, _ := llm2.ChatCompletion(ctx, core.ChatRequest{Model: "yandexgpt/latest", Messages: []core.Message{{Role: "user", Content: "Привет"}}})

	fmt.Println(resp1.Content)
	fmt.Println(resp2.Content)
}
