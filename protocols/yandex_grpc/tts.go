package yandex_grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"

	"github.com/madmike/go-ai-providers/core"
	tts "github.com/madmike/go-ai-providers/protocols/yandex_grpc/proto/generated/tts"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const (
	yandexTTSEndpoint = "tts.api.cloud.yandex.net:443"
)

// Synthesize implements TTSProvider.Synthesize (one-shot synthesis)
func (p *Protocol) Synthesize(ctx context.Context, req core.TTSRequest) (*core.TTSResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	// Set defaults
	voice := req.Voice
	if voice == "" {
		voice = "alena"
	}
	sampleRate := 22050
	if req.Options != nil {
		if sr, ok := req.Options["sample_rate"].(int); ok {
			sampleRate = sr
		}
	}
	speed := 1.0
	if req.Speed != nil {
		speed = *req.Speed
	}

	// Create gRPC connection
	creds := credentials.NewTLS(&tls.Config{})
	conn, err := grpc.NewClient(yandexTTSEndpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Yandex TTS: %w", err)
	}
	defer conn.Close()

	// Add authorization metadata
	md := metadata.New(map[string]string{
		"authorization": fmt.Sprintf("Api-Key %s", p.apiKey),
		"x-folder-id":   p.folderID,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Create synthesizer client
	synthesizerClient := tts.NewSynthesizerClient(conn)

	// Build request
	ttsReq := buildUtteranceRequest(req.Text, req.Model, voice, sampleRate, speed, req.Options)

	// Call synthesis
	stream, err := synthesizerClient.UtteranceSynthesis(ctx, ttsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to start synthesis: %w", err)
	}

	// Collect audio data
	var audioData []byte
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to receive audio: %w", err)
		}

		if resp.AudioChunk != nil && len(resp.AudioChunk.Data) > 0 {
			audioData = append(audioData, resp.AudioChunk.Data...)
		}
	}

	return &core.TTSResponse{
		Audio: audioData,
		Model: req.Model,
	}, nil
}

// StreamSynthesize implements TTSProvider.StreamSynthesize
func (p *Protocol) StreamSynthesize(ctx context.Context, req core.TTSRequest) (core.TTSStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	// Set defaults
	voice := req.Voice
	if voice == "" {
		voice = "ermil"
	}
	sampleRate := 22050
	if req.Options != nil {
		if sr, ok := req.Options["sample_rate"].(int); ok {
			sampleRate = sr
		}
	}
	speed := 1.0
	if req.Speed != nil {
		speed = *req.Speed
	}

	// Create gRPC connection
	creds := credentials.NewTLS(&tls.Config{})
	conn, err := grpc.NewClient(yandexTTSEndpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Yandex TTS: %w", err)
	}

	// Create streaming client
	client := &yandexTTSStream{
		conn:       conn,
		apiKey:     p.apiKey,
		folderID:   p.folderID,
		model:      req.Model,
		voice:      voice,
		sampleRate: sampleRate,
		speed:      speed,
		options:    req.Options,
		audioCh:    make(chan []byte, 100),
		errCh:      make(chan error, 1),
		doneCh:     make(chan struct{}),
		ctx:        ctx,
		closed:     false,
	}

	return client, nil
}

// yandexTTSStream implements core.TTSStream
type yandexTTSStream struct {
	conn       *grpc.ClientConn
	stream     tts.Synthesizer_StreamSynthesisClient
	apiKey     string
	folderID   string
	model      string
	voice      string
	sampleRate int
	speed      float64
	options    map[string]any
	audioCh    chan []byte
	errCh      chan error
	doneCh     chan struct{}
	mu         sync.Mutex
	closed     bool
	finished   bool
	ctx        context.Context
	wg         sync.WaitGroup
	closeOnce  sync.Once
	doneOnce   sync.Once
	finishOnce sync.Once
}

// Send sends text to be synthesized
func (c *yandexTTSStream) Send(ctx context.Context, text string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("TTS stream is closed")
	}
	if c.finished {
		c.mu.Unlock()
		return fmt.Errorf("TTS stream input already finished")
	}

	// Initialize stream on first Send
	if c.stream == nil {
		if err := c.initStream(); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("failed to initialize stream: %w", err)
		}
	}
	c.mu.Unlock()

	if text == "" {
		return nil
	}

	// Send text input to stream
	req := &tts.StreamSynthesisRequest{
		Event: &tts.StreamSynthesisRequest_SynthesisInput{
			SynthesisInput: &tts.SynthesisInput{
				Text: text,
			},
		},
	}

	if err := c.stream.Send(req); err != nil {
		return fmt.Errorf("failed to send text: %w", err)
	}

	return nil
}

