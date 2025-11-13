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

func newMutationRepoMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestMutationRepositoryCreateAndGet(t *testing.T) {
	db, mock, cleanup := newMutationRepoMock(t)
	defer cleanup()

	repo := NewMutationRepository(db)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mutations")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mutation := &models.Mutation{
		Type:             models.MutationTypeStudentData,
		Entity:           "student",
		EntityID:         "student-1",
		Reason:           "fix typo",
		RequestedBy:      "user-1",
		RequestedChanges: []byte(`{"fullName":"John"}`),
		CurrentSnapshot:  []byte(`{"fullName":"Jon"}`),
	}
	require.NoError(t, repo.Create(context.Background(), mutation))

	rows := sqlmock.NewRows([]string{"id", "type", "entity", "entity_id", "current_snapshot", "requested_changes", "status", "reason", "requested_by", "reviewed_by", "requested_at", "reviewed_at", "note"}).
		AddRow(mutation.ID, "STUDENT_DATA", "student", "student-1", `{"fullName":"Jon"}`, `{"fullName":"John"}`, "PENDING", "fix typo", "user-1", nil, time.Now(), nil, nil)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, type, entity, entity_id")).
		WithArgs(mutation.ID).
		WillReturnRows(rows)

	found, err := repo.GetByID(context.Background(), mutation.ID)
	require.NoError(t, err)
	require.Equal(t, mutation.ID, found.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMutationRepositoryListFilters(t *testing.T) {
	db, mock, cleanup := newMutationRepoMock(t)
	defer cleanup()

	repo := NewMutationRepository(db)
	rows := sqlmock.NewRows([]string{"id", "type", "entity", "entity_id", "current_snapshot", "requested_changes", "status", "reason", "requested_by", "reviewed_by", "requested_at", "reviewed_at", "note"}).
		AddRow("mut-1", "CLASS_CHANGE", "class", "class-1", `{}`, `{}`, "PENDING", "need change", "admin-1", nil, time.Now(), nil, nil)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, type, entity, entity_id")).
		WithArgs("PENDING", "class").
		WillReturnRows(rows)

	list, err := repo.List(context.Background(), models.MutationFilter{
		Status: []models.MutationStatus{models.MutationStatusPending},
		Entity: "class",
	})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "mut-1", list[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMutationRepositoryUpdateStatus(t *testing.T) {
	db, mock, cleanup := newMutationRepoMock(t)
	defer cleanup()

	repo := NewMutationRepository(db)
	now := time.Now()
	note := "looks good"
	mock.ExpectExec(regexp.QuoteMeta("UPDATE mutations SET")).WillReturnResult(sqlmock.NewResult(0, 1))
	err := repo.UpdateStatusAndSnapshot(context.Background(), UpdateMutationParams{
		ID:         "mut-1",
		Status:     models.MutationStatusApproved,
		ReviewedBy: "admin-1",
		ReviewedAt: now,
		Note:       &note,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	mock.ExpectExec(regexp.QuoteMeta("UPDATE mutations SET")).WillReturnResult(sqlmock.NewResult(0, 0))
	err = repo.UpdateStatusAndSnapshot(context.Background(), UpdateMutationParams{
		ID:         "mut-1",
		Status:     models.MutationStatusApproved,
		ReviewedBy: "admin-1",
		ReviewedAt: now,
	})
	require.Error(t, err)
}
