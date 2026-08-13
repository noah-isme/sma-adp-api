package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeadersApplyToSuccessAndErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders(true))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.GET("/error", func(c *gin.Context) { c.AbortWithStatus(http.StatusInternalServerError) })

	for _, path := range []string{"/ok", "/error"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(recorder, request)

		require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"), path)
		require.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"), path)
		require.Equal(t, "strict-origin-when-cross-origin", recorder.Header().Get("Referrer-Policy"), path)
		require.Equal(t, "geolocation=(), microphone=(), camera=()", recorder.Header().Get("Permissions-Policy"), path)
		require.Equal(t, "max-age=31536000; includeSubDomains", recorder.Header().Get("Strict-Transport-Security"), path)
		require.Equal(t, "default-src 'none'; frame-ancestors 'none'; base-uri 'none'", recorder.Header().Get("Content-Security-Policy"), path)
		require.Empty(t, recorder.Header().Get("X-XSS-Protection"), path)
	}
}

func TestSecurityHeadersApplyToOptionsAndSkipProductionOnlyPoliciesLocally(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.OPTIONS("/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/health", nil)
	r.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
	require.Equal(t, "strict-origin-when-cross-origin", recorder.Header().Get("Referrer-Policy"))
	require.Equal(t, "geolocation=(), microphone=(), camera=()", recorder.Header().Get("Permissions-Policy"))
	require.Empty(t, recorder.Header().Get("Strict-Transport-Security"))
	require.Empty(t, recorder.Header().Get("Content-Security-Policy"))
}
