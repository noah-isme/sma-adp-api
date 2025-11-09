package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type teacherAssignmentRepo interface {
	ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error)
	Exists(ctx context.Context, teacherID, classID, subjectID, termID string) (bool, error)
	Create(ctx context.Context, assignment *models.TeacherAssignment) error
	Delete(ctx context.Context, teacherID, assignmentID string) error
	CountByTeacherAndTerm(ctx context.Context, teacherID, termID string) (int, error)
}

type classReader interface {
	FindByID(ctx context.Context, id string) (*models.Class, error)
}

type subjectReader interface {
	FindByID(ctx context.Context, id string) (*models.Subject, error)
}

type termReader interface {
	FindByID(ctx context.Context, id string) (*models.Term, error)
}

type scheduleReader interface {
	ListByClass(ctx context.Context, classID string) ([]models.Schedule, error)
	ListByTeacher(ctx context.Context, teacherID string) ([]models.Schedule, error)
}

type teacherPreferenceReader interface {
	GetByTeacher(ctx context.Context, teacherID string) (*models.TeacherPreference, error)
}

// CreateTeacherAssignmentRequest describes assignment payload.
type CreateTeacherAssignmentRequest struct {
	ClassID   string `json:"class_id" validate:"required"`
	SubjectID string `json:"subject_id" validate:"required"`
	TermID    string `json:"term_id" validate:"required"`
}

// TeacherAssignmentService handles roster assignments.
type TeacherAssignmentService struct {
	teachers    teacherRepository
	classes     classReader
	subjects    subjectReader
	terms       termReader
	assignments teacherAssignmentRepo
	schedules   scheduleReader
	prefs       teacherPreferenceReader
	validator   *validator.Validate
	logger      *zap.Logger
}

// NewTeacherAssignmentService creates a service instance.
func NewTeacherAssignmentService(
	teachers teacherRepository,
	classes classReader,
	subjects subjectReader,
	terms termReader,
	assignments teacherAssignmentRepo,
	schedules scheduleReader,
	prefs teacherPreferenceReader,
	validate *validator.Validate,
	logger *zap.Logger,
) *TeacherAssignmentService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TeacherAssignmentService{
		teachers:    teachers,
		classes:     classes,
		subjects:    subjects,
		terms:       terms,
		assignments: assignments,
		schedules:   schedules,
		prefs:       prefs,
		validator:   validate,
		logger:      logger,
	}
}

// ListByTeacher returns assignments for the teacher.
func (s *TeacherAssignmentService) ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error) {
	if _, err := s.teachers.FindByID(ctx, teacherID); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "teacher not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher")
	}
	assignments, err := s.assignments.ListByTeacher(ctx, teacherID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list assignments")
	}
	return assignments, nil
}

// Assign creates a new mapping between teacher-class-subject-term.
func (s *TeacherAssignmentService) Assign(ctx context.Context, teacherID string, req CreateTeacherAssignmentRequest) (*models.TeacherAssignment, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid assignment payload")
	}

	teacher, err := s.teachers.FindByID(ctx, teacherID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "teacher not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher")
	}
	if !teacher.Active {
		return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, "teacher inactive")
	}

	if err := s.ensureClassSubjectTerm(ctx, req.ClassID, req.SubjectID, req.TermID); err != nil {
		return nil, err
	}

	exists, err := s.assignments.Exists(ctx, teacherID, req.ClassID, req.SubjectID, req.TermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check assignment uniqueness")
	}
	if exists {
		return nil, appErrors.Clone(appErrors.ErrConflict, "teacher already assigned to this class and subject")
	}

	if err := s.ensureScheduleAvailability(ctx, teacherID, req.ClassID, req.SubjectID, req.TermID); err != nil {
		return nil, err
	}

	if err := s.ensureLoadCapacity(ctx, teacherID, req.TermID); err != nil {
		return nil, err
	}

	assignment := &models.TeacherAssignment{
		TeacherID: teacherID,
		ClassID:   req.ClassID,
		SubjectID: req.SubjectID,
		TermID:    req.TermID,
	}
	if err := s.assignments.Create(ctx, assignment); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create assignment")
	}
	return assignment, nil
}

