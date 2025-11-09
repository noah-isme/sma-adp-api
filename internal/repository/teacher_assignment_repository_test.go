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

func newTeacherAssignmentMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestTeacherAssignmentRepositoryList(t *testing.T) {
	db, mock, cleanup := newTeacherAssignmentMock(t)
	defer cleanup()
	repo := NewTeacherAssignmentRepository(db)

	rows := sqlmock.NewRows([]string{"id", "teacher_id", "class_id", "subject_id", "term_id", "created_at", "class_name", "subject_name", "term_name", "teacher_name"}).
		AddRow("assign-1", "teacher-1", "class-1", "subject-1", "term-1", time.Now(), "Class A", "Math", "Semester 1", "Teacher One")
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT ta.id, ta.teacher_id, ta.class_id, ta.subject_id, ta.term_id, ta.created_at,
       c.name AS class_name, s.name AS subject_name, t.name AS term_name, tr.full_name AS teacher_name
FROM teacher_assignments ta
JOIN classes c ON c.id = ta.class_id
JOIN subjects s ON s.id = ta.subject_id
JOIN terms t ON t.id = ta.term_id
JOIN teachers tr ON tr.id = ta.teacher_id
WHERE ta.teacher_id = $1
ORDER BY t.start_date DESC, c.name ASC`)).
		WithArgs("teacher-1").
		WillReturnRows(rows)

	assignments, err := repo.ListByTeacher(context.Background(), "teacher-1")
	require.NoError(t, err)
	assert.Len(t, assignments, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeacherAssignmentRepositoryCreateDelete(t *testing.T) {
	db, mock, cleanup := newTeacherAssignmentMock(t)
	defer cleanup()
	repo := NewTeacherAssignmentRepository(db)

	mock.ExpectExec("INSERT INTO teacher_assignments").
		WithArgs(sqlmock.AnyArg(), "teacher-1", "class-1", "subject-1", "term-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Create(context.Background(), &models.TeacherAssignment{
		TeacherID: "teacher-1",
		ClassID:   "class-1",
		SubjectID: "subject-1",
		TermID:    "term-1",
	})
	require.NoError(t, err)

	mock.ExpectExec("DELETE FROM teacher_assignments").
		WithArgs("assignment-1", "teacher-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, repo.Delete(context.Background(), "teacher-1", "assignment-1"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTeacherAssignmentRepositoryExistsAndCount(t *testing.T) {
	db, mock, cleanup := newTeacherAssignmentMock(t)
	defer cleanup()
	repo := NewTeacherAssignmentRepository(db)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM teacher_assignments WHERE teacher_id = $1 AND class_id = $2 AND subject_id = $3 AND term_id = $4 LIMIT 1")).
		WithArgs("teacher-1", "class-1", "subject-1", "term-1").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	exists, err := repo.Exists(context.Background(), "teacher-1", "class-1", "subject-1", "term-1")
	require.NoError(t, err)
	assert.True(t, exists)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM teacher_assignments WHERE teacher_id = $1 AND term_id = $2")).
		WithArgs("teacher-1", "term-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	count, err := repo.CountByTeacherAndTerm(context.Background(), "teacher-1", "term-1")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}
