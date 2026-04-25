package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetAndValidOpenAIImageRequestAppliesGPTImageDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{
		"model":"gpt-image-2",
		"prompt":"draw a small volcano"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	request, err := GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesGenerations)

	require.NoError(t, err)
	require.Equal(t, "gpt-image-2", request.Model)
	require.Equal(t, "auto", request.Quality)
	require.NotNil(t, request.N)
	require.Equal(t, uint(1), *request.N)
}
