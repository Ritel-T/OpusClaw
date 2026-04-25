package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsImageGenerationModelIncludesGPTImageFamily(t *testing.T) {
	require.True(t, IsImageGenerationModel("gpt-image-1"))
	require.True(t, IsImageGenerationModel("gpt-image-1-mini"))
	require.True(t, IsImageGenerationModel("gpt-image-1.5"))
	require.True(t, IsImageGenerationModel("gpt-image-2"))
	require.True(t, IsImageGenerationModel("GPT-IMAGE-2"))
}

func TestIsGPTImageModel(t *testing.T) {
	require.True(t, IsGPTImageModel("gpt-image-2"))
	require.True(t, IsGPTImageModel("gpt-image-1-mini"))
	require.False(t, IsGPTImageModel("dall-e-3"))
	require.False(t, IsGPTImageModel("not-gpt-image-2"))
}
