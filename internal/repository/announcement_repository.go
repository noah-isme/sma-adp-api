package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// AnnouncementRepository provides persistence for announcements.
type AnnouncementRepository struct {
	db *sqlx.DB
}

// NewAnnouncementRepository creates the repository.
func NewAnnouncementRepository(db *sqlx.DB) *AnnouncementRepository {
	return &AnnouncementRepository{db: db}
}

// List returns announcements visible to the provided audiences.
func (r *AnnouncementRepository) List(ctx context.Context, filter models.AnnouncementFilter) ([]models.Announcement, int, error) {
	base := "FROM announcements"
	where := []string{"published_at <= NOW()"}
	where = append(where, "(expires_at IS NULL OR expires_at > NOW())")
	args := []interface{}{}
	allowedAudiences := map[string]struct{}{}
	for _, role := range filter.AudienceRoles {
		switch role {
		case models.RoleTeacher:
			allowedAudiences[string(models.AnnouncementAudienceGuru)] = struct{}{}
		case models.RoleStudent:
			allowedAudiences[string(models.AnnouncementAudienceSiswa)] = struct{}{}
		case models.RoleAdmin, models.RoleSuperAdmin:
			allowedAudiences[string(models.AnnouncementAudienceGuru)] = struct{}{}
			allowedAudiences[string(models.AnnouncementAudienceSiswa)] = struct{}{}
			allowedAudiences[string(models.AnnouncementAudienceClass)] = struct{}{}
		}
	}
	allowedAudiences[string(models.AnnouncementAudienceAll)] = struct{}{}
	if len(filter.ClassIDs) > 0 {
		where = append(where, fmt.Sprintf("(audience <> 'CLASS' OR target_class_id = ANY($%d))", len(args)+1))
		args = append(args, pqStringArray(filter.ClassIDs))
		allowedAudiences[string(models.AnnouncementAudienceClass)] = struct{}{}
	}
	if len(allowedAudiences) > 0 {
		values := make([]string, 0, len(allowedAudiences))
		for v := range allowedAudiences {
			values = append(values, v)
		}
		where = append(where, fmt.Sprintf("audience = ANY($%d)", len(args)+1))
		args = append(args, pqStringArray(values))
	}
	whereClause := strings.Join(where, " AND ")

	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size

	query := fmt.Sprintf(`SELECT id, title, content, audience, target_class_id, priority, is_pinned, published_at, expires_at, created_by, created_at, updated_at
%s WHERE %s
ORDER BY is_pinned DESC, priority DESC, published_at DESC
LIMIT %d OFFSET %d`, base, whereClause, size, offset)
	var announcements []models.Announcement
	if err := r.db.SelectContext(ctx, &announcements, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list announcements: %w", err)
	}
	countQuery := fmt.Sprintf("SELECT COUNT(*) %s WHERE %s", base, whereClause)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count announcements: %w", err)
	}
	return announcements, total, nil
}

// GetByID returns an announcement by identifier.
func (r *AnnouncementRepository) GetByID(ctx context.Context, id string) (*models.Announcement, error) {
	const query = `SELECT id, title, content, audience, target_class_id, priority, is_pinned, published_at, expires_at, created_by, created_at, updated_at
FROM announcements WHERE id = $1`
	var announcement models.Announcement
	if err := r.db.GetContext(ctx, &announcement, query, id); err != nil {
		return nil, err
	}
	return &announcement, nil
}

// FindByID is an alias for GetByID to match the service interface.
func (r *AnnouncementRepository) FindByID(ctx context.Context, id string) (*models.Announcement, error) {
	return r.GetByID(ctx, id)
}

