package deepgram_ws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/madmike/go-ai-providers/core"
	"github.com/madmike/go-infra/telemetry"
)

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
		audioData = append(audioData, chunk.Audio...)
		if chunk.Done {
			break
		}
	}

	return &core.TTSResponse{Audio: audioData, Model: req.Model}, nil
}

// StreamSynthesize implements TTSProvider.StreamSynthesize.
//
// Deepgram's TTS WebSocket protocol:
//   - Connection params (model, encoding, sample_rate) are URL query args.
//   - Client sends JSON control messages: {"type":"Speak","text":...},
//     {"type":"Flush"}, {"type":"Clear"}, {"type":"Close"}.
//   - Server sends binary frames for audio and JSON for metadata/events
//     (Metadata, Flushed, Cleared, Warning).
//
// Unlike Cartesia, Deepgram's protocol has no per-request correlation id:
// one connection has one queue. We still implement CancelableTTSStream by
// tagging audio with the caller's turnID at the stream wrapper layer —
// since the pipeline serialises one sentence at a time per TTS stream,
// this is unambiguous in practice. Cancel maps to a Clear message which
// wipes all buffered audio server-side.
func (p *Protocol) StreamSynthesize(ctx context.Context, req core.TTSRequest) (core.TTSStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	model := req.Model
	if req.Voice != "" {
		if model != "" {
			model = model + "-" + req.Voice
		} else {
			model = req.Voice
		}
	}
	if model == "" {
		model = "aura-2-thalia-en"
	}

	// Pull desired output format out of req.Options if the caller set one;
	// otherwise default to 16kHz linear16 (Deepgram accepts 24k as its
	// native default, but voice agents almost always want to match the
	// call's sample rate, so we key off caller-supplied values).
	encoding := "linear16"
	sampleRate := 24000
	if fmtOpts, ok := req.Options["output_format"].(map[string]any); ok {
		if enc, ok := fmtOpts["encoding"].(string); ok && enc != "" {
			encoding = mapEncoding(enc)
		}
		switch v := fmtOpts["sample_rate"].(type) {
		case int:
			if v > 0 {
				sampleRate = v
			}
		case int64:
			if v > 0 {
				sampleRate = int(v)
			}
		case float64:
			if v > 0 {
				sampleRate = int(v)
			}
		}
	}

	u, _ := url.Parse("wss://api.deepgram.com/v1/speak")
	q := u.Query()
	q.Set("model", model)
	q.Set("encoding", encoding)
	q.Set("sample_rate", fmt.Sprintf("%d", sampleRate))
	u.RawQuery = q.Encode()

	if p.apiKey == "" {
		return nil, fmt.Errorf("Deepgram API key is not set. Please set DEEPGRAM_API_KEY environment variable")
	}

	p.logger.Info("Connecting to Deepgram TTS",
		telemetry.String("model", model),
		telemetry.String("encoding", encoding),
		telemetry.Int("sample_rate", sampleRate),
		telemetry.String("url", u.String()))

	dialer := websocket.DefaultDialer
	header := make(map[string][]string)
	header["Authorization"] = []string{fmt.Sprintf("Token %s", p.apiKey)}

	conn, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("failed to connect to Deepgram TTS (HTTP %d): %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("failed to connect to Deepgram TTS: %w", err)
	}

	s := &deepgramTTSStream{
		conn:    conn,
		audioCh: make(chan deepgramTTSFrame, 16),
		errCh:   make(chan error, 1),
		doneCh:  make(chan struct{}),
		logger:  p.logger,
	}

	go s.readMessages()

	return s, nil
}

// mapEncoding normalises the caller-supplied encoding hint (which follows
// Cartesia's vocabulary — pcm_s16le / pcm_mulaw / pcm_alaw) into the tokens
// Deepgram expects on the URL.
func mapEncoding(in string) string {
	switch in {
	case "pcm_s16le", "linear16", "pcm16":
		return "linear16"
	case "pcm_mulaw", "mulaw", "ulaw":
		return "mulaw"
	case "pcm_alaw", "alaw":
		return "alaw"
	default:
		// Pass through anything we don't recognise — the caller may be
		// using a Deepgram-native value already.
		return in
	}
}

