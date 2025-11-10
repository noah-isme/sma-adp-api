package repository

import (
	"context"
	"database/sql"
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

func newSemesterScheduleRepoMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestSemesterScheduleRepositoryCreateVersioned(t *testing.T) {
	db, mock, cleanup := newSemesterScheduleRepoMock(t)
	defer cleanup()
	repo := NewSemesterScheduleRepository(db)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) + 1 FROM semester_schedules WHERE term_id = $1 AND class_id = $2")).
		WithArgs("term-1", "class-1").
		WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(2))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO semester_schedules")).
		WithArgs(sqlmock.AnyArg(), "term-1", "class-1", 2, string(models.SemesterScheduleStatusDraft), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	payload := &models.SemesterSchedule{
		TermID:  "term-1",
		ClassID: "class-1",
		Meta:    types.JSONText(`{"score":95}`),
	}
	err := repo.CreateVersioned(context.Background(), nil, payload)
	require.NoError(t, err)
	assert.Equal(t, 2, payload.Version)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSemesterScheduleRepositoryListByTermClass(t *testing.T) {
	db, mock, cleanup := newSemesterScheduleRepoMock(t)
	defer cleanup()
	repo := NewSemesterScheduleRepository(db)

	rows := sqlmock.NewRows([]string{"id", "term_id", "class_id", "version", "status", "meta", "created_at", "updated_at"}).
		AddRow("sch-1", "term-1", "class-1", 1, string(models.SemesterScheduleStatusDraft), types.JSONText(`{}`), time.Now(), time.Now())
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, term_id, class_id, version, status, meta, created_at, updated_at FROM semester_schedules WHERE term_id = $1 AND class_id = $2 ORDER BY version DESC")).
		WithArgs("term-1", "class-1").
		WillReturnRows(rows)

	list, err := repo.ListByTermClass(context.Background(), "term-1", "class-1")
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSemesterScheduleRepositoryDelete(t *testing.T) {
	db, mock, cleanup := newSemesterScheduleRepoMock(t)
	defer cleanup()
	repo := NewSemesterScheduleRepository(db)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM semester_schedules WHERE id = $1")).
		WithArgs("sch-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, repo.Delete(context.Background(), "sch-1"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSemesterScheduleRepositoryDeleteNotFound(t *testing.T) {
	db, mock, cleanup := newSemesterScheduleRepoMock(t)
	defer cleanup()
	repo := NewSemesterScheduleRepository(db)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM semester_schedules WHERE id = $1")).
		WithArgs("sch-1").
		WillReturnResult(sqlmock.NewResult(1, 0))

	err := repo.Delete(context.Background(), "sch-1")
	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSemesterScheduleRepositoryUpdateStatus(t *testing.T) {
	db, mock, cleanup := newSemesterScheduleRepoMock(t)
	defer cleanup()
	repo := NewSemesterScheduleRepository(db)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE semester_schedules SET status = $1, meta = $2, updated_at = $3 WHERE id = $4")).
		WithArgs(string(models.SemesterScheduleStatusPublished), types.JSONText(`{"score":98}`), sqlmock.AnyArg(), "sch-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.UpdateStatus(context.Background(), nil, "sch-1", models.SemesterScheduleStatusPublished, types.JSONText(`{"score":98}`))
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSemesterScheduleRepositoryUpdateStatusNoMeta(t *testing.T) {
	db, mock, cleanup := newSemesterScheduleRepoMock(t)
	defer cleanup()
	repo := NewSemesterScheduleRepository(db)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE semester_schedules SET status = $1, updated_at = $2 WHERE id = $3")).
		WithArgs(string(models.SemesterScheduleStatusDraft), sqlmock.AnyArg(), "sch-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.UpdateStatus(context.Background(), nil, "sch-1", models.SemesterScheduleStatusDraft, nil)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