// Finish signals that no more text will be sent to the stream.
func (c *yandexTTSStream) Finish(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("TTS stream is closed")
	}
	stream := c.stream
	c.mu.Unlock()

	// Nothing to finish if stream has not been initialized (no text sent).
	if stream == nil {
		return nil
	}

	var finishErr error
	c.finishOnce.Do(func() {
		c.mu.Lock()
		c.finished = true
		c.mu.Unlock()
		if err := stream.CloseSend(); err != nil {
			finishErr = fmt.Errorf("failed to finish stream: %w", err)
		}
	})
	return finishErr
}

// initStream initializes the streaming synthesis connection
func (c *yandexTTSStream) initStream() error {
	// Add authorization metadata
	md := metadata.New(map[string]string{
		"authorization": fmt.Sprintf("Api-Key %s", c.apiKey),
		"x-folder-id":   c.folderID,
	})
	streamCtx := metadata.NewOutgoingContext(c.ctx, md)

	// Create synthesizer client
	synthesizerClient := tts.NewSynthesizerClient(c.conn)

	// Start bidirectional stream
	stream, err := synthesizerClient.StreamSynthesis(streamCtx)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}
	c.stream = stream

	// Send initial options
	opts := c.buildSynthesisOptions()
	req := &tts.StreamSynthesisRequest{
		Event: &tts.StreamSynthesisRequest_Options{
			Options: opts,
		},
	}

	if err := stream.Send(req); err != nil {
		return fmt.Errorf("failed to send options: %w", err)
	}

	// Start receiver goroutine
	c.wg.Add(1)
	go c.receiveAudio()

	return nil
}

// buildSynthesisOptions creates synthesis options
func (c *yandexTTSStream) buildSynthesisOptions() *tts.SynthesisOptions {
	audioSpec := &tts.AudioFormatOptions{
		AudioFormat: &tts.AudioFormatOptions_RawAudio{
			RawAudio: &tts.RawAudio{
				AudioEncoding:   tts.RawAudio_LINEAR16_PCM,
				SampleRateHertz: int64(c.sampleRate),
			},
		},
	}

	volume := -19.0 // LUFS default
	if c.options != nil {
		if v, ok := c.options["volume"].(float64); ok {
			volume = v
		}
	}

	return &tts.SynthesisOptions{
		Model:                     c.model,
		Voice:                     c.voice,
		Speed:                     c.speed,
		Volume:                    volume,
		OutputAudioSpec:           audioSpec,
		LoudnessNormalizationType: tts.LoudnessNormalizationType_LUFS,
	}
}

// receiveAudio receives audio chunks from the stream
func (c *yandexTTSStream) receiveAudio() {
	defer c.wg.Done()
	defer c.closeOnce.Do(func() {
		close(c.audioCh)
	})

	for {
		resp, err := c.stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			select {
			case c.errCh <- fmt.Errorf("failed to receive audio: %w", err):
			default:
			}
			return
		}

		if resp.AudioChunk != nil && len(resp.AudioChunk.Data) > 0 {
			select {
			case c.audioCh <- resp.AudioChunk.Data:
			case <-c.doneCh:
				return
			}
		}
	}
}

// Receive receives synthesized audio data
func (c *yandexTTSStream) Receive(ctx context.Context) (*core.TTSChunk, error) {
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

// Close closes the TTS stream and releases resources
func (c *yandexTTSStream) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// Signal termination to internal goroutines first.
	c.doneOnce.Do(func() {
		close(c.doneCh)
	})

	// Close send side if it has been initialized.
	if c.stream != nil {
		_ = c.Finish(context.Background())
	}

	// Wait for receiver goroutine
	c.wg.Wait()

	// Close audio channel for streams that were never initialized.
	c.closeOnce.Do(func() {
		close(c.audioCh)
	})

	// Close the connection
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// buildUtteranceRequest creates an utterance synthesis request
func buildUtteranceRequest(text, model, voice string, sampleRate int, speed float64, options map[string]any) *tts.UtteranceSynthesisRequest {
	audioSpec := &tts.AudioFormatOptions{
		AudioFormat: &tts.AudioFormatOptions_RawAudio{
			RawAudio: &tts.RawAudio{
				AudioEncoding:   tts.RawAudio_LINEAR16_PCM,
				SampleRateHertz: int64(sampleRate),
			},
		},
	}

	volume := -19.0 // LUFS default
	if options != nil {
		if v, ok := options["volume"].(float64); ok {
			volume = v
		}
	}

	hints := []*tts.Hints{
		{Hint: &tts.Hints_Voice{Voice: voice}},
		{Hint: &tts.Hints_Speed{Speed: speed}},
		{Hint: &tts.Hints_Volume{Volume: volume}},
	}

	return &tts.UtteranceSynthesisRequest{
		Model: model,
		Utterance: &tts.UtteranceSynthesisRequest_Text{
			Text: text,
		},
		Hints:                     hints,
		OutputAudioSpec:           audioSpec,
		LoudnessNormalizationType: tts.UtteranceSynthesisRequest_LUFS,
		UnsafeMode:                false,
	}
}
