package repository

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// TestAuditLogEdgeCases_NullUserID verifies CreateAuditLog and AuditRepository methods
// correctly handle entries with nil UserID.
func TestAuditLogEdgeCases_NullUserID(t *testing.T) {
	t.Run("CreateAuditLog with null UserID", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()
		repo := NewUserRepository(db)
		now := time.Now().UTC()

		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs (id, user_id, action, resource, resource_id, old_values, new_values, ip_address, user_agent, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)")).
			WithArgs("audit-null-user", nil, "SYSTEM_EVENT", "system", nil, nil, "{\"status\":\"ok\"}", "127.0.0.1", "test", now).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.CreateAuditLog(context.Background(), &models.AuditLog{
			ID:        "audit-null-user",
			UserID:    nil,
			Action:    "SYSTEM_EVENT",
			Resource:  "system",
			NewValues: []byte("{\"status\":\"ok\"}"),
			IPAddress: "127.0.0.1",
			UserAgent: "test",
			CreatedAt: now,
		})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("AuditRepository List with null UserID row", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()
		repo := NewAuditRepository(db)

		now := time.Now().UTC()
		mock.ExpectQuery(`SELECT .* FROM audit_logs al LEFT JOIN users u .* WHERE 1=1 ORDER BY al\.created_at DESC LIMIT \$1 OFFSET \$2`).
			WithArgs(50, 0).
			WillReturnRows(sqlmock.NewRows(auditRowColumns).
				AddRow("log-null-user", nil, "SYSTEM_EVENT", "system", nil, nil, []byte("{\"status\":\"ok\"}"), "127.0.0.1", "test", now, nil, nil, nil))

		mock.ExpectQuery(`SELECT COUNT\(\*\)`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		entries, total, err := repo.List(context.Background(), models.AuditLogFilter{})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, entries, 1)
		assert.Equal(t, "log-null-user", entries[0].ID)
		assert.Nil(t, entries[0].UserID)
		assert.Nil(t, entries[0].UserEmail)
		assert.Nil(t, entries[0].UserFullName)
		assert.Nil(t, entries[0].UserRole)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("AuditRepository FindByID with null UserID", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()
		repo := NewAuditRepository(db)

		now := time.Now().UTC()
		mock.ExpectQuery(`WHERE al\.id = \$1`).
			WithArgs("log-null-user").
			WillReturnRows(sqlmock.NewRows(auditRowColumns).
				AddRow("log-null-user", nil, "SYSTEM_EVENT", "system", nil, nil, nil, "127.0.0.1", "test", now, nil, nil, nil))

		entry, err := repo.FindByID(context.Background(), "log-null-user")
		require.NoError(t, err)
		assert.Equal(t, "log-null-user", entry.ID)
		assert.Nil(t, entry.UserID)
		assert.Nil(t, entry.UserEmail)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuditLogEdgeCases_EmptyDetailsJSON tests empty, nil, empty string, and valid JSON payloads for details_json.
func TestAuditLogEdgeCases_EmptyDetailsJSON(t *testing.T) {
	t.Run("CreateAuditLog with nil OldValues and NewValues", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()
		repo := NewUserRepository(db)
		now := time.Now().UTC()

		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs (id, user_id, action, resource, resource_id, old_values, new_values, ip_address, user_agent, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)")).
			WithArgs("log-empty-json-1", "user-1", "DELETE", "students", "std-1", nil, nil, "127.0.0.1", "test", now).
			WillReturnResult(sqlmock.NewResult(1, 1))

		uID := "user-1"
		rID := "std-1"
		err := repo.CreateAuditLog(context.Background(), &models.AuditLog{
			ID:         "log-empty-json-1",
			UserID:     &uID,
			Action:     "DELETE",
			Resource:   "students",
			ResourceID: &rID,
			OldValues:  nil,
			NewValues:  nil,
			IPAddress:  "127.0.0.1",
			UserAgent:  "test",
			CreatedAt:  now,
		})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CreateAuditLog with zero-length slice OldValues and NewValues", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()
		repo := NewUserRepository(db)
		now := time.Now().UTC()

		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs (id, user_id, action, resource, resource_id, old_values, new_values, ip_address, user_agent, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)")).
			WithArgs("log-empty-json-2", "user-1", "DELETE", "students", "std-1", nil, nil, "127.0.0.1", "test", now).
			WillReturnResult(sqlmock.NewResult(1, 1))

		uID := "user-1"
		rID := "std-1"
		err := repo.CreateAuditLog(context.Background(), &models.AuditLog{
			ID:         "log-empty-json-2",
			UserID:     &uID,
			Action:     "DELETE",
			Resource:   "students",
			ResourceID: &rID,
			OldValues:  []byte(""),
			NewValues:  []byte(""),
			IPAddress:  "127.0.0.1",
			UserAgent:  "test",
			CreatedAt:  now,
		})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CreateAuditLog with valid JSON object and array", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()
		repo := NewUserRepository(db)
		now := time.Now().UTC()

		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs (id, user_id, action, resource, resource_id, old_values, new_values, ip_address, user_agent, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)")).
			WithArgs("log-json-3", "user-1", "UPDATE", "students", "std-1", "{\"name\":\"Old\"}", "{\"name\":\"New\"}", "127.0.0.1", "test", now).
			WillReturnResult(sqlmock.NewResult(1, 1))

		uID := "user-1"
		rID := "std-1"
		err := repo.CreateAuditLog(context.Background(), &models.AuditLog{
			ID:         "log-json-3",
			UserID:     &uID,
			Action:     "UPDATE",
			Resource:   "students",
			ResourceID: &rID,
			OldValues:  []byte("{\"name\":\"Old\"}"),
			NewValues:  []byte("{\"name\":\"New\"}"),
			IPAddress:  "127.0.0.1",
			UserAgent:  "test",
			CreatedAt:  now,
		})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuditLogEdgeCases_ResourceNameLength verifies handling of maximum boundary (100 chars)
// and over-length strings for resource and action.
func TestAuditLogEdgeCases_ResourceNameLength(t *testing.T) {
	t.Run("Exactly 100 character resource name", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()
		repo := NewUserRepository(db)
		now := time.Now().UTC()

		res100 := strings.Repeat("r", 100)
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs (id, user_id, action, resource, resource_id, old_values, new_values, ip_address, user_agent, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)")).
			WithArgs("log-res-100", nil, "TEST", res100, nil, nil, nil, "127.0.0.1", "test", now).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.CreateAuditLog(context.Background(), &models.AuditLog{
			ID:        "log-res-100",
			Resource:  res100,
			Action:    "TEST",
			IPAddress: "127.0.0.1",
			UserAgent: "test",
			CreatedAt: now,
		})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Over 100 character resource name passes through to DB", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()
		repo := NewUserRepository(db)
		now := time.Now().UTC()

		res255 := strings.Repeat("r", 255)
		// Note: CreateAuditLog does not truncate before sending to DB. If DB schema has VARCHAR(100), DB returns error.
		// Testing how driver returns DB error.
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs (id, user_id, action, resource, resource_id, old_values, new_values, ip_address, user_agent, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)")).
			WithArgs("log-res-255", nil, "TEST", res255, nil, nil, nil, "127.0.0.1", "test", now).
			WillReturnError(sql.ErrConnDone) // Simulating DB rejection for oversized varchar

		err := repo.CreateAuditLog(context.Background(), &models.AuditLog{
			ID:        "log-res-255",
			Resource:  res255,
			Action:    "TEST",
			IPAddress: "127.0.0.1",
			UserAgent: "test",
			CreatedAt: now,
		})
		require.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuditLogEdgeCases_FiltersAndSearch checks edge cases in List filter parameters.
func TestAuditLogEdgeCases_FiltersAndSearch(t *testing.T) {
	t.Run("Negative page and oversize pageSize normalization", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()
		repo := NewAuditRepository(db)

		// Page < 1 -> 1 (offset 0), PageSize > 200 -> 50 limit
		mock.ExpectQuery(`ORDER BY al\.created_at DESC LIMIT \$1 OFFSET \$2`).
			WithArgs(50, 0).
			WillReturnRows(sqlmock.NewRows(auditRowColumns))
		mock.ExpectQuery(`SELECT COUNT\(\*\)`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		_, _, err := repo.List(context.Background(), models.AuditLogFilter{
			Page:     -5,
			PageSize: 999,
		})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SQL injection in search parameter is sanitized via parameterized query", func(t *testing.T) {
		db, mock, cleanup := newMock(t)
		defer cleanup()
		repo := NewAuditRepository(db)

		injectionStr := "'; DROP TABLE audit_logs; --"
		mock.ExpectQuery(`WHERE 1=1 AND \(LOWER\(al\.action\) LIKE \$1 OR LOWER\(al\.resource\) LIKE \$1 OR LOWER\(COALESCE\(u\.email, ''\)\) LIKE \$1 OR LOWER\(COALESCE\(u\.full_name, ''\)\) LIKE \$1\)`).
			WithArgs("%"+strings.ToLower(injectionStr)+"%", 50, 0).
			WillReturnRows(sqlmock.NewRows(auditRowColumns))
		mock.ExpectQuery(`SELECT COUNT\(\*\)`).
			WithArgs("%" + strings.ToLower(injectionStr) + "%").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		_, _, err := repo.List(context.Background(), models.AuditLogFilter{
			Search: injectionStr,
		})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
