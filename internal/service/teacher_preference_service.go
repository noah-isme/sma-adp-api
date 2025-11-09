package service

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx/types"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type teacherPreferenceRepo interface {
	GetByTeacher(ctx context.Context, teacherID string) (*models.TeacherPreference, error)
	Upsert(ctx context.Context, pref *models.TeacherPreference) error
}

// UpsertTeacherPreferenceRequest captures payload to store preferences.
type UpsertTeacherPreferenceRequest struct {
	MaxLoadPerDay  int                             `json:"max_load_per_day" validate:"min=0"`
	MaxLoadPerWeek int                             `json:"max_load_per_week" validate:"min=0"`
	Unavailable    []models.TeacherUnavailableSlot `json:"unavailable"`
}

// TeacherPreferenceService handles preference logic.
type TeacherPreferenceService struct {
	teachers  teacherRepository
	repo      teacherPreferenceRepo
	validator *validator.Validate
	logger    *zap.Logger
}

// NewTeacherPreferenceService builds the service.
func NewTeacherPreferenceService(teachers teacherRepository, repo teacherPreferenceRepo, validate *validator.Validate, logger *zap.Logger) *TeacherPreferenceService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TeacherPreferenceService{
		teachers:  teachers,
		repo:      repo,
		validator: validate,
		logger:    logger,
	}
}

// Get returns stored preferences or defaults.
func (s *TeacherPreferenceService) Get(ctx context.Context, teacherID string) (*models.TeacherPreference, error) {
	if _, err := s.teachers.FindByID(ctx, teacherID); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "teacher not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher")
	}

	pref, err := s.repo.GetByTeacher(ctx, teacherID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &models.TeacherPreference{
				TeacherID:      teacherID,
				MaxLoadPerDay:  0,
				MaxLoadPerWeek: 0,
				Unavailable:    types.JSONText("[]"),
			}, nil
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher preferences")
	}
	return pref, nil
}

// Upsert stores preferences for a teacher.
func (s *TeacherPreferenceService) Upsert(ctx context.Context, teacherID string, req UpsertTeacherPreferenceRequest) (*models.TeacherPreference, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid preference payload")
	}
	if _, err := s.teachers.FindByID(ctx, teacherID); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "teacher not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher")
	}

	var raw types.JSONText = types.JSONText("[]")
	if len(req.Unavailable) > 0 {
		bytes, err := json.Marshal(req.Unavailable)
		if err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid unavailable payload")
		}
		raw = types.JSONText(bytes)
	}

	payload := &models.TeacherPreference{
		TeacherID:      teacherID,
		MaxLoadPerDay:  req.MaxLoadPerDay,
		MaxLoadPerWeek: req.MaxLoadPerWeek,
		Unavailable:    raw,
	}

	existing, err := s.repo.GetByTeacher(ctx, teacherID)
	if err != nil && err != sql.ErrNoRows {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher preferences")
	}
	if existing != nil {
		payload.ID = existing.ID
		payload.CreatedAt = existing.CreatedAt
	}

	if err := s.repo.Upsert(ctx, payload); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to upsert teacher preferences")
	}
	return payload, nil
}
