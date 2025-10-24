package handler

import (
	"net/http"
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
