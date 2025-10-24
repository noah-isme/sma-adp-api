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

// EnrollmentRepository handles persistence of enrollments.
type EnrollmentRepository struct {
	db *sqlx.DB
}

// NewEnrollmentRepository constructs the repository.
func NewEnrollmentRepository(db *sqlx.DB) *EnrollmentRepository {
	return &EnrollmentRepository{db: db}
}

// List returns enrollments filtered by the provided criteria.
func (r *EnrollmentRepository) List(ctx context.Context, filter models.EnrollmentFilter) ([]models.EnrollmentDetail, int, error) {
	base := `FROM enrollments e
LEFT JOIN students s ON s.id = e.student_id
LEFT JOIN classes c ON c.id = e.class_id
LEFT JOIN terms t ON t.id = e.term_id`
	var conditions []string
	var args []interface{}

	if filter.StudentID != "" {
		conditions = append(conditions, fmt.Sprintf("e.student_id = $%d", len(args)+1))
		args = append(args, filter.StudentID)
	}
	if filter.ClassID != "" {
		conditions = append(conditions, fmt.Sprintf("e.class_id = $%d", len(args)+1))
		args = append(args, filter.ClassID)
	}
	if filter.TermID != "" {
		conditions = append(conditions, fmt.Sprintf("e.term_id = $%d", len(args)+1))
		args = append(args, filter.TermID)
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("e.status = $%d", len(args)+1))
		args = append(args, filter.Status)
	}

	clause := ""
	if len(conditions) > 0 {
		clause = " WHERE " + strings.Join(conditions, " AND ")
	}

	allowedSorts := map[string]string{
		"joined_at":    "e.joined_at",
		"student_name": "s.full_name",
		"class_name":   "c.name",
	}
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "joined_at"
	}
	orderBy := allowedSorts[sortBy]
	if orderBy == "" {
		orderBy = "e.joined_at"
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

	query := fmt.Sprintf(`SELECT e.id, e.student_id, e.class_id, e.term_id, e.joined_at, e.left_at, e.status,
        s.full_name AS student_name, s.nis AS student_nis, c.name AS class_name, t.name AS term_name
        %s ORDER BY %s %s LIMIT %d OFFSET %d`, base+clause, orderBy, order, size, offset)

	var enrollments []models.EnrollmentDetail
	if err := r.db.SelectContext(ctx, &enrollments, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list enrollments: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s", base+clause)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count enrollments: %w", err)
	}
	return enrollments, total, nil
}

// FindByID returns an enrollment by its ID.
func (r *EnrollmentRepository) FindByID(ctx context.Context, id string) (*models.Enrollment, error) {
	const query = `SELECT id, student_id, class_id, term_id, joined_at, left_at, status FROM enrollments WHERE id = $1`
	var enrollment models.Enrollment
	if err := r.db.GetContext(ctx, &enrollment, query, id); err != nil {
		return nil, err
	}
	return &enrollment, nil
}

// FindDetailByID returns an enrollment with contextual info.
func (r *EnrollmentRepository) FindDetailByID(ctx context.Context, id string) (*models.EnrollmentDetail, error) {
	const query = `SELECT e.id, e.student_id, e.class_id, e.term_id, e.joined_at, e.left_at, e.status,
        s.full_name AS student_name, s.nis AS student_nis, c.name AS class_name, t.name AS term_name
        FROM enrollments e
        LEFT JOIN students s ON s.id = e.student_id
        LEFT JOIN classes c ON c.id = e.class_id
        LEFT JOIN terms t ON t.id = e.term_id
        WHERE e.id = $1`
	var detail models.EnrollmentDetail
	if err := r.db.GetContext(ctx, &detail, query, id); err != nil {
		return nil, err
	}
	return &detail, nil
}

// ExistsActive checks if an active enrollment exists for combination.
func (r *EnrollmentRepository) ExistsActive(ctx context.Context, studentID, classID, termID, excludeID string) (bool, error) {
	query := "SELECT 1 FROM enrollments WHERE student_id = $1 AND class_id = $2 AND term_id = $3 AND status = $4"
	args := []interface{}{studentID, classID, termID, models.EnrollmentStatusActive}
	if excludeID != "" {
		query += fmt.Sprintf(" AND id <> $%d", len(args)+1)
		args = append(args, excludeID)
	}
	query += " LIMIT 1"
	var exists int
	if err := r.db.GetContext(ctx, &exists, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check active enrollment: %w", err)
	}
	return true, nil
}

