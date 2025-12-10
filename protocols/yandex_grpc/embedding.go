package yandex_grpc

import (
	"context"
	"fmt"

	"github.com/creastat/providers/core"
	pb "github.com/creastat/providers/protocols/yandex_grpc/proto/generated/embedding"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// GenerateEmbedding implements EmbeddingProvider.GenerateEmbedding
func (p *Protocol) GenerateEmbedding(ctx context.Context, req core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if !p.SupportsCapability(core.CapabilityEmbedding) {
		return nil, fmt.Errorf("embedding capability not supported by this provider")
	}

	// Create gRPC connection
	conn, err := p.createGRPCConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}
	defer conn.Close()

	client := pb.NewEmbeddingsServiceClient(conn)

	// Build request
	pbReq := &pb.TextEmbeddingRequest{
		ModelUri: req.Model,
		Text:     req.Text,
	}

	// Add optional dimension parameter
	if req.Options != nil {
		if dim, ok := req.Options["dim"].(int64); ok {
			pbReq.Dim = wrapperspb.Int64(dim)
		}
	}

	// Add authorization metadata
	ctx = metadata.AppendToOutgoingContext(ctx,
		"authorization", fmt.Sprintf("Bearer %s", p.apiKey),
		"x-folder-id", p.folderID,
	)

	// Call the service
	resp, err := client.TextEmbedding(ctx, pbReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	if resp == nil {
		return nil, fmt.Errorf("empty response from embedding service")
	}

	// Convert embedding to float32 slice
	vector := make([]float32, len(resp.Embedding))
	for i, v := range resp.Embedding {
		vector[i] = float32(v)
	}

	return &core.EmbeddingResponse{
		Vector: vector,
		Model:  req.Model,
	}, nil
}
