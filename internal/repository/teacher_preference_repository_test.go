package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

func newTeacherPrefMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestTeacherPreferenceRepositoryGetAndUpsert(t *testing.T) {
	db, mock, cleanup := newTeacherPrefMock(t)
	defer cleanup()
	repo := NewTeacherPreferenceRepository(db)

	mock.ExpectExec("INSERT INTO teacher_preferences").
		WithArgs(sqlmock.AnyArg(), "teacher-1", 6, 30, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Upsert(context.Background(), &models.TeacherPreference{
		TeacherID:      "teacher-1",
		MaxLoadPerDay:  6,
		MaxLoadPerWeek: 30,
		Unavailable:    types.JSONText(`[]`),
	})
	require.NoError(t, err)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "teacher_id", "max_load_per_day", "max_load_per_week", "unavailable", "created_at", "updated_at"}).
		AddRow("pref-1", "teacher-1", 6, 30, `[]`, now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, teacher_id, max_load_per_day, max_load_per_week, unavailable, created_at, updated_at FROM teacher_preferences WHERE teacher_id = $1")).
		WithArgs("teacher-1").
		WillReturnRows(rows)

	pref, err := repo.GetByTeacher(context.Background(), "teacher-1")
	require.NoError(t, err)
	assert.Equal(t, "pref-1", pref.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}
