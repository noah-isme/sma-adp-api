package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// ClassSubjectHandler handles class subject assignments.
type ClassSubjectHandler struct {
	service *service.ClassService
}

// NewClassSubjectHandler constructs handler.
func NewClassSubjectHandler(service *service.ClassService) *ClassSubjectHandler {
	return &ClassSubjectHandler{service: service}
}

// List godoc
// @Summary List class subjects
// @Tags Class-Subjects
// @Produce json
// @Param id path string true "Class ID"
// @Success 200 {object} response.Envelope
// @Router /classes/{id}/subjects [get]
func (h *ClassSubjectHandler) List(c *gin.Context) {
	assignments, err := h.service.ListSubjects(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, assignments, nil)
}

// ListAll godoc
// @Summary List all class-subject assignments
// @Tags Class-Subjects
// @Produce json
// @Param classId query string false "Filter by class ID"
// @Param subjectId query string false "Filter by subject ID"
// @Param teacherId query string false "Filter by teacher ID"
// @Param termId query string false "Filter by term ID"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Param sortBy query string false "Sort by field"
// @Param sortOrder query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Envelope
// @Router /class-subjects [get]
func (h *ClassSubjectHandler) ListAll(c *gin.Context) {
	filter := models.ClassSubjectFilter{
		ClassID:   c.Query("classId"),
		SubjectID: c.Query("subjectId"),
		TeacherID: c.Query("teacherId"),
		TermID:    c.Query("termId"),
		Page:      parseQueryInt(c, "page", 1),
		PageSize:  parseQueryInt(c, "limit", 20),
		SortBy:    c.Query("sortBy"),
		SortOrder: strings.ToLower(c.Query("sortOrder")),
	}

	assignments, pagination, err := h.service.ListAll(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, assignments, pagination)
}

// Assign godoc
// @Summary Assign subjects to class
// @Tags Class-Subjects
// @Accept json
// @Produce json
// @Param id path string true "Class ID"
// @Param payload body service.AssignSubjectsRequest true "Assignments payload"
// @Success 200 {object} response.Envelope
// @Router /classes/{id}/subjects [post]
func (h *ClassSubjectHandler) Assign(c *gin.Context) {
	var req service.AssignSubjectsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	if err := h.service.AssignSubjects(c.Request.Context(), c.Param("id"), req); err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"status": "updated"}, nil)
}
