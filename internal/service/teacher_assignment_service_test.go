package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

type teacherRepoStub struct {
	items map[string]*models.Teacher
}

func (s *teacherRepoStub) List(ctx context.Context, filter models.TeacherFilter) ([]models.Teacher, int, error) {
	return nil, 0, nil
}

func (s *teacherRepoStub) FindByID(ctx context.Context, id string) (*models.Teacher, error) {
	if teacher, ok := s.items[id]; ok {
		cp := *teacher
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}

func (s *teacherRepoStub) ExistsByEmail(ctx context.Context, email, excludeID string) (bool, error) {
	return false, nil
}

func (s *teacherRepoStub) ExistsByNIP(ctx context.Context, nip, excludeID string) (bool, error) {
	return false, nil
}

func (s *teacherRepoStub) Create(ctx context.Context, teacher *models.Teacher) error { return nil }
func (s *teacherRepoStub) Update(ctx context.Context, teacher *models.Teacher) error { return nil }
func (s *teacherRepoStub) Deactivate(ctx context.Context, id string) error           { return nil }

type stubClassRepo struct{}

func (stubClassRepo) FindByID(ctx context.Context, id string) (*models.Class, error) {
	return &models.Class{ID: id}, nil
}

type stubSubjectRepo struct{}

func (stubSubjectRepo) FindByID(ctx context.Context, id string) (*models.Subject, error) {
	return &models.Subject{ID: id}, nil
}

type stubTermRepo struct{}

func (stubTermRepo) FindByID(ctx context.Context, id string) (*models.Term, error) {
	return &models.Term{ID: id}, nil
}

type assignmentRepoStub struct {
	exists     bool
	created    []*models.TeacherAssignment
	deleteErr  error
	count      int
	deleteArgs []string
}

func (s *assignmentRepoStub) ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error) {
	return nil, nil
}

func (s *assignmentRepoStub) Exists(ctx context.Context, teacherID, classID, subjectID, termID string) (bool, error) {
	return s.exists, nil
}

func (s *assignmentRepoStub) Create(ctx context.Context, assignment *models.TeacherAssignment) error {
	s.created = append(s.created, assignment)
	return nil
}

func (s *assignmentRepoStub) Delete(ctx context.Context, teacherID, assignmentID string) error {
	s.deleteArgs = append(s.deleteArgs, teacherID+":"+assignmentID)
	return s.deleteErr
}

func (s *assignmentRepoStub) CountByTeacherAndTerm(ctx context.Context, teacherID, termID string) (int, error) {
	return s.count, nil
}

type scheduleReaderStub struct {
	class    []models.Schedule
	teacher  []models.Schedule
	classErr error
	teachErr error
}

func (s *scheduleReaderStub) ListByClass(ctx context.Context, classID string) ([]models.Schedule, error) {
	return s.class, s.classErr
}

func (s *scheduleReaderStub) ListByTeacher(ctx context.Context, teacherID string) ([]models.Schedule, error) {
	return s.teacher, s.teachErr
}

type preferenceRepoStub struct {
	pref *models.TeacherPreference
	err  error
}

func (s *preferenceRepoStub) GetByTeacher(ctx context.Context, teacherID string) (*models.TeacherPreference, error) {
	return s.pref, s.err
}

func (s *preferenceRepoStub) Upsert(ctx context.Context, pref *models.TeacherPreference) error {
	return nil
}

func TestTeacherAssignmentServiceAssign(t *testing.T) {
	teacherRepo := &teacherRepoStub{
		items: map[string]*models.Teacher{"teacher-1": {ID: "teacher-1", Active: true}},
	}
	assignRepo := &assignmentRepoStub{}
	schedules := &scheduleReaderStub{
		class: []models.Schedule{
			{ClassID: "class-1", SubjectID: "subject-1", TermID: "term-1", DayOfWeek: "MONDAY", TimeSlot: "1"},
		},
		teacher: []models.Schedule{},
	}
	prefs := &preferenceRepoStub{}

	service := NewTeacherAssignmentService(teacherRepo, stubClassRepo{}, stubSubjectRepo{}, stubTermRepo{}, assignRepo, schedules, prefs, validator.New(), zap.NewNop())

	assignment, err := service.Assign(context.Background(), "teacher-1", CreateTeacherAssignmentRequest{
		ClassID:   "class-1",
		SubjectID: "subject-1",
		TermID:    "term-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "teacher-1", assignment.TeacherID)
	assert.Len(t, assignRepo.created, 1)
}

func TestTeacherAssignmentServiceAssignDuplicate(t *testing.T) {
	teacherRepo := &teacherRepoStub{
		items: map[string]*models.Teacher{"teacher-1": {ID: "teacher-1", Active: true}},
	}
	assignRepo := &assignmentRepoStub{exists: true}
	service := NewTeacherAssignmentService(teacherRepo, stubClassRepo{}, stubSubjectRepo{}, stubTermRepo{}, assignRepo, &scheduleReaderStub{}, &preferenceRepoStub{}, validator.New(), zap.NewNop())

	_, err := service.Assign(context.Background(), "teacher-1", CreateTeacherAssignmentRequest{
		ClassID:   "class-1",
		SubjectID: "subject-1",
		TermID:    "term-1",
	})
	require.Error(t, err)
}

func TestTeacherAssignmentServiceAssignScheduleConflict(t *testing.T) {
	teacherRepo := &teacherRepoStub{
		items: map[string]*models.Teacher{"teacher-1": {ID: "teacher-1", Active: true}},
	}
	assignRepo := &assignmentRepoStub{}
	schedules := &scheduleReaderStub{
		class: []models.Schedule{
			{ClassID: "class-1", SubjectID: "subject-1", TermID: "term-1", DayOfWeek: "MONDAY", TimeSlot: "1"},
		},
		teacher: []models.Schedule{
			{ID: "sched-1", ClassID: "another", SubjectID: "subject-x", TermID: "term-1", DayOfWeek: "MONDAY", TimeSlot: "1"},
		},
	}
	service := NewTeacherAssignmentService(teacherRepo, stubClassRepo{}, stubSubjectRepo{}, stubTermRepo{}, assignRepo, schedules, &preferenceRepoStub{}, validator.New(), zap.NewNop())

	_, err := service.Assign(context.Background(), "teacher-1", CreateTeacherAssignmentRequest{
		ClassID:   "class-1",
		SubjectID: "subject-1",
		TermID:    "term-1",
	})
	require.Error(t, err)
}

func TestTeacherAssignmentServiceRemove(t *testing.T) {
	teacherRepo := &teacherRepoStub{
		items: map[string]*models.Teacher{"teacher-1": {ID: "teacher-1", Active: true}},
	}
	assignRepo := &assignmentRepoStub{}
	service := NewTeacherAssignmentService(teacherRepo, stubClassRepo{}, stubSubjectRepo{}, stubTermRepo{}, assignRepo, &scheduleReaderStub{}, &preferenceRepoStub{}, validator.New(), zap.NewNop())

	err := service.Remove(context.Background(), "teacher-1", "assignment-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"teacher-1:assignment-1"}, assignRepo.deleteArgs)
}
