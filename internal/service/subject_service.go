package service

import (
	"context"
	"database/sql"
	"strings"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type subjectRepository interface {
	List(ctx context.Context, filter models.SubjectFilter) ([]models.Subject, int, error)
	FindByID(ctx context.Context, id string) (*models.Subject, error)
	ExistsByCode(ctx context.Context, code string, excludeID string) (bool, error)
	Create(ctx context.Context, subject *models.Subject) error
	Update(ctx context.Context, subject *models.Subject) error
	Delete(ctx context.Context, id string) error
	CountClassSubjects(ctx context.Context, id string) (int, error)
}

// CreateSubjectRequest captures fields for creating subjects.
type CreateSubjectRequest struct {
	Code         string `json:"code" validate:"required"`
	Name         string `json:"name" validate:"required"`
	Track        string `json:"track" validate:"required"`
	SubjectGroup string `json:"subject_group" validate:"required"`
}

// UpdateSubjectRequest modifies subject fields.
type UpdateSubjectRequest struct {
	Code         string `json:"code" validate:"required"`
	Name         string `json:"name" validate:"required"`
	Track        string `json:"track" validate:"required"`
	SubjectGroup string `json:"subject_group" validate:"required"`
}

// SubjectService handles subject domain workflows.
type SubjectService struct {
	repo      subjectRepository
	validator *validator.Validate
	logger    *zap.Logger
}

// NewSubjectService creates a new subject service.
func NewSubjectService(repo subjectRepository, validate *validator.Validate, logger *zap.Logger) *SubjectService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SubjectService{repo: repo, validator: validate, logger: logger}
}

// List returns paginated subjects.
func (s *SubjectService) List(ctx context.Context, filter models.SubjectFilter) ([]models.Subject, *models.Pagination, error) {
	subjects, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list subjects")
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 {
		size = 20
	}
	pagination := &models.Pagination{Page: page, PageSize: size, TotalCount: total}
	return subjects, pagination, nil
}

// Get returns subject by identifier.
func (s *SubjectService) Get(ctx context.Context, id string) (*models.Subject, error) {
	subject, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "subject not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load subject")
	}
	return subject, nil
}

// Create adds a new subject ensuring code uniqueness.
func (s *SubjectService) Create(ctx context.Context, req CreateSubjectRequest) (*models.Subject, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid subject payload")
	}

	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))

	exists, err := s.repo.ExistsByCode(ctx, req.Code, "")
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check subject code")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "subject code already exists")
	}

	subject := &models.Subject{
		Code:         req.Code,
		Name:         req.Name,
		Track:        req.Track,
		SubjectGroup: req.SubjectGroup,
	}

	if err := s.repo.Create(ctx, subject); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create subject")
	}
	return subject, nil
}

// Update modifies an existing subject.
func (s *SubjectService) Update(ctx context.Context, id string, req UpdateSubjectRequest) (*models.Subject, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid subject payload")
	}

	subject, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "subject not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load subject")
	}

	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))

	exists, err := s.repo.ExistsByCode(ctx, req.Code, id)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check subject code")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "subject code already exists")
	}

	subject.Code = req.Code
	subject.Name = req.Name
	subject.Track = req.Track
	subject.SubjectGroup = req.SubjectGroup

	if err := s.repo.Update(ctx, subject); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update subject")
	}
	return subject, nil
}

// Delete removes a subject when no class mappings exist.
func (s *SubjectService) Delete(ctx context.Context, id string) error {
	subject, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "subject not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load subject")
	}

	count, err := s.repo.CountClassSubjects(ctx, subject.ID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check subject dependencies")
	}
	if count > 0 {
		return appErrors.Clone(appErrors.ErrPreconditionFailed, "subject mapped to classes")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete subject")
	}
	return nil
}
