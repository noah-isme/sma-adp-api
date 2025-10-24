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

// ClassRepository manages persistence for classes.
type ClassRepository struct {
	db *sqlx.DB
}

// NewClassRepository constructs a new class repository.
func NewClassRepository(db *sqlx.DB) *ClassRepository {
	return &ClassRepository{db: db}
}

// List returns classes matching filter criteria.
func (r *ClassRepository) List(ctx context.Context, filter models.ClassFilter) ([]models.Class, int, error) {
	base := "FROM classes WHERE 1=1"
	var conditions []string
	var args []interface{}

	if filter.Grade != "" {
		conditions = append(conditions, fmt.Sprintf("grade = $%d", len(args)+1))
		args = append(args, filter.Grade)
	}
	if filter.Track != "" {
		conditions = append(conditions, fmt.Sprintf("track = $%d", len(args)+1))
		args = append(args, filter.Track)
	}
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(name) LIKE $%d)", len(args)+1))
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
		"name":       true,
		"grade":      true,
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

	query := fmt.Sprintf("SELECT id, name, grade, track, homeroom_teacher_id, created_at, updated_at %s ORDER BY %s %s LIMIT %d OFFSET %d", base, sortBy, order, size, offset)
	var classes []models.Class
	if err := r.db.SelectContext(ctx, &classes, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list classes: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s", base)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count classes: %w", err)
	}
	return classes, total, nil
}

// FindByID returns a class record by ID.
func (r *ClassRepository) FindByID(ctx context.Context, id string) (*models.Class, error) {
	const query = `SELECT id, name, grade, track, homeroom_teacher_id, created_at, updated_at FROM classes WHERE id = $1`
	var class models.Class
	if err := r.db.GetContext(ctx, &class, query, id); err != nil {
		return nil, err
	}
	return &class, nil
}

// FindDetailByID returns class with joined homeroom teacher name if available.
func (r *ClassRepository) FindDetailByID(ctx context.Context, id string) (*models.ClassDetail, error) {
	const query = `SELECT c.id, c.name, c.grade, c.track, c.homeroom_teacher_id, c.created_at, c.updated_at, u.full_name AS homeroom_teacher_name FROM classes c LEFT JOIN users u ON u.id = c.homeroom_teacher_id WHERE c.id = $1`
	var detail models.ClassDetail
	if err := r.db.GetContext(ctx, &detail, query, id); err != nil {
		return nil, err
	}
	return &detail, nil
}

// ExistsByName checks if a class with the same name already exists.
func (r *ClassRepository) ExistsByName(ctx context.Context, name string, excludeID string) (bool, error) {
	query := "SELECT 1 FROM classes WHERE LOWER(name) = LOWER($1)"
	args := []interface{}{name}
	if excludeID != "" {
		query += " AND id <> $2"
		args = append(args, excludeID)
	}
	var exists int
	if err := r.db.GetContext(ctx, &exists, query+" LIMIT 1", args...); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check class name: %w", err)
	}
	return true, nil
}

// Create persists a class record.
func (r *ClassRepository) Create(ctx context.Context, class *models.Class) error {
	if class.ID == "" {
		class.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if class.CreatedAt.IsZero() {
		class.CreatedAt = now
	}
	class.UpdatedAt = now

	const query = `INSERT INTO classes (id, name, grade, track, homeroom_teacher_id, created_at, updated_at) VALUES (:id, :name, :grade, :track, :homeroom_teacher_id, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, class); err != nil {
		return fmt.Errorf("create class: %w", err)
	}
	return nil
}

// Update modifies a class record.
func (r *ClassRepository) Update(ctx context.Context, class *models.Class) error {
	class.UpdatedAt = time.Now().UTC()
	const query = `UPDATE classes SET name = :name, grade = :grade, track = :track, homeroom_teacher_id = :homeroom_teacher_id, updated_at = :updated_at WHERE id = :id`
	if _, err := r.db.NamedExecContext(ctx, query, class); err != nil {
		return fmt.Errorf("update class: %w", err)
	}
	return nil
}

// Delete removes a class record.
func (r *ClassRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM classes WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete class: %w", err)
	}
	return nil
}

// CountClassSubjects returns how many mappings are attached to a class.
func (r *ClassRepository) CountClassSubjects(ctx context.Context, classID string) (int, error) {
	const query = `SELECT COUNT(*) FROM class_subjects WHERE class_id = $1`
	var count int
	if err := r.db.GetContext(ctx, &count, query, classID); err != nil {
		return 0, fmt.Errorf("count class subjects: %w", err)
	}
	return count, nil
}

// CountSchedules returns number of schedules for the class.
func (r *ClassRepository) CountSchedules(ctx context.Context, classID string) (int, error) {
	const query = `SELECT COUNT(*) FROM schedules WHERE class_id = $1`
	var count int
	if err := r.db.GetContext(ctx, &count, query, classID); err != nil {
		return 0, fmt.Errorf("count class schedules: %w", err)
	}
	return count, nil
}
