package service

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type announcementRepository interface {
	List(ctx context.Context, filter models.AnnouncementFilter) ([]models.Announcement, int, error)
	GetByID(ctx context.Context, id string) (*models.Announcement, error)
	Create(ctx context.Context, announcement *models.Announcement) error
	Update(ctx context.Context, announcement *models.Announcement) error
	Delete(ctx context.Context, id string) error
}

// AnnouncementService handles announcement workflows.
type AnnouncementService struct {
	repo      announcementRepository
	validator *validator.Validate
	logger    *zap.Logger
}

// NewAnnouncementService constructs the service.
func NewAnnouncementService(repo announcementRepository, validate *validator.Validate, logger *zap.Logger) *AnnouncementService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	svc := &AnnouncementService{repo: repo, validator: validate, logger: logger}
	svc.validator.RegisterValidation("audience", func(fl validator.FieldLevel) bool {
		switch models.AnnouncementAudience(strings.ToUpper(fl.Field().String())) {
		case models.AnnouncementAudienceAll, models.AnnouncementAudienceGuru, models.AnnouncementAudienceSiswa, models.AnnouncementAudienceClass:
			return true
		default:
			return false
		}
	})
	svc.validator.RegisterValidation("priority", func(fl validator.FieldLevel) bool {
		switch models.AnnouncementPriority(strings.ToUpper(fl.Field().String())) {
		case models.AnnouncementPriorityLow, models.AnnouncementPriorityNormal, models.AnnouncementPriorityHigh:
			return true
		default:
			return false
		}
	})
	return svc
}

// AnnouncementListRequest describes filters for listing announcements.
type AnnouncementListRequest struct {
	AudienceRoles []models.UserRole `json:"audience_roles"`
	ClassIDs      []string          `json:"class_ids"`
	Page          int               `json:"page"`
	PageSize      int               `json:"page_size"`
	IncludePinned bool              `json:"include_pinned"`
}

// CreateAnnouncementRequest describes create payload.
type CreateAnnouncementRequest struct {
	Title         string     `json:"title" validate:"required"`
	Content       string     `json:"content" validate:"required"`
	Audience      string     `json:"audience" validate:"required,audience"`
	TargetClassID *string    `json:"target_class_id"`
	Priority      string     `json:"priority" validate:"required,priority"`
	IsPinned      bool       `json:"is_pinned"`
	PublishedAt   time.Time  `json:"published_at" validate:"required"`
	ExpiresAt     *time.Time `json:"expires_at"`
	CreatedBy     string     `json:"created_by" validate:"required"`
}

// UpdateAnnouncementRequest describes update payload.
type UpdateAnnouncementRequest struct {
	Title         string     `json:"title" validate:"required"`
	Content       string     `json:"content" validate:"required"`
	Audience      string     `json:"audience" validate:"required,audience"`
	TargetClassID *string    `json:"target_class_id"`
	Priority      string     `json:"priority" validate:"required,priority"`
	IsPinned      bool       `json:"is_pinned"`
	PublishedAt   time.Time  `json:"published_at" validate:"required"`
	ExpiresAt     *time.Time `json:"expires_at"`
}

// List returns announcements with pagination.
func (s *AnnouncementService) List(ctx context.Context, req AnnouncementListRequest) ([]models.Announcement, *models.Pagination, error) {
	filter := models.AnnouncementFilter{
		AudienceRoles: req.AudienceRoles,
		ClassIDs:      req.ClassIDs,
		IncludePinned: req.IncludePinned,
		Page:          req.Page,
		PageSize:      req.PageSize,
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	rows, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list announcements")
	}
	pagination := &models.Pagination{Page: filter.Page, PageSize: filter.PageSize, TotalCount: total}
	return rows, pagination, nil
}

// Get returns an announcement by id.
func (s *AnnouncementService) Get(ctx context.Context, id string) (*models.Announcement, error) {
	ann, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "announcement not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to get announcement")
	}
	return ann, nil
}

// Create registers a new announcement.
func (s *AnnouncementService) Create(ctx context.Context, req CreateAnnouncementRequest) (*models.Announcement, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid payload")
	}
	if err := s.ensureAudienceTarget(req.Audience, req.TargetClassID); err != nil {
		return nil, err
	}
	if req.ExpiresAt != nil && req.ExpiresAt.Before(req.PublishedAt) {
		return nil, appErrors.Clone(appErrors.ErrValidation, "expires_at must be after published_at")
	}
	announcement := &models.Announcement{
		Title:         req.Title,
		Content:       req.Content,
		Audience:      models.AnnouncementAudience(strings.ToUpper(req.Audience)),
		TargetClassID: req.TargetClassID,
		Priority:      models.AnnouncementPriority(strings.ToUpper(req.Priority)),
		IsPinned:      req.IsPinned,
		PublishedAt:   req.PublishedAt,
		ExpiresAt:     req.ExpiresAt,
		CreatedBy:     req.CreatedBy,
	}
	if err := s.repo.Create(ctx, announcement); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create announcement")
	}
	return announcement, nil
}

// Update modifies an existing announcement.
func (s *AnnouncementService) Update(ctx context.Context, id string, req UpdateAnnouncementRequest) (*models.Announcement, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid payload")
	}
	if err := s.ensureAudienceTarget(req.Audience, req.TargetClassID); err != nil {
		return nil, err
	}
	if req.ExpiresAt != nil && req.ExpiresAt.Before(req.PublishedAt) {
		return nil, appErrors.Clone(appErrors.ErrValidation, "expires_at must be after published_at")
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "announcement not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load announcement")
	}
	existing.Title = req.Title
	existing.Content = req.Content
	existing.Audience = models.AnnouncementAudience(strings.ToUpper(req.Audience))
	existing.TargetClassID = req.TargetClassID
	existing.Priority = models.AnnouncementPriority(strings.ToUpper(req.Priority))
	existing.IsPinned = req.IsPinned
	existing.PublishedAt = req.PublishedAt
	existing.ExpiresAt = req.ExpiresAt
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update announcement")
	}
	return existing, nil
}

// Delete removes an announcement by id.
func (s *AnnouncementService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete announcement")
	}
	return nil
}

func (s *AnnouncementService) ensureAudienceTarget(audience string, target *string) error {
	if strings.ToUpper(audience) == string(models.AnnouncementAudienceClass) && (target == nil || *target == "") {
		return appErrors.Clone(appErrors.ErrValidation, "target_class_id required for CLASS audience")
	}
	return nil
}
