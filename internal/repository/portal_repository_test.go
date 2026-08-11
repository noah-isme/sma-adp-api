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

func newParentStudentRepositoryMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestParentStudentRepositoryFindByParentAndStudentScopesBothIDs(t *testing.T) {
	db, mock, cleanup := newParentStudentRepositoryMock(t)
	defer cleanup()
	repo := NewParentStudentRepository(db)

	now := time.Now().UTC()
	query := `SELECT id, parent_id, student_id, relationship, can_view_grades, can_view_attendance, can_view_behavior, can_view_announcements, can_receive_notifications, created_at, updated_at FROM parent_students WHERE parent_id = $1 AND student_id = $2 LIMIT 1`
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("parent-1", "student-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "parent_id", "student_id", "relationship", "can_view_grades", "can_view_attendance",
			"can_view_behavior", "can_view_announcements", "can_receive_notifications", "created_at", "updated_at",
		}).AddRow("link-1", "parent-1", "student-1", models.RelationshipParent, true, true, true, true, true, now, now))

	link, err := repo.FindByParentAndStudent(context.Background(), "parent-1", "student-1")
	require.NoError(t, err)
	assert.Equal(t, "parent-1", link.ParentID)
	assert.Equal(t, "student-1", link.StudentID)
	assert.NoError(t, mock.ExpectationsWereMet())
}
