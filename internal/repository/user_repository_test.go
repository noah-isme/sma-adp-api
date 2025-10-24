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

func newMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	sqlxdb := sqlx.NewDb(db, "sqlmock")
	return sqlxdb, mock, func() {
		db.Close()
	}
}

func TestFindByEmail(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()
	repo := NewUserRepository(db)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "email", "password_hash", "full_name", "role", "active", "last_login", "created_at", "updated_at"}).
		AddRow("1", "user@example.com", "hash", "User", string(models.RoleAdmin), true, now, now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, password_hash, full_name, role, active, last_login, created_at, updated_at FROM users WHERE email = $1 LIMIT 1")).
		WithArgs("user@example.com").
		WillReturnRows(rows)

	user, err := repo.FindByEmail(context.Background(), "user@example.com")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", user.Email)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateRefreshToken(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()
	repo := NewUserRepository(db)

	mock.ExpectExec("INSERT INTO refresh_tokens").WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.CreateRefreshToken(context.Background(), &models.RefreshToken{ID: "1", UserID: "u1", Token: "token", ExpiresAt: time.Now(), CreatedAt: time.Now()})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListUsers(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()
	repo := NewUserRepository(db)

	now := time.Now()
	listRows := sqlmock.NewRows([]string{"id", "email", "password_hash", "full_name", "role", "active", "last_login", "created_at", "updated_at"}).
		AddRow("1", "a@example.com", "hash", "A", string(models.RoleAdmin), true, now, now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, password_hash, full_name, role, active, last_login, created_at, updated_at FROM users WHERE 1=1 ORDER BY created_at DESC LIMIT 20 OFFSET 0")).
		WillReturnRows(listRows)

	countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM users WHERE 1=1")).WillReturnRows(countRows)

	users, total, err := repo.List(context.Background(), models.UserFilter{})
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, 1, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}
