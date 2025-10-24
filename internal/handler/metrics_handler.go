package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/service"
)

// MetricsHandler exposes observability endpoints.
type MetricsHandler struct {
	metrics *service.MetricsService
}

// NewMetricsHandler constructs a metrics handler.
func NewMetricsHandler(metrics *service.MetricsService) *MetricsHandler {
	return &MetricsHandler{metrics: metrics}
}

// Prometheus serves the Prometheus metrics endpoint.
func (h *MetricsHandler) Prometheus(c *gin.Context) {
	if h.metrics == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	h.metrics.Handler().ServeHTTP(c.Writer, c.Request)
}

// Health responds with a generic OK payload for readiness/liveness usage.
func (h *MetricsHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
