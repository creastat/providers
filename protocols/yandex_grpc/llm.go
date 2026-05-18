package yandex_grpc

import (
	"context"
	"fmt"
	"io"

	"github.com/madmike/go-ai-providers/core"
	pb "github.com/madmike/go-ai-providers/protocols/yandex_grpc/proto/generated/text_generation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// ChatCompletion implements LLMProvider.ChatCompletion
func (p *Protocol) ChatCompletion(ctx context.Context, req core.ChatRequest) (*core.ChatResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if !p.SupportsCapability(core.CapabilityLLM) {
		return nil, fmt.Errorf("LLM capability not supported by this provider")
	}

	// Create gRPC connection
	conn, err := p.createGRPCConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}
	defer conn.Close()

	client := pb.NewTextGenerationServiceClient(conn)

	// Convert messages
	messages := make([]*pb.Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = &pb.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Build completion options
	completionOpts := &pb.CompletionOptions{}
	if req.Temperature != nil {
		completionOpts.Temperature = float32(*req.Temperature)
	}
	if req.MaxTokens != nil {
		completionOpts.MaxTokens = int32(*req.MaxTokens)
	}
	if req.TopP != nil {
		completionOpts.TopP = float32(*req.TopP)
	}

	// Build request
	pbReq := &pb.CompletionRequest{
		ModelUri:          req.Model,
		CompletionOptions: completionOpts,
		Messages:          messages,
	}
	if req.JSONMode {
		pbReq.ResponseFormat = &pb.CompletionRequest_JsonObject{JsonObject: true}
	}

	// Add authorization metadata
	ctx = metadata.AppendToOutgoingContext(ctx,
		"authorization", fmt.Sprintf("Bearer %s", p.apiKey),
		"x-folder-id", p.folderID,
	)

	// Create stream
	stream, err := client.Completion(ctx, pbReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create completion stream: %w", err)
	}

	// Collect all responses
	var fullContent string
	var totalInputTokens int64
	var totalOutputTokens int64

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to receive response: %w", err)
		}

		if resp == nil {
			continue
		}

		// Collect content from alternatives
		if len(resp.Alternatives) > 0 {
			fullContent += resp.Alternatives[0].Message
		}

		// Accumulate usage
		if resp.Usage != nil {
			totalInputTokens += resp.Usage.InputTokens
			totalOutputTokens += resp.Usage.OutputTokens
		}
	}

	return &core.ChatResponse{
		Content: fullContent,
		Model:   req.Model,
		Usage: &core.Usage{
			InputTokens:  int(totalInputTokens),
			OutputTokens: int(totalOutputTokens),
			TotalTokens:  int(totalInputTokens + totalOutputTokens),
		},
	}, nil
}

// StreamChatCompletion implements LLMProvider.StreamChatCompletion
func (p *Protocol) StreamChatCompletion(ctx context.Context, req core.ChatRequest) (core.ChatStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if !p.SupportsCapability(core.CapabilityLLM) {
		return nil, fmt.Errorf("LLM capability not supported by this provider")
	}

	// Create gRPC connection
	conn, err := p.createGRPCConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	client := pb.NewTextGenerationServiceClient(conn)

	// Convert messages
	messages := make([]*pb.Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = &pb.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Build completion options
	completionOpts := &pb.CompletionOptions{}
	if req.Temperature != nil {
		completionOpts.Temperature = float32(*req.Temperature)
	}
	if req.MaxTokens != nil {
		completionOpts.MaxTokens = int32(*req.MaxTokens)
	}
	if req.TopP != nil {
		completionOpts.TopP = float32(*req.TopP)
	}

	// Build request
	pbReq := &pb.CompletionRequest{
		ModelUri:          req.Model,
		CompletionOptions: completionOpts,
		Messages:          messages,
	}
	if req.JSONMode {
		pbReq.ResponseFormat = &pb.CompletionRequest_JsonObject{JsonObject: true}
	}

	// Add authorization metadata
	ctx = metadata.AppendToOutgoingContext(ctx,
		"authorization", fmt.Sprintf("Bearer %s", p.apiKey),
		"x-folder-id", p.folderID,
	)

	// Create stream
	stream, err := client.Completion(ctx, pbReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create completion stream: %w", err)
	}

	return &chatStream{
		stream: stream,
		conn:   conn,
	}, nil
}

// chatStream implements core.ChatStream for Yandex gRPC
type chatStream struct {
	stream pb.TextGenerationService_CompletionClient
	conn   *grpc.ClientConn
}

func (s *chatStream) Receive(ctx context.Context) (*core.ChatChunk, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		if err == io.EOF {
			return &core.ChatChunk{Done: true}, nil
		}
		return nil, fmt.Errorf("failed to receive chunk: %w", err)
	}

	if resp == nil {
		return &core.ChatChunk{Done: false}, nil
	}

	var content string
	var finishReason string

	if len(resp.Alternatives) > 0 {
		content = resp.Alternatives[0].Message
		finishReason = resp.Alternatives[0].FinishReason
	}

	return &core.ChatChunk{
		Content:      content,
		Done:         false,
		FinishReason: finishReason,
	}, nil
}

func (s *chatStream) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// createGRPCConnection creates a gRPC connection to Yandex API
func (p *Protocol) createGRPCConnection(ctx context.Context) (*grpc.ClientConn, error) {
	// Use TLS for secure connection
	creds := credentials.NewClientTLSFromCert(nil, "")

	conn, err := grpc.DialContext(
		ctx,
		"llm.api.cloud.yandex.net:50051",
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server: %w", err)
	}

	return conn, nil
}
