package elevenlabs_ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/madmike/go-ai-providers/core"
	"github.com/madmike/go-infra/telemetry"
)

// Transcribe implements STTProvider.Transcribe (one-shot) - Not efficiently supported by ElevenLabs stream API, but we can wrap it
func (p *Protocol) Transcribe(ctx context.Context, req core.STTRequest) (*core.STTResponse, error) {
	// This is a bit tricky since ElevenLabs doesn't have a simple REST API for STT documented here, only WebSocket.
	// We'll skip implementing one-shot Transcribe for now or implement it via streaming if critical.
	// Given the request is "streaming api for STT", we focus on StreamTranscribe.
	return nil, fmt.Errorf("one-shot transcription not supported, use CreateStream")
}

// StreamTranscribe implements STTProvider.StreamTranscribe
func (p *Protocol) StreamTranscribe(ctx context.Context, req core.STTRequest) (core.STTStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	// Default model ID as per docs
	modelID := "scribe_v1"
	if req.Model != "" {
		modelID = req.Model
	}

	// Construct WebSocket URL
	wsURL := fmt.Sprintf("wss://api.elevenlabs.io/v1/speech-to-text/realtime?model_id=%s&language_code=%s", modelID, req.Language)

	if p.logger != nil {
		if logger, ok := p.logger.(telemetry.Logger); ok {
			logger.Trace("ElevenLabs STT WebSocket connection",
				telemetry.String("url", wsURL),
				telemetry.String("model_id", modelID),
				telemetry.String("language", req.Language))
		}
	}

	// Add other query params if needed
	// token, include_timestamps, audio_format, etc.

	dialer := websocket.DefaultDialer
	header := make(map[string][]string)
	header["xi-api-key"] = []string{p.apiKey}

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ElevenLabs STT: %w", err)
	}

	stream := &elevenlabsSTTStream{
		conn:     conn,
		resultCh: make(chan *core.STTChunk, 100),
		errCh:    make(chan error, 1),
		logger:   p.logger,
	}

	go stream.readLoop()

	return stream, nil
}

type elevenlabsSTTStream struct {
	// core.STTResult is not defined, we should use *core.STTChunk
	// STTStream interface expects Receive() (*STTChunk, error)
	// So our channel should be *core.STTChunk
	conn     *websocket.Conn
	resultCh chan *core.STTChunk
	errCh    chan error
	mu       sync.Mutex
	closed   bool
	logger   any
}

func (s *elevenlabsSTTStream) Send(ctx context.Context, audio []byte) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("stream closed")
	}
	s.mu.Unlock()

	// Create InputAudioChunk message
	msg := map[string]any{
		"message_type":  "input_audio_chunk",
		"audio_base_64": base64.StdEncoding.EncodeToString(audio),
	}

	return s.conn.WriteJSON(msg)
}

func (s *elevenlabsSTTStream) Receive(ctx context.Context) (*core.STTChunk, error) {
	select {
	case res, ok := <-s.resultCh:
		if !ok {
			return nil, io.EOF
		}
		return res, nil
	case err := <-s.errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *elevenlabsSTTStream) Finish(ctx context.Context) error {
	return s.Close()
}

func (s *elevenlabsSTTStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.resultCh) // Signal EOF on channel
	return s.conn.Close()
}

func (s *elevenlabsSTTStream) readLoop() {
	defer func() {
		// handle panic or cleanup
	}()

	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		_, message, err := s.conn.ReadMessage()
		if err != nil {
			if !websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				// Normal close
			} else {
				select {
				case s.errCh <- err:
				default:
				}
			}
			s.Close()
			return
		}

		var msg map[string]any
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		msgType, _ := msg["message_type"].(string)

		switch msgType {
		case "partial_transcript":
			text, _ := msg["text"].(string)
			s.resultCh <- &core.STTChunk{
				Text:    text,
				IsFinal: false,
			}

		case "committed_transcript", "committed_transcript_with_timestamps":
			text, _ := msg["text"].(string)
			s.resultCh <- &core.STTChunk{
				Text:    text,
				IsFinal: true,
			}

		case "final_transcript":
			// Final transcript marks the end of the stream
			text, _ := msg["text"].(string)
			if text != "" {
				s.resultCh <- &core.STTChunk{
					Text:    text,
					IsFinal: true,
				}
			}
			// Close the result channel to signal end of stream
			s.mu.Lock()
			if !s.closed {
				close(s.resultCh)
			}
			s.mu.Unlock()
			s.Close()
			return

		case "error":
			errMsg, _ := msg["error"].(string)
			select {
			case s.errCh <- fmt.Errorf("ElevenLabs STT error: %s", errMsg):
			default:
			}
			s.Close()
			return
		}
	}
}
