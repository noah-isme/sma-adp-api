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

type gradeComponentRepo interface {
	List(ctx context.Context, search string) ([]models.GradeComponent, error)
	ExistsByCode(ctx context.Context, code string, excludeID string) (bool, error)
	Create(ctx context.Context, component *models.GradeComponent) error
	FindByID(ctx context.Context, id string) (*models.GradeComponent, error)
	FindByCode(ctx context.Context, code string) (*models.GradeComponent, error)
	Update(ctx context.Context, component *models.GradeComponent) error
	Delete(ctx context.Context, id string) error
}

// CreateGradeComponentRequest describes creation payload.
type CreateGradeComponentRequest struct {
	Code        string  `json:"code" validate:"required"`
	Name        string  `json:"name" validate:"required"`
	Description *string `json:"description"`
}

// UpdateGradeComponentRequest describes an edit payload. Code is optional so
// the admin edit form can preserve the existing code while changing the name
// and description.
type UpdateGradeComponentRequest struct {
	Code        string  `json:"code"`
	Name        string  `json:"name" validate:"required"`
	Description *string `json:"description"`
}

// GradeComponentService handles component operations.
type GradeComponentService struct {
	repo      gradeComponentRepo
	validator *validator.Validate
	logger    *zap.Logger
}

// NewGradeComponentService constructs service.
func NewGradeComponentService(repo gradeComponentRepo, validate *validator.Validate, logger *zap.Logger) *GradeComponentService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &GradeComponentService{repo: repo, validator: validate, logger: logger}
}

// List returns grade components optionally filtered.
func (s *GradeComponentService) List(ctx context.Context, search string) ([]models.GradeComponent, error) {
	components, err := s.repo.List(ctx, search)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list grade components")
	}
	return components, nil
}

// Create inserts new grade component ensuring uniqueness.
func (s *GradeComponentService) Create(ctx context.Context, req CreateGradeComponentRequest) (*models.GradeComponent, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid grade component payload")
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	exists, err := s.repo.ExistsByCode(ctx, code, "")
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to validate grade component")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "component code already exists")
	}
	component := &models.GradeComponent{Code: code, Name: req.Name, Description: req.Description, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := s.repo.Create(ctx, component); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create grade component")
	}
	created, err := s.repo.FindByCode(ctx, component.Code)
	if err != nil {
		if err == sql.ErrNoRows {
			return component, nil
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade component")
	}
	return created, nil
}

// Update edits an active grade component while preserving code when omitted.
func (s *GradeComponentService) Update(ctx context.Context, id string, req UpdateGradeComponentRequest) (*models.GradeComponent, error) {
	if strings.TrimSpace(id) == "" {
		return nil, appErrors.Clone(appErrors.ErrValidation, "grade component id required")
	}
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid grade component payload")
	}
	component, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "grade component not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade component")
	}
	code := component.Code
	if strings.TrimSpace(req.Code) != "" {
		code = strings.ToUpper(strings.TrimSpace(req.Code))
	}
	exists, err := s.repo.ExistsByCode(ctx, code, id)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to validate grade component")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "component code already exists")
	}
	component.Code = code
	component.Name = strings.TrimSpace(req.Name)
	component.Description = req.Description
	if err := s.repo.Update(ctx, component); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "grade component not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update grade component")
	}
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return component, nil
	}
	return updated, nil
}

// Delete soft-deletes an active grade component.
func (s *GradeComponentService) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return appErrors.Clone(appErrors.ErrValidation, "grade component id required")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "grade component not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete grade component")
	}
	return nil
}
