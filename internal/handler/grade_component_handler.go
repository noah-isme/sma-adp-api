package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// GradeComponentHandler exposes grade component endpoints.
type GradeComponentHandler struct {
	components *service.GradeComponentService
}

// NewGradeComponentHandler constructs handler.
func NewGradeComponentHandler(components *service.GradeComponentService) *GradeComponentHandler {
	return &GradeComponentHandler{components: components}
}

// List godoc
// @Summary List grade components
// @Tags Grade Components
// @Produce json
// @Param search query string false "Search by code or name"
// @Success 200 {object} response.Envelope
// @Router /grade-components [get]
func (h *GradeComponentHandler) List(c *gin.Context) {
	components, err := h.components.List(c.Request.Context(), c.Query("search"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, components, nil)
}

// Create godoc
// @Summary Create grade component
// @Tags Grade Components
// @Accept json
// @Produce json
// @Param payload body service.CreateGradeComponentRequest true "Component payload"
// @Success 201 {object} response.Envelope
// @Router /grade-components [post]
func (h *GradeComponentHandler) Create(c *gin.Context) {
	var req service.CreateGradeComponentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	component, err := h.components.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, component)
}
