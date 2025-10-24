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

// StudentRepository manages persistence for student records.
type StudentRepository struct {
	db *sqlx.DB
}

// NewStudentRepository constructs a StudentRepository.
func NewStudentRepository(db *sqlx.DB) *StudentRepository {
	return &StudentRepository{db: db}
}

// List returns students matching the provided filters.
func (r *StudentRepository) List(ctx context.Context, filter models.StudentFilter) ([]models.StudentDetail, int, error) {
	base := "FROM students s LEFT JOIN enrollments e ON e.student_id = s.id AND e.status = $1 LEFT JOIN classes c ON c.id = e.class_id"
	args := []interface{}{models.EnrollmentStatusActive}
	conditions := []string{"1=1"}

	if filter.ClassID != "" {
		conditions = append(conditions, fmt.Sprintf("e.class_id = $%d", len(args)+1))
		args = append(args, filter.ClassID)
	}
	if filter.Active != nil {
		conditions = append(conditions, fmt.Sprintf("s.active = $%d", len(args)+1))
		args = append(args, *filter.Active)
	}
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(s.full_name) LIKE $%d OR LOWER(s.nis) LIKE $%d)", len(args)+1, len(args)+1))
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
	}

	base = fmt.Sprintf("%s WHERE %s", base, strings.Join(conditions, " AND "))

	sortBy := filter.SortBy
	allowedSorts := map[string]string{
		"full_name":  "s.full_name",
		"nis":        "s.nis",
		"created_at": "s.created_at",
	}
	if sortBy == "" {
		sortBy = "created_at"
	}
	column, ok := allowedSorts[sortBy]
	if !ok {
		column = "s.created_at"
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

	query := fmt.Sprintf(`SELECT s.id, s.nis, s.full_name, s.gender, s.birth_date, s.address, s.phone, s.active, s.created_at, s.updated_at,
        e.class_id AS current_class_id, c.name AS current_class_name, e.term_id AS current_term_id, e.joined_at
        %s ORDER BY %s %s LIMIT %d OFFSET %d`, base, column, order, size, offset)

	var students []models.StudentDetail
	if err := r.db.SelectContext(ctx, &students, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list students: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT s.id) %s", base)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count students: %w", err)
	}
	return students, total, nil
}

// FindByID fetches a student detail by ID.
func (r *StudentRepository) FindByID(ctx context.Context, id string) (*models.StudentDetail, error) {
	query := `SELECT s.id, s.nis, s.full_name, s.gender, s.birth_date, s.address, s.phone, s.active, s.created_at, s.updated_at,
        e.class_id AS current_class_id, c.name AS current_class_name, e.term_id AS current_term_id, e.joined_at
        FROM students s
        LEFT JOIN enrollments e ON e.student_id = s.id AND e.status = $2
        LEFT JOIN classes c ON c.id = e.class_id
        WHERE s.id = $1`
	var detail models.StudentDetail
	if err := r.db.GetContext(ctx, &detail, query, id, models.EnrollmentStatusActive); err != nil {
		return nil, err
	}
	return &detail, nil
}

// ExistsByNIS checks if a student with given NIS exists optionally excluding an ID.
func (r *StudentRepository) ExistsByNIS(ctx context.Context, nis string, excludeID string) (bool, error) {
	query := "SELECT 1 FROM students WHERE nis = $1"
	args := []interface{}{nis}
	if excludeID != "" {
		query += " AND id <> $2"
		args = append(args, excludeID)
	}
	var exists int
	if err := r.db.GetContext(ctx, &exists, query+" LIMIT 1", args...); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check nis: %w", err)
	}
	return true, nil
}

// Create inserts a new student record.
func (r *StudentRepository) Create(ctx context.Context, student *models.Student) error {
	if student.ID == "" {
		student.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if student.CreatedAt.IsZero() {
		student.CreatedAt = now
	}
	student.UpdatedAt = now
	const query = `INSERT INTO students (id, nis, full_name, gender, birth_date, address, phone, active, created_at, updated_at)
        VALUES (:id, :nis, :full_name, :gender, :birth_date, :address, :phone, :active, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, student); err != nil {
		return fmt.Errorf("create student: %w", err)
	}
	return nil
}

// Update modifies an existing student.
func (r *StudentRepository) Update(ctx context.Context, student *models.Student) error {
	student.UpdatedAt = time.Now().UTC()
	const query = `UPDATE students SET nis = :nis, full_name = :full_name, gender = :gender, birth_date = :birth_date, address = :address, phone = :phone, active = :active, updated_at = :updated_at WHERE id = :id`
	if _, err := r.db.NamedExecContext(ctx, query, student); err != nil {
		return fmt.Errorf("update student: %w", err)
	}
	return nil
}

// Deactivate marks a student as inactive.
func (r *StudentRepository) Deactivate(ctx context.Context, id string) error {
	const query = `UPDATE students SET active = false, updated_at = $2 WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id, time.Now().UTC()); err != nil {
		return fmt.Errorf("deactivate student: %w", err)
	}
	return nil
}
