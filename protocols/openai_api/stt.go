package openai_api

import (
	"context"
	"fmt"

	"github.com/madmike/go-ai-providers/core"
	"github.com/sashabaranov/go-openai"
)

// Transcribe implements STTProvider.Transcribe
func (p *Protocol) Transcribe(ctx context.Context, req core.STTRequest) (*core.STTResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if len(req.Audio) == 0 {
		return nil, fmt.Errorf("audio data is required")
	}

	// Create OpenAI client
	config := openai.DefaultConfig(p.apiKey)
	config.BaseURL = p.baseURL
	client := openai.NewClientWithConfig(config)

	// Build request
	audioReq := openai.AudioRequest{
		Model:    req.Model,
		FilePath: "audio.wav", // Placeholder, actual audio is in Reader
		Reader:   nil,         // Will be set from req.Audio
		Language: req.Language,
	}

	// Note: The go-openai library expects a file or reader
	// For now, this is a placeholder implementation

	resp, err := client.CreateTranscription(ctx, audioReq)
	if err != nil {
		return nil, fmt.Errorf("transcription failed: %w", err)
	}

	return &core.STTResponse{
		Text:  resp.Text,
		Model: req.Model,
	}, nil
}

// StreamTranscribe implements STTProvider.StreamTranscribe
func (p *Protocol) StreamTranscribe(ctx context.Context, req core.STTRequest) (core.STTStream, error) {
	// OpenAI Whisper API doesn't support streaming
	return nil, fmt.Errorf("streaming transcription not supported by OpenAI Whisper API")
}
