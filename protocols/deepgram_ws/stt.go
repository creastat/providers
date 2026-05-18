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

// Transcribe implements STTProvider.Transcribe (one-shot)
func (p *Protocol) Transcribe(ctx context.Context, req core.STTRequest) (*core.STTResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	stream, err := p.StreamTranscribe(ctx, req)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	// Send audio
	if req.Audio != nil {
		stream.Send(ctx, req.Audio)
	}

	// Collect results
	var fullText string
	for {
		chunk, err := stream.Receive(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if chunk.IsFinal {
			fullText += chunk.Text + " "
		}
	}

	return &core.STTResponse{Text: fullText, Model: req.Model}, nil
}

// StreamTranscribe implements STTProvider.StreamTranscribe
func (p *Protocol) StreamTranscribe(ctx context.Context, req core.STTRequest) (core.STTStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	// Set defaults
	model := req.Model
	if model == "" {
		model = "nova-3"
	}
	language := req.Language
	if language == "" {
		language = "en"
	}
	sampleRate := req.SampleRate
	if sampleRate == 0 {
		sampleRate = 16000
	}
	encoding := req.Encoding
	if encoding == "" || encoding == "raw" || encoding == "pcm" || encoding == "pcm16" {
		encoding = "linear16"
	}

	// Build WebSocket URL
	u, _ := url.Parse("wss://api.deepgram.com/v1/listen")
	query := u.Query()
	query.Set("model", model)
	query.Set("encoding", encoding)
	query.Set("sample_rate", fmt.Sprintf("%d", sampleRate))
	query.Set("channels", "1")
	query.Set("language", language)
	query.Set("punctuate", "true")
	query.Set("smart_format", "true")

	// Real-time / VOIP streaming options. All configurable via req.Options with
	// sensible low-latency defaults suitable for the VOIP agent use case.
	// See https://developers.deepgram.com/reference/listen-live-streaming
	applyOpt := func(key string, def string) {
		if v, ok := stringOpt(req.Options, key); ok {
			if v != "" {
				query.Set(key, v)
			}
			return
		}
		if def != "" {
			query.Set(key, def)
		}
	}
	applyOpt("interim_results", "true")  // stream partial hypotheses
	applyOpt("vad_events", "true")       // SpeechStarted events
	applyOpt("endpointing", "300")       // ms of silence → final
	applyOpt("utterance_end_ms", "1000") // UtteranceEnd event threshold
	applyOpt("filler_words", "")         // off by default; caller may opt in
	applyOpt("no_delay", "true")         // prioritise low latency
	applyOpt("diarize", "")
	applyOpt("numerals", "")

	// Add keyterms if provided
	if len(req.Keyterms) > 0 {
		for _, keyterm := range req.Keyterms {
			query.Add("keyterm", keyterm)
		}
	}

	u.RawQuery = query.Encode()

	p.logger.Debug("Deepgram connect params",
		telemetry.String("url", u.String()))

	// Validate API key
	if p.apiKey == "" {
		return nil, fmt.Errorf("Deepgram API key is not set. Please set DEEPGRAM_API_KEY environment variable")
	}

	// Log connection details for debugging
	p.logger.Info("Connecting to Deepgram",
		telemetry.String("model", model),
		telemetry.String("encoding", encoding),
		telemetry.Int("sample_rate", sampleRate),
		telemetry.String("language", language),
		telemetry.String("url", u.String()),
		telemetry.Any("keyterms", req.Keyterms))

	// Connect
	dialer := websocket.DefaultDialer
	header := make(map[string][]string)
	header["Authorization"] = []string{fmt.Sprintf("Token %s", p.apiKey)}

	conn, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		if resp != nil {
			p.logger.Error("Deepgram connection failed",
				telemetry.Int("status_code", resp.StatusCode),
				telemetry.String("status", resp.Status),
				telemetry.Err(err))
			return nil, fmt.Errorf("failed to connect to Deepgram (HTTP %d): %w. Please check your DEEPGRAM_API_KEY", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("failed to connect to Deepgram: %w. Please check your DEEPGRAM_API_KEY and network connection", err)
	}

	client := &deepgramSTTStream{
		conn:     conn,
		resultCh: make(chan *core.STTChunk, 10),
		errCh:    make(chan error, 1),
		doneCh:   make(chan struct{}),
	}

	go client.readMessages()
	go client.keepAliveLoop()

	return client, nil
}

// deepgramSTTStream implements core.STTStream
type deepgramSTTStream struct {
	conn     *websocket.Conn
	resultCh chan *core.STTChunk
	errCh    chan error
	doneCh   chan struct{}
	mu       sync.Mutex
	closed   bool
}

func (c *deepgramSTTStream) Send(ctx context.Context, audio []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("stream closed")
	}
	return c.conn.WriteMessage(websocket.BinaryMessage, audio)
}

func (c *deepgramSTTStream) Receive(ctx context.Context) (*core.STTChunk, error) {
	select {
	case result := <-c.resultCh:
		return result, nil
	case err := <-c.errCh:
		return nil, err
	case <-c.doneCh:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Finalize implements core.FinalizableSTTStream. Sends Deepgram's Finalize
// control message, which flushes buffered audio and forces an is_final
// response for the current segment without closing the stream. Intended
// for callers with client-side VAD that want to bypass Deepgram's own
// endpointing latency on short utterances.
// See https://developers.deepgram.com/docs/finalize
func (c *deepgramSTTStream) Finalize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("stream closed")
	}
	msg := map[string]any{"type": "Finalize"}
	jsonData, _ := json.Marshal(msg)
	return c.conn.WriteMessage(websocket.TextMessage, jsonData)
}

func (c *deepgramSTTStream) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// Send close message to signal end of audio
	closeMsg := map[string]any{"type": "CloseStream"}
	jsonData, _ := json.Marshal(closeMsg)
	c.conn.WriteMessage(websocket.TextMessage, jsonData)

	// Wait for readMessages goroutine to finish receiving results with timeout
	select {
	case <-c.doneCh:
		// readMessages finished normally
	case <-time.After(500 * time.Millisecond):
		// Timeout - force close
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	// Force close immediately by setting deadline
	c.conn.SetReadDeadline(time.Now())
	err := c.conn.Close()
	return err
}

func (c *deepgramSTTStream) keepAliveLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			if c.closed {
				c.mu.Unlock()
				return
			}
			keepAliveMsg := map[string]string{"type": "KeepAlive"}
			jsonData, _ := json.Marshal(keepAliveMsg)
			_ = c.conn.WriteMessage(websocket.TextMessage, jsonData)
			c.mu.Unlock()
		case <-c.doneCh:
			return
		}
	}
}

