package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

func TestCreatePasswordResetTokenStoresTokenHash(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()
	repo := NewUserRepository(db)

	token := &models.PasswordResetToken{
		ID:        "reset-1",
		UserID:    "user-1",
		TokenHash: "hash-only",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	mock.ExpectExec("INSERT INTO password_reset_tokens").WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, repo.CreatePasswordResetToken(context.Background(), token))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConsumePasswordResetTokenAtomicallyMarksTokenUsed(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()
	repo := NewUserRepository(db)
	now := time.Now().UTC()
	expiresAt := now.Add(time.Hour)
	query := `UPDATE password_reset_tokens
SET used_at = $2
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
RETURNING id, user_id, token_hash, expires_at, created_at, used_at`
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("hash-only", now).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token_hash", "expires_at", "created_at", "used_at"}).
			AddRow("reset-1", "user-1", "hash-only", expiresAt, now.Add(-time.Minute), now))

	token, err := repo.ConsumePasswordResetToken(context.Background(), "hash-only", now)
	require.NoError(t, err)
	assert.Equal(t, "user-1", token.UserID)
	assert.Equal(t, "hash-only", token.TokenHash)
	assert.Equal(t, now, *token.UsedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConsumePasswordResetTokenReturnsNotFoundForUsedOrExpiredRows(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()
	repo := NewUserRepository(db)
	now := time.Now().UTC()
	query := `UPDATE password_reset_tokens
SET used_at = $2
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
RETURNING id, user_id, token_hash, expires_at, created_at, used_at`
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("hash-only", now).
		WillReturnError(sql.ErrNoRows)

	token, err := repo.ConsumePasswordResetToken(context.Background(), "hash-only", now)
	assert.Nil(t, token)
	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.NoError(t, mock.ExpectationsWereMet())
}
