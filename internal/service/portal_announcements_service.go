package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// PortalAnnouncementsService provides announcements data for parent/student portal.
type PortalAnnouncementsService struct {
	announcementRepo announcementReader
	enrollmentRepo   enrollmentReader
	studentRepo      studentReader
	validator        *validator.Validate
	logger           *zap.Logger
}

// NewPortalAnnouncementsService constructs the portal announcements service.
func NewPortalAnnouncementsService(
	announcementRepo announcementReader,
	enrollmentRepo enrollmentReader,
	studentRepo studentReader,
	validate *validator.Validate,
	logger *zap.Logger,
) *PortalAnnouncementsService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PortalAnnouncementsService{
		announcementRepo: announcementRepo,
		enrollmentRepo:   enrollmentRepo,
		studentRepo:      studentRepo,
		validator:        validate,
		logger:           logger,
	}
}

// GetAnnouncements returns announcements for a student in a term.
func (s *PortalAnnouncementsService) GetAnnouncements(ctx context.Context, req models.PortalAnnouncementsRequest) (*models.PortalAnnouncementsResponse, error) {
	// Get student detail
	_, err := s.studentRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch student")
	}

	// Get announcements
	announcements, err := s.announcementRepo.ListByStudentAndTerm(ctx, req.StudentID, req.TermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch announcements")
	}

	// Build response
	items := make([]*models.PortalAnnouncement, len(announcements))
	for i, a := range announcements {
		publishedAt := a.PublishedAt.Format(time.RFC3339)
		var expiresAt *string
		if a.ExpiresAt != nil {
			exp := a.ExpiresAt.Format(time.RFC3339)
			expiresAt = &exp
		}
		items[i] = &models.PortalAnnouncement{
			ID:          a.ID,
			Title:       a.Title,
			Content:     a.Content,
			Priority:    string(a.Priority),
			IsPinned:    a.IsPinned,
			PublishedAt: &publishedAt,
			ExpiresAt:   expiresAt,
		}
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	return &models.PortalAnnouncementsResponse{
		Announcements: items,
		Pagination: &models.PaginationMeta{
			Page:       page,
			PageSize:   limit,
			TotalCount: len(items),
		},
	}, nil
}

// GetAnnouncementByID returns a single announcement by ID.
func (s *PortalAnnouncementsService) GetAnnouncementByID(ctx context.Context, id string) (*models.PortalAnnouncement, error) {
	announcement, err := s.announcementRepo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "announcement not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch announcement")
	}

	publishedAt := announcement.PublishedAt.Format(time.RFC3339)
	var expiresAt *string
	if announcement.ExpiresAt != nil {
		exp := announcement.ExpiresAt.Format(time.RFC3339)
		expiresAt = &exp
	}

	return &models.PortalAnnouncement{
		ID:          announcement.ID,
		Title:       announcement.Title,
		Content:     announcement.Content,
		Priority:    string(announcement.Priority),
		IsPinned:    announcement.IsPinned,
		PublishedAt: &publishedAt,
		ExpiresAt:   expiresAt,
	}, nil
}
