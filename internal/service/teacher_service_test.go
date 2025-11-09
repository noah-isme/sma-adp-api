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

type mockTeacherRepo struct {
	items       map[string]*models.Teacher
	emailIndex  map[string]string
	nipIndex    map[string]string
	listResult  []models.Teacher
	listTotal   int
	listErr     error
	deactivated []string
}

func (m *mockTeacherRepo) List(ctx context.Context, filter models.TeacherFilter) ([]models.Teacher, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.listResult, m.listTotal, nil
}

func (m *mockTeacherRepo) FindByID(ctx context.Context, id string) (*models.Teacher, error) {
	if teacher, ok := m.items[id]; ok {
		cp := *teacher
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockTeacherRepo) ExistsByEmail(ctx context.Context, email, excludeID string) (bool, error) {
	if owner, ok := m.emailIndex[email]; ok {
		if excludeID == "" || owner != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockTeacherRepo) ExistsByNIP(ctx context.Context, nip, excludeID string) (bool, error) {
	if owner, ok := m.nipIndex[nip]; ok {
		if excludeID == "" || owner != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockTeacherRepo) Create(ctx context.Context, teacher *models.Teacher) error {
	if m.items == nil {
		m.items = make(map[string]*models.Teacher)
	}
	if teacher.ID == "" {
		teacher.ID = "generated"
	}
	now := time.Now()
	teacher.CreatedAt = now
	teacher.UpdatedAt = now
	cp := *teacher
	m.items[teacher.ID] = &cp
	return nil
}

func (m *mockTeacherRepo) Update(ctx context.Context, teacher *models.Teacher) error {
	if m.items == nil {
		m.items = make(map[string]*models.Teacher)
	}
	cp := *teacher
	m.items[teacher.ID] = &cp
	return nil
}

func (m *mockTeacherRepo) Deactivate(ctx context.Context, id string) error {
	m.deactivated = append(m.deactivated, id)
	if t, ok := m.items[id]; ok {
		t.Active = false
	}
	return nil
}

func TestTeacherServiceCreate(t *testing.T) {
	repo := &mockTeacherRepo{}
	service := NewTeacherService(repo, validator.New(), zap.NewNop())

	teacher, err := service.Create(context.Background(), CreateTeacherRequest{
		Email:    "teach@example.com",
		FullName: "Teacher One",
	})
	require.NoError(t, err)
	assert.Equal(t, "teach@example.com", teacher.Email)
	assert.True(t, teacher.Active)
	assert.Len(t, repo.items, 1)
}

func TestTeacherServiceCreateDuplicateEmail(t *testing.T) {
	repo := &mockTeacherRepo{emailIndex: map[string]string{"teach@example.com": "another"}}
	service := NewTeacherService(repo, validator.New(), zap.NewNop())

	_, err := service.Create(context.Background(), CreateTeacherRequest{
		Email:    "teach@example.com",
		FullName: "Teacher One",
	})
	require.Error(t, err)
}

func TestTeacherServiceUpdate(t *testing.T) {
	repo := &mockTeacherRepo{
		items: map[string]*models.Teacher{
			"t1": {ID: "t1", Email: "teach@example.com", FullName: "Teacher One", Active: true},
		},
	}
	service := NewTeacherService(repo, validator.New(), zap.NewNop())

	active := true
	updated, err := service.Update(context.Background(), "t1", UpdateTeacherRequest{
		Email:    "updated@example.com",
		FullName: "Teacher Updated",
		Active:   &active,
	})
	require.NoError(t, err)
	assert.Equal(t, "updated@example.com", updated.Email)
	assert.Equal(t, "Teacher Updated", updated.FullName)
}

func TestTeacherServiceDeactivate(t *testing.T) {
	repo := &mockTeacherRepo{
		items: map[string]*models.Teacher{
			"t1": {ID: "t1", Email: "teach@example.com", FullName: "Teacher One", Active: true},
		},
	}
	service := NewTeacherService(repo, validator.New(), zap.NewNop())

	err := service.Deactivate(context.Background(), "t1")
	require.NoError(t, err)
	assert.Equal(t, []string{"t1"}, repo.deactivated)
}
