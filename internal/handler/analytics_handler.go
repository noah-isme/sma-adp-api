package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// AnalyticsHandler exposes dashboard-ready analytics endpoints.
type AnalyticsHandler struct {
	analytics *service.AnalyticsService
}

// NewAnalyticsHandler constructs the analytics handler.
func NewAnalyticsHandler(analytics *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{analytics: analytics}
}

// Attendance returns aggregated attendance data.
// @Summary Attendance analytics
// @Tags Analytics
// @Produce json
// @Success 200 {object} response.Envelope
// @Param term_id query string true "Term ID"
// @Param class_id query string true "Class ID"
// @Param date_from query string false "Date From (RFC3339)"
// @Param date_to query string false "Date To (RFC3339)"
// @Security BearerAuth
// @Router /analytics/attendance [get]
func (h *AnalyticsHandler) Attendance(c *gin.Context) {
	if h.analytics == nil {
		response.Error(c, appErrors.ErrInternal)
		return
	}
	filter, err := parseAttendanceFilter(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := requireAnalyticsTermQuery(filter.TermID); err != nil {
		response.Error(c, err)
		return
	}
	start := time.Now()
	summaries, cacheHit, err := h.analytics.Attendance(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	middleware.SetCacheHit(c, cacheHit)
	meta := middleware.ExtractMeta(c)
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["processing_time_ms"] = time.Since(start).Milliseconds()
	response.JSON(c, http.StatusOK, summaries, nil, meta)
}

// Grades returns aggregated grade analytics.
// @Summary Grade analytics
// @Tags Analytics
// @Produce json
// @Success 200 {object} response.Envelope
// @Param term_id query string true "Term ID"
// @Param class_id query string true "Class ID"
// @Param subject_id query string true "Subject ID"
// @Security BearerAuth
// @Router /analytics/grades [get]
func (h *AnalyticsHandler) Grades(c *gin.Context) {
	if h.analytics == nil {
		response.Error(c, appErrors.ErrInternal)
		return
	}
	filter := models.AnalyticsGradeFilter{
		TermID:    c.Query("term_id"),
		ClassID:   c.Query("class_id"),
		SubjectID: c.Query("subject_id"),
	}
	if err := requireAnalyticsTermQuery(filter.TermID); err != nil {
		response.Error(c, err)
		return
	}
	start := time.Now()
	summaries, cacheHit, err := h.analytics.Grades(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	middleware.SetCacheHit(c, cacheHit)
	meta := middleware.ExtractMeta(c)
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["processing_time_ms"] = time.Since(start).Milliseconds()
	response.JSON(c, http.StatusOK, summaries, nil, meta)
}

// Behavior returns behaviour analytics.
// @Summary Behavior analytics
// @Tags Analytics
// @Produce json
// @Success 200 {object} response.Envelope
// @Param term_id query string true "Term ID"
// @Param student_id query string true "Student ID"
// @Param class_id query string true "Class ID"
// @Param date_from query string false "Date From (RFC3339)"
// @Param date_to query string false "Date To (RFC3339)"
// @Security BearerAuth
// @Router /analytics/behavior [get]
func (h *AnalyticsHandler) Behavior(c *gin.Context) {
	if h.analytics == nil {
		response.Error(c, appErrors.ErrInternal)
		return
	}
	filter, err := parseBehaviorFilter(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := requireAnalyticsTermQuery(filter.TermID); err != nil {
		response.Error(c, err)
		return
	}
	start := time.Now()
	summaries, cacheHit, err := h.analytics.Behavior(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	middleware.SetCacheHit(c, cacheHit)
	meta := middleware.ExtractMeta(c)
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["processing_time_ms"] = time.Since(start).Milliseconds()
	response.JSON(c, http.StatusOK, summaries, nil, meta)
}

// System returns instrumentation metrics snapshots.
// @Summary System analytics
// @Tags Analytics
// @Produce json
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /analytics/system [get]
func (h *AnalyticsHandler) System(c *gin.Context) {
	if h.analytics == nil {
		response.Error(c, appErrors.ErrInternal)
		return
	}
	start := time.Now()
	metrics := h.analytics.SystemMetrics()
	middleware.SetCacheHit(c, false)
	meta := middleware.ExtractMeta(c)
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["processing_time_ms"] = time.Since(start).Milliseconds()
	response.JSON(c, http.StatusOK, metrics, nil, meta)
}

// Class returns class-level analytics for a required term.
// @Summary Class analytics drilldown
// @Tags Analytics
// @Produce json
// @Param class_id path string true "Class ID"
// @Param term_id query string true "Term ID"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /analytics/class/{class_id} [get]
func (h *AnalyticsHandler) Class(c *gin.Context) {
	if h.analytics == nil {
		response.Error(c, appErrors.ErrInternal)
		return
	}
	claims, err := analyticsClaims(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	if claims.Role == models.RoleStudent || claims.Role == models.RoleOrtu {
		response.Error(c, appErrors.ErrForbidden)
		return
	}
	classID := strings.TrimSpace(c.Param("class_id"))
	if classID == "" {
		classID = strings.TrimSpace(c.Param("id"))
	}
	termID := strings.TrimSpace(c.Query("term_id"))
	start := time.Now()
	result, cacheHit, err := h.analytics.ClassForClaims(c.Request.Context(), classID, termID, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	writeAnalyticsResponse(c, result, cacheHit, start)
}

// Student returns student-level analytics for a required term.
// @Summary Student analytics drilldown
// @Tags Analytics
// @Produce json
// @Param student_id path string true "Student ID"
// @Param term_id query string true "Term ID"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /analytics/student/{student_id} [get]
func (h *AnalyticsHandler) Student(c *gin.Context) {
	if h.analytics == nil {
		response.Error(c, appErrors.ErrInternal)
		return
	}
	claims, err := analyticsClaims(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	studentID := strings.TrimSpace(c.Param("student_id"))
	if studentID == "" {
		studentID = strings.TrimSpace(c.Param("id"))
	}
	if claims.Role == models.RoleStudent {
		if claims.StudentID == "" || claims.StudentID != studentID {
			response.Error(c, appErrors.Clone(appErrors.ErrForbidden, "students can only access their own analytics"))
			return
		}
	} else if claims.Role == models.RoleOrtu {
		response.Error(c, appErrors.ErrForbidden)
		return
	}
	termID := strings.TrimSpace(c.Query("term_id"))
	start := time.Now()
	result, cacheHit, err := h.analytics.StudentForClaims(c.Request.Context(), studentID, termID, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	writeAnalyticsResponse(c, result, cacheHit, start)
}

// Subject returns subject-level analytics for a required term.
// @Summary Subject analytics drilldown
// @Tags Analytics
// @Produce json
// @Param subject_id path string true "Subject ID"
// @Param term_id query string true "Term ID"
// @Param class_id query string false "Class ID"
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /analytics/subject/{subject_id} [get]
func (h *AnalyticsHandler) Subject(c *gin.Context) {
	if h.analytics == nil {
		response.Error(c, appErrors.ErrInternal)
		return
	}
	claims, err := analyticsClaims(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	if claims.Role == models.RoleStudent || claims.Role == models.RoleOrtu {
		response.Error(c, appErrors.ErrForbidden)
		return
	}
	subjectID := strings.TrimSpace(c.Param("subject_id"))
	if subjectID == "" {
		subjectID = strings.TrimSpace(c.Param("id"))
	}
	classID := strings.TrimSpace(c.Query("class_id"))
	termID := strings.TrimSpace(c.Query("term_id"))
	start := time.Now()
	result, cacheHit, err := h.analytics.SubjectForClaims(c.Request.Context(), subjectID, classID, termID, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	writeAnalyticsResponse(c, result, cacheHit, start)
}

// LeaderboardGPA returns the highest GPA students.
// @Summary GPA leaderboard
// @Tags Analytics
// @Produce json
// @Param term_id query string true "Term ID"
// @Param class_id query string false "Class ID"
// @Param limit query int false "Number of rows (1-100)" default(10)
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /analytics/leaderboard/gpa [get]
func (h *AnalyticsHandler) LeaderboardGPA(c *gin.Context) {
	h.leaderboard(c, "gpa")
}

// LeaderboardAttendance returns the highest attendance students.
// @Summary Attendance leaderboard
// @Tags Analytics
// @Produce json
// @Param term_id query string true "Term ID"
// @Param class_id query string false "Class ID"
// @Param limit query int false "Number of rows (1-100)" default(10)
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /analytics/leaderboard/attendance [get]
func (h *AnalyticsHandler) LeaderboardAttendance(c *gin.Context) {
	h.leaderboard(c, "attendance")
}

// LeaderboardBehavior returns the highest behaviour points students.
// @Summary Behavior leaderboard
// @Tags Analytics
// @Produce json
// @Param term_id query string true "Term ID"
// @Param class_id query string false "Class ID"
// @Param limit query int false "Number of rows (1-100)" default(10)
// @Success 200 {object} response.Envelope
// @Security BearerAuth
// @Router /analytics/leaderboard/behavior [get]
func (h *AnalyticsHandler) LeaderboardBehavior(c *gin.Context) {
	h.leaderboard(c, "behavior")
}

func (h *AnalyticsHandler) leaderboard(c *gin.Context, metric string) {
	if h.analytics == nil {
		response.Error(c, appErrors.ErrInternal)
		return
	}
	claims, err := analyticsClaims(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	if claims.Role == models.RoleStudent || claims.Role == models.RoleOrtu {
		response.Error(c, appErrors.ErrForbidden)
		return
	}
	limit, err := parseAnalyticsLimit(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	termID := strings.TrimSpace(c.Query("term_id"))
	classID := strings.TrimSpace(c.Query("class_id"))
	if claims.Role == models.RoleTeacher && classID == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrForbidden, "teachers must scope leaderboards to a class"))
		return
	}
	start := time.Now()
	result, cacheHit, err := h.analytics.LeaderboardForClaims(c.Request.Context(), metric, models.AnalyticsLeaderboardFilter{
		TermID:  termID,
		ClassID: classID,
		Limit:   limit,
	}, claims)
	if err != nil {
		response.Error(c, err)
		return
	}
	writeAnalyticsResponse(c, result, cacheHit, start)
}

func analyticsClaims(c *gin.Context) (*models.JWTClaims, error) {
	value, ok := c.Get(middleware.ContextUserKey)
	if !ok {
		return nil, appErrors.ErrUnauthorized
	}
	claims, ok := value.(*models.JWTClaims)
	if !ok || claims == nil {
		return nil, appErrors.ErrUnauthorized
	}
	return claims, nil
}

func parseAnalyticsLimit(c *gin.Context) (int, error) {
	raw := strings.TrimSpace(c.Query("limit"))
	if raw == "" {
		return 10, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > 100 {
		return 0, appErrors.Clone(appErrors.ErrValidation, "limit must be between 1 and 100")
	}
	return limit, nil
}

func requireAnalyticsTermQuery(termID string) error {
	if strings.TrimSpace(termID) == "" {
		return appErrors.Clone(appErrors.ErrValidation, "term_id is required")
	}
	return nil
}

func writeAnalyticsResponse(c *gin.Context, data interface{}, cacheHit bool, start time.Time) {
	middleware.SetCacheHit(c, cacheHit)
	meta := middleware.ExtractMeta(c)
	if meta == nil {
		meta = map[string]interface{}{}
	}
	meta["processing_time_ms"] = time.Since(start).Milliseconds()
	response.JSON(c, http.StatusOK, data, nil, meta)
}

func parseAttendanceFilter(c *gin.Context) (models.AnalyticsAttendanceFilter, error) {
	filter := models.AnalyticsAttendanceFilter{
		TermID:  c.Query("term_id"),
		ClassID: c.Query("class_id"),
	}
	if raw := c.Query("date_from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, appErrors.Clone(appErrors.ErrValidation, "invalid date_from parameter")
		}
		filter.DateFrom = &parsed
	}
	if raw := c.Query("date_to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, appErrors.Clone(appErrors.ErrValidation, "invalid date_to parameter")
		}
		filter.DateTo = &parsed
	}
	return filter, nil
}

func parseBehaviorFilter(c *gin.Context) (models.AnalyticsBehaviorFilter, error) {
	filter := models.AnalyticsBehaviorFilter{
		TermID:    c.Query("term_id"),
		StudentID: c.Query("student_id"),
		ClassID:   c.Query("class_id"),
	}
	if raw := c.Query("date_from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, appErrors.Clone(appErrors.ErrValidation, "invalid date_from parameter")
		}
		filter.DateFrom = &parsed
	}
	if raw := c.Query("date_to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, appErrors.Clone(appErrors.ErrValidation, "invalid date_to parameter")
		}
		filter.DateTo = &parsed
	}
	return filter, nil
}
