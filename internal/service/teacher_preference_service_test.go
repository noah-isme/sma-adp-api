package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

type prefRepoMock struct {
	stored *models.TeacherPreference
	err    error
}

func (m *prefRepoMock) GetByTeacher(ctx context.Context, teacherID string) (*models.TeacherPreference, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.stored == nil {
		return nil, sql.ErrNoRows
	}
	cp := *m.stored
	return &cp, nil
}

func (m *prefRepoMock) Upsert(ctx context.Context, pref *models.TeacherPreference) error {
	cp := *pref
	m.stored = &cp
	return nil
}

func TestTeacherPreferenceServiceGetDefault(t *testing.T) {
	teacherRepo := &teacherRepoStub{
		items: map[string]*models.Teacher{"teacher-1": {ID: "teacher-1", Active: true}},
	}
	repo := &prefRepoMock{}
	service := NewTeacherPreferenceService(teacherRepo, repo, validator.New(), zap.NewNop())

	pref, err := service.Get(context.Background(), "teacher-1")
	require.NoError(t, err)
	assert.Equal(t, "teacher-1", pref.TeacherID)
	assert.Equal(t, types.JSONText("[]"), pref.Unavailable)
}

func TestTeacherPreferenceServiceUpsert(t *testing.T) {
	teacherRepo := &teacherRepoStub{
		items: map[string]*models.Teacher{"teacher-1": {ID: "teacher-1", Active: true}},
	}
	repo := &prefRepoMock{}
	service := NewTeacherPreferenceService(teacherRepo, repo, validator.New(), zap.NewNop())

	result, err := service.Upsert(context.Background(), "teacher-1", UpsertTeacherPreferenceRequest{
		MaxLoadPerDay:  4,
		MaxLoadPerWeek: 12,
		Unavailable: []models.TeacherUnavailableSlot{
			{DayOfWeek: "MONDAY", TimeRange: "1"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 4, result.MaxLoadPerDay)
	assert.NotNil(t, repo.stored)
}
