package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type configurationService interface {
	List(ctx context.Context) ([]dto.ConfigurationItem, error)
	Get(ctx context.Context, key string) (*dto.ConfigurationItem, error)
	Update(ctx context.Context, key, value string, actor *models.JWTClaims) (*dto.ConfigurationItem, error)
	BulkUpdate(ctx context.Context, req dto.BulkUpdateConfigurationRequest, actor *models.JWTClaims) ([]dto.ConfigurationItem, error)
}

// ConfigurationHandler exposes configuration endpoints.
type ConfigurationHandler struct {
	service configurationService
}

// NewConfigurationHandler builds a new handler.
func NewConfigurationHandler(service configurationService) *ConfigurationHandler {
	return &ConfigurationHandler{service: service}
}

// List godoc
// @Summary List configurations
// @Tags Configuration
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /configuration [get]
func (h *ConfigurationHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, items, nil)
}

// Get godoc
// @Summary Get configuration by key
// @Tags Configuration
// @Produce json
// @Param key path string true "Configuration key"
// @Success 200 {object} response.Envelope
// @Router /configuration/{key} [get]
func (h *ConfigurationHandler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("key"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, item, nil)
}

// Update godoc
// @Summary Update configuration
// @Tags Configuration
// @Accept json
// @Produce json
// @Param key path string true "Configuration key"
// @Param payload body dto.UpdateConfigurationRequest true "Configuration payload"
// @Success 200 {object} response.Envelope
// @Router /configuration/{key} [put]
func (h *ConfigurationHandler) Update(c *gin.Context) {
	var req dto.UpdateConfigurationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid configuration payload"))
		return
	}
	if req.Key == "" {
		req.Key = c.Param("key")
	}
	if req.Key != c.Param("key") {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "key mismatch between path and body"))
		return
	}
	claims := claimsFromContext(c)
	item, err := h.service.Update(c.Request.Context(), req.Key, req.Value, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, item, nil)
}

// BulkUpdate godoc
// @Summary Bulk update configurations
// @Tags Configuration
// @Accept json
// @Produce json
// @Param payload body dto.BulkUpdateConfigurationRequest true "Bulk configuration payload"
// @Success 200 {object} response.Envelope
// @Router /configuration/bulk [put]
func (h *ConfigurationHandler) BulkUpdate(c *gin.Context) {
	var req dto.BulkUpdateConfigurationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid bulk payload"))
		return
	}
	claims := claimsFromContext(c)
	items, err := h.service.BulkUpdate(c.Request.Context(), req, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, items, nil)
}
