package yandex_grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"

	"github.com/madmike/go-ai-providers/core"
	stt "github.com/madmike/go-ai-providers/protocols/yandex_grpc/proto/generated/stt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const (
	yandexSTTEndpoint = "stt.api.cloud.yandex.net:443"
)

// Transcribe implements STTProvider.Transcribe (one-shot transcription)
func (p *Protocol) Transcribe(ctx context.Context, req core.STTRequest) (*core.STTResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	// Create streaming client
	stream, err := p.StreamTranscribe(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	// Send audio data
	if len(req.Audio) > 0 {
		if err := stream.Send(ctx, req.Audio); err != nil {
			return nil, fmt.Errorf("failed to send audio: %w", err)
		}
	}

	// Collect results
	var fullText string
	for {
		chunk, err := stream.Receive(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to receive result: %w", err)
		}

		if chunk.IsFinal {
			fullText += chunk.Text + " "
		}
	}

	return &core.STTResponse{
		Text:  fullText,
		Model: req.Model,
	}, nil
}

// StreamTranscribe implements STTProvider.StreamTranscribe
func (p *Protocol) StreamTranscribe(ctx context.Context, req core.STTRequest) (core.STTStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	// Set defaults
	model := req.Model
	if model == "" {
		model = "general"
	}
	language := req.Language
	if language == "" {
		language = "ru-RU"
	}
	sampleRate := req.SampleRate
	if sampleRate == 0 {
		sampleRate = 8000
	}
	encoding := req.Encoding
	if encoding == "" {
		encoding = "linear16"
	}

	// Create gRPC connection
	creds := credentials.NewTLS(&tls.Config{})
	conn, err := grpc.NewClient(
		yandexSTTEndpoint,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10*1024*1024)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Yandex STT: %w", err)
	}

	// Create streaming client
	client := &yandexSTTStream{
		conn:       conn,
		apiKey:     p.apiKey,
		folderID:   p.folderID,
		model:      model,
		language:   language,
		sampleRate: sampleRate,
		encoding:   encoding,
		resultCh:   make(chan *core.STTChunk, 10),
		errCh:      make(chan error, 1),
		doneCh:     make(chan struct{}),
		closed:     false,
	}

	// Initialize the stream
	if err := client.initStream(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize stream: %w", err)
	}

	return client, nil
}

// yandexSTTStream implements core.STTStream
type yandexSTTStream struct {
	conn       *grpc.ClientConn
	stream     stt.Recognizer_RecognizeStreamingClient
	apiKey     string
	folderID   string
	model      string
	language   string
	sampleRate int
	encoding   string
	resultCh   chan *core.STTChunk
	errCh      chan error
	doneCh     chan struct{}
	mu         sync.Mutex
	closed     bool
}

