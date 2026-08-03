package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// auditRowColumns mirrors the projection in auditSelectColumns.
var auditRowColumns = []string{
	"id", "user_id", "action", "resource", "resource_id",
	"old_values", "new_values", "ip_address", "user_agent", "created_at",
	"user_email", "user_full_name", "user_role",
}

func TestAuditRepositoryListAppliesEveryFilter(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()

	repo := NewAuditRepository(db)

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
	// Positional order must match buildAuditWhere.
	filterArgs := []driver.Value{"user-1", "LOGIN", "users", "user-9", from, to, "%admin%"}

	mock.ExpectQuery(`SELECT .* FROM audit_logs al LEFT JOIN users u .* WHERE 1=1 AND al\.user_id = \$1 AND al\.action = \$2 AND al\.resource = \$3 AND al\.resource_id = \$4 AND al\.created_at >= \$5 AND al\.created_at <= \$6 AND \(LOWER\(al\.action\) LIKE \$7 .*\) ORDER BY al\.created_at DESC LIMIT \$8 OFFSET \$9`).
		WithArgs(append(append([]driver.Value{}, filterArgs...), 25, 25)...).
		WillReturnRows(sqlmock.NewRows(auditRowColumns).
			AddRow("log-1", "user-1", "LOGIN", "users", "user-9", []byte("{}"), []byte("{}"),
				"1.2.3.4", "agent", time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC),
				"test@example.com", "Test User", "ADMIN_TU"))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_logs al LEFT JOIN users u .* WHERE 1=1 AND al\.user_id = \$1`).
		WithArgs(filterArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	entries, total, err := repo.List(context.Background(), models.AuditLogFilter{
		UserID:     "user-1",
		Action:     "login", // lower-cased input must be normalised to LOGIN
		Resource:   "users",
		ResourceID: "user-9",
		Search:     "Admin", // mixed case must be lower-cased for the LIKE
		DateFrom:   &from,
		DateTo:     &to,
		Page:       2,
		PageSize:   25,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, entries, 1)
	assert.Equal(t, "log-1", entries[0].ID)
	require.NotNil(t, entries[0].UserEmail)
	assert.Equal(t, "test@example.com", *entries[0].UserEmail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditRepositoryListDefaultsPaginationAndSort(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()

	repo := NewAuditRepository(db)

	mock.ExpectQuery(`ORDER BY al\.created_at DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(50, 0).
		WillReturnRows(sqlmock.NewRows(auditRowColumns))
	mock.ExpectQuery(`SELECT COUNT\(\*\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	entries, total, err := repo.List(context.Background(), models.AuditLogFilter{})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Len(t, entries, 0)
	require.NoError(t, mock.ExpectationsWereMet())
}

// An unrecognised sort field must fall back to created_at rather than being
// interpolated into the query.
func TestAuditRepositoryListRejectsUnknownSortColumn(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()

	repo := NewAuditRepository(db)

	mock.ExpectQuery(`ORDER BY al\.created_at ASC`).
		WillReturnRows(sqlmock.NewRows(auditRowColumns))
	mock.ExpectQuery(`SELECT COUNT\(\*\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	_, _, err := repo.List(context.Background(), models.AuditLogFilter{
		SortBy:    "1; DROP TABLE audit_logs",
		SortOrder: "asc",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditRepositoryListHonoursWhitelistedSort(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()

	repo := NewAuditRepository(db)

	mock.ExpectQuery(`ORDER BY al\.action ASC`).
		WillReturnRows(sqlmock.NewRows(auditRowColumns))
	mock.ExpectQuery(`SELECT COUNT\(\*\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	_, _, err := repo.List(context.Background(), models.AuditLogFilter{SortBy: "action", SortOrder: "asc"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditRepositoryFindByIDReturnsNoRows(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()

	repo := NewAuditRepository(db)

	mock.ExpectQuery(`WHERE al\.id = \$1`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.FindByID(context.Background(), "missing")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestAuditRepositoryFindByIDSuccess(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()

	repo := NewAuditRepository(db)

	mock.ExpectQuery(`WHERE al\.id = \$1`).
		WithArgs("log-1").
		WillReturnRows(sqlmock.NewRows(auditRowColumns).
			AddRow("log-1", nil, "CSV_IMPORT", "students", nil, nil, []byte("{}"),
				"10.0.0.1", "agent", time.Now().UTC(), nil, nil, nil))

	entry, err := repo.FindByID(context.Background(), "log-1")
	require.NoError(t, err)
	assert.Equal(t, "CSV_IMPORT", entry.Action)
	assert.Nil(t, entry.UserID)
	assert.Nil(t, entry.UserEmail)
}

func TestAuditRepositoryFacets(t *testing.T) {
	db, mock, cleanup := newMock(t)
	defer cleanup()

	repo := NewAuditRepository(db)

	mock.ExpectQuery(`SELECT action AS value, COUNT\(\*\) AS count FROM audit_logs GROUP BY action`).
		WillReturnRows(sqlmock.NewRows([]string{"value", "count"}).
			AddRow("LOGIN", 50).AddRow("LOGOUT", 48))
	mock.ExpectQuery(`SELECT resource AS value, COUNT\(\*\) AS count FROM audit_logs GROUP BY resource`).
		WillReturnRows(sqlmock.NewRows([]string{"value", "count"}).
			AddRow("users", 30).AddRow("grades", 25))

	facets, err := repo.Facets(context.Background())
	require.NoError(t, err)
	require.Len(t, facets.Actions, 2)
	assert.Equal(t, "LOGIN", facets.Actions[0].Value)
	assert.Equal(t, 50, facets.Actions[0].Count)
	require.Len(t, facets.Resources, 2)
	assert.Equal(t, "users", facets.Resources[0].Value)
	require.NoError(t, mock.ExpectationsWereMet())
}
