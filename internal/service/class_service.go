package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type classRepository interface {
	List(ctx context.Context, filter models.ClassFilter) ([]models.Class, int, error)
	FindByID(ctx context.Context, id string) (*models.Class, error)
	FindDetailByID(ctx context.Context, id string) (*models.ClassDetail, error)
	ExistsByName(ctx context.Context, name string, excludeID string) (bool, error)
	Create(ctx context.Context, class *models.Class) error
	Update(ctx context.Context, class *models.Class) error
	Delete(ctx context.Context, id string) error
	CountClassSubjects(ctx context.Context, classID string) (int, error)
	CountSchedules(ctx context.Context, classID string) (int, error)
}

type classSubjectRepo interface {
	ListByClass(ctx context.Context, classID string) ([]models.ClassSubjectAssignment, error)
	ReplaceAssignments(ctx context.Context, classID string, assignments []models.ClassSubject) error
}

// CreateClassRequest captures creation payload.
type CreateClassRequest struct {
	Name              string  `json:"name" validate:"required"`
	Grade             string  `json:"grade" validate:"required"`
	Track             string  `json:"track" validate:"required"`
	HomeroomTeacherID *string `json:"homeroom_teacher_id"`
}

// UpdateClassRequest modifies class fields.
type UpdateClassRequest struct {
	Name              string  `json:"name" validate:"required"`
	Grade             string  `json:"grade" validate:"required"`
	Track             string  `json:"track" validate:"required"`
	HomeroomTeacherID *string `json:"homeroom_teacher_id"`
}

// AssignSubjectPayload describes class-subject assignment.
type AssignSubjectPayload struct {
	SubjectID string  `json:"subject_id" validate:"required"`
	TeacherID *string `json:"teacher_id"`
}

// AssignSubjectsRequest handles bulk assignment.
type AssignSubjectsRequest struct {
	Subjects []AssignSubjectPayload `json:"subjects" validate:"dive"`
}

// ClassService coordinates class operations.
type ClassService struct {
	repo        classRepository
	subjectRepo subjectRepository
	mappingRepo classSubjectRepo
	validator   *validator.Validate
	logger      *zap.Logger
}

// NewClassService constructs ClassService.
func NewClassService(repo classRepository, subjectRepo subjectRepository, mappingRepo classSubjectRepo, validate *validator.Validate, logger *zap.Logger) *ClassService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ClassService{repo: repo, subjectRepo: subjectRepo, mappingRepo: mappingRepo, validator: validate, logger: logger}
}

// List returns classes with pagination metadata.
func (s *ClassService) List(ctx context.Context, filter models.ClassFilter) ([]models.Class, *models.Pagination, error) {
	classes, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list classes")
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
	return classes, pagination, nil
}

// Get returns detailed class information.
func (s *ClassService) Get(ctx context.Context, id string) (*models.ClassDetail, error) {
	detail, err := s.repo.FindDetailByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "class not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class")
	}
	return detail, nil
}

// Create adds a new class.
func (s *ClassService) Create(ctx context.Context, req CreateClassRequest) (*models.Class, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid class payload")
	}

	exists, err := s.repo.ExistsByName(ctx, req.Name, "")
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check class name")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "class name already exists")
	}

	class := &models.Class{
		Name:              req.Name,
		Grade:             req.Grade,
		Track:             req.Track,
		HomeroomTeacherID: req.HomeroomTeacherID,
	}
	if err := s.repo.Create(ctx, class); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create class")
	}
	return class, nil
}

// Update modifies a class record.
func (s *ClassService) Update(ctx context.Context, id string, req UpdateClassRequest) (*models.Class, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid class payload")
	}

	class, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "class not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class")
	}

	exists, err := s.repo.ExistsByName(ctx, req.Name, id)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check class name")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "class name already exists")
	}

	class.Name = req.Name
	class.Grade = req.Grade
	class.Track = req.Track
	class.HomeroomTeacherID = req.HomeroomTeacherID

	if err := s.repo.Update(ctx, class); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update class")
	}
	return class, nil
}

// Delete removes a class ensuring no schedules or subject mappings remain.
func (s *ClassService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "class not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class")
	}

	if count, err := s.repo.CountClassSubjects(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check class mappings")
	} else if count > 0 {
		return appErrors.Clone(appErrors.ErrPreconditionFailed, "class has subject assignments")
	}

	if count, err := s.repo.CountSchedules(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check class schedules")
	} else if count > 0 {
		return appErrors.Clone(appErrors.ErrPreconditionFailed, "class has schedules")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete class")
	}
	return nil
}

// ListSubjects returns subject assignments for the class.
func (s *ClassService) ListSubjects(ctx context.Context, classID string) ([]models.ClassSubjectAssignment, error) {
	if _, err := s.repo.FindByID(ctx, classID); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "class not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class")
	}

	assignments, err := s.mappingRepo.ListByClass(ctx, classID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list class subjects")
	}
	return assignments, nil
}

// AssignSubjects replaces the class subject assignments.
func (s *ClassService) AssignSubjects(ctx context.Context, classID string, req AssignSubjectsRequest) error {
	if err := s.validator.Struct(req); err != nil {
		return appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid assignment payload")
	}

	if _, err := s.repo.FindByID(ctx, classID); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "class not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class")
	}

	seen := make(map[string]struct{})
	assignments := make([]models.ClassSubject, 0, len(req.Subjects))
	now := time.Now().UTC()

	for _, item := range req.Subjects {
		if err := s.validator.Struct(item); err != nil {
			return appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, fmt.Sprintf("invalid subject entry %s", item.SubjectID))
		}
		if _, ok := seen[item.SubjectID]; ok {
			return appErrors.Clone(appErrors.ErrValidation, "duplicate subject in assignments")
		}
		seen[item.SubjectID] = struct{}{}

		if s.subjectRepo != nil {
			if _, err := s.subjectRepo.FindByID(ctx, item.SubjectID); err != nil {
				if err == sql.ErrNoRows {
					return appErrors.Clone(appErrors.ErrNotFound, "subject not found")
				}
				return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to validate subject")
			}
		}

		assignments = append(assignments, models.ClassSubject{
			ClassID:   classID,
			SubjectID: item.SubjectID,
			TeacherID: item.TeacherID,
			CreatedAt: now,
		})
	}

	if err := s.mappingRepo.ReplaceAssignments(ctx, classID, assignments); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to assign class subjects")
	}
	return nil
}
