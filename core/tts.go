package core

import "context"

// TTSProvider provides text-to-speech capabilities
type TTSProvider interface {
	Provider
	Synthesize(ctx context.Context, req TTSRequest) (*TTSResponse, error)
	StreamSynthesize(ctx context.Context, req TTSRequest) (TTSStream, error)
}

// TTSRequest represents a text-to-speech request
type TTSRequest struct {
	Model    string
	Text     string // For one-shot synthesis
	Voice    string
	Language string
	Speed    *float64
	Options  map[string]any // Provider-specific options
}

// TTSResponse represents a text-to-speech response
type TTSResponse struct {
	Audio []byte
	Model string
}

// TTSStream represents a streaming text-to-speech session
type TTSStream interface {
	Send(ctx context.Context, text string) error
	Receive(ctx context.Context) (*TTSChunk, error)
	Close() error
}

// TTSChunk represents a chunk of streaming audio
type TTSChunk struct {
	Audio []byte
	Done  bool
}
