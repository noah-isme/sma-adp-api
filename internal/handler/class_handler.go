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

// ClassHandler exposes class CRUD endpoints.
type ClassHandler struct {
	service *service.ClassService
}

// NewClassHandler constructs a class handler.
func NewClassHandler(svc *service.ClassService) *ClassHandler {
	return &ClassHandler{service: svc}
}

// List godoc
// @Summary List classes
// @Tags Classes
// @Produce json
// @Param grade query string false "Filter by grade"
// @Param track query string false "Filter by track"
// @Param search query string false "Search keyword"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Router /classes [get]
func (h *ClassHandler) List(c *gin.Context) {
	var filter models.ClassFilter
	filter.Grade = c.Query("grade")
	filter.Track = c.Query("track")
	filter.Search = strings.TrimSpace(c.Query("search"))
	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		filter.Page = page
	}
	if size, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil {
		filter.PageSize = size
	}
	filter.SortBy = c.Query("sort")
	filter.SortOrder = c.Query("order")

	classes, pagination, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, classes, pagination)
}

// Get godoc
// @Summary Get class detail
// @Tags Classes
// @Produce json
// @Param id path string true "Class ID"
// @Success 200 {object} response.Envelope
// @Router /classes/{id} [get]
func (h *ClassHandler) Get(c *gin.Context) {
	classDetail, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, classDetail, nil)
}

// Create godoc
// @Summary Create class
// @Tags Classes
// @Accept json
// @Produce json
// @Param payload body service.CreateClassRequest true "Class payload"
// @Success 201 {object} response.Envelope
// @Router /classes [post]
func (h *ClassHandler) Create(c *gin.Context) {
	var req service.CreateClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	class, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, class)
}

// Update godoc
// @Summary Update class
// @Tags Classes
// @Accept json
// @Produce json
// @Param id path string true "Class ID"
// @Param payload body service.UpdateClassRequest true "Class payload"
// @Success 200 {object} response.Envelope
// @Router /classes/{id} [put]
func (h *ClassHandler) Update(c *gin.Context) {
	var req service.UpdateClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	class, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, class, nil)
}

// Delete godoc
// @Summary Delete class
// @Tags Classes
// @Produce json
// @Param id path string true "Class ID"
// @Success 204
// @Router /classes/{id} [delete]
func (h *ClassHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
