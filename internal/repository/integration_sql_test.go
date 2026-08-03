//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// These tests run the new audit and lesson-attendance SQL against a real
// Postgres so column names, joins, and placeholder numbering are verified for
// real. sqlmock only checks the string we built, not that it is valid SQL.
//
//	docker compose -f docker/docker-compose.yml up -d
//	go test -tags=integration ./internal/repository/ -run Integration
func openIntegrationDB(t *testing.T) *sqlx.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/admin_panel_sma?sslmode=disable"
	}

	db, err := sqlx.Open("postgres", dsn)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("integration database unavailable: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// seedAuditFixture inserts a user plus one audit row and returns their ids
// alongside the row's timestamp.
//
// created_at is written explicitly in UTC because that is what the services do
// (models.AuditLog.CreatedAt is set from time.Now().UTC()) and what the handler
// assumes when it parses a bare date. Using SQL NOW() here would compare a
// container-local clock against Go's local clock and drift by the TZ offset.
func seedAuditFixture(t *testing.T, db *sqlx.DB) (userID, logID string, createdAt time.Time) {
	t.Helper()
	ctx := context.Background()

	userID = "itest-user-" + uuid.NewString()
	logID = "itest-log-" + uuid.NewString()
	createdAt = time.Now().UTC().Truncate(time.Second)

	_, err := db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, role, active, created_at, updated_at)
		 VALUES ($1, $2, 'hash', 'Integration Actor', 'ADMIN_TU', TRUE, $3, $3)`,
		userID, fmt.Sprintf("%s@example.test", userID), createdAt)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO audit_logs (id, user_id, action, resource, resource_id, old_values, new_values, ip_address, user_agent, created_at)
		 VALUES ($1, $2, 'USER_UPDATE', 'users', $3, '{}'::jsonb, '{"changed":true}'::jsonb, '10.0.0.9', 'integration-agent', $4)`,
		logID, userID, userID, createdAt)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DELETE FROM audit_logs WHERE id = $1`, logID)
		db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	return userID, logID, createdAt
}

func TestIntegrationAuditRepositoryListAndGet(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewAuditRepository(db)
	userID, logID, createdAt := seedAuditFixture(t, db)

	ctx := context.Background()
	from := createdAt.Add(-time.Hour)
	to := createdAt.Add(time.Hour)

	// Every filter at once, which is what exercises placeholder numbering.
	entries, total, err := repo.List(ctx, models.AuditLogFilter{
		UserID:     userID,
		Action:     "user_update",
		Resource:   "users",
		ResourceID: userID,
		Search:     "integration",
		DateFrom:   &from,
		DateTo:     &to,
		Page:       1,
		PageSize:   10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, logID, entry.ID)
	assert.Equal(t, "USER_UPDATE", entry.Action)
	// The join must resolve the actor's identity.
	require.NotNil(t, entry.UserFullName)
	assert.Equal(t, "Integration Actor", *entry.UserFullName)
	require.NotNil(t, entry.UserRole)
	assert.Equal(t, "ADMIN_TU", *entry.UserRole)

	single, err := repo.FindByID(ctx, logID)
	require.NoError(t, err)
	assert.Equal(t, logID, single.ID)

	_, err = repo.FindByID(ctx, "does-not-exist")
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

// Each whitelisted sort column must be valid SQL against the real schema.
func TestIntegrationAuditRepositorySortColumns(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewAuditRepository(db)
	seedAuditFixture(t, db)

	for _, sortBy := range []string{"created_at", "action", "resource", "user_id", "bogus"} {
		for _, order := range []string{"asc", "desc"} {
			_, _, err := repo.List(context.Background(), models.AuditLogFilter{
				SortBy:    sortBy,
				SortOrder: order,
				PageSize:  1,
			})
			require.NoErrorf(t, err, "sortBy=%s order=%s", sortBy, order)
		}
	}
}

func TestIntegrationAuditRepositoryFacets(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewAuditRepository(db)
	seedAuditFixture(t, db)

	facets, err := repo.Facets(context.Background())
	require.NoError(t, err)
	require.NotNil(t, facets)

	var found bool
	for _, action := range facets.Actions {
		if action.Value == "USER_UPDATE" {
			found = true
			assert.Positive(t, action.Count)
		}
	}
	assert.True(t, found, "expected the seeded action to appear in facets")
}

// lessonFixture is the id set needed to write a subject_attendance row.
type lessonFixture struct {
	TermID       string
	SubjectID    string
	ClassID      string
	StudentID    string
	EnrollmentID string
	ScheduleID   string
}

func seedLessonFixture(t *testing.T, db *sqlx.DB) lessonFixture {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()[:8]

	f := lessonFixture{
		TermID:       "itest-term-" + suffix,
		SubjectID:    "itest-subject-" + suffix,
		ClassID:      "itest-class-" + suffix,
		StudentID:    "itest-student-" + suffix,
		EnrollmentID: "itest-enroll-" + suffix,
		ScheduleID:   "itest-sched-" + suffix,
	}

	mustExec := func(query string, args ...interface{}) {
		_, err := db.ExecContext(ctx, query, args...)
		require.NoError(t, err)
	}

	mustExec(`INSERT INTO terms (id, name, type, academic_year, start_date, end_date, is_active)
		VALUES ($1, 'Integration Term', 'ODD', '2024/2025', NOW() - INTERVAL '30 days', NOW() + INTERVAL '30 days', FALSE)`, f.TermID)
	mustExec(`INSERT INTO subjects (id, code, name) VALUES ($1, $2, 'Integration Subject')`,
		f.SubjectID, "ITEST-"+suffix)
	mustExec(`INSERT INTO classes (id, name, grade) VALUES ($1, 'Integration Class', '10')`, f.ClassID)
	mustExec(`INSERT INTO students (id, nis, full_name, gender, birth_date, active)
		VALUES ($1, $2, 'Integration Student', 'M', NOW() - INTERVAL '17 years', TRUE)`,
		f.StudentID, "ITEST"+suffix)
	mustExec(`INSERT INTO enrollments (id, student_id, class_id, term_id, status)
		VALUES ($1, $2, $3, $4, 'ACTIVE')`, f.EnrollmentID, f.StudentID, f.ClassID, f.TermID)
	mustExec(`INSERT INTO schedules (id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room)
		VALUES ($1, $2, $3, $4, NULL, 'MONDAY', '1', 'R-101')`,
		f.ScheduleID, f.TermID, f.ClassID, f.SubjectID)

	t.Cleanup(func() {
		bg := context.Background()
		db.ExecContext(bg, `DELETE FROM subject_attendance WHERE schedule_id = $1`, f.ScheduleID)
		db.ExecContext(bg, `DELETE FROM schedules WHERE id = $1`, f.ScheduleID)
		db.ExecContext(bg, `DELETE FROM enrollments WHERE id = $1`, f.EnrollmentID)
		db.ExecContext(bg, `DELETE FROM students WHERE id = $1`, f.StudentID)
		db.ExecContext(bg, `DELETE FROM classes WHERE id = $1`, f.ClassID)
		db.ExecContext(bg, `DELETE FROM subjects WHERE id = $1`, f.SubjectID)
		db.ExecContext(bg, `DELETE FROM terms WHERE id = $1`, f.TermID)
	})

	return f
}

func TestIntegrationSubjectAttendanceLessonQueries(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewSubjectAttendanceRepository(db)
	f := seedLessonFixture(t, db)
	ctx := context.Background()

	day := time.Now().UTC().Truncate(24 * time.Hour)
	stored, err := repo.Upsert(ctx, &models.SubjectAttendance{
		EnrollmentID: f.EnrollmentID,
		ScheduleID:   f.ScheduleID,
		Date:         day,
		Status:       models.AttendanceStatusPresent,
	})
	require.NoError(t, err)
	require.NotEmpty(t, stored.ID)

	// The class/subject/term filters must resolve through the join chain, which
	// is the whole point of the new lesson-level read path.
	from := day.Add(-24 * time.Hour)
	to := day.Add(24 * time.Hour)
	rows, total, err := repo.List(ctx, models.SubjectAttendanceFilter{
		ClassID:   f.ClassID,
		SubjectID: f.SubjectID,
		TermID:    f.TermID,
		StudentID: f.StudentID,
		DateFrom:  &from,
		DateTo:    &to,
		Page:      1,
		PageSize:  10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, rows, 1)
	assert.Equal(t, "Integration Student", rows[0].StudentName)
	require.NotNil(t, rows[0].SubjectName)
	assert.Equal(t, "Integration Subject", *rows[0].SubjectName)

	// Filtering by schedule and enrollment must work too.
	_, total, err = repo.List(ctx, models.SubjectAttendanceFilter{
		ScheduleID:   f.ScheduleID,
		EnrollmentID: f.EnrollmentID,
		Date:         &day,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	// Every whitelisted sort column must be valid SQL.
	for _, sortBy := range []string{"date", "created_at", "student_name", "status", "bogus"} {
		_, _, err := repo.List(ctx, models.SubjectAttendanceFilter{SortBy: sortBy, PageSize: 1})
		require.NoErrorf(t, err, "sortBy=%s", sortBy)
	}

	summary, err := repo.Summary(ctx, models.SubjectAttendanceFilter{
		ClassID:   f.ClassID,
		SubjectID: f.SubjectID,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Total)
	assert.Equal(t, 1, summary.Present)
	assert.InDelta(t, 100.0, summary.Percent, 0.001)

	single, err := repo.FindByID(ctx, stored.ID)
	require.NoError(t, err)
	assert.Equal(t, stored.ID, single.ID)
	assert.Equal(t, f.ClassID, single.ClassID)

	require.NoError(t, repo.Delete(ctx, stored.ID))

	// Deleting a row that is already gone must surface as ErrNoRows so the
	// service can answer 404 instead of a misleading 204.
	assert.ErrorIs(t, repo.Delete(ctx, stored.ID), sql.ErrNoRows)
	_, err = repo.FindByID(ctx, stored.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

// An empty result set must summarise as zeroes rather than erroring on NULL sums.
func TestIntegrationSubjectAttendanceSummaryHandlesEmptySet(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewSubjectAttendanceRepository(db)

	summary, err := repo.Summary(context.Background(), models.SubjectAttendanceFilter{
		ScheduleID: "no-such-schedule",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, summary.Total)
	assert.Equal(t, 0, summary.Present)
	assert.Zero(t, summary.Percent)
}

func TestIntegrationTeacherPreferenceListSortColumns(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewTeacherPreferenceRepository(db)

	for _, sortBy := range []string{
		"teacher_id", "max_load_per_day", "max_load_per_week", "created_at", "updated_at", "bogus",
	} {
		_, _, err := repo.ListAll(context.Background(), models.TeacherPreferenceFilter{
			SortBy:    sortBy,
			SortOrder: "desc",
			PageSize:  1,
		})
		require.NoErrorf(t, err, "sortBy=%s", sortBy)
	}
}
