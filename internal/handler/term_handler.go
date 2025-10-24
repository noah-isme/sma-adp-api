package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// TermHandler exposes term endpoints.
type TermHandler struct {
	service *service.TermService
}

// NewTermHandler constructs a term handler.
func NewTermHandler(svc *service.TermService) *TermHandler {
	return &TermHandler{service: svc}
}

// List godoc
// @Summary List terms
// @Description List terms with filters
// @Tags Terms
// @Produce json
// @Param academicYear query string false "Filter by academic year"
// @Param type query string false "Filter by type"
// @Param isActive query bool false "Filter by active flag"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Router /terms [get]
func (h *TermHandler) List(c *gin.Context) {
	var filter models.TermFilter
	filter.AcademicYear = c.Query("academicYear")
	if termType := c.Query("type"); termType != "" {
		filter.Type = models.TermType(termType)
	}
	if isActive := c.Query("isActive"); isActive != "" {
		if val, err := strconv.ParseBool(isActive); err == nil {
			filter.IsActive = &val
		}
	}
	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		filter.Page = page
	}
	if size, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil {
		filter.PageSize = size
	}
	filter.SortBy = c.Query("sort")
	filter.SortOrder = c.Query("order")

	terms, pagination, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, terms, pagination)
}

// GetActive godoc
// @Summary Get active term
// @Tags Terms
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /terms/active [get]
func (h *TermHandler) GetActive(c *gin.Context) {
	term, err := h.service.GetActive(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, term, nil)
}

// Create godoc
// @Summary Create term
// @Tags Terms
// @Accept json
// @Produce json
// @Param payload body service.CreateTermRequest true "Term payload"
// @Success 201 {object} response.Envelope
// @Router /terms [post]
func (h *TermHandler) Create(c *gin.Context) {
	var req service.CreateTermRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	term, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, term)
}

// Update godoc
// @Summary Update term
// @Tags Terms
// @Accept json
// @Produce json
// @Param id path string true "Term ID"
// @Param payload body service.UpdateTermRequest true "Term payload"
// @Success 200 {object} response.Envelope
// @Router /terms/{id} [put]
func (h *TermHandler) Update(c *gin.Context) {
	var req service.UpdateTermRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	term, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, term, nil)
}

// SetActive godoc
// @Summary Set active term
// @Tags Terms
// @Accept json
// @Produce json
// @Param payload body service.SetActiveTermRequest true "Set active payload"
// @Success 200 {object} response.Envelope
// @Router /terms/set-active [post]
func (h *TermHandler) SetActive(c *gin.Context) {
	var req service.SetActiveTermRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	term, err := h.service.SetActive(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, term, nil)
}

// Delete godoc
// @Summary Delete term
// @Tags Terms
// @Produce json
// @Param id path string true "Term ID"
// @Success 204
// @Router /terms/{id} [delete]
func (h *TermHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
