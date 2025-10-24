package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// SubjectRepository handles persistence for subjects.
type SubjectRepository struct {
	db *sqlx.DB
}

// NewSubjectRepository creates a new repository instance.
func NewSubjectRepository(db *sqlx.DB) *SubjectRepository {
	return &SubjectRepository{db: db}
}

// List returns subjects matching filters with pagination metadata.
func (r *SubjectRepository) List(ctx context.Context, filter models.SubjectFilter) ([]models.Subject, int, error) {
	base := "FROM subjects WHERE 1=1"
	var conditions []string
	var args []interface{}

	if filter.Track != "" {
		conditions = append(conditions, fmt.Sprintf("track = $%d", len(args)+1))
		args = append(args, filter.Track)
	}
	if filter.Group != "" {
		conditions = append(conditions, fmt.Sprintf("subject_group = $%d", len(args)+1))
		args = append(args, filter.Group)
	}
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(code) LIKE $%d OR LOWER(name) LIKE $%d)", len(args)+1, len(args)+1))
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
	}

	if len(conditions) > 0 {
		base += " AND " + strings.Join(conditions, " AND ")
	}

	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	allowedSorts := map[string]bool{
		"code":       true,
		"name":       true,
		"track":      true,
		"created_at": true,
		"updated_at": true,
	}
	if !allowedSorts[sortBy] {
		sortBy = "created_at"
	}

	order := strings.ToUpper(filter.SortOrder)
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size

	query := fmt.Sprintf("SELECT id, code, name, track, subject_group, created_at, updated_at %s ORDER BY %s %s LIMIT %d OFFSET %d", base, sortBy, order, size, offset)
	var subjects []models.Subject
	if err := r.db.SelectContext(ctx, &subjects, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list subjects: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s", base)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count subjects: %w", err)
	}

	return subjects, total, nil
}

// FindByID returns a subject by id.
func (r *SubjectRepository) FindByID(ctx context.Context, id string) (*models.Subject, error) {
	const query = `SELECT id, code, name, track, subject_group, created_at, updated_at FROM subjects WHERE id = $1`
	var subject models.Subject
	if err := r.db.GetContext(ctx, &subject, query, id); err != nil {
		return nil, err
	}
	return &subject, nil
}

// ExistsByCode checks uniqueness of subject code.
func (r *SubjectRepository) ExistsByCode(ctx context.Context, code string, excludeID string) (bool, error) {
	query := "SELECT 1 FROM subjects WHERE LOWER(code) = LOWER($1)"
	args := []interface{}{code}
	if excludeID != "" {
		query += " AND id <> $2"
		args = append(args, excludeID)
	}

	var exists int
	if err := r.db.GetContext(ctx, &exists, query+" LIMIT 1", args...); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check subject code: %w", err)
	}
	return true, nil
}

// Create persists a new subject.
func (r *SubjectRepository) Create(ctx context.Context, subject *models.Subject) error {
	if subject.ID == "" {
		subject.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if subject.CreatedAt.IsZero() {
		subject.CreatedAt = now
	}
	subject.UpdatedAt = now

	const query = `INSERT INTO subjects (id, code, name, track, subject_group, created_at, updated_at) VALUES (:id, :code, :name, :track, :subject_group, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, subject); err != nil {
		return fmt.Errorf("create subject: %w", err)
	}
	return nil
}

// Update modifies a subject.
func (r *SubjectRepository) Update(ctx context.Context, subject *models.Subject) error {
	subject.UpdatedAt = time.Now().UTC()
	const query = `UPDATE subjects SET code = :code, name = :name, track = :track, subject_group = :subject_group, updated_at = :updated_at WHERE id = :id`
	if _, err := r.db.NamedExecContext(ctx, query, subject); err != nil {
		return fmt.Errorf("update subject: %w", err)
	}
	return nil
}

// Delete removes a subject record.
func (r *SubjectRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM subjects WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete subject: %w", err)
	}
	return nil
}

// CountClassSubjects returns number of class-subject mappings referencing the subject.
func (r *SubjectRepository) CountClassSubjects(ctx context.Context, id string) (int, error) {
	const query = `SELECT COUNT(*) FROM class_subjects WHERE subject_id = $1`
	var count int
	if err := r.db.GetContext(ctx, &count, query, id); err != nil {
		return 0, fmt.Errorf("count class subjects: %w", err)
	}
	return count, nil
}
