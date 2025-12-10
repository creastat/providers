package core

import "context"

// LLMProvider provides language model capabilities
type LLMProvider interface {
	Provider
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	StreamChatCompletion(ctx context.Context, req ChatRequest) (ChatStream, error)
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature *float64
	MaxTokens   *int
	TopP        *float64
	Options     map[string]any // Provider-specific options
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	Content string
	Model   string
	Usage   *Usage
}

// ChatStream represents a streaming chat response
type ChatStream interface {
	Receive(ctx context.Context) (*ChatChunk, error)
	Close() error
}

// ChatChunk represents a chunk of streaming chat response
type ChatChunk struct {
	Content      string
	Done         bool
	FinishReason string
}
