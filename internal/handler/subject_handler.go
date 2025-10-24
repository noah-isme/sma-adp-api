package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// SubjectHandler handles subject endpoints.
type SubjectHandler struct {
	service *service.SubjectService
}

// NewSubjectHandler constructs a subject handler.
func NewSubjectHandler(svc *service.SubjectService) *SubjectHandler {
	return &SubjectHandler{service: svc}
}

// List godoc
// @Summary List subjects
// @Tags Subjects
// @Produce json
// @Param track query string false "Filter by track"
// @Param group query string false "Filter by group"
// @Param search query string false "Search keyword"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Router /subjects [get]
func (h *SubjectHandler) List(c *gin.Context) {
	var filter models.SubjectFilter
	filter.Track = c.Query("track")
	filter.Group = c.Query("group")
	filter.Search = strings.TrimSpace(c.Query("search"))
	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		filter.Page = page
	}
	if limit, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil {
		filter.PageSize = limit
	}
	filter.SortBy = c.Query("sort")
	filter.SortOrder = c.Query("order")

	subjects, pagination, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, subjects, pagination)
}

// Get godoc
// @Summary Get subject by id
// @Tags Subjects
// @Produce json
// @Param id path string true "Subject ID"
// @Success 200 {object} response.Envelope
// @Router /subjects/{id} [get]
func (h *SubjectHandler) Get(c *gin.Context) {
	subject, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, subject, nil)
}

// Create godoc
// @Summary Create subject
// @Tags Subjects
// @Accept json
// @Produce json
// @Param payload body service.CreateSubjectRequest true "Subject payload"
// @Success 201 {object} response.Envelope
// @Router /subjects [post]
func (h *SubjectHandler) Create(c *gin.Context) {
	var req service.CreateSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	subject, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, subject)
}

// Update godoc
// @Summary Update subject
// @Tags Subjects
// @Accept json
// @Produce json
// @Param id path string true "Subject ID"
// @Param payload body service.UpdateSubjectRequest true "Subject payload"
// @Success 200 {object} response.Envelope
// @Router /subjects/{id} [put]
func (h *SubjectHandler) Update(c *gin.Context) {
	var req service.UpdateSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	subject, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, subject, nil)
}

// Delete godoc
// @Summary Delete subject
// @Tags Subjects
// @Produce json
// @Param id path string true "Subject ID"
// @Success 204
// @Router /subjects/{id} [delete]
func (h *SubjectHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
