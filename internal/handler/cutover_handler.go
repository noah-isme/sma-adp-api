package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// CutoverHealthService defines the subset of the cutover service used by the handler.
type CutoverHealthService interface {
	PingLegacy(ctx context.Context) (models.CutoverPingResult, error)
	PingGo(ctx context.Context) (models.CutoverPingResult, error)
}

// CutoverHandler exposes internal observability endpoints for the cutover.
type CutoverHandler struct {
	service CutoverHealthService
}

// NewCutoverHandler constructs a CutoverHandler.
func NewCutoverHandler(svc CutoverHealthService) *CutoverHandler {
	return &CutoverHandler{service: svc}
}

// PingLegacy reports the health status of the legacy NestJS backend.
func (h *CutoverHandler) PingLegacy(c *gin.Context) {
	h.ping(c, h.service.PingLegacy)
}

// PingGo reports the health status of the Go API backend.
func (h *CutoverHandler) PingGo(c *gin.Context) {
	h.ping(c, h.service.PingGo)
}

func (h *CutoverHandler) ping(c *gin.Context, fn func(ctx context.Context) (models.CutoverPingResult, error)) {
	if h == nil || h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cutover service unavailable"})
		return
	}
	result, err := fn(c.Request.Context())
	status := http.StatusOK
	if err != nil || !result.Reachable {
		status = http.StatusServiceUnavailable
	}
	if err != nil {
		c.Header("X-Cutover-Error", err.Error())
	}
	c.JSON(status, result)
}
