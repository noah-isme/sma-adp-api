package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterRejectsAfterBurstAndSetsRetryHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewRateLimiter(RateLimiterConfig{RequestsPerMinute: 60, Burst: 2, MaxClients: 10})
	limiter.now = func() time.Time { return time.Unix(100, 0) }
	r := gin.New()
	r.Use(limiter.Handler())
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
		require.Equal(t, http.StatusNoContent, recorder.Code)
	}
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Equal(t, "0", recorder.Header().Get("X-RateLimit-Remaining"))
	require.NotEmpty(t, recorder.Header().Get("Retry-After"))
}

func TestRateLimiterRefillsAndUsesCanonicalForwardedIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clock := time.Unix(200, 0)
	limiter := NewRateLimiter(RateLimiterConfig{RequestsPerMinute: 60, Burst: 1, MaxClients: 10})
	limiter.now = func() time.Time { return clock }
	r := gin.New()
	r.Use(limiter.Handler())
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("CF-Connecting-IP", "203.0.113.10")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)

	clock = clock.Add(time.Second)
	request = httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("CF-Connecting-IP", "203.0.113.10")
	recorder = httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
}
