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

// CancelableTTSStream is an optional capability that lets callers reuse a
// single streaming connection across multiple utterances ("turns") and cancel
// an in-flight synthesis without closing the underlying transport. Providers
// whose server protocol supports per-request correlation (e.g. Cartesia's
// context_id) may implement this; callers must type-assert.
type CancelableTTSStream interface {
	TTSStream
	// SendWithContext sends text tagged with a caller-chosen turnID. The
	// provider will echo the same turnID on TTSChunk.TurnID for audio that
	// belongs to this request so the caller can drop late audio from a
	// cancelled turn.
	SendWithContext(ctx context.Context, text, turnID string) error
	// Cancel aborts synthesis for the given turnID without closing the
	// stream. Implementations should be safe to call even if no synthesis
	// is in flight for that turnID.
	Cancel(ctx context.Context, turnID string) error
}

// TTSChunk represents a chunk of streaming audio
type TTSChunk struct {
	Audio []byte
	Done  bool
	// TurnID correlates the chunk back to the SendWithContext turn that
	// produced it. Empty for providers that do not implement
	// CancelableTTSStream.
	TurnID string
}