// Remove deletes an assignment.
func (s *TeacherAssignmentService) Remove(ctx context.Context, teacherID, assignmentID string) error {
	if _, err := s.teachers.FindByID(ctx, teacherID); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "teacher not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher")
	}
	if err := s.assignments.Delete(ctx, teacherID, assignmentID); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "assignment not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete assignment")
	}
	return nil
}

func (s *TeacherAssignmentService) ensureClassSubjectTerm(ctx context.Context, classID, subjectID, termID string) error {
	if _, err := s.classes.FindByID(ctx, classID); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "class not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class")
	}
	if _, err := s.subjects.FindByID(ctx, subjectID); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "subject not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load subject")
	}
	if _, err := s.terms.FindByID(ctx, termID); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "term not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load term")
	}
	return nil
}

func (s *TeacherAssignmentService) ensureScheduleAvailability(ctx context.Context, teacherID, classID, subjectID, termID string) error {
	if s.schedules == nil {
		return nil
	}
	classSchedules, err := s.schedules.ListByClass(ctx, classID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class schedules")
	}
	teacherSchedules, err := s.schedules.ListByTeacher(ctx, teacherID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher schedules")
	}

	slotMap := make(map[string]models.Schedule)
	for _, sched := range teacherSchedules {
		if sched.TermID != termID {
			continue
		}
		key := buildScheduleKey(sched.DayOfWeek, sched.TimeSlot)
		slotMap[key] = sched
	}

	for _, sched := range classSchedules {
		if sched.TermID != termID || !strings.EqualFold(sched.SubjectID, subjectID) {
			continue
		}
		key := buildScheduleKey(sched.DayOfWeek, sched.TimeSlot)
		if conflict, ok := slotMap[key]; ok && conflict.ClassID != classID {
			return s.wrapScheduleConflict(conflict)
		}
	}
	return nil
}

func (s *TeacherAssignmentService) ensureLoadCapacity(ctx context.Context, teacherID, termID string) error {
	if s.prefs == nil {
		return nil
	}
	pref, err := s.prefs.GetByTeacher(ctx, teacherID)
	if err != nil && err != sql.ErrNoRows {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to read teacher preferences")
	}
	if pref == nil || pref.MaxLoadPerWeek <= 0 {
		return nil
	}
	count, err := s.assignments.CountByTeacherAndTerm(ctx, teacherID, termID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to read assignment load")
	}
	if count >= pref.MaxLoadPerWeek {
		return appErrors.Clone(appErrors.ErrPreconditionFailed, "teacher has reached weekly load limit")
	}
	return nil
}

func (s *TeacherAssignmentService) wrapScheduleConflict(existing models.Schedule) error {
	conflict := models.ScheduleConflict{
		ScheduleID: existing.ID,
		TermID:     existing.TermID,
		ClassID:    existing.ClassID,
		SubjectID:  existing.SubjectID,
		TeacherID:  existing.TeacherID,
		DayOfWeek:  existing.DayOfWeek,
		TimeSlot:   existing.TimeSlot,
		Room:       existing.Room,
		Dimension:  "TEACHER",
	}
	detail := &models.ScheduleConflictError{
		Type:     "TEACHER",
		Message:  fmt.Sprintf("teacher already scheduled on %s %s", existing.DayOfWeek, existing.TimeSlot),
		Conflict: conflict,
	}
	return appErrors.Wrap(detail, appErrors.ErrConflict.Code, appErrors.ErrConflict.Status, "schedule conflict detected")
}

func buildScheduleKey(day, slot string) string {
	return strings.ToUpper(day) + "|" + slot
}
