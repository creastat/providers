package minimax_ws

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/madmike/go-ai-providers/core"
	"github.com/madmike/go-infra/telemetry"
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
	_ = stream.Send(ctx, req.Text)

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
		voice = "presenter_male"
	}
	model := "speech-2.6-hd"
	if req.Model != "" {
		model = req.Model
	}
	speed := 1.0
	if req.Speed != nil {
		speed = *req.Speed
	}

	// Connect to WebSocket
	wsURL := "wss://api.minimax.io/ws/v1/t2a_v2"
	dialer := websocket.DefaultDialer
	header := make(map[string][]string)
	header["Authorization"] = []string{fmt.Sprintf("Bearer %s", p.apiKey)}

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("emitting done eventailed to connect to Minimax: %w", err)
	}

	client := &minimaxTTSStream{
		conn:    conn,
		model:   model,
		voice:   voice,
		speed:   speed,
		audioCh: make(chan []byte, 10),
		errCh:   make(chan error, 1),
		doneCh:  make(chan struct{}),
		logger:  p.logger,
	}

	// Wait for connection success
	if err := client.waitForConnection(); err != nil {
		conn.Close()
		return nil, err
	}

	// Start task with raw PCM audio format
	if err := client.startTask(); err != nil {
		conn.Close()
		return nil, err
	}

	go client.readMessages()

	return client, nil
}

// minimaxTTSStream implements core.TTSStream
type minimaxTTSStream struct {
	conn    *websocket.Conn
	model   string
	voice   string
	speed   float64
	audioCh chan []byte
	errCh   chan error
	doneCh  chan struct{}
	mu      sync.Mutex
	closed  bool
	logger  any // telemetry.Logger
}

func (c *minimaxTTSStream) waitForConnection() error {
	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read connection message: %w", err)
	}

	if logger, ok := c.logger.(telemetry.Logger); ok {
		logger.Trace("Received message from Minimax", telemetry.String("message", string(message)))
	}

	var response map[string]any
	if err := json.Unmarshal(message, &response); err != nil {
		return fmt.Errorf("failed to parse connection message: %w", err)
	}

	if event, ok := response["event"].(string); !ok || event != "connected_success" {
		return fmt.Errorf("unexpected connection response: %v", response)
	}

	return nil
}

func (c *minimaxTTSStream) startTask() error {
	request := map[string]any{
		"event": "task_start",
		"model": c.model,
		"voice_setting": map[string]any{
			"voice_id": c.voice,
			"speed":    c.speed,
			"vol":      1.0,
		},
		"audio_setting": map[string]any{
			"sample_rate": 22050,
			"format":      "pcm",
			"channel":     1,
		},
	}

	if err := c.conn.WriteJSON(request); err != nil {
		return fmt.Errorf("failed to send task_start: %w", err)
	}

	// Wait for task_started
	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read task_started: %w", err)
	}

	var response map[string]any
	if err := json.Unmarshal(message, &response); err != nil {
		return fmt.Errorf("failed to parse task_started: %w", err)
	}

	if event, ok := response["event"].(string); !ok || event != "task_started" {
		return fmt.Errorf("unexpected task_started response: %v", response)
	}

	return nil
}

func (c *minimaxTTSStream) Send(ctx context.Context, text string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("stream closed")
	}
	c.mu.Unlock()

	request := map[string]any{
		"event": "task_continue",
		"text":  text,
	}

	return c.conn.WriteJSON(request)
}

func (c *minimaxTTSStream) Finish(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("stream closed")
	}
	c.mu.Unlock()

	// Send task_finish to signal no more text is coming
	// Don't hold mutex during network I/O to avoid deadlock
	finishMsg := map[string]any{"event": "task_finish"}
	return c.conn.WriteJSON(finishMsg)
}

func (c *minimaxTTSStream) Receive(ctx context.Context) (*core.TTSChunk, error) {
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

func (c *minimaxTTSStream) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	close(c.doneCh)
	return c.conn.Close()
}

// stripBinaryData recursively replaces binary data fields with "<binary>" placeholder
func (c *minimaxTTSStream) stripBinaryData(data any) {
	switch v := data.(type) {
	case map[string]any:
		for key, val := range v {
			// Check if this looks like binary data (hex strings, base64, etc.)
			if strVal, ok := val.(string); ok {
				// Common binary field names
				if key == "audio" || key == "data" || key == "binary" || key == "payload" {
					// Check if it looks like hex or base64
					if len(strVal) > 20 && (isHexString(strVal) || isBase64String(strVal)) {
						v[key] = "<binary>"
						continue
					}
				}
			}
			// Recursively process nested structures
			c.stripBinaryData(val)
		}
	case []any:
		for i, item := range v {
			c.stripBinaryData(item)
			if strVal, ok := item.(string); ok {
				if len(strVal) > 20 && (isHexString(strVal) || isBase64String(strVal)) {
					v[i] = "<binary>"
				}
			}
		}
	}
}

// isHexString checks if a string looks like hex-encoded data
func isHexString(s string) bool {
	if len(s) < 20 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// isBase64String checks if a string looks like base64-encoded data
func isBase64String(s string) bool {
	if len(s) < 20 {
		return false
	}
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=') {
			return false
		}
	}
	return true
}

func (c *minimaxTTSStream) readMessages() {
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

		// Log message (strip binary data if present)
		if logger, ok := c.logger.(telemetry.Logger); ok {
			var logMsg map[string]any
			if err := json.Unmarshal(message, &logMsg); err == nil {
				// Strip binary data from nested structures
				c.stripBinaryData(logMsg)
				// Re-marshal for clean logging with SetEscapeHTML(false) to avoid escaping
				buf := &strings.Builder{}
				encoder := json.NewEncoder(buf)
				encoder.SetEscapeHTML(false)
				if err := encoder.Encode(logMsg); err == nil {
					// Remove trailing newline added by Encoder
					payload := strings.TrimSuffix(buf.String(), "\n")
					logger.Trace("Received message from Minimax: " + payload)
				} else {
					logger.Trace("Received message from Minimax: " + string(message))
				}
			} else {
				logger.Trace("Received message from Minimax: " + string(message))
			}
		}

		var response map[string]any
		if err := json.Unmarshal(message, &response); err != nil {
			continue
		}

		event, _ := response["event"].(string)

		switch event {
		case "task_continued":
			if data, ok := response["data"].(map[string]any); ok {
				if audioHex, ok := data["audio"].(string); ok && audioHex != "" {
					audioData, err := hex.DecodeString(audioHex)
					if err != nil {
						continue
					}

					select {
					case c.audioCh <- audioData:
					case <-c.doneCh:
						return
					}
				}
			}

		case "task_finished":
			// Close audio channel to signal end of stream
			c.mu.Lock()
			if !c.closed {
				close(c.audioCh)
			}
			c.mu.Unlock()
			c.Close()
			return

		case "task_failed":
			errMsg := "TTS task failed"
			if msg, ok := response["error"].(string); ok {
				errMsg = msg
			}
			select {
			case c.errCh <- fmt.Errorf("%s", errMsg):
			default:
			}
			c.Close()
			return
		}
	}
}
