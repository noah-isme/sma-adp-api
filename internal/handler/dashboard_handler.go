package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type dashboardService interface {
	Admin(ctx context.Context, termID string) (*dto.AdminDashboardResponse, bool, error)
	Teacher(ctx context.Context, teacherID, termID string, date time.Time) (*dto.TeacherDashboardResponse, bool, error)
}

// DashboardHandler wires dashboard service to HTTP endpoints.
type DashboardHandler struct {
	service dashboardService
}

// NewDashboardHandler constructs the handler.
func NewDashboardHandler(service dashboardService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// Admin godoc
// @Summary Admin dashboard summary
// @Tags Dashboard
// @Produce json
// @Param termId query string true "Term ID"
// @Success 200 {object} response.Envelope
// @Router /dashboard [get]
func (h *DashboardHandler) Admin(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.ErrInternal)
		return
	}
	termID := strings.TrimSpace(c.Query("termId"))
	if termID == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "termId is required"))
		return
	}
	start := time.Now()
	summary, cacheHit, err := h.service.Admin(c.Request.Context(), termID)
	if err != nil {
		response.Error(c, err)
		return
	}
	middleware.SetCacheHit(c, cacheHit)
	meta := middleware.ExtractMeta(c)
	if meta == nil {
		meta = map[string]interface{}{}
	}
	meta["processing_time_ms"] = time.Since(start).Milliseconds()
	response.JSON(c, http.StatusOK, summary, nil, meta)
}

// Teacher godoc
// @Summary Teacher academics dashboard
// @Tags Dashboard
// @Produce json
// @Param termId query string true "Term ID"
// @Param date query string false "Date (YYYY-MM-DD). Defaults to today"
// @Success 200 {object} response.Envelope
// @Router /dashboard/academics [get]
func (h *DashboardHandler) Teacher(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.ErrInternal)
		return
	}
	claimsValue, exists := c.Get(middleware.ContextUserKey)
	if !exists {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	claims, ok := claimsValue.(*models.JWTClaims)
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	termID := strings.TrimSpace(c.Query("termId"))
	if termID == "" {
		response.Error(c, appErrors.Clone(appErrors.ErrValidation, "termId is required"))
		return
	}
	dateStr := strings.TrimSpace(c.Query("date"))
	var date time.Time
	if dateStr == "" {
		date = time.Now().UTC()
	} else {
		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			response.Error(c, appErrors.Clone(appErrors.ErrValidation, "invalid date format, expected YYYY-MM-DD"))
			return
		}
		date = parsed
	}
	start := time.Now()
	summary, cacheHit, err := h.service.Teacher(c.Request.Context(), claims.UserID, termID, date)
	if err != nil {
		response.Error(c, err)
		return
	}
	middleware.SetCacheHit(c, cacheHit)
	meta := middleware.ExtractMeta(c)
	if meta == nil {
		meta = map[string]interface{}{}
	}
	meta["processing_time_ms"] = time.Since(start).Milliseconds()
	response.JSON(c, http.StatusOK, summary, nil, meta)
}
