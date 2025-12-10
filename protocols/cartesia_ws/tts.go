package cartesia_ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/creastat/providers/core"
	"github.com/gorilla/websocket"
)

// Synthesize implements TTSProvider.Synthesize (one-shot)
func (p *Protocol) Synthesize(ctx context.Context, req core.TTSRequest) (*core.TTSResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	stream, err := p.StreamSynthesize(ctx, req)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	// Send text
	stream.Send(ctx, req.Text)

	// Collect audio
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

// StreamSynthesize implements TTSProvider.StreamSynthesize
func (p *Protocol) StreamSynthesize(ctx context.Context, req core.TTSRequest) (core.TTSStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	// Set defaults
	voice := req.Voice
	if voice == "" {
		voice = "79a125e8-cd45-4c13-8a67-188112f4dd22" // Default voice
	}
	model := "sonic-english"
	if req.Model != "" {
		model = req.Model
	}

	// Connect to WebSocket
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
		audioCh: make(chan []byte, 10),
		errCh:   make(chan error, 1),
		doneCh:  make(chan struct{}),
	}

	go client.readMessages()

	return client, nil
}

// cartesiaTTSStream implements core.TTSStream
type cartesiaTTSStream struct {
	conn    *websocket.Conn
	voice   string
	model   string
	audioCh chan []byte
	errCh   chan error
	doneCh  chan struct{}
	mu      sync.Mutex
	closed  bool
}

func (c *cartesiaTTSStream) Send(ctx context.Context, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("stream closed")
	}

	req := map[string]any{
		"model_id":      c.model,
		"voice":         map[string]string{"mode": "id", "id": c.voice},
		"transcript":    text,
		"output_format": map[string]any{"container": "raw", "encoding": "pcm_f32le", "sample_rate": 22050},
		"context_id":    "default",
	}

	return c.conn.WriteJSON(req)
}

func (c *cartesiaTTSStream) Receive(ctx context.Context) (*core.TTSChunk, error) {
	select {
	case audio, ok := <-c.audioCh:
		if !ok {
			return &core.TTSChunk{Done: true}, nil
		}
		return &core.TTSChunk{Audio: audio, Done: false}, nil
	case err := <-c.errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *cartesiaTTSStream) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.doneCh)
	return c.conn.Close()
}

func (c *cartesiaTTSStream) readMessages() {
	defer func() {
		c.mu.Lock()
		if !c.closed {
			close(c.audioCh)
		}
		c.mu.Unlock()
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
				c.Close()
			}
			return
		}

		var rawResult map[string]any
		if err := json.Unmarshal(message, &rawResult); err != nil {
			continue
		}

		// Handle different message types
		if data, ok := rawResult["data"].(string); ok && data != "" {
			// Decode base64 audio
			audioData, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				continue
			}

			select {
			case c.audioCh <- audioData:
			case <-c.doneCh:
				return
			}
		}

		// Check for done
		if done, ok := rawResult["done"].(bool); ok && done {
			c.Close()
			return
		}
	}
}