func (c *deepgramSTTStream) readMessages() {
	defer func() {
		close(c.doneCh)
	}()

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

			if !wasClosed {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					select {
					case c.errCh <- fmt.Errorf("read error: %w", err):
					default:
					}
				}
				c.Close()
			}
			return
		}

		if messageType == websocket.TextMessage {
			var rawResult map[string]any
			if err := json.Unmarshal(message, &rawResult); err != nil {
				continue
			}

			msgType, _ := rawResult["type"].(string)
			if msgType == "Results" {
				result := parseDeepgramResult(rawResult)
				if result != nil {
					select {
					case c.resultCh <- result:
					case <-c.doneCh:
						return
					}
				}
			} else if msgType == "SpeechStarted" {
				select {
				case c.resultCh <- &core.STTChunk{EventType: "speech_started"}:
				case <-c.doneCh:
					return
				}
			} else if msgType == "UtteranceEnd" {
				select {
				case c.resultCh <- &core.STTChunk{EventType: "utterance_end"}:
				case <-c.doneCh:
					return
				}
			}
			// Other message types (Metadata, etc.) are
			// ignored by the STT channel.
		}
	}
}

// stringOpt pulls a string value from the request options map, accepting
// either native strings or bool/number shapes that Deepgram expects as
// lowercase strings in the query params.
func stringOpt(opts map[string]any, key string) (string, bool) {
	if opts == nil {
		return "", false
	}
	v, ok := opts[key]
	if !ok {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	case int:
		return fmt.Sprintf("%d", x), true
	case int64:
		return fmt.Sprintf("%d", x), true
	case float64:
		// query params for ms thresholds arrive as JSON numbers → floats
		return fmt.Sprintf("%d", int64(x)), true
	default:
		return fmt.Sprintf("%v", x), true
	}
}

func parseDeepgramResult(raw map[string]any) *core.STTChunk {
	result := &core.STTChunk{}

	if isFinal, ok := raw["is_final"].(bool); ok {
		result.IsFinal = isFinal
	}

	// Extract channel data
	var channelMap map[string]any
	if channel, ok := raw["channel"].(map[string]any); ok {
		channelMap = channel
	} else if channelData, ok := raw["channel"].([]any); ok && len(channelData) > 0 {
		if ch, ok := channelData[0].(map[string]any); ok {
			channelMap = ch
		}
	}

	if channelMap != nil {
		if alternatives, ok := channelMap["alternatives"].([]any); ok && len(alternatives) > 0 {
			if alt, ok := alternatives[0].(map[string]any); ok {
				if transcript, ok := alt["transcript"].(string); ok {
					result.Text = transcript
				}
				if confidence, ok := alt["confidence"].(float64); ok {
					result.Confidence = confidence
				}
			}
		}
	}

	return result
}
