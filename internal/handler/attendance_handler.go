package handler

import (
	"context"
	"net/http"
	"strings"

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
	ListSubject(ctx context.Context, req service.SubjectAttendanceListRequest) ([]models.SubjectAttendanceRecord, *models.Pagination, error)
	GetSubject(ctx context.Context, id string) (*models.SubjectAttendanceRecord, error)
	DeleteSubject(ctx context.Context, id string) error
	SubjectSummary(ctx context.Context, req service.SubjectAttendanceListRequest) (*models.SubjectAttendanceSummary, error)
}

// AttendanceHandler handles CRUD operations for daily and subject attendance.
type AttendanceHandler struct {
	service attendanceService
}

// LegacyUpsert keeps the generic /attendance CRUD contract used by the admin
// panel. Legacy records do not carry a schedule_id, so they map to the daily
// attendance store, whose upsert semantics make POST and PUT idempotent.
// @Summary Compatibility attendance upsert
// @Tags Attendance
// @Accept json
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /attendance [post]
// @Router /attendance/{id} [put]
// @Router /attendance/{id} [patch]
func (h *AttendanceHandler) LegacyUpsert(c *gin.Context) {
	var payload struct {
		EnrollmentID string `json:"enrollment_id" binding:"required"`
		Date         string `json:"date" binding:"required"`
		Status       string `json:"status" binding:"required"`
		Note         string `json:"note"`
		Notes        string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, appErrors.Wrap(err, appErrors.ErrValidation.Code, http.StatusBadRequest, "invalid attendance payload"))
		return
	}
	note := payload.Note
	if note == "" {
		note = payload.Notes
	}
	var notes *string
	if note != "" {
		notes = &note
	}
	record, err := h.service.MarkDaily(c.Request.Context(), service.MarkDailyAttendanceRequest{EnrollmentID: payload.EnrollmentID, Date: payload.Date, Status: payload.Status, Notes: notes})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, record, nil)
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

// ListSubject godoc
// @Summary List lesson (per-subject) attendance records
// @Description Reads session attendance. Pin one session with scheduleId, or query a class-subject range with classId/subjectId plus dateFrom/dateTo.
// @Tags Attendance
// @Produce json
// @Param scheduleId query string false "Schedule (session) ID"
// @Param classId query string false "Class ID"
// @Param subjectId query string false "Subject ID"
// @Param termId query string false "Term ID"
// @Param enrollmentId query string false "Enrollment ID"
// @Param studentId query string false "Student ID"
// @Param date query string false "Exact date (YYYY-MM-DD)"
// @Param dateFrom query string false "From date (YYYY-MM-DD)"
// @Param dateTo query string false "To date (YYYY-MM-DD)"
// @Param status query string false "Attendance status (H/S/I/A)"
// @Param page query int false "Page"
// @Param limit query int false "Page size (max 200)"
// @Param sortBy query string false "Sort field (date, created_at, student_name, status)"
// @Param sortOrder query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Envelope
// @Router /attendance/subject [get]
func (h *AttendanceHandler) ListSubject(c *gin.Context) {
	req, err := subjectAttendanceRequestFromQuery(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	rows, pagination, err := h.service.ListSubject(c.Request.Context(), *req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, rows, pagination)
}

// SubjectSummary godoc
// @Summary Summarise lesson attendance counts
// @Description Aggregates H/S/I/A counts and attendance percentage for the same filters accepted by the list endpoint.
// @Tags Attendance
// @Produce json
// @Param scheduleId query string false "Schedule (session) ID"
// @Param classId query string false "Class ID"
// @Param subjectId query string false "Subject ID"
// @Param termId query string false "Term ID"
// @Param studentId query string false "Student ID"
// @Param dateFrom query string false "From date (YYYY-MM-DD)"
// @Param dateTo query string false "To date (YYYY-MM-DD)"
// @Success 200 {object} response.Envelope
// @Router /attendance/subject/summary [get]
func (h *AttendanceHandler) SubjectSummary(c *gin.Context) {
	req, err := subjectAttendanceRequestFromQuery(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	summary, err := h.service.SubjectSummary(c.Request.Context(), *req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, summary, nil)
}

// GetSubject godoc
// @Summary Get one lesson attendance record
// @Tags Attendance
// @Produce json
// @Param id path string true "Subject attendance ID"
// @Success 200 {object} response.Envelope
// @Router /attendance/subject/{id} [get]
func (h *AttendanceHandler) GetSubject(c *gin.Context) {
	record, err := h.service.GetSubject(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, record, nil)
}

// DeleteSubject godoc
// @Summary Delete one lesson attendance record
// @Description Used when a session is cancelled or a row was recorded against the wrong lesson.
// @Tags Attendance
// @Produce json
// @Param id path string true "Subject attendance ID"
// @Success 204
// @Router /attendance/subject/{id} [delete]
func (h *AttendanceHandler) DeleteSubject(c *gin.Context) {
	if err := h.service.DeleteSubject(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// subjectAttendanceRequestFromQuery maps query parameters onto the service
// request, accepting both camelCase (admin panel) and snake_case spellings.
func subjectAttendanceRequestFromQuery(c *gin.Context) (*service.SubjectAttendanceListRequest, error) {
	req := service.SubjectAttendanceListRequest{
		ScheduleID:   queryAny(c, "scheduleId", "schedule_id"),
		ClassID:      queryAny(c, "classId", "class_id"),
		SubjectID:    queryAny(c, "subjectId", "subject_id"),
		TermID:       queryAny(c, "termId", "term_id"),
		EnrollmentID: queryAny(c, "enrollmentId", "enrollment_id"),
		StudentID:    queryAny(c, "studentId", "student_id"),
		Page:         parseQueryInt(c, "page", 1),
		PageSize:     parseQueryInt(c, "limit", 50),
		SortBy:       queryAny(c, "sortBy", "sort_by"),
		SortOrder:    strings.ToLower(queryAny(c, "sortOrder", "sort_order")),
	}
	if status := queryAny(c, "status", "status"); status != "" {
		req.Status = &status
	}

	date, err := parseDateParam(c.Query("date"))
	if err != nil {
		return nil, err
	}
	from, err := parseDateParam(queryAny(c, "dateFrom", "date_from"))
	if err != nil {
		return nil, err
	}
	to, err := parseDateParam(queryAny(c, "dateTo", "date_to"))
	if err != nil {
		return nil, err
	}
	req.Date = date
	req.DateFrom = from
	req.DateTo = to
	return &req, nil
}

// queryAny returns the first non-empty value among the provided query keys.
func queryAny(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return ""
}