// Create persists a new enrollment record.
func (r *EnrollmentRepository) Create(ctx context.Context, enrollment *models.Enrollment) error {
	if enrollment.ID == "" {
		enrollment.ID = uuid.NewString()
	}
	if enrollment.JoinedAt.IsZero() {
		enrollment.JoinedAt = time.Now().UTC()
	}
	if enrollment.Status == "" {
		enrollment.Status = models.EnrollmentStatusActive
	}
	const query = `INSERT INTO enrollments (id, student_id, class_id, term_id, joined_at, left_at, status)
        VALUES (:id, :student_id, :class_id, :term_id, :joined_at, :left_at, :status)`
	if _, err := r.db.NamedExecContext(ctx, query, enrollment); err != nil {
		return fmt.Errorf("create enrollment: %w", err)
	}
	return nil
}

// UpdateClass updates the class reference for an enrollment.
func (r *EnrollmentRepository) UpdateClass(ctx context.Context, id, classID string) error {
	const query = `UPDATE enrollments SET class_id = $2, status = $3, left_at = NULL WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id, classID, models.EnrollmentStatusActive); err != nil {
		return fmt.Errorf("transfer enrollment: %w", err)
	}
	return nil
}

// UpdateStatus updates status and left_at for an enrollment.
func (r *EnrollmentRepository) UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus, leftAt *time.Time) error {
	const query = `UPDATE enrollments SET status = $2, left_at = $3 WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id, status, leftAt); err != nil {
		return fmt.Errorf("update enrollment status: %w", err)
	}
	return nil
}

// ListByClassAndTerm returns active enrollments for a class and term.
func (r *EnrollmentRepository) ListByClassAndTerm(ctx context.Context, classID, termID string) ([]models.Enrollment, error) {
	const query = `SELECT id, student_id, class_id, term_id, joined_at, left_at, status FROM enrollments WHERE class_id = $1 AND term_id = $2 AND status = $3`
	var enrollments []models.Enrollment
	if err := r.db.SelectContext(ctx, &enrollments, query, classID, termID, models.EnrollmentStatusActive); err != nil {
		return nil, fmt.Errorf("list class enrollments: %w", err)
	}
	return enrollments, nil
}

// FindActiveByStudentAndSubject returns the active enrollment for subject operations.
func (r *EnrollmentRepository) FindActiveByStudentAndTerm(ctx context.Context, studentID, termID string) ([]models.Enrollment, error) {
	const query = `SELECT id, student_id, class_id, term_id, joined_at, left_at, status FROM enrollments WHERE student_id = $1 AND term_id = $2 AND status = $3`
	var enrollments []models.Enrollment
	if err := r.db.SelectContext(ctx, &enrollments, query, studentID, termID, models.EnrollmentStatusActive); err != nil {
		return nil, fmt.Errorf("find student enrollments: %w", err)
	}
	return enrollments, nil
}

// ValidateBulkIDs ensures all IDs exist returning missing ones.
func (r *EnrollmentRepository) ValidateBulkIDs(ctx context.Context, enrollmentIDs []string) (map[string]bool, error) {
	if len(enrollmentIDs) == 0 {
		return map[string]bool{}, nil
	}
	const chunkSize = 100
	existing := make(map[string]bool, len(enrollmentIDs))
	for start := 0; start < len(enrollmentIDs); start += chunkSize {
		end := start + chunkSize
		if end > len(enrollmentIDs) {
			end = len(enrollmentIDs)
		}
		chunk := enrollmentIDs[start:end]
		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = id
		}
		query := fmt.Sprintf("SELECT id FROM enrollments WHERE id IN (%s)", strings.Join(placeholders, ","))
		rows, err := r.db.QueryxContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("validate enrollments: %w", err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan enrollment id: %w", err)
			}
			existing[id] = true
		}
		rows.Close()
	}
	return existing, nil
}
