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

func newStudentMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestStudentRepositoryList(t *testing.T) {
	db, mock, cleanup := newStudentMock(t)
	defer cleanup()
	repo := NewStudentRepository(db)

	rows := sqlmock.NewRows([]string{"id", "nis", "full_name", "gender", "birth_date", "address", "phone", "active", "created_at", "updated_at", "current_class_id", "current_class_name", "current_term_id", "joined_at"}).
		AddRow("1", "001", "Student", "M", time.Now(), "Street", "123", true, time.Now(), time.Now(), "class", "Class", "term", time.Now())
	mock.ExpectQuery(regexp.QuoteMeta("SELECT s.id, s.nis, s.full_name, s.gender, s.birth_date, s.address, s.phone, s.active, s.created_at, s.updated_at,\n        e.class_id AS current_class_id, c.name AS current_class_name, e.term_id AS current_term_id, e.joined_at\n        FROM students s LEFT JOIN enrollments e ON e.student_id = s.id AND e.status = $1 LEFT JOIN classes c ON c.id = e.class_id WHERE 1=1 ORDER BY s.created_at DESC LIMIT 20 OFFSET 0")).
		WithArgs(models.EnrollmentStatusActive).
		WillReturnRows(rows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(DISTINCT s.id) FROM students s LEFT JOIN enrollments e ON e.student_id = s.id AND e.status = $1 LEFT JOIN classes c ON c.id = e.class_id WHERE 1=1")).
		WithArgs(models.EnrollmentStatusActive).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	students, total, err := repo.List(context.Background(), models.StudentFilter{})
	require.NoError(t, err)
	assert.Len(t, students, 1)
	assert.Equal(t, 1, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStudentRepositoryCreate(t *testing.T) {
	db, mock, cleanup := newStudentMock(t)
	defer cleanup()
	repo := NewStudentRepository(db)

	mock.ExpectExec("INSERT INTO students").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Create(context.Background(), &models.Student{NIS: "123", FullName: "Student", Gender: "M", BirthDate: time.Now(), Address: "Street", Phone: "123", Active: true})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
