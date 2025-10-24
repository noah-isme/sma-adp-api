package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type scheduleRepository interface {
	List(ctx context.Context, filter models.ScheduleFilter) ([]models.Schedule, int, error)
	ListByClass(ctx context.Context, classID string) ([]models.Schedule, error)
	ListByTeacher(ctx context.Context, teacherID string) ([]models.Schedule, error)
	FindByID(ctx context.Context, id string) (*models.Schedule, error)
	FindConflicts(ctx context.Context, termID, dayOfWeek, timeSlot string) ([]models.Schedule, error)
	Create(ctx context.Context, schedule *models.Schedule) error
	BulkCreate(ctx context.Context, schedules []models.Schedule) error
	Update(ctx context.Context, schedule *models.Schedule) error
	Delete(ctx context.Context, id string) error
}

// CreateScheduleRequest describes payload for creating a schedule.
type CreateScheduleRequest struct {
	TermID    string `json:"term_id" validate:"required"`
	ClassID   string `json:"class_id" validate:"required"`
	SubjectID string `json:"subject_id" validate:"required"`
	TeacherID string `json:"teacher_id" validate:"required"`
	DayOfWeek string `json:"day_of_week" validate:"required"`
	TimeSlot  string `json:"time_slot" validate:"required"`
	Room      string `json:"room" validate:"required"`
}

// UpdateScheduleRequest updates an existing schedule.
type UpdateScheduleRequest struct {
	TermID    string `json:"term_id" validate:"required"`
	ClassID   string `json:"class_id" validate:"required"`
	SubjectID string `json:"subject_id" validate:"required"`
	TeacherID string `json:"teacher_id" validate:"required"`
	DayOfWeek string `json:"day_of_week" validate:"required"`
	TimeSlot  string `json:"time_slot" validate:"required"`
	Room      string `json:"room" validate:"required"`
}

// BulkCreateSchedulesRequest holds multiple schedules for creation.
type BulkCreateSchedulesRequest struct {
	Items          []CreateScheduleRequest `json:"items" validate:"required,min=1,dive"`
	PartialOnError bool                    `json:"partial_on_error"`
}

// BulkCreateSchedulesResult summarises bulk creation results.
type BulkCreateSchedulesResult struct {
	Created   []models.Schedule         `json:"created"`
	Conflicts []models.ScheduleConflict `json:"conflicts,omitempty"`
}

// ScheduleService coordinates scheduling logic.
type ScheduleService struct {
	repo      scheduleRepository
	validator *validator.Validate
	logger    *zap.Logger
}

// NewScheduleService instantiates ScheduleService.
func NewScheduleService(repo scheduleRepository, validate *validator.Validate, logger *zap.Logger) *ScheduleService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ScheduleService{repo: repo, validator: validate, logger: logger}
}

// List returns schedules with pagination metadata.
func (s *ScheduleService) List(ctx context.Context, filter models.ScheduleFilter) ([]models.Schedule, *models.Pagination, error) {
	schedules, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list schedules")
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
	return schedules, pagination, nil
}

// ListByClass returns schedules for a class.
func (s *ScheduleService) ListByClass(ctx context.Context, classID string) ([]models.Schedule, error) {
	schedules, err := s.repo.ListByClass(ctx, classID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list class schedules")
	}
	return schedules, nil
}

// ListByTeacher returns schedules for a teacher.
func (s *ScheduleService) ListByTeacher(ctx context.Context, teacherID string) ([]models.Schedule, error) {
	schedules, err := s.repo.ListByTeacher(ctx, teacherID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list teacher schedules")
	}
	return schedules, nil
}

// Create inserts a new schedule after conflict detection.
func (s *ScheduleService) Create(ctx context.Context, req CreateScheduleRequest) (*models.Schedule, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid schedule payload")
	}

	schedule := models.Schedule{
		TermID:    req.TermID,
		ClassID:   req.ClassID,
		SubjectID: req.SubjectID,
		TeacherID: req.TeacherID,
		DayOfWeek: strings.ToUpper(req.DayOfWeek),
		TimeSlot:  req.TimeSlot,
		Room:      req.Room,
	}

	if err := s.ensureNoConflict(ctx, schedule, ""); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, &schedule); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create schedule")
	}
	return &schedule, nil
}

