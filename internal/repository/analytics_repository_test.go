package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

func newAnalyticsRepositoryMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { _ = db.Close() }
}

func TestAnalyticsRepositoryClassAnalyticsUsesPreAggregatedViews(t *testing.T) {
	db, mock, cleanup := newAnalyticsRepositoryMock(t)
	defer cleanup()
	repo := NewAnalyticsRepository(db)

	classRows := sqlmock.NewRows([]string{
		"class_id", "class_name", "grade", "track", "term_id", "term_name",
		"total_students", "total_subjects", "avg_attendance_rate", "avg_grade", "students_passed", "students_failed",
	}).AddRow("class-1", "X IPA 1", "X", "IPA", "term-1", "Term 1", 2, 3, 95.5, 82.25, 2, 0)
	mock.ExpectQuery(regexp.QuoteMeta("FROM mv_class_statistics")).WithArgs("class-1", "term-1").WillReturnRows(classRows)

	studentRows := sqlmock.NewRows([]string{"student_id", "student_name", "nis", "gpa", "attendance_percentage", "rank"}).
		AddRow("student-1", "Student One", "1001", 90.0, 100.0, 1)
	mock.ExpectQuery(regexp.QuoteMeta("FROM mv_student_performance")).WithArgs("class-1", "term-1").WillReturnRows(studentRows)

	subjectRows := sqlmock.NewRows([]string{"subject_id", "subject_name", "total_students", "avg_grade", "pass_rate"}).
		AddRow("subject-1", "Matematika", 2, 82.25, 100.0)
	mock.ExpectQuery(regexp.QuoteMeta("FROM mv_subject_statistics")).WithArgs("class-1", "term-1").WillReturnRows(subjectRows)

	result, err := repo.ClassAnalytics(context.Background(), "class-1", "term-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Students, 1)
	require.Len(t, result.SubjectPerformance, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAnalyticsRepositoryLeaderboardRejectsUnknownMetric(t *testing.T) {
	db, mock, cleanup := newAnalyticsRepositoryMock(t)
	defer cleanup()
	repo := NewAnalyticsRepository(db)

	_, err := repo.Leaderboard(context.Background(), "unknown", models.AnalyticsLeaderboardFilter{TermID: "term-1", Limit: 10})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
