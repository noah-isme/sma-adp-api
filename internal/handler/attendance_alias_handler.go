package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type attendanceAliasService interface {
	ListDaily(ctx context.Context, req dto.AttendanceDailyRequest, claims *models.JWTClaims) ([]models.DailyAttendanceRecord, *models.Pagination, error)
	Summary(ctx context.Context, req dto.AttendanceSummaryRequest, claims *models.JWTClaims) (*dto.AttendanceSummaryResponse, bool, error)
}

// AttendanceAliasHandler exposes /attendance and /attendance/daily adapters.
type AttendanceAliasHandler struct {
	service attendanceAliasService
}

// NewAttendanceAliasHandler constructs the handler.
func NewAttendanceAliasHandler(service attendanceAliasService) *AttendanceAliasHandler {
	return &AttendanceAliasHandler{service: service}
}

// Daily godoc
// @Summary Daily attendance alias endpoint
// @Tags Attendance
// @Produce json
// @Param termId query string true "Term ID"
// @Param classId query string false "Class ID"
// @Param studentId query string false "Student ID"
// @Param status query string false "Attendance status (H/S/I/A)"
// @Param dateFrom query string false "From date (YYYY-MM-DD)"
// @Param dateTo query string false "To date (YYYY-MM-DD)"
// @Param page query int false "Page"
// @Param limit query int false "Page size"
// @Param sortBy query string false "Sort by field"
// @Param sortOrder query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Envelope
// @Router /attendance/daily [get]
func (h *AttendanceAliasHandler) Daily(c *gin.Context) {
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	req := dto.AttendanceDailyRequest{
		TermID:    c.Query("termId"),
		ClassID:   c.Query("classId"),
		StudentID: c.Query("studentId"),
		SortBy:    c.Query("sortBy"),
		SortOrder: c.Query("sortOrder"),
		Page:      parseQueryInt(c, "page", 1),
		PageSize:  parseQueryInt(c, "limit", 50),
	}
	if status := c.Query("status"); status != "" {
		req.Status = &status
	}
	from, err := parseDateParam(c.Query("dateFrom"))
	if err != nil {
		response.Error(c, err)
		return
	}
	to, err := parseDateParam(c.Query("dateTo"))
	if err != nil {
		response.Error(c, err)
		return
	}
	req.DateFrom = from
	req.DateTo = to

	rows, pagination, err := h.service.ListDaily(c.Request.Context(), req, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, rows, pagination)
}

// Summary godoc
// @Summary Attendance summary alias endpoint
// @Tags Attendance
// @Produce json
// @Param termId query string true "Term ID"
// @Param classId query string false "Class ID"
// @Param studentId query string false "Student ID"
// @Param from query string false "From date (YYYY-MM-DD)"
// @Param to query string false "To date (YYYY-MM-DD)"
// @Success 200 {object} response.Envelope
// @Router /attendance [get]
func (h *AttendanceAliasHandler) Summary(c *gin.Context) {
	claims := claimsFromContext(c)
	if claims == nil {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	req := dto.AttendanceSummaryRequest{
		TermID:    c.Query("termId"),
		ClassID:   c.Query("classId"),
		StudentID: c.Query("studentId"),
	}
	from, err := parseDateParam(c.Query("from"))
	if err != nil {
		response.Error(c, err)
		return
	}
	to, err := parseDateParam(c.Query("to"))
	if err != nil {
		response.Error(c, err)
		return
	}
	req.FromDate = from
	req.ToDate = to

	start := time.Now()
	summary, cacheHit, err := h.service.Summary(c.Request.Context(), req, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	meta := map[string]interface{}{
		"cache_hit":          cacheHit,
		"processing_time_ms": time.Since(start).Milliseconds(),
	}
	response.JSON(c, http.StatusOK, summary, nil, meta)
}

func parseDateParam(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "invalid date, expected YYYY-MM-DD")
	}
	return &parsed, nil
}

func parseQueryInt(c *gin.Context, key string, def int) int {
	raw := c.Query(key)
	if raw == "" {
		return def
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return val
}