// initStream initializes the bidirectional streaming connection
func (c *yandexSTTStream) initStream(ctx context.Context) error {
	// Add authorization metadata
	md := metadata.New(map[string]string{
		"authorization": fmt.Sprintf("Api-Key %s", c.apiKey),
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Create the recognizer client
	recognizerClient := stt.NewRecognizerClient(c.conn)

	// Start bidirectional stream
	stream, err := recognizerClient.RecognizeStreaming(ctx)
	if err != nil {
		return fmt.Errorf("failed to start streaming: %w", err)
	}

	c.stream = stream

	// Send session options as first message
	sessionOptions := c.buildSessionOptions()
	req := &stt.StreamingRequest{
		Event: &stt.StreamingRequest_SessionOptions{
			SessionOptions: sessionOptions,
		},
	}

	if err := stream.Send(req); err != nil {
		return fmt.Errorf("failed to send session options: %w", err)
	}

	// Start reading responses in background
	go c.readMessages()

	return nil
}

// buildSessionOptions creates the session options from config
func (c *yandexSTTStream) buildSessionOptions() *stt.StreamingOptions {
	// Map encoding
	audioEncoding := stt.RawAudio_LINEAR16_PCM

	// Build audio format options
	audioFormatOptions := &stt.AudioFormatOptions{
		AudioFormat: &stt.AudioFormatOptions_RawAudio{
			RawAudio: &stt.RawAudio{
				AudioEncoding:     audioEncoding,
				SampleRateHertz:   int64(c.sampleRate),
				AudioChannelCount: 1,
			},
		},
	}

	// Build recognition model options
	recognitionModel := &stt.RecognitionModelOptions{
		Model:               c.model,
		AudioFormat:         audioFormatOptions,
		AudioProcessingType: stt.RecognitionModelOptions_REAL_TIME,
	}

	// Add language restriction
	if c.language != "" {
		normalizedLang := normalizeLanguageCode(c.language)
		recognitionModel.LanguageRestriction = &stt.LanguageRestrictionOptions{
			RestrictionType: stt.LanguageRestrictionOptions_WHITELIST,
			LanguageCode:    []string{normalizedLang},
		}
	}

	// Add text normalization
	recognitionModel.TextNormalization = &stt.TextNormalizationOptions{
		TextNormalization: stt.TextNormalizationOptions_TEXT_NORMALIZATION_ENABLED,
		ProfanityFilter:   false,
		LiteratureText:    false,
	}

	// Build EOU classifier options
	eouClassifier := &stt.EouClassifierOptions{
		Classifier: &stt.EouClassifierOptions_DefaultClassifier{
			DefaultClassifier: &stt.DefaultEouClassifier{
				Type:                       stt.DefaultEouClassifier_DEFAULT,
				MaxPauseBetweenWordsHintMs: 1000,
			},
		},
	}

	return &stt.StreamingOptions{
		RecognitionModel: recognitionModel,
		EouClassifier:    eouClassifier,
	}
}

// Send sends audio data to the STT service
func (c *yandexSTTStream) Send(ctx context.Context, audio []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("STT stream is closed")
	}

	// Send audio chunk
	req := &stt.StreamingRequest{
		Event: &stt.StreamingRequest_Chunk{
			Chunk: &stt.AudioChunk{
				Data: audio,
			},
		},
	}

	if err := c.stream.Send(req); err != nil {
		return fmt.Errorf("failed to send audio: %w", err)
	}

	return nil
}

// Receive receives transcription results from the STT service
func (c *yandexSTTStream) Receive(ctx context.Context) (*core.STTChunk, error) {
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

// Close closes the STT stream and releases resources
func (c *yandexSTTStream) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.doneCh)

	if c.stream != nil {
		c.stream.CloseSend()
	}

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// readMessages reads messages from the STT stream
func (c *yandexSTTStream) readMessages() {
	defer func() {
		c.mu.Lock()
		if !c.closed {
			close(c.doneCh)
		}
		c.mu.Unlock()
	}()

	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		stream := c.stream
		c.mu.Unlock()

		resp, err := stream.Recv()
		if err != nil {
			c.mu.Lock()
			wasClosed := c.closed
			c.mu.Unlock()

			if !wasClosed {
				if err == io.EOF {
					c.Close()
					return
				}

				select {
				case c.errCh <- fmt.Errorf("STT read error: %w", err):
				default:
				}
				c.Close()
			}
			return
		}

		if resp != nil {
			result := parseSTTResponse(resp)
			if result != nil {
				select {
				case c.resultCh <- result:
				case <-c.doneCh:
					return
				}
			}
		}
	}
}

// parseSTTResponse converts Yandex response to STTChunk
func parseSTTResponse(resp *stt.StreamingResponse) *core.STTChunk {
	result := &core.STTChunk{}

	switch event := resp.Event.(type) {
	case *stt.StreamingResponse_Partial:
		if event.Partial != nil && len(event.Partial.Alternatives) > 0 {
			alt := event.Partial.Alternatives[0]
			result.Text = alt.Text
			result.IsFinal = false
			result.Confidence = alt.Confidence
		}

	case *stt.StreamingResponse_Final:
		if event.Final != nil && len(event.Final.Alternatives) > 0 {
			alt := event.Final.Alternatives[0]
			result.Text = alt.Text
			result.IsFinal = true
			result.Confidence = alt.Confidence
		}

	case *stt.StreamingResponse_FinalRefinement:
		if event.FinalRefinement != nil && event.FinalRefinement.GetNormalizedText() != nil {
			normalized := event.FinalRefinement.GetNormalizedText()
			if len(normalized.Alternatives) > 0 {
				alt := normalized.Alternatives[0]
				result.Text = alt.Text
				result.IsFinal = true
				result.Confidence = alt.Confidence
			}
		}

	case *stt.StreamingResponse_EouUpdate:
		return nil // Don't send EOU as a result

	case *stt.StreamingResponse_StatusCode:
		return nil // Don't send status as a result

	default:
		return nil
	}

	return result
}

// normalizeLanguageCode converts language codes to Yandex-supported format
func normalizeLanguageCode(lang string) string {
	langMap := map[string]string{
		"en": "en-US", "en-US": "en-US", "en-GB": "en-US",
		"de": "de-DE", "de-DE": "de-DE",
		"es": "es-ES", "es-ES": "es-ES",
		"fr": "fr-FR", "fr-FR": "fr-FR",
		"pt": "pt-PT", "pt-PT": "pt-PT", "pt-BR": "pt-BR",
		"ru": "ru-RU", "ru-RU": "ru-RU",
		"fi": "fi-FI", "fi-FI": "fi-FI",
		"he": "he-IL", "he-IL": "he-IL",
		"it": "it-IT", "it-IT": "it-IT",
		"kk": "kk-KZ", "kk-KZ": "kk-KZ",
		"nl": "nl-NL", "nl-NL": "nl-NL",
		"pl": "pl-PL", "pl-PL": "pl-PL",
		"sv": "sv-SE", "sv-SE": "sv-SE",
		"tr": "tr-TR", "tr-TR": "tr-TR",
		"uz": "uz-UZ", "uz-UZ": "uz-UZ",
	}

	if normalized, ok := langMap[lang]; ok {
		return normalized
	}
	return "en-US"
}
