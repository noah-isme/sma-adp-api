package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type gradeConfigRepository interface {
	List(ctx context.Context, filter models.FinalGradeFilter) ([]models.GradeConfig, error)
	FindByID(ctx context.Context, id string) (*models.GradeConfig, error)
	FindByScope(ctx context.Context, classID, subjectID, termID string) (*models.GradeConfig, error)
	Exists(ctx context.Context, classID, subjectID, termID, excludeID string) (bool, error)
	Create(ctx context.Context, config *models.GradeConfig) error
	Update(ctx context.Context, config *models.GradeConfig) error
	Finalize(ctx context.Context, id string, finalized bool) error
}

type gradeComponentReader interface {
	FindByID(ctx context.Context, id string) (*models.GradeComponent, error)
}

// GradeConfigComponentRequest captures payload for config components.
type GradeConfigComponentRequest struct {
	ComponentID string  `json:"component_id" validate:"required"`
	Weight      float64 `json:"weight"`
}

// CreateGradeConfigRequest handles creation payload.
type CreateGradeConfigRequest struct {
	ClassID           string                        `json:"class_id" validate:"required"`
	SubjectID         string                        `json:"subject_id" validate:"required"`
	TermID            string                        `json:"term_id" validate:"required"`
	CalculationScheme models.GradeCalculationScheme `json:"calculation_scheme" validate:"required"`
	Components        []GradeConfigComponentRequest `json:"components" validate:"required,dive"`
}

// UpdateGradeConfigRequest handles update payload.
type UpdateGradeConfigRequest struct {
	CalculationScheme models.GradeCalculationScheme `json:"calculation_scheme" validate:"required"`
	Components        []GradeConfigComponentRequest `json:"components" validate:"required,dive"`
}

// GradeConfigService manages grade configuration logic.
type GradeConfigService struct {
	repo       gradeConfigRepository
	components gradeComponentReader
	validator  *validator.Validate
	logger     *zap.Logger
}

// NewGradeConfigService constructs service.
func NewGradeConfigService(repo gradeConfigRepository, components gradeComponentReader, validate *validator.Validate, logger *zap.Logger) *GradeConfigService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &GradeConfigService{repo: repo, components: components, validator: validate, logger: logger}
}

// List returns grade configurations for filter.
func (s *GradeConfigService) List(ctx context.Context, filter models.FinalGradeFilter) ([]models.GradeConfig, error) {
	configs, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list grade configs")
	}
	return configs, nil
}

// Get returns grade config by ID.
func (s *GradeConfigService) Get(ctx context.Context, id string) (*models.GradeConfig, error) {
	config, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "grade config not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade config")
	}
	return config, nil
}

// Create inserts new grade config.
func (s *GradeConfigService) Create(ctx context.Context, req CreateGradeConfigRequest) (*models.GradeConfig, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid grade config payload")
	}
	if err := s.validateScheme(req.CalculationScheme, req.Components); err != nil {
		return nil, err
	}
	exists, err := s.repo.Exists(ctx, req.ClassID, req.SubjectID, req.TermID, "")
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to validate grade config")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "grade config already exists for scope")
	}
	comps, err := s.resolveComponents(ctx, req.Components)
	if err != nil {
		return nil, err
	}
	config := &models.GradeConfig{
		ClassID:           req.ClassID,
		SubjectID:         req.SubjectID,
		TermID:            req.TermID,
		CalculationScheme: req.CalculationScheme,
		Finalized:         false,
		Components:        comps,
	}
	if err := s.repo.Create(ctx, config); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create grade config")
	}
	created, err := s.repo.FindByID(ctx, config.ID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade config")
	}
	return created, nil
}

// Update modifies existing grade config.
func (s *GradeConfigService) Update(ctx context.Context, id string, req UpdateGradeConfigRequest) (*models.GradeConfig, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid grade config payload")
	}
	config, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "grade config not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade config")
	}
	if config.Finalized {
		return nil, appErrors.Clone(appErrors.ErrFinalized, "grade config finalized")
	}
	if err := s.validateScheme(req.CalculationScheme, req.Components); err != nil {
		return nil, err
	}
	comps, err := s.resolveComponents(ctx, req.Components)
	if err != nil {
		return nil, err
	}
	config.CalculationScheme = req.CalculationScheme
	config.Components = comps
	if err := s.repo.Update(ctx, config); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update grade config")
	}
	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade config")
	}
	return updated, nil
}

// Finalize locks grade config structure.
func (s *GradeConfigService) Finalize(ctx context.Context, id string) (*models.GradeConfig, error) {
	config, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "grade config not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade config")
	}
	if config.Finalized {
		return config, nil
	}
	if err := s.repo.Finalize(ctx, id, true); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to finalize grade config")
	}
	config.Finalized = true
	return config, nil
}

func (s *GradeConfigService) validateScheme(scheme models.GradeCalculationScheme, components []GradeConfigComponentRequest) error {
	if scheme != models.GradeSchemeWeighted && scheme != models.GradeSchemeAverage {
		return appErrors.Clone(appErrors.ErrValidation, fmt.Sprintf("unsupported calculation scheme %s", scheme))
	}
	if len(components) == 0 {
		return appErrors.Clone(appErrors.ErrValidation, "components required")
	}
	seen := make(map[string]struct{}, len(components))
	total := 0.0
	for _, comp := range components {
		if _, ok := seen[comp.ComponentID]; ok {
			return appErrors.Clone(appErrors.ErrValidation, "duplicate component")
		}
		seen[comp.ComponentID] = struct{}{}
		total += comp.Weight
	}
	if scheme == models.GradeSchemeWeighted {
		if total < 99.999 || total > 100.001 {
			return appErrors.Clone(appErrors.ErrInvalidWeights, "weights must sum to 100")
		}
	}
	return nil
}

func (s *GradeConfigService) resolveComponents(ctx context.Context, payload []GradeConfigComponentRequest) ([]models.GradeConfigComponent, error) {
	components := make([]models.GradeConfigComponent, len(payload))
	for i, p := range payload {
		component, err := s.components.FindByID(ctx, p.ComponentID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, appErrors.Clone(appErrors.ErrNotFound, "grade component not found")
			}
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade component")
		}
		components[i] = models.GradeConfigComponent{ComponentID: component.ID, Weight: p.Weight, ComponentCode: component.Code, ComponentName: component.Name}
	}
	return components, nil
}
