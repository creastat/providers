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
	Keyterms   []string       // STT keyterm prompting for improved accuracy
	Options    map[string]any // Provider-specific options
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

// FinalizableSTTStream is an optional capability some providers implement
// (e.g. Deepgram). Finalize flushes any in-flight audio server-side and
// forces an immediate is_final response for the current segment, without
// closing the stream. Use it when the caller has its own end-of-utterance
// signal (local VAD) and wants to bypass the provider's own endpointing
// latency. Callers should type-assert and fall back gracefully when the
// underlying stream doesn't implement this interface.
type FinalizableSTTStream interface {
	STTStream
	Finalize(ctx context.Context) error
}

// STTChunk represents a chunk of streaming transcription
type STTChunk struct {
	Text       string
	Done       bool
	IsFinal    bool
	Confidence float64
	EventType  string // "", "speech_started", "utterance_end"
}
