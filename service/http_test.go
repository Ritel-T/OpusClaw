package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIOCopyBytesGracefullyReturnsContextErrorBeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	requestCtx, cancel := context.WithCancel(req.Context())
	cancel()
	ctx.Request = req.WithContext(requestCtx)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}

	err := IOCopyBytesGracefully(ctx, resp, []byte(`{"data":[]}`))

	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}
