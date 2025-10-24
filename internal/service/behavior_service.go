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

type behaviorRepository interface {
	List(ctx context.Context, filter models.BehaviorNoteFilter) ([]models.BehaviorNote, int, error)
	Create(ctx context.Context, note *models.BehaviorNote) error
	Update(ctx context.Context, note *models.BehaviorNote) error
	Delete(ctx context.Context, id string) error
	Summary(ctx context.Context, studentID string) (*models.BehaviorSummary, error)
}

// BehaviorService handles behaviour notes.
type BehaviorService struct {
	repo      behaviorRepository
	validator *validator.Validate
	logger    *zap.Logger
}

// NewBehaviorService constructs the service.
func NewBehaviorService(repo behaviorRepository, validate *validator.Validate, logger *zap.Logger) *BehaviorService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	svc := &BehaviorService{repo: repo, validator: validate, logger: logger}
	svc.validator.RegisterValidation("note_type", func(fl validator.FieldLevel) bool {
		switch models.BehaviorNoteType(fl.Field().String()) {
		case models.BehaviorNotePositive, models.BehaviorNoteNegative, models.BehaviorNoteNeutral:
			return true
		default:
			return false
		}
	})
	return svc
}

// BehaviorListRequest describes filters for listing notes.
type BehaviorListRequest struct {
	StudentID string     `json:"student_id"`
	DateFrom  *time.Time `json:"date_from"`
	DateTo    *time.Time `json:"date_to"`
	NoteTypes []string   `json:"note_types"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}

// CreateBehaviorRequest describes create payload.
type CreateBehaviorRequest struct {
	StudentID   string    `json:"student_id" validate:"required"`
	Date        time.Time `json:"date" validate:"required"`
	NoteType    string    `json:"note_type" validate:"required,note_type"`
	Points      int       `json:"points"`
	Description string    `json:"description" validate:"required"`
	CreatedBy   string    `json:"created_by" validate:"required"`
}

// UpdateBehaviorRequest describes update payload.
type UpdateBehaviorRequest struct {
	StudentID   string    `json:"student_id" validate:"required"`
	Date        time.Time `json:"date" validate:"required"`
	NoteType    string    `json:"note_type" validate:"required,note_type"`
	Points      int       `json:"points"`
	Description string    `json:"description" validate:"required"`
}

// List returns behaviour notes with pagination.
func (s *BehaviorService) List(ctx context.Context, req BehaviorListRequest) ([]models.BehaviorNote, *models.Pagination, error) {
	filter := models.BehaviorNoteFilter{
		StudentID: req.StudentID,
		DateFrom:  req.DateFrom,
		DateTo:    req.DateTo,
		Page:      req.Page,
		PageSize:  req.PageSize,
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 50
	}
	if len(req.NoteTypes) > 0 {
		filter.NoteTypes = make([]models.BehaviorNoteType, len(req.NoteTypes))
		for i, t := range req.NoteTypes {
			filter.NoteTypes[i] = models.BehaviorNoteType(t)
		}
	}
	notes, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list behavior notes")
	}
	pagination := &models.Pagination{Page: filter.Page, PageSize: filter.PageSize, TotalCount: total}
	return notes, pagination, nil
}

// Create adds a behaviour note.
func (s *BehaviorService) Create(ctx context.Context, req CreateBehaviorRequest) (*models.BehaviorNote, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid payload")
	}
	note := &models.BehaviorNote{
		StudentID:   req.StudentID,
		NoteDate:    req.Date,
		NoteType:    models.BehaviorNoteType(req.NoteType),
		Points:      req.Points,
		Description: req.Description,
		CreatedBy:   req.CreatedBy,
	}
	if err := s.repo.Create(ctx, note); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create behavior note")
	}
	return note, nil
}

// Update modifies an existing note.
func (s *BehaviorService) Update(ctx context.Context, id string, req UpdateBehaviorRequest) (*models.BehaviorNote, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid payload")
	}
	note := &models.BehaviorNote{
		ID:          id,
		StudentID:   req.StudentID,
		NoteDate:    req.Date,
		NoteType:    models.BehaviorNoteType(req.NoteType),
		Points:      req.Points,
		Description: req.Description,
	}
	if err := s.repo.Update(ctx, note); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update behavior note")
	}
	return note, nil
}

// Delete removes a behaviour note.
func (s *BehaviorService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete behavior note")
	}
	return nil
}

// Summary returns aggregated metrics.
func (s *BehaviorService) Summary(ctx context.Context, studentID string) (*models.BehaviorSummary, error) {
	summary, err := s.repo.Summary(ctx, studentID)
	if err != nil {
		if err == sql.ErrNoRows {
			empty := &models.BehaviorSummary{StudentID: studentID}
			return empty, nil
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to summarise behavior")
	}
	return summary, nil
}
