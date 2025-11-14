package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

func newConfigurationRepoMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	sqlxDB := sqlx.NewDb(db, "postgres")
	return sqlxDB, mock, func() {
		sqlxDB.Close()
		db.Close()
	}
}

func TestConfigurationRepositoryListByKeys(t *testing.T) {
	db, mock, cleanup := newConfigurationRepoMock(t)
	defer cleanup()

	repo := NewConfigurationRepository(db)
	rows := sqlmock.NewRows([]string{"key", "value", "type", "description", "updated_by", "updated_at"}).
		AddRow("active_term_id", "term-1", "STRING", "desc", "admin", time.Now())
	mock.ExpectQuery("SELECT key, value").
		WithArgs("active_term_id").
		WillReturnRows(rows)

	result, err := repo.ListByKeys(context.Background(), []string{"active_term_id"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "term-1", result[0].Value)
}

func TestConfigurationRepositoryUpsert(t *testing.T) {
	db, mock, cleanup := newConfigurationRepoMock(t)
	defer cleanup()
	repo := NewConfigurationRepository(db)
	mock.ExpectExec("INSERT INTO configurations").
		WithArgs("active_term_id", "term-1", "STRING", sqlmock.AnyArg(), "admin", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	cfg := &models.Configuration{
		Key:       "active_term_id",
		Value:     "term-1",
		Type:      models.ConfigurationTypeString,
		UpdatedBy: strPtr("admin"),
	}
	require.NoError(t, repo.Upsert(context.Background(), cfg))
}

func TestConfigurationRepositoryBulkUpsert(t *testing.T) {
	db, mock, cleanup := newConfigurationRepoMock(t)
	defer cleanup()
	repo := NewConfigurationRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO configurations").
		WithArgs("active_term_id", "term-1", "STRING", sqlmock.AnyArg(), "admin", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO configurations").
		WithArgs("enable_reports_ui", "true", "BOOLEAN", sqlmock.AnyArg(), "admin", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	items := []models.Configuration{
		{Key: "active_term_id", Value: "term-1", Type: models.ConfigurationTypeString, UpdatedBy: strPtr("admin")},
		{Key: "enable_reports_ui", Value: "true", Type: models.ConfigurationTypeBoolean, UpdatedBy: strPtr("admin")},
	}
	require.NoError(t, repo.BulkUpsert(context.Background(), items))
}

func strPtr(value string) *string {
	return &value
}
