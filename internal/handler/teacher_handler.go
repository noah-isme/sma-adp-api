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

// TeacherHandler wires teacher services to HTTP routes.
type TeacherHandler struct {
	teachers    *service.TeacherService
	assignments *service.TeacherAssignmentService
	prefs       *service.TeacherPreferenceService
}

// NewTeacherHandler constructs a new TeacherHandler.
func NewTeacherHandler(teachers *service.TeacherService, assignments *service.TeacherAssignmentService, prefs *service.TeacherPreferenceService) *TeacherHandler {
	return &TeacherHandler{
		teachers:    teachers,
		assignments: assignments,
		prefs:       prefs,
	}
}

// List godoc
// @Summary List teachers
// @Tags Teachers
// @Produce json
// @Param search query string false "Search by name/email/NIP"
// @Param active query bool false "Filter by active status"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Param sort query string false "Sort field (full_name,email,created_at)"
// @Param order query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Envelope
// @Router /teachers [get]
func (h *TeacherHandler) List(c *gin.Context) {
	filter := models.TeacherFilter{
		Search:    strings.TrimSpace(c.Query("search")),
		SortBy:    c.Query("sort"),
		SortOrder: c.Query("order"),
	}
	if active := c.Query("active"); active != "" {
		switch strings.ToLower(active) {
		case "true":
			val := true
			filter.Active = &val
		case "false":
			val := false
			filter.Active = &val
		}
	}
	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		filter.Page = page
	}
	if size, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil {
		filter.PageSize = size
	}

	teachers, pagination, err := h.teachers.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, teachers, pagination)
}

// Get godoc
// @Summary Get teacher detail
// @Tags Teachers
// @Produce json
// @Param id path string true "Teacher ID"
// @Success 200 {object} response.Envelope
// @Router /teachers/{id} [get]
func (h *TeacherHandler) Get(c *gin.Context) {
	teacher, err := h.teachers.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, teacher, nil)
}

// Create godoc
// @Summary Create teacher
// @Tags Teachers
// @Accept json
// @Produce json
// @Param payload body service.CreateTeacherRequest true "Teacher payload"
// @Success 201 {object} response.Envelope
// @Router /teachers [post]
func (h *TeacherHandler) Create(c *gin.Context) {
	var req service.CreateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid teacher payload"))
		return
	}
	teacher, err := h.teachers.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, teacher)
}

// Update godoc
// @Summary Update teacher
// @Tags Teachers
// @Accept json
// @Produce json
// @Param id path string true "Teacher ID"
// @Param payload body service.UpdateTeacherRequest true "Teacher payload"
// @Success 200 {object} response.Envelope
// @Router /teachers/{id} [put]
func (h *TeacherHandler) Update(c *gin.Context) {
	var req service.UpdateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid teacher payload"))
		return
	}
	teacher, err := h.teachers.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, teacher, nil)
}

// Delete godoc
// @Summary Deactivate teacher
// @Tags Teachers
// @Param id path string true "Teacher ID"
// @Success 204
// @Router /teachers/{id} [delete]
func (h *TeacherHandler) Delete(c *gin.Context) {
	if err := h.teachers.Deactivate(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// ListAssignments godoc
// @Summary List teacher assignments
// @Tags Teacher Assignments
// @Param id path string true "Teacher ID"
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /teachers/{id}/assignments [get]
func (h *TeacherHandler) ListAssignments(c *gin.Context) {
	assignments, err := h.assignments.ListByTeacher(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, assignments, nil)
}

// CreateAssignment godoc
// @Summary Create teacher assignment
// @Tags Teacher Assignments
// @Accept json
// @Produce json
// @Param id path string true "Teacher ID"
// @Param payload body service.CreateTeacherAssignmentRequest true "Assignment payload"
// @Success 201 {object} response.Envelope
// @Router /teachers/{id}/assignments [post]
func (h *TeacherHandler) CreateAssignment(c *gin.Context) {
	var req service.CreateTeacherAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid assignment payload"))
		return
	}
	assignment, err := h.assignments.Assign(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, assignment)
}

// DeleteAssignment godoc
// @Summary Delete teacher assignment
// @Tags Teacher Assignments
// @Param id path string true "Teacher ID"
// @Param aid path string true "Assignment ID"
// @Success 204
// @Router /teachers/{id}/assignments/{aid} [delete]
func (h *TeacherHandler) DeleteAssignment(c *gin.Context) {
	if err := h.assignments.Remove(c.Request.Context(), c.Param("id"), c.Param("aid")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// GetPreferences godoc
// @Summary Get teacher preferences
// @Tags Teacher Preferences
// @Param id path string true "Teacher ID"
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /teachers/{id}/preferences [get]
func (h *TeacherHandler) GetPreferences(c *gin.Context) {
	pref, err := h.prefs.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, pref, nil)
}

// UpsertPreferences godoc
// @Summary Upsert teacher preferences
// @Tags Teacher Preferences
// @Accept json
// @Produce json
// @Param id path string true "Teacher ID"
// @Param payload body service.UpsertTeacherPreferenceRequest true "Preference payload"
// @Success 200 {object} response.Envelope
// @Router /teachers/{id}/preferences [put]
func (h *TeacherHandler) UpsertPreferences(c *gin.Context) {
	var req service.UpsertTeacherPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid preference payload"))
		return
	}
	pref, err := h.prefs.Upsert(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, pref, nil)
}
