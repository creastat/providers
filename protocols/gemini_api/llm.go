package gemini_api

import (
	"context"
	"fmt"
	"iter"

	"github.com/madmike/go-ai-providers/core"
	"google.golang.org/genai"
)

// ChatCompletion implements LLMProvider.ChatCompletion
func (p *Protocol) ChatCompletion(ctx context.Context, req core.ChatRequest) (*core.ChatResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if req.Model == "" {
		req.Model = "gemini-2.0-flash-exp"
	}

	// Convert messages to Gemini format
	contents := genai.Text(req.Messages[len(req.Messages)-1].Content)

	// Build config
	config := &genai.GenerateContentConfig{}

	// Set temperature if provided
	if req.Temperature != nil {
		temp := float32(*req.Temperature)
		config.Temperature = &temp
	}

	// Set max tokens if provided
	if req.MaxTokens != nil {
		maxTokens := int32(*req.MaxTokens)
		config.MaxOutputTokens = maxTokens
	}

	// Set top P if provided
	if req.TopP != nil {
		topP := float32(*req.TopP)
		config.TopP = &topP
	}

	if req.JSONMode {
		config.ResponseMIMEType = "application/json"
	}

	// Generate response
	resp, err := p.client.Models.GenerateContent(ctx, req.Model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	// Extract text from first candidate
	candidate := resp.Candidates[0]
	var content string
	if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
		if part := candidate.Content.Parts[0]; part != nil {
			content = part.Text
		}
	}

	// Calculate usage
	usage := &core.Usage{
		InputTokens:  int(resp.UsageMetadata.PromptTokenCount),
		OutputTokens: int(resp.UsageMetadata.CandidatesTokenCount),
		TotalTokens:  int(resp.UsageMetadata.TotalTokenCount),
	}

	return &core.ChatResponse{
		Content: content,
		Model:   req.Model,
		Usage:   usage,
	}, nil
}

// StreamChatCompletion implements LLMProvider.StreamChatCompletion
func (p *Protocol) StreamChatCompletion(ctx context.Context, req core.ChatRequest) (core.ChatStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if req.Model == "" {
		req.Model = "gemini-2.0-flash-exp"
	}

	// Convert messages to Gemini format
	contents := genai.Text(req.Messages[len(req.Messages)-1].Content)

	// Build config
	config := &genai.GenerateContentConfig{}

	// Set temperature if provided
	if req.Temperature != nil {
		temp := float32(*req.Temperature)
		config.Temperature = &temp
	}

	// Set max tokens if provided
	if req.MaxTokens != nil {
		maxTokens := int32(*req.MaxTokens)
		config.MaxOutputTokens = maxTokens
	}

	// Set top P if provided
	if req.TopP != nil {
		topP := float32(*req.TopP)
		config.TopP = &topP
	}

	if req.JSONMode {
		config.ResponseMIMEType = "application/json"
	}

	// Start streaming
	streamIter := p.client.Models.GenerateContentStream(ctx, req.Model, contents, config)

	return &geminiChatStream{
		iter: streamIter,
	}, nil
}

// geminiChatStream implements core.ChatStream for Gemini
type geminiChatStream struct {
	iter iter.Seq2[*genai.GenerateContentResponse, error]
}

// Receive receives the next chunk from the stream
func (s *geminiChatStream) Receive(ctx context.Context) (*core.ChatChunk, error) {
	// Get next response from iterator
	for resp, err := range s.iter {
		if err != nil {
			return nil, err
		}

		if resp == nil {
			return &core.ChatChunk{Done: true}, nil
		}

		var content string
		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			if len(resp.Candidates[0].Content.Parts) > 0 && resp.Candidates[0].Content.Parts[0] != nil {
				content = resp.Candidates[0].Content.Parts[0].Text
			}
		}

		finishReason := ""
		if len(resp.Candidates) > 0 {
			finishReason = string(resp.Candidates[0].FinishReason)
		}

		return &core.ChatChunk{
			Content:      content,
			Done:         false,
			FinishReason: finishReason,
		}, nil
	}

	return &core.ChatChunk{Done: true}, nil
}

// Close closes the stream
func (s *geminiChatStream) Close() error {
	return nil
}
