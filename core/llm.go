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
	Tools       []Tool // Optional: tool definitions for function calling
	Temperature *float64
	MaxTokens   *int
	TopP        *float64
	// JSONMode instructs the provider to enforce JSON output (response_format=json_object).
	// When true, callers must still describe the expected JSON schema in the prompt,
	// but should omit "return JSON only / no markdown" boilerplate.
	JSONMode bool
	Options  map[string]any // Provider-specific options
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	Content   string
	Model     string
	Usage     *Usage
	ToolCalls []ToolCall // Non-empty when the LLM wants to call tools
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
	// ToolCalls is populated only in the final Done chunk when the model
	// decided to call tools instead of (or after) emitting content.
	ToolCalls []ToolCall
	// Usage is populated in the final Done chunk when the provider sends it.
	Usage *Usage
}
