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

// TeacherRepository manages persistence for teachers.
type TeacherRepository struct {
	db *sqlx.DB
}

// NewTeacherRepository constructs a TeacherRepository.
func NewTeacherRepository(db *sqlx.DB) *TeacherRepository {
	return &TeacherRepository{db: db}
}

// List returns teachers matching filters along with total count.
func (r *TeacherRepository) List(ctx context.Context, filter models.TeacherFilter) ([]models.Teacher, int, error) {
	base := "FROM teachers WHERE 1=1"
	var conditions []string
	var args []interface{}

	if filter.Active != nil {
		conditions = append(conditions, fmt.Sprintf("active = $%d", len(args)+1))
		args = append(args, *filter.Active)
	}
	if filter.Search != "" {
		search := "%" + strings.ToLower(filter.Search) + "%"
		conditions = append(conditions, fmt.Sprintf("(LOWER(full_name) LIKE $%d OR LOWER(email) LIKE $%d OR LOWER(COALESCE(nip, '')) LIKE $%d)", len(args)+1, len(args)+1, len(args)+1))
		args = append(args, search)
	}

	if len(conditions) > 0 {
		base += " AND " + strings.Join(conditions, " AND ")
	}

	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	allowedSorts := map[string]string{
		"full_name":  "full_name",
		"email":      "email",
		"created_at": "created_at",
		"updated_at": "updated_at",
	}
	column, ok := allowedSorts[sortBy]
	if !ok {
		column = "created_at"
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

	query := fmt.Sprintf("SELECT id, nip, email, full_name, phone, expertise, active, created_at, updated_at %s ORDER BY %s %s LIMIT %d OFFSET %d", base, column, order, size, offset)
	var teachers []models.Teacher
	if err := r.db.SelectContext(ctx, &teachers, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list teachers: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s", base)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count teachers: %w", err)
	}

	return teachers, total, nil
}

// FindByID fetches a teacher by ID.
func (r *TeacherRepository) FindByID(ctx context.Context, id string) (*models.Teacher, error) {
	const query = `SELECT id, nip, email, full_name, phone, expertise, active, created_at, updated_at FROM teachers WHERE id = $1`
	var teacher models.Teacher
	if err := r.db.GetContext(ctx, &teacher, query, id); err != nil {
		return nil, err
	}
	return &teacher, nil
}

// FindByEmail fetches a teacher by email.
func (r *TeacherRepository) FindByEmail(ctx context.Context, email string) (*models.Teacher, error) {
	const query = `SELECT id, nip, email, full_name, phone, expertise, active, created_at, updated_at FROM teachers WHERE LOWER(email) = LOWER($1)`
	var teacher models.Teacher
	if err := r.db.GetContext(ctx, &teacher, query, email); err != nil {
		return nil, err
	}
	return &teacher, nil
}

// FindByNIP fetches a teacher by NIP.
func (r *TeacherRepository) FindByNIP(ctx context.Context, nip string) (*models.Teacher, error) {
	const query = `SELECT id, nip, email, full_name, phone, expertise, active, created_at, updated_at FROM teachers WHERE nip = $1`
	var teacher models.Teacher
	if err := r.db.GetContext(ctx, &teacher, query, nip); err != nil {
		return nil, err
	}
	return &teacher, nil
}

// ExistsByEmail checks if another teacher uses the same email.
func (r *TeacherRepository) ExistsByEmail(ctx context.Context, email string, excludeID string) (bool, error) {
	query := "SELECT 1 FROM teachers WHERE LOWER(email) = LOWER($1)"
	args := []interface{}{email}
	if excludeID != "" {
		query += " AND id <> $2"
		args = append(args, excludeID)
	}
	var exists int
	if err := r.db.GetContext(ctx, &exists, query+" LIMIT 1", args...); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check teacher email: %w", err)
	}
	return true, nil
}

// ExistsByNIP checks if another teacher uses the same NIP.
func (r *TeacherRepository) ExistsByNIP(ctx context.Context, nip string, excludeID string) (bool, error) {
	if strings.TrimSpace(nip) == "" {
		return false, nil
	}
	query := "SELECT 1 FROM teachers WHERE nip = $1"
	args := []interface{}{nip}
	if excludeID != "" {
		query += " AND id <> $2"
		args = append(args, excludeID)
	}
	var exists int
	if err := r.db.GetContext(ctx, &exists, query+" LIMIT 1", args...); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check teacher nip: %w", err)
	}
	return true, nil
}

// Create inserts a new teacher record.
func (r *TeacherRepository) Create(ctx context.Context, teacher *models.Teacher) error {
	if teacher.ID == "" {
		teacher.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if teacher.CreatedAt.IsZero() {
		teacher.CreatedAt = now
	}
	teacher.UpdatedAt = now

	const query = `INSERT INTO teachers (id, nip, email, full_name, phone, expertise, active, created_at, updated_at)
		VALUES (:id, :nip, :email, :full_name, :phone, :expertise, :active, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, teacher); err != nil {
		return fmt.Errorf("create teacher: %w", err)
	}
	return nil
}

// Update modifies an existing teacher record.
func (r *TeacherRepository) Update(ctx context.Context, teacher *models.Teacher) error {
	teacher.UpdatedAt = time.Now().UTC()
	const query = `UPDATE teachers SET nip = :nip, email = :email, full_name = :full_name, phone = :phone, expertise = :expertise, active = :active, updated_at = :updated_at WHERE id = :id`
	if _, err := r.db.NamedExecContext(ctx, query, teacher); err != nil {
		return fmt.Errorf("update teacher: %w", err)
	}
	return nil
}

// Deactivate sets a teacher's active flag to false.
func (r *TeacherRepository) Deactivate(ctx context.Context, id string) error {
	const query = `UPDATE teachers SET active = FALSE, updated_at = $2 WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id, time.Now().UTC()); err != nil {
		return fmt.Errorf("deactivate teacher: %w", err)
	}
	return nil
}
