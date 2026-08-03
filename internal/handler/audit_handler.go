package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

type auditService interface {
	List(ctx context.Context, req service.AuditLogListRequest) ([]models.AuditLogEntry, *models.Pagination, error)
	Get(ctx context.Context, id string) (*models.AuditLogEntry, error)
	Facets(ctx context.Context) (*models.AuditLogFacets, error)
}

// AuditHandler exposes read-only audit trail endpoints.
type AuditHandler struct {
	service auditService
}

// NewAuditHandler constructs the handler.
func NewAuditHandler(service auditService) *AuditHandler {
	return &AuditHandler{service: service}
}

// List godoc
// @Summary List audit log entries
// @Description Read-only audit trail viewer. Supports filtering by actor, action, resource, and date range.
// @Tags Audit
// @Produce json
// @Param userId query string false "Filter by acting user ID"
// @Param action query string false "Filter by action (e.g. LOGIN, USER_UPDATE)"
// @Param resource query string false "Filter by resource name"
// @Param resourceId query string false "Filter by affected resource ID"
// @Param search query string false "Free-text search across action, resource, and actor"
// @Param dateFrom query string false "From timestamp (YYYY-MM-DD or RFC3339)"
// @Param dateTo query string false "To timestamp (YYYY-MM-DD or RFC3339)"
// @Param page query int false "Page number"
// @Param limit query int false "Page size (max 200)"
// @Param sortBy query string false "Sort field (created_at, action, resource, user_id)"
// @Param sortOrder query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Envelope
// @Router /audit-logs [get]
func (h *AuditHandler) List(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "audit service not configured"))
		return
	}

	from, err := parseAuditTimeParam(c.Query("dateFrom"), false)
	if err != nil {
		response.Error(c, err)
		return
	}
	to, err := parseAuditTimeParam(c.Query("dateTo"), true)
	if err != nil {
		response.Error(c, err)
		return
	}

	req := service.AuditLogListRequest{
		UserID:     c.Query("userId"),
		Action:     c.Query("action"),
		Resource:   c.Query("resource"),
		ResourceID: c.Query("resourceId"),
		Search:     c.Query("search"),
		DateFrom:   from,
		DateTo:     to,
		Page:       parseQueryInt(c, "page", 1),
		PageSize:   parseQueryInt(c, "limit", 50),
		SortBy:     c.Query("sortBy"),
		SortOrder:  strings.ToLower(c.Query("sortOrder")),
	}

	entries, pagination, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, entries, pagination)
}

// Facets godoc
// @Summary List available audit filter values
// @Description Returns distinct actions and resources with counts so the viewer can build filter dropdowns.
// @Tags Audit
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /audit-logs/facets [get]
func (h *AuditHandler) Facets(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "audit service not configured"))
		return
	}
	facets, err := h.service.Facets(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, facets, nil)
}

// Get godoc
// @Summary Get a single audit log entry
// @Tags Audit
// @Produce json
// @Param id path string true "Audit log ID"
// @Success 200 {object} response.Envelope
// @Router /audit-logs/{id} [get]
func (h *AuditHandler) Get(c *gin.Context) {
	if h.service == nil {
		response.Error(c, appErrors.Clone(appErrors.ErrInternal, "audit service not configured"))
		return
	}
	entry, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, entry, nil)
}

// parseAuditTimeParam accepts either a plain date or a full RFC3339 timestamp.
// A bare date used as an upper bound is widened to the end of that day so
// `dateFrom=dateTo=2024-01-01` returns that day's entries rather than nothing.
func parseAuditTimeParam(raw string, endOfDay bool) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return &parsed, nil
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "invalid date, expected YYYY-MM-DD or RFC3339")
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return &parsed, nil
}
