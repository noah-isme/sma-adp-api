package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// GradeConfigHandler exposes grade configuration endpoints.
type GradeConfigHandler struct {
	configs *service.GradeConfigService
}

// NewGradeConfigHandler constructs handler.
func NewGradeConfigHandler(configs *service.GradeConfigService) *GradeConfigHandler {
	return &GradeConfigHandler{configs: configs}
}

// List godoc
// @Summary List grade configurations
// @Tags Grade Configs
// @Produce json
// @Param classId query string false "Filter by class"
// @Param subjectId query string false "Filter by subject"
// @Param termId query string false "Filter by term"
// @Success 200 {object} response.Envelope
// @Router /grade-configs [get]
func (h *GradeConfigHandler) List(c *gin.Context) {
	filter := models.FinalGradeFilter{ClassID: c.Query("classId"), SubjectID: c.Query("subjectId"), TermID: c.Query("termId")}
	configs, err := h.configs.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, configs, nil)
}

// Get godoc
// @Summary Get grade configuration
// @Tags Grade Configs
// @Produce json
// @Param id path string true "Config ID"
// @Success 200 {object} response.Envelope
// @Router /grade-configs/{id} [get]
func (h *GradeConfigHandler) Get(c *gin.Context) {
	config, err := h.configs.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, config, nil)
}

// Create godoc
// @Summary Create grade configuration
// @Tags Grade Configs
// @Accept json
// @Produce json
// @Param payload body service.CreateGradeConfigRequest true "Config payload"
// @Success 201 {object} response.Envelope
// @Router /grade-configs [post]
func (h *GradeConfigHandler) Create(c *gin.Context) {
	var req service.CreateGradeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	config, err := h.configs.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, config)
}

// Update godoc
// @Summary Update grade configuration
// @Tags Grade Configs
// @Accept json
// @Produce json
// @Param id path string true "Config ID"
// @Param payload body service.UpdateGradeConfigRequest true "Config payload"
// @Success 200 {object} response.Envelope
// @Router /grade-configs/{id} [put]
func (h *GradeConfigHandler) Update(c *gin.Context) {
	var req service.UpdateGradeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	config, err := h.configs.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, config, nil)
}

// Finalize godoc
// @Summary Finalize grade configuration
// @Tags Grade Configs
// @Produce json
// @Param id path string true "Config ID"
// @Success 200 {object} response.Envelope
// @Router /grade-configs/{id}/finalize [post]
func (h *GradeConfigHandler) Finalize(c *gin.Context) {
	config, err := h.configs.Finalize(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, config, nil)
}
