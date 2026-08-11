package service

import (
	"context"
	"database/sql"
	"errors"
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

type portalAnnouncementPager interface {
	ListByStudentAndTermPage(ctx context.Context, studentID, termID string, page, limit int, activeOnly bool) ([]models.Announcement, int, error)
}

type portalAnnouncementByStudentReader interface {
	FindByIDForStudent(ctx context.Context, id, studentID string) (*models.Announcement, error)
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch student")
	}

	page := req.Page
	if page == 0 {
		page = 1
	}
	if page < 1 {
		return nil, appErrors.Clone(appErrors.ErrValidation, "page must be at least 1")
	}
	limit := req.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return nil, appErrors.Clone(appErrors.ErrValidation, "limit must be between 1 and 100")
	}

	var (
		announcements []models.Announcement
		total         int
	)
	if pager, ok := s.announcementRepo.(portalAnnouncementPager); ok {
		var err error
		announcements, total, err = pager.ListByStudentAndTermPage(ctx, req.StudentID, req.TermID, page, limit, req.ActiveOnly)
		if err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch announcements")
		}
	} else {
		// Keep older repository fakes usable while the concrete repository
		// provides the database-backed pagination path above.
		var err error
		announcements, err = s.announcementRepo.ListByStudentAndTerm(ctx, req.StudentID, req.TermID)
		if err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch announcements")
		}
		total = len(announcements)
		start := (page - 1) * limit
		if start >= total {
			announcements = []models.Announcement{}
		} else {
			end := start + limit
			if end > total {
				end = total
			}
			announcements = announcements[start:end]
		}
	}

	// Build response
	items := make([]*models.PortalAnnouncement, len(announcements))
	for i, a := range announcements {
		items[i] = portalAnnouncementModel(&a)
	}

	return &models.PortalAnnouncementsResponse{
		Announcements: items,
		Pagination: &models.PaginationMeta{
			Page:       page,
			PageSize:   limit,
			TotalCount: total,
		},
	}, nil
}

// GetAnnouncementByIDForStudent applies the same audience scope as the list
// endpoint so a guessed announcement ID cannot bypass portal authorization.
func (s *PortalAnnouncementsService) GetAnnouncementByIDForStudent(ctx context.Context, id, studentID string) (*models.PortalAnnouncement, error) {
	reader, ok := s.announcementRepo.(portalAnnouncementByStudentReader)
	if !ok {
		return nil, appErrors.Wrap(errors.New("announcement repository does not support portal scoping"), appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "portal announcement unavailable")
	}
	announcement, err := reader.FindByIDForStudent(ctx, id, studentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "announcement not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch announcement")
	}
	return portalAnnouncementModel(announcement), nil
}

// GetAnnouncementByID returns a single announcement by ID.
func (s *PortalAnnouncementsService) GetAnnouncementByID(ctx context.Context, id string) (*models.PortalAnnouncement, error) {
	announcement, err := s.announcementRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "announcement not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch announcement")
	}

	return portalAnnouncementModel(announcement), nil
}

func portalAnnouncementModel(announcement *models.Announcement) *models.PortalAnnouncement {
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
		Audience:    string(announcement.Audience),
		Priority:    string(announcement.Priority),
		IsPinned:    announcement.IsPinned,
		PublishedAt: &publishedAt,
		ExpiresAt:   expiresAt,
	}
}
