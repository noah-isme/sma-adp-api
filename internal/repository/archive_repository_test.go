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

func newArchiveRepoMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestArchiveRepositoryCreateAndGet(t *testing.T) {
	db, mock, cleanup := newArchiveRepoMock(t)
	defer cleanup()

	repo := NewArchiveRepository(db)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO archives")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	item := &models.ArchiveItem{
		Title:      "Policy",
		Category:   "ADMIN",
		Scope:      models.ArchiveScopeGlobal,
		FilePath:   "/archives/file.pdf",
		MimeType:   "application/pdf",
		SizeBytes:  1024,
		UploadedBy: "admin-1",
		UploadedAt: time.Now(),
	}
	require.NoError(t, repo.Create(context.Background(), item))

	rows := sqlmock.NewRows([]string{"id", "title", "category", "scope", "ref_term_id", "ref_class_id", "ref_student_id", "file_path", "mime_type", "size_bytes", "uploaded_by", "uploaded_at", "deleted_at"}).
		AddRow(item.ID, item.Title, item.Category, item.Scope, nil, nil, nil, item.FilePath, item.MimeType, item.SizeBytes, item.UploadedBy, time.Now(), nil)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, category, scope")).
		WithArgs(item.ID).
		WillReturnRows(rows)

	found, err := repo.GetByID(context.Background(), item.ID)
	require.NoError(t, err)
	require.Equal(t, item.ID, found.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestArchiveRepositoryListFilters(t *testing.T) {
	db, mock, cleanup := newArchiveRepoMock(t)
	defer cleanup()

	repo := NewArchiveRepository(db)
	rows := sqlmock.NewRows([]string{"id", "title", "category", "scope", "ref_term_id", "ref_class_id", "ref_student_id", "file_path", "mime_type", "size_bytes", "uploaded_by", "uploaded_at", "deleted_at"}).
		AddRow("arch-1", "Report", "ACADEMIC", "TERM", "term-1", nil, nil, "/a.pdf", "application/pdf", 2048, "admin-1", time.Now(), nil)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, category, scope")).
		WithArgs("TERM", "ACADEMIC", "term-1").
		WillReturnRows(rows)

	items, err := repo.List(context.Background(), models.ArchiveFilter{
		Scope:    models.ArchiveScopeTerm,
		Category: "ACADEMIC",
		TermID:   "term-1",
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "arch-1", items[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestArchiveRepositorySoftDelete(t *testing.T) {
	db, mock, cleanup := newArchiveRepoMock(t)
	defer cleanup()

	repo := NewArchiveRepository(db)
	now := time.Now()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE archives SET deleted_at = $2")).
		WithArgs("arch-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.SoftDelete(context.Background(), "arch-1", now))

	mock.ExpectExec(regexp.QuoteMeta("UPDATE archives SET deleted_at = $2")).
		WithArgs("arch-2", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, repo.SoftDelete(context.Background(), "arch-2", now))
}