// Create inserts a new announcement.
func (r *AnnouncementRepository) Create(ctx context.Context, announcement *models.Announcement) error {
	if announcement.ID == "" {
		announcement.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if announcement.CreatedAt.IsZero() {
		announcement.CreatedAt = now
	}
	announcement.UpdatedAt = now
	query := `INSERT INTO announcements (id, title, content, audience, target_class_id, priority, is_pinned, published_at, expires_at, created_by, created_at, updated_at)
VALUES (:id, :title, :content, :audience, :target_class_id, :priority, :is_pinned, :published_at, :expires_at, :created_by, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, announcement); err != nil {
		return fmt.Errorf("create announcement: %w", err)
	}
	return nil
}

// Update modifies an existing announcement.
func (r *AnnouncementRepository) Update(ctx context.Context, announcement *models.Announcement) error {
	announcement.UpdatedAt = time.Now().UTC()
	query := `UPDATE announcements SET title = :title, content = :content, audience = :audience, target_class_id = :target_class_id,
priority = :priority, is_pinned = :is_pinned, published_at = :published_at, expires_at = :expires_at, updated_at = :updated_at
WHERE id = :id`
	if _, err := r.db.NamedExecContext(ctx, query, announcement); err != nil {
		return fmt.Errorf("update announcement: %w", err)
	}
	return nil
}

// Delete removes an announcement.
func (r *AnnouncementRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM announcements WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete announcement: %w", err)
	}
	return nil
}

// pqStringArray helper ensures we pass string arrays consistently.
func pqStringArray(values []string) interface{} {
	return pq.Array(values)
}

// ListByStudentAndTerm returns announcements visible to a student in a term.
func (r *AnnouncementRepository) ListByStudentAndTerm(ctx context.Context, studentID, termID string) ([]models.Announcement, error) {
	// First get the student's class for this term
	const studentClassQuery = `SELECT e.class_id FROM enrollments e WHERE e.student_id = $1 AND e.term_id = $2 AND e.status = $3 LIMIT 1`
	var classID string
	err := r.db.GetContext(ctx, &classID, studentClassQuery, studentID, termID, models.EnrollmentStatusActive)
	if err != nil {
		return nil, fmt.Errorf("get student class for term: %w", err)
	}

	// Now get announcements visible to this student (SISWA audience, CLASS for their class, or ALL)
	const query = `SELECT id, title, content, audience, target_class_id, priority, is_pinned, published_at, expires_at, created_by, created_at, updated_at
	FROM announcements
	WHERE published_at <= NOW()
	  AND (expires_at IS NULL OR expires_at > NOW())
	  AND (
	    audience = $1  -- SISWA
	    OR audience = $2  -- ALL
	    OR (audience = $3 AND target_class_id = $4)  -- CLASS for their class
	  )
	ORDER BY is_pinned DESC, priority DESC, published_at DESC`
	var announcements []models.Announcement
	if err := r.db.SelectContext(ctx, &announcements, query,
		models.AnnouncementAudienceSiswa,
		models.AnnouncementAudienceAll,
		models.AnnouncementAudienceClass,
		classID); err != nil {
		return nil, fmt.Errorf("list student announcements by term: %w", err)
	}
	return announcements, nil
}

// ListByStudentAndTermPage returns announcements visible to a student with
// database-level pagination and a total count for response metadata.
func (r *AnnouncementRepository) ListByStudentAndTermPage(ctx context.Context, studentID, termID string, page, limit int, activeOnly bool) ([]models.Announcement, int, error) {
	classID, err := r.studentClassID(ctx, studentID, termID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, fmt.Errorf("get student class for paginated announcements: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	where := []string{
		"(audience = $1 OR audience = $2 OR (audience = $3 AND target_class_id = $4))",
	}
	args := []interface{}{
		models.AnnouncementAudienceSiswa,
		models.AnnouncementAudienceAll,
		models.AnnouncementAudienceClass,
		classID,
	}
	if activeOnly {
		where = append(where, "published_at <= NOW()", "(expires_at IS NULL OR expires_at > NOW())")
	}
	whereClause := strings.Join(where, " AND ")
	const selectColumns = "id, title, content, audience, target_class_id, priority, is_pinned, published_at, expires_at, created_by, created_at, updated_at"

	query := fmt.Sprintf(`SELECT %s FROM announcements WHERE %s
ORDER BY is_pinned DESC, priority DESC, published_at DESC, id DESC
LIMIT $5 OFFSET $6`, selectColumns, whereClause)
	queryArgs := append(append([]interface{}{}, args...), limit, offset)
	var announcements []models.Announcement
	if err := r.db.SelectContext(ctx, &announcements, query, queryArgs...); err != nil {
		return nil, 0, fmt.Errorf("list paginated student announcements: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM announcements WHERE %s", whereClause)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count paginated student announcements: %w", err)
	}
	return announcements, total, nil
}

// FindByIDForStudent returns an active announcement only when it is visible to
// the student's audience or current class.
func (r *AnnouncementRepository) FindByIDForStudent(ctx context.Context, id, studentID string) (*models.Announcement, error) {
	classID, err := r.studentClassID(ctx, studentID, "")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get student class for announcement: %w", err)
	}

	const query = `SELECT id, title, content, audience, target_class_id, priority, is_pinned, published_at, expires_at, created_by, created_at, updated_at
FROM announcements
WHERE id = $1
  AND published_at <= NOW()
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (audience = $2 OR audience = $3 OR (audience = $4 AND target_class_id = $5))`
	var announcement models.Announcement
	if err := r.db.GetContext(ctx, &announcement, query, id,
		models.AnnouncementAudienceSiswa,
		models.AnnouncementAudienceAll,
		models.AnnouncementAudienceClass,
		classID,
	); err != nil {
		return nil, err
	}
	return &announcement, nil
}

func (r *AnnouncementRepository) studentClassID(ctx context.Context, studentID, termID string) (string, error) {
	var classID string
	if termID != "" {
		const query = `SELECT e.class_id FROM enrollments e WHERE e.student_id = $1 AND e.term_id = $2 AND e.status = $3 ORDER BY e.joined_at DESC LIMIT 1`
		err := r.db.GetContext(ctx, &classID, query, studentID, termID, models.EnrollmentStatusActive)
		return classID, err
	}

	const query = `SELECT e.class_id FROM enrollments e WHERE e.student_id = $1 AND e.status = $2 ORDER BY e.joined_at DESC LIMIT 1`
	err := r.db.GetContext(ctx, &classID, query, studentID, models.EnrollmentStatusActive)
	return classID, err
}
