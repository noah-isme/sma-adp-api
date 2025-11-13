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

func newReportRepoMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestReportRepositoryCreateAndGet(t *testing.T) {
	db, mock, cleanup := newReportRepoMock(t)
	defer cleanup()

	repo := NewReportRepository(db)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO report_jobs")).
		WithArgs(sqlmock.AnyArg(), "grades", sqlmock.AnyArg(), "QUEUED", 0, nil, "user-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	job := &models.ReportJob{
		Type:      models.ReportTypeGrades,
		Params:    models.ReportJobParams{TermID: "term-1", Format: models.ReportFormatCSV},
		CreatedBy: "user-1",
	}
	require.NoError(t, repo.Create(context.Background(), job))

	rows := sqlmock.NewRows([]string{"id", "type", "params", "status", "progress", "result_url", "created_by", "created_at", "finished_at", "error_message"}).
		AddRow(job.ID, "grades", `{"termId":"term-1","format":"csv","extras":{}}`, "QUEUED", 0, nil, "user-1", time.Now(), nil, nil)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, type, params, status, progress, result_url, created_by, created_at, finished_at, error_message FROM report_jobs WHERE id = $1")).
		WithArgs(job.ID).
		WillReturnRows(rows)

	fetched, err := repo.GetByID(context.Background(), job.ID)
	require.NoError(t, err)
	require.Equal(t, job.ID, fetched.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReportRepositoryUpdate(t *testing.T) {
	db, mock, cleanup := newReportRepoMock(t)
	defer cleanup()
	repo := NewReportRepository(db)

	now := time.Now()
	status := models.ReportStatusFinished
	progress := 100
	result := "/api/v1/export/token"
	mock.ExpectExec(regexp.QuoteMeta("UPDATE report_jobs SET status = $1, progress = $2, result_url = $3, finished_at = $4 WHERE id = $5")).
		WithArgs(status, progress, result, now, "job-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Update(context.Background(), "job-1", UpdateReportJobParams{
		Status:     &status,
		Progress:   &progress,
		ResultURL:  &result,
		FinishedAt: &now,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReportRepositoryListQueued(t *testing.T) {
	db, mock, cleanup := newReportRepoMock(t)
	defer cleanup()
	repo := NewReportRepository(db)

	rows := sqlmock.NewRows([]string{"id", "type", "params", "status", "progress", "result_url", "created_by", "created_at", "finished_at", "error_message"}).
		AddRow("job-1", "attendance", `{"termId":"term-1","format":"csv","extras":{}}`, "QUEUED", 0, nil, "user-1", time.Now(), nil, nil)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, type, params, status, progress, result_url, created_by, created_at, finished_at, error_message FROM report_jobs WHERE status = 'QUEUED' ORDER BY created_at ASC LIMIT $1")).
		WithArgs(20).
		WillReturnRows(rows)

	jobs, err := repo.ListQueued(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReportRepositoryListFinishedBefore(t *testing.T) {
	db, mock, cleanup := newReportRepoMock(t)
	defer cleanup()
	repo := NewReportRepository(db)

	rows := sqlmock.NewRows([]string{"id", "type", "params", "status", "progress", "result_url", "created_by", "created_at", "finished_at", "error_message"}).
		AddRow("job-1", "grades", `{"termId":"term-1","format":"csv","extras":{}}`, "FINISHED", 100, "/api/v1/export/token", "user-1", time.Now().Add(-48*time.Hour), time.Now().Add(-25*time.Hour), nil)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, type, params, status, progress, result_url, created_by, created_at, finished_at, error_message FROM report_jobs WHERE status = 'FINISHED' AND finished_at IS NOT NULL AND finished_at < $1 ORDER BY finished_at ASC LIMIT $2")).
		WithArgs(sqlmock.AnyArg(), 50).
		WillReturnRows(rows)

	jobs, err := repo.ListFinishedBefore(context.Background(), time.Now(), 0)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}
