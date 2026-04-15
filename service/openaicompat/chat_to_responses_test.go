package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsRequestToResponsesRequestAddsNameToFunctionCallOutput(t *testing.T) {
	t.Parallel()

	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4.1",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "what is the weather",
			},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []byte(`[
					{"id":"fc0","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}
				]`),
			},
			{
				Role:       "tool",
				ToolCallId: "fc0",
				Content:    `{"temperature":"18C"}`,
			},
		},
	}

	responsesReq, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	var items []map[string]any
	err = common.Unmarshal(responsesReq.Input, &items)
	require.NoError(t, err)

	var toolOutput map[string]any
	for _, item := range items {
		if item["type"] == "function_call_output" {
			toolOutput = item
			break
		}
	}
	require.NotNil(t, toolOutput)
	require.Equal(t, "function_call_output", toolOutput["type"])
	require.Equal(t, "fc0", toolOutput["call_id"])
	require.Equal(t, "get_weather", toolOutput["name"])
	require.Equal(t, `{"temperature":"18C"}`, toolOutput["output"])
}

func TestChatCompletionsRequestToResponsesRequestPrefersExplicitToolName(t *testing.T) {
	t.Parallel()

	toolName := "lookup_weather"
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4.1",
		Messages: []dto.Message{
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []byte(`[
					{"id":"fc0","type":"function","function":{"name":"get_weather","arguments":"{}"}}
				]`),
			},
			{
				Role:       "tool",
				Name:       &toolName,
				ToolCallId: "fc0",
				Content:    `{"ok":true}`,
			},
		},
	}

	responsesReq, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	var items []map[string]any
	err = common.Unmarshal(responsesReq.Input, &items)
	require.NoError(t, err)

	var toolOutput map[string]any
	for _, item := range items {
		if item["type"] == "function_call_output" {
			toolOutput = item
			break
		}
	}
	require.NotNil(t, toolOutput)
	require.Equal(t, "lookup_weather", toolOutput["name"])
}

func TestChatCompletionsRequestToResponsesRequestFallsBackToUnknownFunctionName(t *testing.T) {
	t.Parallel()

	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4.1",
		Messages: []dto.Message{
			{
				Role:       "tool",
				ToolCallId: "fc-missing",
				Content:    `{"ok":true}`,
			},
		},
	}

	responsesReq, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	var items []map[string]any
	err = common.Unmarshal(responsesReq.Input, &items)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "function_call_output", items[0]["type"])
	require.Equal(t, "fc-missing", items[0]["call_id"])
	require.Equal(t, "unknown_function", items[0]["name"])
}
