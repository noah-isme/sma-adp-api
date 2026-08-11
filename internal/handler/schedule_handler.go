package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// ScheduleHandler manages schedule endpoints.
type ScheduleHandler struct {
	service *service.ScheduleService
}

// NewScheduleHandler constructs handler.
func NewScheduleHandler(svc *service.ScheduleService) *ScheduleHandler {
	return &ScheduleHandler{service: svc}
}

// List godoc
// @Summary List schedules
// @Tags Schedules
// @Produce json
// @Param termId query string false "Filter by term"
// @Param classId query string false "Filter by class"
// @Param teacherId query string false "Filter by teacher"
// @Param dayOfWeek query string false "Filter by day"
// @Param timeSlot query string false "Filter by time slot"
// @Param room query string false "Filter by room"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /schedules [get]
func (h *ScheduleHandler) List(c *gin.Context) {
	var filter models.ScheduleFilter
	filter.TermID = c.Query("termId")
	filter.ClassID = c.Query("classId")
	filter.TeacherID = c.Query("teacherId")
	filter.DayOfWeek = strings.ToUpper(c.Query("dayOfWeek"))
	filter.TimeSlot = c.Query("timeSlot")
	filter.Room = c.Query("room")
	if page, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		filter.Page = page
	}
	if limit, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil {
		filter.PageSize = limit
	}
	filter.SortBy = c.Query("sort")
	filter.SortOrder = c.Query("order")

	schedules, pagination, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, schedules, pagination)
}

// ListByClass godoc
// @Summary List schedules by class
// @Tags Schedules
// @Produce json
// @Param id path string true "Class ID"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /classes/{id}/schedules [get]
func (h *ScheduleHandler) ListByClass(c *gin.Context) {
	schedules, err := h.service.ListByClass(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, schedules, nil)
}

// ListByTeacher godoc
// @Summary List schedules by teacher
// @Tags Schedules
// @Produce json
// @Param id path string true "Teacher ID"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /teachers/{id}/schedules [get]
func (h *ScheduleHandler) ListByTeacher(c *gin.Context) {
	schedules, err := h.service.ListByTeacher(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, schedules, nil)
}

// Create godoc
// @Summary Create schedule
// @Tags Schedules
// @Accept json
// @Produce json
// @Param payload body service.CreateScheduleRequest true "Schedule payload"
// @Success 201 {object} response.Envelope
// @Security BearerAuth
// @Router /schedules [post]
func (h *ScheduleHandler) Create(c *gin.Context) {
	var req service.CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	schedule, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, schedule)
}

// BulkCreate godoc
// @Summary Bulk create schedules
// @Tags Schedules
// @Accept json
// @Produce json
// @Param payload body service.BulkCreateSchedulesRequest true "Bulk payload"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /schedules/bulk [post]
func (h *ScheduleHandler) BulkCreate(c *gin.Context) {
	var req service.BulkCreateSchedulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	result, err := h.service.BulkCreate(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, result, nil)
}

// Update godoc
// @Summary Update schedule
// @Tags Schedules
// @Accept json
// @Produce json
// @Param id path string true "Schedule ID"
// @Param payload body service.UpdateScheduleRequest true "Schedule payload"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /schedules/{id} [put]
func (h *ScheduleHandler) Update(c *gin.Context) {
	var req service.UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}
	schedule, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, schedule, nil)
}

// Delete godoc
// @Summary Delete schedule
// @Tags Schedules
// @Produce json
// @Param id path string true "Schedule ID"
// @Success 204
// @Security BearerAuth
// @Router /schedules/{id} [delete]
func (h *ScheduleHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// ExportPDF godoc
// @Summary Export schedule grid as PDF
// @Tags Schedules
// @Produce application/pdf
// @Param class_id query string false "Class ID"
// @Param classId query string false "Class ID (camelCase)"
// @Param term_id query string false "Term ID"
// @Param termId query string false "Term ID (camelCase)"
// @Success 200 {file} binary
// @Failure 400 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Security BearerAuth
// @Router /schedules/export/pdf [get]
func (h *ScheduleHandler) ExportPDF(c *gin.Context) {
	classID := strings.TrimSpace(c.Query("class_id"))
	if classID == "" {
		classID = strings.TrimSpace(c.Query("classId"))
	}
	termID := strings.TrimSpace(c.Query("term_id"))
	if termID == "" {
		termID = strings.TrimSpace(c.Query("termId"))
	}

	if classID == "" || termID == "" {
		response.Error(c, appErrors.Wrap(errors.New("class_id and term_id are required"), appErrors.ErrValidation.Code, http.StatusBadRequest, "class_id and term_id query parameters are required"))
		return
	}

	pdfBytes, filename, err := h.service.ExportPDF(c.Request.Context(), classID, termID)
	if err != nil {
		response.Error(c, err)
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

