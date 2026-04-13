package service

import (
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
	require.Contains(t, toolMessage.StringContent(), "temperature")
	require.NotContains(t, toolMessage.StringContent(), "tool_output_missing_call_id")
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
