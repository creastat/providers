package openai_api

import (
	"context"
	"fmt"

	"github.com/creastat/providers/core"
	"github.com/sashabaranov/go-openai"
)

// GenerateEmbedding implements EmbeddingProvider.GenerateEmbedding
func (p *Protocol) GenerateEmbedding(ctx context.Context, req core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	// Create OpenAI client
	config := openai.DefaultConfig(p.apiKey)
	config.BaseURL = p.baseURL
	client := openai.NewClientWithConfig(config)

	// Build request
	embReq := openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(req.Model),
		Input: []string{req.Text},
	}

	// Make request
	resp, err := client.CreateEmbeddings(ctx, embReq)
	if err != nil {
		return nil, fmt.Errorf("embedding generation failed: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	// Convert to float32
	embedding := resp.Data[0].Embedding
	vector := make([]float32, len(embedding))
	copy(vector, embedding)

	return &core.EmbeddingResponse{
		Vector: vector,
		Model:  req.Model,
	}, nil
}
