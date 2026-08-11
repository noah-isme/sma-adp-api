package repository

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordResetMigrationStoresOnlyHashedSingleUseTokens(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrationsDir := filepath.Join(filepath.Dir(sourceFile), "..", "..", "migrations")
	up, err := os.ReadFile(filepath.Join(migrationsDir, "000025_password_reset_tokens.up.sql"))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(migrationsDir, "000025_password_reset_tokens.down.sql"))
	require.NoError(t, err)

	sql := strings.ToLower(string(up))
	assertions := []string{
		"create table if not exists password_reset_tokens",
		"token_hash char(64) unique not null",
		"expires_at timestamp not null",
		"used_at timestamp",
	}
	for _, assertion := range assertions {
		require.Contains(t, sql, assertion)
	}
	require.NotContains(t, sql, "token varchar")
	require.Contains(t, strings.ToLower(string(down)), "drop table if exists password_reset_tokens")
}
