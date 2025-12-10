package core

import "context"

// STTProvider provides speech-to-text capabilities
type STTProvider interface {
	Provider
	Transcribe(ctx context.Context, req STTRequest) (*STTResponse, error)
	StreamTranscribe(ctx context.Context, req STTRequest) (STTStream, error)
}

// STTRequest represents a speech-to-text request
type STTRequest struct {
	Model      string
	Audio      []byte // For one-shot transcription
	Language   string
	SampleRate int
	Encoding   string
	Keyterms   []string           // STT keyterm prompting for improved accuracy
	Options    map[string]any     // Provider-specific options
}

// STTResponse represents a speech-to-text response
type STTResponse struct {
	Text  string
	Model string
}

// STTStream represents a streaming speech-to-text session
type STTStream interface {
	Send(ctx context.Context, audio []byte) error
	Receive(ctx context.Context) (*STTChunk, error)
	Close() error
}

// STTChunk represents a chunk of streaming transcription
type STTChunk struct {
	Text       string
	Done       bool
	IsFinal    bool
	Confidence float64
}
