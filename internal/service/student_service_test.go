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

type mockStudentRepo struct {
	students    map[string]models.Student
	existsByNIS map[string]string
	deactivated []string
	lastFilter  models.StudentFilter
	listTotal   int
	err         error
}

func (m *mockStudentRepo) List(ctx context.Context, filter models.StudentFilter) ([]models.StudentDetail, int, error) {
	m.lastFilter = filter
	if m.err != nil {
		return nil, 0, m.err
	}
	details := make([]models.StudentDetail, 0, len(m.students))
	for _, s := range m.students {
		details = append(details, models.StudentDetail{Student: s})
	}
	return details, m.listTotal, nil
}

func (m *mockStudentRepo) FindByID(ctx context.Context, id string) (*models.StudentDetail, error) {
	if s, ok := m.students[id]; ok {
		detail := models.StudentDetail{Student: s}
		return &detail, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockStudentRepo) ExistsByNIS(ctx context.Context, nis string, excludeID string) (bool, error) {
	if id, ok := m.existsByNIS[nis]; ok {
		if excludeID == "" || id != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockStudentRepo) Create(ctx context.Context, student *models.Student) error {
	if m.students == nil {
		m.students = make(map[string]models.Student)
	}
	if student.ID == "" {
		student.ID = "generated"
	}
	m.students[student.ID] = *student
	return nil
}

func (m *mockStudentRepo) Update(ctx context.Context, student *models.Student) error {
	if m.students == nil {
		m.students = make(map[string]models.Student)
	}
	m.students[student.ID] = *student
	return nil
}

func (m *mockStudentRepo) Deactivate(ctx context.Context, id string) error {
	m.deactivated = append(m.deactivated, id)
	if s, ok := m.students[id]; ok {
		s.Active = false
		m.students[id] = s
	}
	return nil
}

func TestStudentServiceCreate(t *testing.T) {
	repo := &mockStudentRepo{existsByNIS: make(map[string]string)}
	svc := NewStudentService(repo, validator.New(), zap.NewNop())

	student, err := svc.Create(context.Background(), CreateStudentRequest{
		NIS:       "1234",
		FullName:  "John Doe",
		Gender:    "M",
		BirthDate: time.Now(),
		Address:   "Street",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, student.ID)
	assert.True(t, student.Active)
	assert.Equal(t, 1, len(repo.students))
}

func TestStudentServiceCreateDuplicate(t *testing.T) {
	repo := &mockStudentRepo{existsByNIS: map[string]string{"123": "another"}}
	svc := NewStudentService(repo, validator.New(), zap.NewNop())

	_, err := svc.Create(context.Background(), CreateStudentRequest{NIS: "123", FullName: "A", Gender: "M", BirthDate: time.Now()})
	require.Error(t, err)
}

func TestStudentServiceUpdate(t *testing.T) {
	repo := &mockStudentRepo{students: map[string]models.Student{"id1": {ID: "id1", NIS: "111", FullName: "Old", Gender: "M", Active: true}}, existsByNIS: make(map[string]string)}
	svc := NewStudentService(repo, validator.New(), zap.NewNop())

	updated, err := svc.Update(context.Background(), "id1", UpdateStudentRequest{NIS: "222", FullName: "New", Gender: "F", BirthDate: time.Now(), Active: true})
	require.NoError(t, err)
	assert.Equal(t, "222", updated.NIS)
	assert.Equal(t, "New", updated.FullName)
}

func TestStudentServiceDeactivate(t *testing.T) {
	repo := &mockStudentRepo{students: map[string]models.Student{"id1": {ID: "id1", NIS: "111", FullName: "Old", Gender: "M", Active: true}}}
	svc := NewStudentService(repo, validator.New(), zap.NewNop())

	err := svc.Deactivate(context.Background(), "id1")
	require.NoError(t, err)
	assert.Contains(t, repo.deactivated, "id1")
}
