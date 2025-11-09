package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

func newTeacherRepoMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestTeacherRepositoryList(t *testing.T) {
	db, mock, cleanup := newTeacherRepoMock(t)
	defer cleanup()
	repo := NewTeacherRepository(db)

	rows := sqlmock.NewRows([]string{"id", "nip", "email", "full_name", "phone", "expertise", "active", "created_at", "updated_at"}).
		AddRow("t1", nil, "a@example.com", "Teacher A", nil, nil, true, time.Now(), time.Now())
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, nip, email, full_name, phone, expertise, active, created_at, updated_at FROM teachers WHERE 1=1 ORDER BY created_at DESC LIMIT 20 OFFSET 0")).
		WillReturnRows(rows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM teachers WHERE 1=1")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	list, total, err := repo.List(context.Background(), models.TeacherFilter{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, 1, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeacherRepositoryCreateAndDeactivate(t *testing.T) {
	db, mock, cleanup := newTeacherRepoMock(t)
	defer cleanup()
	repo := NewTeacherRepository(db)

	mock.ExpectExec("INSERT INTO teachers").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "a@example.com", "Teacher A", sqlmock.AnyArg(), sqlmock.AnyArg(), true, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Create(context.Background(), &models.Teacher{Email: "a@example.com", FullName: "Teacher A", Active: true})
	require.NoError(t, err)

	mock.ExpectExec("UPDATE teachers SET active = FALSE").
		WithArgs("id-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, repo.Deactivate(context.Background(), "id-1"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeacherRepositoryExistsByEmail(t *testing.T) {
	db, mock, cleanup := newTeacherRepoMock(t)
	defer cleanup()
	repo := NewTeacherRepository(db)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM teachers WHERE LOWER(email) = LOWER($1) LIMIT 1")).
		WithArgs("a@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	exists, err := repo.ExistsByEmail(context.Background(), "a@example.com", "")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.NoError(t, mock.ExpectationsWereMet())
}
