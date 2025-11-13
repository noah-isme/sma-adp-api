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

type homeroomService interface {
	List(ctx context.Context, filter dto.HomeroomFilter, claims *models.JWTClaims) ([]dto.HomeroomItem, error)
	Get(ctx context.Context, classID, termID string, claims *models.JWTClaims) (*dto.HomeroomItem, error)
	Set(ctx context.Context, req dto.SetHomeroomRequest, actor *models.JWTClaims) (*dto.HomeroomItem, error)
}

// HomeroomHandler exposes homeroom management endpoints.
type HomeroomHandler struct {
	service homeroomService
}

// NewHomeroomHandler builds a new handler.
func NewHomeroomHandler(service homeroomService) *HomeroomHandler {
	return &HomeroomHandler{service: service}
}

// List godoc
// @Summary List homeroom assignments
// @Tags Homerooms
// @Produce json
// @Param termId query string false "Term ID (defaults to current active)"
// @Param classId query string false "Class ID filter"
// @Success 200 {object} response.Envelope
// @Router /homerooms [get]
func (h *HomeroomHandler) List(c *gin.Context) {
	claims := claimsFromContext(c)
	filter := dto.HomeroomFilter{
		TermID:  c.Query("termId"),
		ClassID: c.Query("classId"),
	}
	items, err := h.service.List(c.Request.Context(), filter, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, items, nil)
}

// Get godoc
// @Summary Get homeroom info for a class
// @Tags Homerooms
// @Produce json
// @Param classId path string true "Class ID"
// @Param termId query string false "Term ID (defaults to active)"
// @Success 200 {object} response.Envelope
// @Router /homerooms/{classId} [get]
func (h *HomeroomHandler) Get(c *gin.Context) {
	claims := claimsFromContext(c)
	classID := c.Param("classId")
	item, err := h.service.Get(c.Request.Context(), classID, c.Query("termId"), claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, item, nil)
}

// Set godoc
// @Summary Set or replace a homeroom teacher
// @Tags Homerooms
// @Accept json
// @Produce json
// @Param payload body dto.SetHomeroomRequest true "Homeroom payload"
// @Success 201 {object} response.Envelope
// @Router /homerooms [post]
func (h *HomeroomHandler) Set(c *gin.Context) {
	claims := claimsFromContext(c)
	var req dto.SetHomeroomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid homeroom payload"))
		return
	}
	item, err := h.service.Set(c.Request.Context(), req, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, item)
}
