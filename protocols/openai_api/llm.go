package openai_api

import (
	"context"
	"fmt"
	"io"

	"github.com/creastat/providers/core"
	"github.com/sashabaranov/go-openai"
)

// ChatCompletion implements LLMProvider.ChatCompletion
func (p *Protocol) ChatCompletion(ctx context.Context, req core.ChatRequest) (*core.ChatResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	// Create OpenAI client
	config := openai.DefaultConfig(p.apiKey)
	config.BaseURL = p.baseURL
	client := openai.NewClientWithConfig(config)

	// Convert messages
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Build request
	chatReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
	}

	// Apply options
	if req.Temperature != nil {
		chatReq.Temperature = float32(*req.Temperature)
	}
	if req.MaxTokens != nil {
		chatReq.MaxTokens = *req.MaxTokens
	}
	if req.TopP != nil {
		chatReq.TopP = float32(*req.TopP)
	}

	// Make request
	resp, err := client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from model")
	}

	// Build response
	return &core.ChatResponse{
		Content: resp.Choices[0].Message.Content,
		Model:   resp.Model,
		Usage: &core.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}, nil
}

// StreamChatCompletion implements LLMProvider.StreamChatCompletion
func (p *Protocol) StreamChatCompletion(ctx context.Context, req core.ChatRequest) (core.ChatStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	// Create OpenAI client
	config := openai.DefaultConfig(p.apiKey)
	config.BaseURL = p.baseURL
	client := openai.NewClientWithConfig(config)

	// Convert messages
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Build request
	chatReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   true,
	}

	// Apply options
	if req.Temperature != nil {
		chatReq.Temperature = float32(*req.Temperature)
	}
	if req.MaxTokens != nil {
		chatReq.MaxTokens = *req.MaxTokens
	}
	if req.TopP != nil {
		chatReq.TopP = float32(*req.TopP)
	}

	// Create stream
	stream, err := client.CreateChatCompletionStream(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	return &chatStream{stream: stream}, nil
}

// chatStream implements core.ChatStream
type chatStream struct {
	stream *openai.ChatCompletionStream
}

func (s *chatStream) Receive(ctx context.Context) (*core.ChatChunk, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		if err == io.EOF {
			return &core.ChatChunk{Done: true}, nil
		}
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return &core.ChatChunk{Done: false}, nil
	}

	choice := resp.Choices[0]
	return &core.ChatChunk{
		Content:      choice.Delta.Content,
		Done:         false,
		FinishReason: string(choice.FinishReason),
	}, nil
}

func (s *chatStream) Close() error {
	s.stream.Close()
	return nil
}
