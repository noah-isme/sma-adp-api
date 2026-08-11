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

func newAnnouncementRepositoryMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func announcementColumns() []string {
	return []string{
		"id", "title", "content", "audience", "target_class_id", "priority",
		"is_pinned", "published_at", "expires_at", "created_by", "created_at", "updated_at",
	}
}

func TestAnnouncementRepositoryListByStudentAndTermPageUsesStableOrderAndMetadata(t *testing.T) {
	db, mock, cleanup := newAnnouncementRepositoryMock(t)
	defer cleanup()
	repo := NewAnnouncementRepository(db)

	joinedAt := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT e.class_id FROM enrollments e WHERE e.student_id = $1 AND e.term_id = $2 AND e.status = $3 ORDER BY e.joined_at DESC LIMIT 1")).
		WithArgs("student-1", "term-1", models.EnrollmentStatusActive).
		WillReturnRows(sqlmock.NewRows([]string{"class_id"}).AddRow("class-1"))

	selectQuery := "SELECT id, title, content, audience, target_class_id, priority, is_pinned, published_at, expires_at, created_by, created_at, updated_at FROM announcements WHERE (audience = $1 OR audience = $2 OR (audience = $3 AND target_class_id = $4)) AND published_at <= NOW() AND (expires_at IS NULL OR expires_at > NOW()) ORDER BY is_pinned DESC, priority DESC, published_at DESC, id DESC LIMIT $5 OFFSET $6"
	mock.ExpectQuery(regexp.QuoteMeta(selectQuery)).
		WithArgs(models.AnnouncementAudienceSiswa, models.AnnouncementAudienceAll, models.AnnouncementAudienceClass, "class-1", 3, 3).
		WillReturnRows(sqlmock.NewRows(announcementColumns()).AddRow(
			"announcement-1", "Title", "Content", models.AnnouncementAudienceSiswa, nil,
			models.AnnouncementPriorityNormal, false, joinedAt, nil, "admin-1", joinedAt, joinedAt,
		))

	countQuery := "SELECT COUNT(*) FROM announcements WHERE (audience = $1 OR audience = $2 OR (audience = $3 AND target_class_id = $4)) AND published_at <= NOW() AND (expires_at IS NULL OR expires_at > NOW())"
	mock.ExpectQuery(regexp.QuoteMeta(countQuery)).
		WithArgs(models.AnnouncementAudienceSiswa, models.AnnouncementAudienceAll, models.AnnouncementAudienceClass, "class-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	announcements, total, err := repo.ListByStudentAndTermPage(context.Background(), "student-1", "term-1", 2, 3, true)
	require.NoError(t, err)
	require.Len(t, announcements, 1)
	assert.Equal(t, "announcement-1", announcements[0].ID)
	assert.Equal(t, 5, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}
