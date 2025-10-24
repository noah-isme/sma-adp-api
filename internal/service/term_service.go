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

type termRepository interface {
	List(ctx context.Context, filter models.TermFilter) ([]models.Term, int, error)
	FindByID(ctx context.Context, id string) (*models.Term, error)
	FindActive(ctx context.Context) (*models.Term, error)
	ExistsByYearAndType(ctx context.Context, academicYear string, termType models.TermType, excludeID string) (bool, error)
	Create(ctx context.Context, term *models.Term) error
	Update(ctx context.Context, term *models.Term) error
	SetActive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	CountSchedules(ctx context.Context, id string) (int, error)
}

// CreateTermRequest describes payload for creating academic terms.
type CreateTermRequest struct {
	Name         string          `json:"name" validate:"required"`
	Type         models.TermType `json:"type" validate:"required"`
	AcademicYear string          `json:"academic_year" validate:"required"`
	StartDate    time.Time       `json:"start_date" validate:"required"`
	EndDate      time.Time       `json:"end_date" validate:"required"`
	IsActive     bool            `json:"is_active"`
}

// UpdateTermRequest updates mutable fields on a term.
type UpdateTermRequest struct {
	Name         string          `json:"name" validate:"required"`
	Type         models.TermType `json:"type" validate:"required"`
	AcademicYear string          `json:"academic_year" validate:"required"`
	StartDate    time.Time       `json:"start_date" validate:"required"`
	EndDate      time.Time       `json:"end_date" validate:"required"`
	IsActive     *bool           `json:"is_active"`
}

// SetActiveTermRequest toggles active term.
type SetActiveTermRequest struct {
	ID string `json:"id" validate:"required"`
}

// TermService orchestrates term workflows.
type TermService struct {
	repo      termRepository
	validator *validator.Validate
	logger    *zap.Logger
}

// NewTermService creates a new term service instance.
func NewTermService(repo termRepository, validate *validator.Validate, logger *zap.Logger) *TermService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TermService{repo: repo, validator: validate, logger: logger}
}

// List returns paginated terms.
func (s *TermService) List(ctx context.Context, filter models.TermFilter) ([]models.Term, *models.Pagination, error) {
	terms, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list terms")
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 {
		size = 20
	}

	pagination := &models.Pagination{
		Page:       page,
		PageSize:   size,
		TotalCount: total,
	}
	return terms, pagination, nil
}

// Get returns a term by ID.
func (s *TermService) Get(ctx context.Context, id string) (*models.Term, error) {
	term, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "term not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load term")
	}
	return term, nil
}

// GetActive returns currently active term.
func (s *TermService) GetActive(ctx context.Context) (*models.Term, error) {
	term, err := s.repo.FindActive(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "active term not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load active term")
	}
	return term, nil
}

// Create adds a new term ensuring uniqueness and date validation.
func (s *TermService) Create(ctx context.Context, req CreateTermRequest) (*models.Term, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid term payload")
	}
	if !req.StartDate.Before(req.EndDate) {
		return nil, appErrors.Clone(appErrors.ErrValidation, "start_date must be before end_date")
	}

	exists, err := s.repo.ExistsByYearAndType(ctx, req.AcademicYear, req.Type, "")
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check term uniqueness")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "term already exists for academic year and type")
	}

	term := &models.Term{
		Name:         req.Name,
		Type:         req.Type,
		AcademicYear: req.AcademicYear,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		IsActive:     req.IsActive,
	}

	if err := s.repo.Create(ctx, term); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create term")
	}

	if req.IsActive {
		if err := s.repo.SetActive(ctx, term.ID); err != nil {
			s.logger.Error("failed to set active term after create", zap.Error(err))
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to activate term")
		}
		term.IsActive = true
	}

	return term, nil
}

// Update modifies a term record.
func (s *TermService) Update(ctx context.Context, id string, req UpdateTermRequest) (*models.Term, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid term payload")
	}
	if !req.StartDate.Before(req.EndDate) {
		return nil, appErrors.Clone(appErrors.ErrValidation, "start_date must be before end_date")
	}

	term, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "term not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load term")
	}

	exists, err := s.repo.ExistsByYearAndType(ctx, req.AcademicYear, req.Type, id)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check term uniqueness")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "term already exists for academic year and type")
	}

	term.Name = req.Name
	term.Type = req.Type
	term.AcademicYear = req.AcademicYear
	term.StartDate = req.StartDate
	term.EndDate = req.EndDate
	if req.IsActive != nil {
		term.IsActive = *req.IsActive
	}

	if err := s.repo.Update(ctx, term); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update term")
	}

	if req.IsActive != nil && *req.IsActive {
		if err := s.repo.SetActive(ctx, term.ID); err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to activate term")
		}
		term.IsActive = true
	}

	return term, nil
}

// SetActive designates a term as active.
func (s *TermService) SetActive(ctx context.Context, req SetActiveTermRequest) (*models.Term, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid set active payload")
	}

	term, err := s.repo.FindByID(ctx, req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "term not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load term")
	}

	if err := s.repo.SetActive(ctx, term.ID); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to activate term")
	}
	term.IsActive = true
	return term, nil
}

// Delete removes a term when not active and without dependencies.
func (s *TermService) Delete(ctx context.Context, id string) error {
	term, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "term not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load term")
	}

	if term.IsActive {
		return appErrors.Clone(appErrors.ErrPreconditionFailed, "cannot delete active term")
	}

	count, err := s.repo.CountSchedules(ctx, id)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check term dependencies")
	}
	if count > 0 {
		return appErrors.Clone(appErrors.ErrPreconditionFailed, "term has schedules associated")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete term")
	}
	return nil
}
