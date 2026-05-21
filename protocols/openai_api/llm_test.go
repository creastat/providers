package openai_api

import (
	"testing"

	"github.com/madmike/go-ai-providers/core"
	"github.com/stretchr/testify/require"
)

func TestToOpenAITools_Mapping(t *testing.T) {
	tools := []core.Tool{
		{
			Name:        "dashboard:update-tenant-profile",
			Description: "Update tenant profile info",
			InputSchema: map[string]any{"type": "object"},
		},
		{
			Name:        "dashboard:create-assistant",
			Description: "Create a dental clinic assistant",
			InputSchema: map[string]any{"type": "object"},
		},
		{
			Name:        "simple_tool",
			Description: "No colon here",
			InputSchema: map[string]any{"type": "object"},
		},
	}

	openAITools := toOpenAITools(tools)
	require.Len(t, openAITools, 3)

	require.Equal(t, "dashboard__update-tenant-profile", openAITools[0].Function.Name)
	require.Equal(t, "dashboard__create-assistant", openAITools[1].Function.Name)
	require.Equal(t, "simple_tool", openAITools[2].Function.Name)
}

func TestToOpenAIMessages_Mapping(t *testing.T) {
	msgs := []core.Message{
		{
			Role:    "assistant",
			Content: "Let me call a tool.",
			ToolCalls: []core.ToolCall{
				{
					ID:    "call_123",
					Name:  "dashboard:update-tenant-profile",
					Input: `{"name": "test"}`,
				},
				{
					ID:    "call_456",
					Name:  "simple_tool",
					Input: `{"val": 123}`,
				},
			},
		},
		{
			Role:       "tool",
			Content:    "Success",
			ToolCallID: "call_123",
		},
	}

	openAIMessages := toOpenAIMessages(msgs)
	require.Len(t, openAIMessages, 2)

	// Verify the assistant message has converted tool call names
	require.Len(t, openAIMessages[0].ToolCalls, 2)
	require.Equal(t, "dashboard__update-tenant-profile", openAIMessages[0].ToolCalls[0].Function.Name)
	require.Equal(t, "simple_tool", openAIMessages[0].ToolCalls[1].Function.Name)

	// Verify role and tool call ID
	require.Equal(t, "tool", openAIMessages[1].Role)
	require.Equal(t, "call_123", openAIMessages[1].ToolCallID)
}
