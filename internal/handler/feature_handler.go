package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/pkg/config"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// FeatureHandler serves the runtime feature discovery endpoint.
//
// This route is intentionally unauthenticated: it returns only which modules are
// mounted, never data or configuration values, and the admin panel needs it
// before a user has logged in so the login shell can build its navigation.
type FeatureHandler struct {
	payload config.FeatureResponse
}

// NewFeatureHandler snapshots the feature set once at wiring time. Flags come
// from process configuration and cannot change without a restart, so there is no
// reason to recompute per request.
func NewFeatureHandler(cfg *config.Config) *FeatureHandler {
	return &FeatureHandler{payload: cfg.FeatureResponse()}
}

// List godoc
// @Summary List enabled backend features
// @Description Returns which feature-flagged modules this deployment mounted so clients can hide unavailable navigation instead of surfacing 404s. Unauthenticated by design; exposes no data or secrets.
// @Tags Features
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /features [get]
func (h *FeatureHandler) List(c *gin.Context) {
	response.JSON(c, http.StatusOK, h.payload, nil)
}