type deepgramTTSFrame struct {
	Audio  []byte
	TurnID string
	Done   bool
}

type deepgramTTSStream struct {
	conn    *websocket.Conn
	audioCh chan deepgramTTSFrame
	errCh   chan error
	doneCh  chan struct{}
	logger  telemetry.Logger

	mu            sync.Mutex
	closed        bool
	currentTurnID string
}

// Send is the legacy one-shot entrypoint. It wraps the Speak+Flush pair
// under a fixed turnID. Prefer SendWithContext on persistent streams.
func (c *deepgramTTSStream) Send(ctx context.Context, text string) error {
	return c.SendWithContext(ctx, text, "default")
}

// SendWithContext sends a Speak + Flush for the given text tagged with
// turnID. Any audio frames returned by the server until the next Clear /
// SendWithContext are tagged with this turnID so callers can drop stale
// audio from a cancelled turn.
func (c *deepgramTTSStream) SendWithContext(ctx context.Context, text, turnID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("stream closed")
	}
	c.currentTurnID = turnID

	speak := map[string]any{"type": "Speak", "text": text}
	if err := c.conn.WriteJSON(speak); err != nil {
		return fmt.Errorf("write Speak: %w", err)
	}
	// Flush tells Deepgram to emit the final audio for the current queued
	// text; without it a short utterance can stay buffered waiting for
	// more text.
	if err := c.conn.WriteJSON(map[string]any{"type": "Flush"}); err != nil {
		return fmt.Errorf("write Flush: %w", err)
	}
	return nil
}

// Cancel aborts any audio currently being generated / buffered on the
// server by sending a Clear message. Safe to call when no synthesis is in
// flight (Deepgram will just ack with a Cleared event). Any audio already
// enqueued locally in audioCh is left tagged with the previous turnID so
// the caller's barge-in filter can drop it.
func (c *deepgramTTSStream) Cancel(ctx context.Context, turnID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.currentTurnID = ""
	return c.conn.WriteJSON(map[string]any{"type": "Clear"})
}

func (c *deepgramTTSStream) Receive(ctx context.Context) (*core.TTSChunk, error) {
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
	case <-c.doneCh:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *deepgramTTSStream) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// Best-effort graceful close — tell the server we're done before
	// yanking the socket so it doesn't complain about an abrupt shutdown.
	_ = c.conn.WriteJSON(map[string]any{"type": "Close"})

	// Give the read loop a brief moment to drain any trailing frames.
	select {
	case <-c.doneCh:
	case <-time.After(200 * time.Millisecond):
	}

	_ = c.conn.SetReadDeadline(time.Now())
	return c.conn.Close()
}

func (c *deepgramTTSStream) readMessages() {
	defer close(c.doneCh)

	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			wasClosed := c.closed
			c.mu.Unlock()
			if !wasClosed && !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				select {
				case c.errCh <- fmt.Errorf("deepgram TTS read: %w", err):
				default:
				}
			}
			return
		}

		switch messageType {
		case websocket.BinaryMessage:
			c.mu.Lock()
			turnID := c.currentTurnID
			c.mu.Unlock()
			select {
			case c.audioCh <- deepgramTTSFrame{Audio: message, TurnID: turnID}:
			case <-c.doneCh:
				return
			}

		case websocket.TextMessage:
			var evt map[string]any
			if err := json.Unmarshal(message, &evt); err != nil {
				continue
			}
			msgType, _ := evt["type"].(string)
			switch msgType {
			case "Flushed":
				// End of audio for the current Speak batch — emit a Done
				// marker tagged with the turn we're currently on so the
				// caller can finalise the sentence without closing the
				// socket.
				c.mu.Lock()
				turnID := c.currentTurnID
				c.mu.Unlock()
				select {
				case c.audioCh <- deepgramTTSFrame{Done: true, TurnID: turnID}:
				case <-c.doneCh:
					return
				}
			case "Warning":
				desc, _ := evt["description"].(string)
				code, _ := evt["code"].(string)
				c.logger.Warn("Deepgram TTS warning",
					telemetry.String("code", code),
					telemetry.String("description", desc))
			case "Metadata", "Cleared":
				// Informational — nothing for the caller to act on.
			}
		}
	}
}
