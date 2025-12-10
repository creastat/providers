package gemini_api

import (
	"context"
	"fmt"

	"github.com/creastat/providers/core"
	"google.golang.org/genai"
)

// GenerateEmbedding implements EmbeddingProvider.GenerateEmbedding
func (p *Protocol) GenerateEmbedding(ctx context.Context, req core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if req.Model == "" {
		req.Model = "text-embedding-004"
	}

	// Generate embedding
	resp, err := p.client.Models.EmbedContent(ctx, req.Model, genai.Text(req.Text), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if resp.Embeddings == nil || len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embedding in response")
	}

	// Get first embedding
	embedding := resp.Embeddings[0]
	if embedding == nil || len(embedding.Values) == 0 {
		return nil, fmt.Errorf("no embedding values in response")
	}

	// Convert to float32 slice
	values := make([]float32, len(embedding.Values))
	for i, v := range embedding.Values {
		values[i] = float32(v)
	}

	return &core.EmbeddingResponse{
		Vector: values,
		Model:  req.Model,
	}, nil
}
