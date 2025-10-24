package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/service"
)

// Metrics returns middleware that captures request metrics using the provided service.
func Metrics(metricsSvc *service.MetricsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if metricsSvc == nil {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		status := c.Writer.Status()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		metricsSvc.ObserveHTTPRequest(c.Request.Method, path, status, duration)
	}
}
