package cartesia_ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/madmike/go-ai-providers/core"
)

// defaultContextID is used by one-shot callers and by legacy Send() callers
// that don't track turn IDs. Never use it on a persistent stream where
// barge-in cancellation is expected — see SendWithContext / Cancel.
const defaultContextID = "default"

// Synthesize implements TTSProvider.Synthesize (one-shot).
func (p *Protocol) Synthesize(ctx context.Context, req core.TTSRequest) (*core.TTSResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	stream, err := p.StreamSynthesize(ctx, req)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	if err := stream.Send(ctx, req.Text); err != nil {
		return nil, err
	}

	var audioData []byte
	for {
		chunk, err := stream.Receive(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if chunk.Done {
			break
		}
		audioData = append(audioData, chunk.Audio...)
	}

	return &core.TTSResponse{Audio: audioData, Model: req.Model}, nil
}

// StreamSynthesize implements TTSProvider.StreamSynthesize.
func (p *Protocol) StreamSynthesize(ctx context.Context, req core.TTSRequest) (core.TTSStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	voice := req.Voice
	if voice == "" {
		voice = "e07c00bc-4134-4eae-9ea4-1a55fb45746b"
	}
	model := "sonic-3"
	if req.Model != "" {
		model = req.Model
	}

	wsURL := fmt.Sprintf("wss://api.cartesia.ai/tts/websocket?api_key=%s&cartesia_version=2024-06-10", p.apiKey)
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Cartesia: %w", err)
	}

	client := &cartesiaTTSStream{
		conn:    conn,
		voice:   voice,
		model:   model,
		audioCh: make(chan cartesiaFrame, 16),
		errCh:   make(chan error, 1),
		doneCh:  make(chan struct{}),
	}

	if fmtOpts, ok := req.Options["output_format"].(map[string]any); ok {
		client.outputFormat = fmtOpts
	}

	go client.readMessages()

	return client, nil
}

// cartesiaFrame carries an audio chunk together with the Cartesia context_id
// ("turn") it belongs to, so callers can drop chunks for cancelled turns.
type cartesiaFrame struct {
	Audio  []byte
	TurnID string
	Done   bool
}

// cartesiaTTSStream implements core.TTSStream and core.CancelableTTSStream.
type cartesiaTTSStream struct {
	conn    *websocket.Conn
	voice   string
	model   string
	audioCh chan cartesiaFrame
	errCh   chan error
	doneCh  chan struct{}

	mu           sync.Mutex
	closed       bool
	outputFormat map[string]any
}

// Send is the legacy one-shot entrypoint. It uses a fixed context_id and is
// NOT safe on a persistent stream that needs barge-in cancellation. Callers
// running a persistent stream must use SendWithContext + Cancel instead.
func (c *cartesiaTTSStream) Send(ctx context.Context, text string) error {
	return c.SendWithContext(ctx, text, defaultContextID)
}

// SendWithContext sends a synthesis request tagged with turnID. The caller
// is expected to correlate returned TTSChunk.TurnID against this turnID.
func (c *cartesiaTTSStream) SendWithContext(ctx context.Context, text, turnID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("stream closed")
	}

	outputFormat := c.outputFormat
	if outputFormat == nil {
		outputFormat = map[string]any{"container": "raw", "encoding": "pcm_f32le", "sample_rate": 22050}
	}

	req := map[string]any{
		"model_id":      c.model,
		"voice":         map[string]string{"mode": "id", "id": c.voice},
		"transcript":    text,
		"output_format": outputFormat,
		"context_id":    turnID,
	}

	return c.conn.WriteJSON(req)
}

// Cancel aborts synthesis for turnID without closing the underlying socket.
// Safe to call when no synthesis is in flight for that turnID.
func (c *cartesiaTTSStream) Cancel(ctx context.Context, turnID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	msg := map[string]any{
		"context_id": turnID,
		"cancel":     true,
	}
	return c.conn.WriteJSON(msg)
}

// Receive pulls the next audio chunk from the stream. The returned chunk's
// TurnID is the Cartesia context_id that produced it.
func (c *cartesiaTTSStream) Receive(ctx context.Context) (*core.TTSChunk, error) {
	select {
	case frame, ok := <-c.audioCh:
		if !ok {
			return &core.TTSChunk{Done: true}, nil
		}
		return &core.TTSChunk{
			Audio:  frame.Audio,
			Done:   frame.Done,
			TurnID: frame.TurnID,
		}, nil
	case err := <-c.errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *cartesiaTTSStream) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	close(c.doneCh)
	return c.conn.Close()
}

func (c *cartesiaTTSStream) readMessages() {
	defer func() {
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if !closed {
			close(c.audioCh)
		}
	}()

	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			wasClosed := c.closed
			c.mu.Unlock()

			if !wasClosed {
				select {
				case c.errCh <- fmt.Errorf("read error: %w", err):
				default:
				}
				_ = c.Close()
			}
			return
		}

		var rawResult map[string]any
		if err := json.Unmarshal(message, &rawResult); err != nil {
			continue
		}

		if errMsg, ok := rawResult["error"].(string); ok && errMsg != "" {
			select {
			case c.errCh <- fmt.Errorf("Cartesia error: %s", errMsg):
			default:
			}
			continue
		}

		turnID, _ := rawResult["context_id"].(string)

		if data, ok := rawResult["data"].(string); ok && data != "" {
			audioData, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				continue
			}

			select {
			case c.audioCh <- cartesiaFrame{Audio: audioData, TurnID: turnID}:
			case <-c.doneCh:
				return
			}
		}

		if done, ok := rawResult["done"].(bool); ok && done {
			// Emit a per-turn Done marker so the caller can finalize this
			// turn without waiting for the socket to close.
			select {
			case c.audioCh <- cartesiaFrame{Done: true, TurnID: turnID}:
			case <-c.doneCh:
				return
			}
			// Do not return — the socket stays open for subsequent turns.
		}
	}
}
