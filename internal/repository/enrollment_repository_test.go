package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

func newEnrollmentRepoMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestEnrollmentRepositoryListActiveByStudent(t *testing.T) {
	db, mock, cleanup := newEnrollmentRepoMock(t)
	defer cleanup()
	repo := NewEnrollmentRepository(db)

	rows := sqlmock.NewRows([]string{"id", "student_id", "class_id", "term_id", "joined_at", "left_at", "status"}).
		AddRow("enr-1", "stu-1", "class-1", "term-1", time.Now(), nil, models.EnrollmentStatusActive)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, student_id, class_id, term_id, joined_at, left_at, status FROM enrollments WHERE student_id = $1 AND status = $2")).
		WithArgs("stu-1", models.EnrollmentStatusActive).
		WillReturnRows(rows)

	enrollments, err := repo.ListActiveByStudent(context.Background(), "stu-1")
	require.NoError(t, err)
	require.Len(t, enrollments, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}
