package openai_api

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/madmike/go-ai-providers/core"
	"github.com/sashabaranov/go-openai"
)

// ChatCompletion implements LLMProvider.ChatCompletion
func (p *Protocol) ChatCompletion(ctx context.Context, req core.ChatRequest) (*core.ChatResponse, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	config := openai.DefaultConfig(p.apiKey)
	config.BaseURL = p.baseURL
	client := openai.NewClientWithConfig(config)

	chatReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: toOpenAIMessages(req.Messages),
		Tools:    toOpenAITools(req.Tools),
	}
	if req.Temperature != nil {
		chatReq.Temperature = float32(*req.Temperature)
	}
	if req.MaxTokens != nil {
		chatReq.MaxTokens = *req.MaxTokens
	}
	if req.TopP != nil {
		chatReq.TopP = float32(*req.TopP)
	}
	if req.JSONMode {
		chatReq.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}
	}

	resp, err := client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from model")
	}

	choice := resp.Choices[0]
	out := &core.ChatResponse{
		Content: choice.Message.Content,
		Model:   resp.Model,
	}
	if resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
		usage := &core.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
		if resp.Usage.PromptTokensDetails != nil && resp.Usage.PromptTokensDetails.CachedTokens > 0 {
			usage.CachedInputTokens = resp.Usage.PromptTokensDetails.CachedTokens
		}
		out.Usage = usage
	}
	for _, tc := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, core.ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}
	return out, nil
}

// StreamChatCompletion implements LLMProvider.StreamChatCompletion.
// Tool call deltas are accumulated internally; the assembled ToolCalls slice
// is returned in the final Done chunk.
func (p *Protocol) StreamChatCompletion(ctx context.Context, req core.ChatRequest) (core.ChatStream, error) {
	if !p.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	config := openai.DefaultConfig(p.apiKey)
	config.BaseURL = p.baseURL
	client := openai.NewClientWithConfig(config)

	chatReq := openai.ChatCompletionRequest{
		Model:         req.Model,
		Messages:      toOpenAIMessages(req.Messages),
		Tools:         toOpenAITools(req.Tools),
		Stream:        true,
		StreamOptions: &openai.StreamOptions{IncludeUsage: true},
	}
	if req.Temperature != nil {
		chatReq.Temperature = float32(*req.Temperature)
	}
	if req.MaxTokens != nil {
		chatReq.MaxTokens = *req.MaxTokens
	}
	if req.TopP != nil {
		chatReq.TopP = float32(*req.TopP)
	}
	if req.JSONMode {
		chatReq.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}
	}

	stream, err := client.CreateChatCompletionStream(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	return &chatStream{stream: stream, toolAcc: make(map[int]*toolAccumulator)}, nil
}

// toolAccumulator holds partial tool-call data across stream chunks.
type toolAccumulator struct {
	id   string
	name string
	args strings.Builder
}

// chatStream implements core.ChatStream with tool-call accumulation.
// sawFinish is set when finish_reason arrives; Done is only signalled on
// io.EOF so the usage chunk that follows finish_reason is always captured.
type chatStream struct {
	stream     *openai.ChatCompletionStream
	toolAcc    map[int]*toolAccumulator
	usage      *core.Usage
	sawFinish  bool
	finishTool bool // finish_reason was "tool_calls"
}

func (s *chatStream) Receive(ctx context.Context) (*core.ChatChunk, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		if err == io.EOF {
			return &core.ChatChunk{Done: true, ToolCalls: s.assembleToolCalls(), Usage: s.usage}, nil
		}
		return nil, err
	}

	// Capture usage whenever the provider includes it (may be a standalone chunk
	// with empty choices that arrives after finish_reason, per OpenAI spec with
	// stream_options.include_usage=true).
	if resp.Usage != nil && (resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0) {
		usage := &core.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
		if resp.Usage.PromptTokensDetails != nil && resp.Usage.PromptTokensDetails.CachedTokens > 0 {
			usage.CachedInputTokens = resp.Usage.PromptTokensDetails.CachedTokens
		}
		s.usage = usage
	}

	// Empty-choices chunk: usage-only or keep-alive. Don't signal Done yet.
	if len(resp.Choices) == 0 {
		return &core.ChatChunk{}, nil
	}

	choice := resp.Choices[0]

	// Accumulate tool-call deltas.
	for _, tc := range choice.Delta.ToolCalls {
		idx := 0
		if tc.Index != nil {
			idx = *tc.Index
		}
		acc, ok := s.toolAcc[idx]
		if !ok {
			acc = &toolAccumulator{}
			s.toolAcc[idx] = acc
		}
		if tc.ID != "" {
			acc.id = tc.ID
		}
		if tc.Function.Name != "" {
			acc.name = tc.Function.Name
		}
		acc.args.WriteString(tc.Function.Arguments)
	}

	finishReason := string(choice.FinishReason)

	// Tool-call finish: return Done immediately — no usage chunk follows and
	// the caller must act on the assembled tool calls right away.
	if finishReason == "tool_calls" || (finishReason != "" && len(s.toolAcc) > 0) {
		return &core.ChatChunk{
			Done:         true,
			FinishReason: finishReason,
			ToolCalls:    s.assembleToolCalls(),
			Usage:        s.usage,
		}, nil
	}

	// Non-tool finish_reason (typically "stop"): record it but keep reading so
	// the provider can deliver the trailing usage chunk before io.EOF.
	if finishReason != "" {
		s.sawFinish = true
		return &core.ChatChunk{Content: choice.Delta.Content}, nil
	}

	return &core.ChatChunk{Content: choice.Delta.Content}, nil
}

func (s *chatStream) assembleToolCalls() []core.ToolCall {
	if len(s.toolAcc) == 0 {
		return nil
	}
	out := make([]core.ToolCall, 0, len(s.toolAcc))
	for i := 0; i < len(s.toolAcc); i++ {
		acc := s.toolAcc[i]
		out = append(out, core.ToolCall{
			ID:    acc.id,
			Name:  acc.name,
			Input: acc.args.String(),
		})
	}
	return out
}

func (s *chatStream) Close() error {
	s.stream.Close()
	return nil
}

// toOpenAIMessages converts core messages to openai format, including
// tool-call and tool-result fields.
func toOpenAIMessages(msgs []core.Message) []openai.ChatCompletionMessage {
	out := make([]openai.ChatCompletionMessage, len(msgs))
	for i, m := range msgs {
		om := openai.ChatCompletionMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, openai.ToolCall{
				ID:   tc.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tc.Name,
					Arguments: tc.Input,
				},
			})
		}
		out[i] = om
	}
	return out
}

// toOpenAITools converts core tool definitions to the openai format.
func toOpenAITools(tools []core.Tool) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]openai.Tool, len(tools))
	for i, t := range tools {
		out[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}
	return out
}
