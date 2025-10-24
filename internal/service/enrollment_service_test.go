package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

type mockEnrollmentRepo struct {
	enrollments map[string]models.Enrollment
	activeMap   map[string]bool
	created     *models.Enrollment
	transferred []string
	status      map[string]models.EnrollmentStatus
}

func (m *mockEnrollmentRepo) List(ctx context.Context, filter models.EnrollmentFilter) ([]models.EnrollmentDetail, int, error) {
	return nil, 0, nil
}

func (m *mockEnrollmentRepo) FindByID(ctx context.Context, id string) (*models.Enrollment, error) {
	if e, ok := m.enrollments[id]; ok {
		return &e, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockEnrollmentRepo) FindDetailByID(ctx context.Context, id string) (*models.EnrollmentDetail, error) {
	if e, ok := m.enrollments[id]; ok {
		return &models.EnrollmentDetail{Enrollment: e}, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockEnrollmentRepo) ExistsActive(ctx context.Context, studentID, classID, termID, excludeID string) (bool, error) {
	if m.activeMap == nil {
		return false, nil
	}
	key := studentID + classID + termID
	if excludeID != "" {
		key += excludeID
	}
	return m.activeMap[key], nil
}

func (m *mockEnrollmentRepo) Create(ctx context.Context, enrollment *models.Enrollment) error {
	if m.enrollments == nil {
		m.enrollments = make(map[string]models.Enrollment)
	}
	if enrollment.ID == "" {
		enrollment.ID = "new-enroll"
	}
	m.enrollments[enrollment.ID] = *enrollment
	m.created = enrollment
	return nil
}

func (m *mockEnrollmentRepo) UpdateClass(ctx context.Context, id, classID string) error {
	if e, ok := m.enrollments[id]; ok {
		e.ClassID = classID
		m.enrollments[id] = e
	}
	m.transferred = append(m.transferred, id)
	return nil
}

func (m *mockEnrollmentRepo) UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus, leftAt *time.Time) error {
	if m.status == nil {
		m.status = make(map[string]models.EnrollmentStatus)
	}
	m.status[id] = status
	if e, ok := m.enrollments[id]; ok {
		e.Status = status
		e.LeftAt = leftAt
		m.enrollments[id] = e
	}
	return nil
}

func (m *mockEnrollmentRepo) ListByClassAndTerm(ctx context.Context, classID, termID string) ([]models.Enrollment, error) {
	var list []models.Enrollment
	for _, e := range m.enrollments {
		if e.ClassID == classID && e.TermID == termID {
			list = append(list, e)
		}
	}
	return list, nil
}

type mockStudentReader struct {
	students map[string]*models.StudentDetail
}

func (m *mockStudentReader) FindByID(ctx context.Context, id string) (*models.StudentDetail, error) {
	if s, ok := m.students[id]; ok {
		return s, nil
	}
	return nil, sql.ErrNoRows
}

type mockClassReader struct{}

func (m *mockClassReader) FindByID(ctx context.Context, id string) (*models.Class, error) {
	if id == "missing" {
		return nil, sql.ErrNoRows
	}
	return &models.Class{ID: id}, nil
}

type mockTermReader struct{}

func (m *mockTermReader) FindByID(ctx context.Context, id string) (*models.Term, error) {
	if id == "missing" {
		return nil, sql.ErrNoRows
	}
	return &models.Term{ID: id}, nil
}

func TestEnrollmentServiceEnroll(t *testing.T) {
	repo := &mockEnrollmentRepo{}
	students := &mockStudentReader{students: map[string]*models.StudentDetail{"s1": {Student: models.Student{ID: "s1", Active: true}}}}
	svc := NewEnrollmentService(repo, students, &mockClassReader{}, &mockTermReader{}, validator.New(), zap.NewNop())

	detail, err := svc.Enroll(context.Background(), EnrollStudentRequest{StudentID: "s1", ClassID: "c1", TermID: "t1"})
	require.NoError(t, err)
	assert.NotNil(t, detail)
	assert.NotNil(t, repo.created)
}

func TestEnrollmentServiceTransfer(t *testing.T) {
	repo := &mockEnrollmentRepo{enrollments: map[string]models.Enrollment{"e1": {ID: "e1", StudentID: "s1", ClassID: "c1", TermID: "t1", Status: models.EnrollmentStatusActive}}}
	students := &mockStudentReader{students: map[string]*models.StudentDetail{"s1": {Student: models.Student{ID: "s1", Active: true}}}}
	svc := NewEnrollmentService(repo, students, &mockClassReader{}, &mockTermReader{}, validator.New(), zap.NewNop())

	detail, err := svc.Transfer(context.Background(), "e1", TransferEnrollmentRequest{TargetClassID: "c2"})
	require.NoError(t, err)
	assert.Equal(t, "c2", detail.ClassID)
	assert.Contains(t, repo.transferred, "e1")
}

func TestEnrollmentServiceUnenroll(t *testing.T) {
	now := time.Now()
	repo := &mockEnrollmentRepo{enrollments: map[string]models.Enrollment{"e1": {ID: "e1", StudentID: "s1", ClassID: "c1", TermID: "t1", Status: models.EnrollmentStatusActive, JoinedAt: now}}}
	students := &mockStudentReader{students: map[string]*models.StudentDetail{"s1": {Student: models.Student{ID: "s1", Active: true}}}}
	svc := NewEnrollmentService(repo, students, &mockClassReader{}, &mockTermReader{}, validator.New(), zap.NewNop())

	detail, err := svc.Unenroll(context.Background(), "e1")
	require.NoError(t, err)
	assert.Equal(t, models.EnrollmentStatusLeft, detail.Status)
	assert.Equal(t, models.EnrollmentStatusLeft, repo.status["e1"])
}
