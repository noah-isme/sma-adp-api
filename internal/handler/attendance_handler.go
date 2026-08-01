package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type attendanceService interface {
	MarkDaily(ctx context.Context, req service.MarkDailyAttendanceRequest) (*models.DailyAttendance, error)
	BulkMarkDaily(ctx context.Context, req service.BulkMarkDailyAttendanceRequest) (*service.BulkAttendanceResult, error)
	MarkSubject(ctx context.Context, req service.MarkSubjectAttendanceRequest) (*models.SubjectAttendance, error)
	BulkMarkSubject(ctx context.Context, req service.BulkMarkSubjectAttendanceRequest) (*service.BulkAttendanceResult, error)
}

// AttendanceHandler handles CRUD operations for daily and subject attendance.
type AttendanceHandler struct {
	service attendanceService
}

// NewAttendanceHandler constructs the handler.
func NewAttendanceHandler(service attendanceService) *AttendanceHandler {
	return &AttendanceHandler{service: service}
}

// MarkDaily godoc
// @Summary Mark daily attendance for a student
// @Tags Attendance
// @Accept json
// @Produce json
// @Param payload body service.MarkDailyAttendanceRequest true "Attendance payload"
// @Success 200 {object} response.Envelope
// @Router /attendance/daily [post]
func (h *AttendanceHandler) MarkDaily(c *gin.Context) {
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	var req service.MarkDailyAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	record, err := h.service.MarkDaily(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, record, nil)
}

// BulkMarkDaily godoc
// @Summary Bulk mark daily attendance
// @Tags Attendance
// @Accept json
// @Produce json
// @Param payload body service.BulkMarkDailyAttendanceRequest true "Bulk attendance payload"
// @Success 200 {object} response.Envelope
// @Router /attendance/daily/bulk [post]
func (h *AttendanceHandler) BulkMarkDaily(c *gin.Context) {
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	var req service.BulkMarkDailyAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	result, err := h.service.BulkMarkDaily(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, result, nil)
}

// MarkSubject godoc
// @Summary Mark subject attendance for a student session
// @Tags Attendance
// @Accept json
// @Produce json
// @Param payload body service.MarkSubjectAttendanceRequest true "Subject attendance payload"
// @Success 200 {object} response.Envelope
// @Router /attendance/subject [post]
func (h *AttendanceHandler) MarkSubject(c *gin.Context) {
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	var req service.MarkSubjectAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	record, err := h.service.MarkSubject(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, record, nil)
}

// BulkMarkSubject godoc
// @Summary Bulk mark subject attendance
// @Tags Attendance
// @Accept json
// @Produce json
// @Param payload body service.BulkMarkSubjectAttendanceRequest true "Bulk subject attendance payload"
// @Success 200 {object} response.Envelope
// @Router /attendance/subject/bulk [post]
func (h *AttendanceHandler) BulkMarkSubject(c *gin.Context) {
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	var req service.BulkMarkSubjectAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid payload"))
		return
	}

	result, err := h.service.BulkMarkSubject(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, result, nil)
}