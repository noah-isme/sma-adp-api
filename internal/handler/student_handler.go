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

// StudentHandler exposes student endpoints.
type StudentHandler struct {
	students *service.StudentService
}

// NewStudentHandler constructs StudentHandler.
func NewStudentHandler(students *service.StudentService) *StudentHandler {
	return &StudentHandler{students: students}
}

// List godoc
// @Summary List students
// @Tags Students
// @Produce json
// @Param search query string false "Search by name or NIS"
// @Param classId query string false "Filter by class"
// @Param active query bool false "Filter by active state"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Router /students [get]
func (h *StudentHandler) List(c *gin.Context) {
	var filter models.StudentFilter
	filter.Search = strings.TrimSpace(c.Query("search"))
	filter.ClassID = c.Query("classId")
	if active := c.Query("active"); active != "" {
		if active == "true" {
			v := true
			filter.Active = &v
		} else if active == "false" {
			v := false
			filter.Active = &v
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

	students, pagination, err := h.students.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, students, pagination)
}

// Get godoc
// @Summary Get student detail
// @Tags Students
// @Produce json
// @Param id path string true "Student ID"
// @Success 200 {object} response.Envelope
// @Router /students/{id} [get]
func (h *StudentHandler) Get(c *gin.Context) {
	student, err := h.students.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, student, nil)
}

// Create godoc
// @Summary Create student
// @Tags Students
// @Accept json
// @Produce json
// @Param payload body service.CreateStudentRequest true "Student payload"
// @Success 201 {object} response.Envelope
// @Router /students [post]
func (h *StudentHandler) Create(c *gin.Context) {
	var req service.CreateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	student, err := h.students.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, student)
}

// Update godoc
// @Summary Update student
// @Tags Students
// @Accept json
// @Produce json
// @Param id path string true "Student ID"
// @Param payload body service.UpdateStudentRequest true "Student payload"
// @Success 200 {object} response.Envelope
// @Router /students/{id} [put]
func (h *StudentHandler) Update(c *gin.Context) {
	var req service.UpdateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	student, err := h.students.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, student, nil)
}

// Delete godoc
// @Summary Deactivate student
// @Tags Students
// @Produce json
// @Param id path string true "Student ID"
// @Success 204
// @Router /students/{id} [delete]
func (h *StudentHandler) Delete(c *gin.Context) {
	if err := h.students.Deactivate(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
