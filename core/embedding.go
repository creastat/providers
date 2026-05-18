package core

import "context"

// EmbeddingProvider provides embedding generation capabilities
type EmbeddingProvider interface {
	Provider
	GenerateEmbedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
}

// EmbeddingRequest represents an embedding generation request
type EmbeddingRequest struct {
	Model   string
	Text    string
	Options map[string]any // Provider-specific options
}

// EmbeddingResponse represents an embedding generation response
type EmbeddingResponse struct {
	Vector []float32
	Model  string
	Usage  *Usage
}
