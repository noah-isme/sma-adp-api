package repository

import (
	"context"
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
