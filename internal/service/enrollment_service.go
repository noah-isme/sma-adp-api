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

type enrollmentRepository interface {
	List(ctx context.Context, filter models.EnrollmentFilter) ([]models.EnrollmentDetail, int, error)
	FindByID(ctx context.Context, id string) (*models.Enrollment, error)
	FindDetailByID(ctx context.Context, id string) (*models.EnrollmentDetail, error)
	ExistsActive(ctx context.Context, studentID, classID, termID, excludeID string) (bool, error)
	Create(ctx context.Context, enrollment *models.Enrollment) error
	UpdateClass(ctx context.Context, id, classID string) error
	UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus, leftAt *time.Time) error
}

type studentReader interface {
	FindByID(ctx context.Context, id string) (*models.StudentDetail, error)
}

type classReader interface {
	FindByID(ctx context.Context, id string) (*models.Class, error)
}

type termReader interface {
	FindByID(ctx context.Context, id string) (*models.Term, error)
}

// EnrollStudentRequest describes enrollment creation request.
type EnrollStudentRequest struct {
	StudentID string `json:"student_id" validate:"required"`
	ClassID   string `json:"class_id" validate:"required"`
	TermID    string `json:"term_id" validate:"required"`
}

// TransferEnrollmentRequest describes transfer payload.
type TransferEnrollmentRequest struct {
	TargetClassID string `json:"target_class_id" validate:"required"`
}

// EnrollmentService orchestrates enrollment workflows.
type EnrollmentService struct {
	repo      enrollmentRepository
	students  studentReader
	classes   classReader
	terms     termReader
	validator *validator.Validate
	logger    *zap.Logger
}

// NewEnrollmentService constructs EnrollmentService.
func NewEnrollmentService(repo enrollmentRepository, students studentReader, classes classReader, terms termReader, validate *validator.Validate, logger *zap.Logger) *EnrollmentService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &EnrollmentService{repo: repo, students: students, classes: classes, terms: terms, validator: validate, logger: logger}
}

// List returns enrollments with pagination metadata.
func (s *EnrollmentService) List(ctx context.Context, filter models.EnrollmentFilter) ([]models.EnrollmentDetail, *models.Pagination, error) {
	enrollments, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list enrollments")
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
	return enrollments, pagination, nil
}

// Enroll registers a student to a class for a term.
func (s *EnrollmentService) Enroll(ctx context.Context, req EnrollStudentRequest) (*models.EnrollmentDetail, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid enrollment payload")
	}
	student, err := s.students.FindByID(ctx, req.StudentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load student")
	}
	if !student.Active {
		return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, "student inactive")
	}
	if _, err := s.classes.FindByID(ctx, req.ClassID); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "class not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class")
	}
	if _, err := s.terms.FindByID(ctx, req.TermID); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "term not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load term")
	}
	exists, err := s.repo.ExistsActive(ctx, req.StudentID, req.ClassID, req.TermID, "")
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to validate enrollment")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "student already enrolled in class for term")
	}
	enrollment := &models.Enrollment{StudentID: req.StudentID, ClassID: req.ClassID, TermID: req.TermID, JoinedAt: time.Now().UTC(), Status: models.EnrollmentStatusActive}
	if err := s.repo.Create(ctx, enrollment); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create enrollment")
	}
	detail, err := s.repo.FindDetailByID(ctx, enrollment.ID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load enrollment detail")
	}
	return detail, nil
}

// Transfer moves an enrollment to a new class within same term.
func (s *EnrollmentService) Transfer(ctx context.Context, id string, req TransferEnrollmentRequest) (*models.EnrollmentDetail, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid transfer payload")
	}
	enrollment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "enrollment not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load enrollment")
	}
	if enrollment.Status != models.EnrollmentStatusActive {
		return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, "enrollment not active")
	}
	if enrollment.ClassID == req.TargetClassID {
		return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, "already in target class")
	}
	if _, err := s.classes.FindByID(ctx, req.TargetClassID); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "target class not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load target class")
	}
	exists, err := s.repo.ExistsActive(ctx, enrollment.StudentID, req.TargetClassID, enrollment.TermID, enrollment.ID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to validate enrollment")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "student already enrolled in target class")
	}
	if err := s.repo.UpdateClass(ctx, id, req.TargetClassID); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to transfer enrollment")
	}
	detail, err := s.repo.FindDetailByID(ctx, id)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load enrollment detail")
	}
	return detail, nil
}

// Unenroll marks an enrollment as left.
func (s *EnrollmentService) Unenroll(ctx context.Context, id string) (*models.EnrollmentDetail, error) {
	enrollment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "enrollment not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load enrollment")
	}
	if enrollment.Status != models.EnrollmentStatusActive {
		return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, "enrollment already inactive")
	}
	leftAt := time.Now().UTC()
	if err := s.repo.UpdateStatus(ctx, id, models.EnrollmentStatusLeft, &leftAt); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update enrollment status")
	}
	detail, err := s.repo.FindDetailByID(ctx, id)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load enrollment detail")
	}
	return detail, nil
}
