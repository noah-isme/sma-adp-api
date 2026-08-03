package service

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// auditReader is the read surface the service needs from the repository layer.
type auditReader interface {
	List(ctx context.Context, filter models.AuditLogFilter) ([]models.AuditLogEntry, int, error)
	FindByID(ctx context.Context, id string) (*models.AuditLogEntry, error)
	Facets(ctx context.Context) (*models.AuditLogFacets, error)
}

// maxAuditPageSize bounds a single page so a wide date range cannot pull the
// whole table into memory.
const (
	maxAuditPageSize     = 200
	defaultAuditPageSize = 50
)

// AuditService exposes audit trail queries for the admin viewer.
type AuditService struct {
	repo   auditReader
	logger *zap.Logger
}

// NewAuditService constructs the service.
func NewAuditService(repo auditReader, logger *zap.Logger) *AuditService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AuditService{repo: repo, logger: logger}
}

// AuditLogListRequest captures the viewer's filter inputs.
type AuditLogListRequest struct {
	UserID     string
	Action     string
	Resource   string
	ResourceID string
	Search     string
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

// List returns a filtered, paginated slice of audit entries.
func (s *AuditService) List(ctx context.Context, req AuditLogListRequest) ([]models.AuditLogEntry, *models.Pagination, error) {
	if s == nil || s.repo == nil {
		return nil, nil, appErrors.Clone(appErrors.ErrInternal, "audit repository not configured")
	}

	if req.DateFrom != nil && req.DateTo != nil && req.DateTo.Before(*req.DateFrom) {
		return nil, nil, appErrors.Clone(appErrors.ErrValidation, "dateTo must not precede dateFrom")
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.PageSize
	if size <= 0 {
		size = defaultAuditPageSize
	}
	if size > maxAuditPageSize {
		size = maxAuditPageSize
	}

	filter := models.AuditLogFilter{
		UserID:     strings.TrimSpace(req.UserID),
		Action:     strings.TrimSpace(req.Action),
		Resource:   strings.TrimSpace(req.Resource),
		ResourceID: strings.TrimSpace(req.ResourceID),
		Search:     strings.TrimSpace(req.Search),
		DateFrom:   req.DateFrom,
		DateTo:     req.DateTo,
		Page:       page,
		PageSize:   size,
		SortBy:     req.SortBy,
		SortOrder:  req.SortOrder,
	}

	entries, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list audit logs")
	}
	if entries == nil {
		entries = []models.AuditLogEntry{}
	}

	pagination := &models.Pagination{Page: page, PageSize: size, TotalCount: total}
	return entries, pagination, nil
}

// Get returns a single audit entry by identifier.
func (s *AuditService) Get(ctx context.Context, id string) (*models.AuditLogEntry, error) {
	if s == nil || s.repo == nil {
		return nil, appErrors.Clone(appErrors.ErrInternal, "audit repository not configured")
	}
	if strings.TrimSpace(id) == "" {
		return nil, appErrors.Clone(appErrors.ErrValidation, "audit log id is required")
	}
	entry, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "audit log not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load audit log")
	}
	return entry, nil
}

// Facets returns the available action/resource filter values.
func (s *AuditService) Facets(ctx context.Context) (*models.AuditLogFacets, error) {
	if s == nil || s.repo == nil {
		return nil, appErrors.Clone(appErrors.ErrInternal, "audit repository not configured")
	}
	facets, err := s.repo.Facets(ctx)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load audit facets")
	}
	if facets.Actions == nil {
		facets.Actions = []models.AuditLogFacetCount{}
	}
	if facets.Resources == nil {
		facets.Resources = []models.AuditLogFacetCount{}
	}
	return facets, nil
}
