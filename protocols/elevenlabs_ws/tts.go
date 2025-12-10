package elevenlabs_ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/creastat/infra/telemetry"
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
	if err := stream.Send(ctx, req.Text); err != nil {
		return nil, err
	}

	// Finish stream to signal end of input
	if err := stream.Send(ctx, ""); err != nil {
		return nil, err
	}

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
	voiceID := req.Voice
	if voiceID == "" {
		voiceID = "21m00Tcm4TlvDq8ikWAM" // Default voice (Rachel)
	}
	modelID := "eleven_monolingual_v1"
	if req.Model != "" {
		modelID = req.Model
	}

	// Construct WebSocket URL
	// wsURL := fmt.Sprintf("wss://api.elevenlabs.io/v1/text-to-speech/%s/stream-input?model_id=%s&language_code=%s", voiceID, modelID, req.Language)
	wsURL := fmt.Sprintf("wss://api.elevenlabs.io/v1/text-to-speech/%s/stream-input?output_format=pcm_24000", voiceID)

	if p.logger != nil {
		if logger, ok := p.logger.(telemetry.Logger); ok {
			logger.Trace("ElevenLabs TTS WebSocket connection",
				telemetry.String("url", wsURL),
				telemetry.String("voice_id", voiceID),
				telemetry.String("model_id", modelID),
				telemetry.String("language", req.Language))
		}
	}

	dialer := websocket.DefaultDialer
	header := make(map[string][]string)
	header["xi-api-key"] = []string{p.apiKey}

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ElevenLabs: %w", err)
	}

	client := &elevenlabsTTSStream{
		conn:    conn,
		audioCh: make(chan []byte, 10),
		errCh:   make(chan error, 1),
		doneCh:  make(chan struct{}),
		logger:  p.logger,
		bosSent: false,
	}

	// Send BOS (Beginning of Stream) message
	if err := client.sendBOS(); err != nil {
		conn.Close()
		return nil, err
	}

	go client.readMessages()

	return client, nil
}

// elevenlabsTTSStream implements core.TTSStream
type elevenlabsTTSStream struct {
	conn    *websocket.Conn
	audioCh chan []byte
	errCh   chan error
	doneCh  chan struct{}
	mu      sync.Mutex
	closed  bool
	logger  any // telemetry.Logger
	bosSent bool
}

func (c *elevenlabsTTSStream) sendBOS() error {
	// BOS message is required by ElevenLabs to start the stream
	msg := map[string]any{
		"text":                   " ",
		"try_trigger_generation": false, // Don't generate audio for the initialization space
	}

	if logger, ok := c.logger.(telemetry.Logger); ok {
		logger.Trace("Sending BOS to ElevenLabs", telemetry.Any("msg", msg))
	}

	return c.conn.WriteJSON(msg)
}

func (c *elevenlabsTTSStream) Send(ctx context.Context, text string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("stream closed")
	}
	c.mu.Unlock()

	// ElevenLabs expects JSON messages with "text" field
	// It also supports "try_trigger_generation" and "flush".
	// For streaming, we usually want to try triggering generation.
	msg := map[string]any{
		"text":                   text,
		"try_trigger_generation": true,
	}

	return c.conn.WriteJSON(msg)
}

func (c *elevenlabsTTSStream) Finish(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("stream closed")
	}
	c.mu.Unlock()

	// Send connection close message (empty string)
	msg := map[string]any{
		"text": "",
	}
	return c.conn.WriteJSON(msg)
}

func (c *elevenlabsTTSStream) Receive(ctx context.Context) (*core.TTSChunk, error) {
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

func (c *elevenlabsTTSStream) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	close(c.doneCh)
	return c.conn.Close()
}

func (c *elevenlabsTTSStream) readMessages() {
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
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					select {
					case c.errCh <- fmt.Errorf("read error: %w", err):
					default:
					}
				}
				// If normal closure, we just finish
				c.Close()
			}
			return
		}

		var response map[string]any
		if err := json.Unmarshal(message, &response); err != nil {
			// If it's not JSON, might be raw error or something, but ElevenLabs sends JSON
			continue
		}

		// Check for audio data
		if audioBase64, ok := response["audio"].(string); ok && audioBase64 != "" {
			audioData, err := base64.StdEncoding.DecodeString(audioBase64)
			if err != nil {
				if logger, ok := c.logger.(telemetry.Logger); ok {
					logger.Error("Failed to decode audio base64", telemetry.Err(err))
				}
				continue
			}

			select {
			case c.audioCh <- audioData:
			case <-c.doneCh:
				return
			}
		}

		// Check for final message
		if isFinal, ok := response["isFinal"].(bool); ok && isFinal {
			// ElevenLabs sends `isFinal: true` when generation is done for the request.
			// Close audio channel to signal end of stream to the client
			c.mu.Lock()
			if !c.closed {
				close(c.audioCh)
			}
			c.mu.Unlock()
			c.Close()
			return
		}

		// Check for error
		if errorMsg, ok := response["error"].(string); ok {
			select {
			case c.errCh <- fmt.Errorf("ElevenLabs error: %s", errorMsg):
			default:
			}
			c.Close()
			return
		}
	}
}
