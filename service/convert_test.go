package service

import (
	"encoding/base64"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestGeminiToOpenAIRequestMapsFunctionResponsesToToolMessages(t *testing.T) {
	t.Parallel()

	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role:  "user",
				Parts: []dto.GeminiPart{{Text: "what is the weather"}},
			},
			{
				Role: "model",
				Parts: []dto.GeminiPart{{
					FunctionCall: &dto.FunctionCall{
						FunctionName: "get_weather",
						Arguments:    map[string]any{"city": "Paris"},
					},
				}},
			},
			{
				Role: "function",
				Parts: []dto.GeminiPart{{
					FunctionResponse: &dto.GeminiFunctionResponse{
						Name:     "get_weather",
						Response: map[string]any{"temperature": "18C"},
					},
				}},
			},
		},
	}

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-4o-mini",
		},
	}

	openaiRequest, err := GeminiToOpenAIRequest(request, info)
	require.NoError(t, err)
	require.Len(t, openaiRequest.Messages, 3)

	assistantMessage := openaiRequest.Messages[1]
	toolCalls := assistantMessage.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	require.Equal(t, "assistant", assistantMessage.Role)
	require.Equal(t, "get_weather", toolCalls[0].Function.Name)
	require.NotEmpty(t, toolCalls[0].ID)

	toolMessage := openaiRequest.Messages[2]
	require.Equal(t, "tool", toolMessage.Role)
	require.Equal(t, toolCalls[0].ID, toolMessage.ToolCallId)
	require.NotNil(t, toolMessage.Name)
	require.Equal(t, "get_weather", *toolMessage.Name)
	require.Contains(t, toolMessage.StringContent(), "temperature")
	require.NotContains(t, toolMessage.StringContent(), "tool_output_missing_call_id")
}

func TestGeminiToOpenAIRequestUsesFunctionResponseIDWhenPresent(t *testing.T) {
	t.Parallel()

	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "function",
				Parts: []dto.GeminiPart{{
					FunctionResponse: &dto.GeminiFunctionResponse{
						Name:     "lookup_weather",
						ID:       []byte(`"fc0"`),
						Response: map[string]any{"temperature": "18C"},
					},
				}},
			},
		},
	}

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o-mini"},
	}

	openaiRequest, err := GeminiToOpenAIRequest(request, info)
	require.NoError(t, err)
	require.Len(t, openaiRequest.Messages, 1)

	toolMessage := openaiRequest.Messages[0]
	require.Equal(t, "tool", toolMessage.Role)
	require.Equal(t, "fc0", toolMessage.ToolCallId)
	require.NotNil(t, toolMessage.Name)
	require.Equal(t, "lookup_weather", *toolMessage.Name)
	require.Contains(t, toolMessage.StringContent(), "temperature")
}

func TestGeminiToOpenAIRequestMatchesSameNameResponsesInOrder(t *testing.T) {
	t.Parallel()

	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "model",
				Parts: []dto.GeminiPart{
					{FunctionCall: &dto.FunctionCall{FunctionName: "search_docs", Arguments: map[string]any{"query": "alpha"}}},
					{FunctionCall: &dto.FunctionCall{FunctionName: "search_docs", Arguments: map[string]any{"query": "beta"}}},
				},
			},
			{
				Role: "function",
				Parts: []dto.GeminiPart{
					{FunctionResponse: &dto.GeminiFunctionResponse{Name: "search_docs", Response: map[string]any{"query": "alpha"}}},
					{FunctionResponse: &dto.GeminiFunctionResponse{Name: "search_docs", Response: map[string]any{"query": "beta"}}},
				},
			},
		},
	}

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o-mini"}}

	openaiRequest, err := GeminiToOpenAIRequest(request, info)
	require.NoError(t, err)
	require.Len(t, openaiRequest.Messages, 3)

	assistantToolCalls := openaiRequest.Messages[0].ParseToolCalls()
	require.Len(t, assistantToolCalls, 2)
	require.Equal(t, assistantToolCalls[0].ID, openaiRequest.Messages[1].ToolCallId)
	require.Equal(t, assistantToolCalls[1].ID, openaiRequest.Messages[2].ToolCallId)
	require.Contains(t, openaiRequest.Messages[1].StringContent(), "alpha")
	require.Contains(t, openaiRequest.Messages[2].StringContent(), "beta")
}

func TestGeminiToOpenAIRequestKeepsNonImageFileDataOutOfImageURL(t *testing.T) {
	t.Parallel()

	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "user",
				Parts: []dto.GeminiPart{{
					FileData: &dto.GeminiFileData{
						MimeType: "application/pdf",
						FileUri:  "https://example.com/doc.pdf",
					},
				}},
			},
		},
	}

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-4o-mini",
		},
	}

	openaiRequest, err := GeminiToOpenAIRequest(request, info)
	require.NoError(t, err)
	require.Len(t, openaiRequest.Messages, 1)

	contents := openaiRequest.Messages[0].ParseContent()
	require.Len(t, contents, 1)
	require.Equal(t, dto.ContentTypeText, contents[0].Type)
	require.Contains(t, contents[0].Text, "application/pdf")
	require.Contains(t, contents[0].Text, "https://example.com/doc.pdf")
	if image := contents[0].GetImageMedia(); image != nil {
		t.Fatalf("expected non-image file data to avoid image_url conversion, got %+v", image)
	}
}

func TestGeminiToOpenAIRequestConvertsUppercaseToolSchemaTypes(t *testing.T) {
	t.Parallel()

	request := &dto.GeminiChatRequest{}
	request.SetTools([]dto.GeminiChatTool{{
		FunctionDeclarations: []dto.FunctionRequest{{
			Name: "transfer_to_agent",
			Parameters: map[string]any{
				"type": "OBJECT",
				"properties": map[string]any{
					"path":  map[string]any{"type": "STRING"},
					"count": map[string]any{"type": []any{"INTEGER", "null"}},
				},
			},
		}},
	}})

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o-mini"}}

	openaiRequest, err := GeminiToOpenAIRequest(request, info)
	require.NoError(t, err)
	require.Len(t, openaiRequest.Tools, 1)

	params, ok := openaiRequest.Tools[0].Function.Parameters.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "object", params["type"])
	properties := params["properties"].(map[string]any)
	require.Equal(t, "string", properties["path"].(map[string]any)["type"])
	count := properties["count"].(map[string]any)
	require.Equal(t, "integer", count["type"])
	require.Equal(t, true, count["nullable"])
}

func TestGeminiToOpenAIRequestRejectsInvalidInlineImageBytes(t *testing.T) {
	t.Parallel()

	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{
			Role: "user",
			Parts: []dto.GeminiPart{{
				InlineData: &dto.GeminiInlineData{
					MimeType: "image/png",
					Data:     base64.StdEncoding.EncodeToString([]byte("not-an-image")),
				},
			}},
		}},
	}

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o-mini"}}

	openaiRequest, err := GeminiToOpenAIRequest(request, info)
	require.NoError(t, err)
	require.Len(t, openaiRequest.Messages, 1)

	parts := openaiRequest.Messages[0].ParseContent()
	require.Len(t, parts, 1)
	require.Equal(t, dto.ContentTypeFile, parts[0].Type)
	require.NotNil(t, parts[0].GetFile())
	require.Nil(t, parts[0].GetImageMedia())
}