// Update modifies an existing schedule.
func (s *ScheduleService) Update(ctx context.Context, id string, req UpdateScheduleRequest) (*models.Schedule, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid schedule payload")
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "schedule not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load schedule")
	}

	updated := models.Schedule{
		ID:        existing.ID,
		TermID:    req.TermID,
		ClassID:   req.ClassID,
		SubjectID: req.SubjectID,
		TeacherID: req.TeacherID,
		DayOfWeek: strings.ToUpper(req.DayOfWeek),
		TimeSlot:  req.TimeSlot,
		Room:      req.Room,
	}

	if err := s.ensureNoConflict(ctx, updated, existing.ID); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, &updated); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update schedule")
	}
	return &updated, nil
}

// Delete removes a schedule entry.
func (s *ScheduleService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "schedule not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load schedule")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete schedule")
	}
	return nil
}

// BulkCreate inserts multiple schedules optionally allowing partial completion.
func (s *ScheduleService) BulkCreate(ctx context.Context, req BulkCreateSchedulesRequest) (*BulkCreateSchedulesResult, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid bulk schedule payload")
	}

	var toCreate []models.Schedule
	var conflicts []models.ScheduleConflict

	for _, item := range req.Items {
		if err := s.validator.Struct(item); err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid schedule entry")
		}
		schedule := models.Schedule{
			TermID:    item.TermID,
			ClassID:   item.ClassID,
			SubjectID: item.SubjectID,
			TeacherID: item.TeacherID,
			DayOfWeek: strings.ToUpper(item.DayOfWeek),
			TimeSlot:  item.TimeSlot,
			Room:      item.Room,
		}
		if err := s.ensureNoConflict(ctx, schedule, ""); err != nil {
			if appErr := appErrors.FromError(err); appErr.Code == appErrors.ErrConflict.Code {
				var domainErr *models.ScheduleConflictError
				if errors.As(err, &domainErr) {
					conflicts = append(conflicts, domainErr.Conflict)
				}
				if !req.PartialOnError {
					return nil, err
				}
				continue
			}
			return nil, err
		}
		toCreate = append(toCreate, schedule)
	}

	if len(toCreate) > 0 {
		if err := s.repo.BulkCreate(ctx, toCreate); err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to bulk create schedules")
		}
	}

	result := &BulkCreateSchedulesResult{Created: toCreate, Conflicts: conflicts}
	if len(conflicts) > 0 && !req.PartialOnError {
		return nil, appErrors.Clone(appErrors.ErrConflict, "schedule conflicts detected")
	}
	return result, nil
}

func (s *ScheduleService) ensureNoConflict(ctx context.Context, schedule models.Schedule, ignoreID string) error {
	existing, err := s.repo.FindConflicts(ctx, schedule.TermID, schedule.DayOfWeek, schedule.TimeSlot)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check schedule conflicts")
	}

	for _, item := range existing {
		if item.ID == ignoreID {
			continue
		}
		if item.ClassID == schedule.ClassID {
			return s.wrapConflict("CLASS", "class already scheduled for this slot", item)
		}
		if item.TeacherID == schedule.TeacherID {
			return s.wrapConflict("TEACHER", "teacher already scheduled for this slot", item)
		}
		if strings.EqualFold(item.Room, schedule.Room) {
			return s.wrapConflict("ROOM", "room already booked for this slot", item)
		}
	}
	return nil
}

func (s *ScheduleService) wrapConflict(conflictType, message string, existing models.Schedule) error {
	conflict := models.ScheduleConflict{
		ScheduleID: existing.ID,
		TermID:     existing.TermID,
		ClassID:    existing.ClassID,
		SubjectID:  existing.SubjectID,
		TeacherID:  existing.TeacherID,
		DayOfWeek:  existing.DayOfWeek,
		TimeSlot:   existing.TimeSlot,
		Room:       existing.Room,
		Dimension:  conflictType,
	}
	domainErr := &models.ScheduleConflictError{Type: conflictType, Message: message, Conflict: conflict}
	return appErrors.Wrap(domainErr, appErrors.ErrConflict.Code, appErrors.ErrConflict.Status, fmt.Sprintf("schedule conflict: %s", message))
}
