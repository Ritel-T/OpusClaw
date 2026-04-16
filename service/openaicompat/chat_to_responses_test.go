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

func TestChatCompletionsRequestToResponsesRequestReordersToolOutputAfterFunctionCall(t *testing.T) {
	t.Parallel()

	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4.1",
		Messages: []dto.Message{
			{
				Role:       "tool",
				ToolCallId: "fc1",
				Content:    `{"ok":true}`,
			},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []byte(`[
					{"id":"fc1","type":"function","function":{"name":"browser_click","arguments":"{\"selector\":\"#submit\"}"}}
				]`),
			},
		},
	}

	responsesReq, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	var items []map[string]any
	err = common.Unmarshal(responsesReq.Input, &items)
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, "assistant", items[0]["role"])
	require.Equal(t, "function_call", items[1]["type"])
	require.Equal(t, "fc1", items[1]["call_id"])
	require.Equal(t, "function_call_output", items[2]["type"])
	require.Equal(t, "fc1", items[2]["call_id"])
	require.Equal(t, "browser_click", items[2]["name"])
}

func TestChatCompletionsRequestToResponsesRequestNormalizesToolSchemas(t *testing.T) {
	t.Parallel()

	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4.1",
		Tools: []dto.ToolCallRequest{
			{
				Type: "function",
				Function: dto.FunctionRequest{
					Name: "read",
					Parameters: map[string]any{
						"type": "OBJECT",
						"properties": map[string]any{
							"path":  map[string]any{"type": "STRING"},
							"count": map[string]any{"type": []any{"NUMBER", "null"}},
						},
					},
				},
			},
		},
	}

	responsesReq, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	var tools []map[string]any
	err = common.Unmarshal(responsesReq.Tools, &tools)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	params := tools[0]["parameters"].(map[string]any)
	require.Equal(t, "object", params["type"])
	properties := params["properties"].(map[string]any)
	require.Equal(t, "string", properties["path"].(map[string]any)["type"])
	count := properties["count"].(map[string]any)
	require.Equal(t, "number", count["type"])
	require.Equal(t, true, count["nullable"])
}
