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

// GradeRepository handles grade entry persistence.
type GradeRepository struct {
	db *sqlx.DB
}

// NewGradeRepository creates a new grade repository.
func NewGradeRepository(db *sqlx.DB) *GradeRepository {
	return &GradeRepository{db: db}
}

// gradeReportFromClause centralizes the report joins so list/count/status
// queries cannot silently diverge. The joins also provide the display fields
// consumed by the admin report instead of returning IDs as placeholders.
const gradeReportFromClause = `
        FROM grades g
        JOIN grade_components gc ON gc.id = g.component_id
        JOIN enrollments e ON e.id = g.enrollment_id
        JOIN classes c ON c.id = e.class_id
        JOIN students s ON s.id = e.student_id
        JOIN subjects sub ON sub.id = g.subject_id
        JOIN terms t ON t.id = e.term_id
        LEFT JOIN class_subjects cs ON cs.class_id = e.class_id AND cs.subject_id = g.subject_id
        LEFT JOIN teachers tr ON tr.id = cs.teacher_id
        LEFT JOIN grade_configs gcfg ON gcfg.class_id = e.class_id AND gcfg.subject_id = g.subject_id AND gcfg.term_id = e.term_id
        LEFT JOIN grade_config_components gcc ON gcc.grade_config_id = gcfg.id AND gcc.component_id = g.component_id`

func gradeReportSelect() string {
	return `SELECT g.id, g.enrollment_id, e.student_id, s.full_name AS student_name,
        s.nis AS student_nis, e.class_id, c.name AS class_name, e.term_id,
        t.name AS term_name, g.subject_id, sub.name AS subject_name,
        g.component_id, gc.code AS component_code, gc.name AS component_name,
        COALESCE(gc.description, '') AS component_description,
        COALESCE(gcc.weight, 0) AS component_weight,
        COALESCE(cs.teacher_id, '') AS teacher_id, COALESCE(tr.full_name, '') AS teacher_name,
        g.grade_value, g.created_at, g.updated_at`
}

func (r *GradeRepository) gradeReportWhere(filter models.GradeFilter) (string, []interface{}) {
	where := " WHERE g.deleted_at IS NULL"
	var args []interface{}
	if filter.ID != "" {
		where += fmt.Sprintf(" AND g.id = $%d", len(args)+1)
		args = append(args, filter.ID)
	}
	if filter.EnrollmentID != "" {
		where += fmt.Sprintf(" AND g.enrollment_id = $%d", len(args)+1)
		args = append(args, filter.EnrollmentID)
	}
	if filter.SubjectID != "" {
		where += fmt.Sprintf(" AND g.subject_id = $%d", len(args)+1)
		args = append(args, filter.SubjectID)
	}
	if filter.ComponentID != "" {
		where += fmt.Sprintf(" AND g.component_id = $%d", len(args)+1)
		args = append(args, filter.ComponentID)
	}
	if filter.TermID != "" {
		where += fmt.Sprintf(" AND e.term_id = $%d", len(args)+1)
		args = append(args, filter.TermID)
	}
	if filter.ClassID != "" {
		where += fmt.Sprintf(" AND e.class_id = $%d", len(args)+1)
		args = append(args, filter.ClassID)
	}
	if filter.TeacherID != "" {
		where += fmt.Sprintf(" AND cs.teacher_id = $%d", len(args)+1)
		args = append(args, filter.TeacherID)
	}
	if filter.ScoreMin != nil {
		where += fmt.Sprintf(" AND g.grade_value >= $%d", len(args)+1)
		args = append(args, *filter.ScoreMin)
	}
	if filter.ScoreMax != nil {
		where += fmt.Sprintf(" AND g.grade_value <= $%d", len(args)+1)
		args = append(args, *filter.ScoreMax)
	}
	if filter.Search != "" {
		where += fmt.Sprintf(" AND (LOWER(s.full_name) LIKE $%d OR LOWER(s.nis) LIKE $%d OR LOWER(gc.code) LIKE $%d OR LOWER(gc.name) LIKE $%d OR LOWER(sub.name) LIKE $%d)", len(args)+1, len(args)+1, len(args)+1, len(args)+1, len(args)+1)
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
	}
	// The schema does not yet persist a per-config KKM, so report and CSV
	// compatibility use the shared model thresholds.
	switch strings.ToUpper(strings.TrimSpace(filter.Status)) {
	case "PASS":
		where += fmt.Sprintf(" AND g.grade_value >= %v", models.DefaultGradePassMark)
	case "REMEDIAL", "CAUTION":
		where += fmt.Sprintf(" AND g.grade_value >= %v AND g.grade_value < %v", models.DefaultGradeKKM, models.DefaultGradePassMark)
	case "FAIL":
		where += fmt.Sprintf(" AND g.grade_value < %v", models.DefaultGradeKKM)
	}
	return where, args
}

func gradeReportSort(filter models.GradeFilter) (string, string) {
	sortBy := filter.SortBy
	allowedSorts := map[string]string{
		"score":          "g.grade_value",
		"grade_value":    "g.grade_value",
		"studentName":    "s.full_name",
		"student_name":   "s.full_name",
		"subjectName":    "sub.name",
		"subject_name":   "sub.name",
		"componentName":  "gc.name",
		"component_name": "gc.name",
		"lastUpdated":    "g.updated_at",
		"updated_at":     "g.updated_at",
	}
	if sortBy == "" {
		sortBy = "lastUpdated"
	}
	column, ok := allowedSorts[sortBy]
	if !ok {
		column = "g.updated_at"
	}
	order := "DESC"
	switch strings.ToLower(strings.TrimSpace(filter.SortOrder)) {
	case "asc", "ascend":
		order = "ASC"
	case "desc", "descend":
		order = "DESC"
	}
	return column, order
}

// List returns grade entries matching the filter.
func (r *GradeRepository) List(ctx context.Context, filter models.GradeFilter) ([]models.Grade, error) {
	where, args := r.gradeReportWhere(filter)
	column, order := gradeReportSort(filter)
	query := gradeReportSelect() + gradeReportFromClause + where

	query += fmt.Sprintf(" ORDER BY %s %s", column, order)

	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", size, offset)

	var grades []models.Grade
	if err := r.db.SelectContext(ctx, &grades, query, args...); err != nil {
		return nil, fmt.Errorf("list grades: %w", err)
	}
	return grades, nil
}

// Count returns the number of rows matching the same report filter without
// pagination. It keeps the admin pagination contract truthful.
func (r *GradeRepository) Count(ctx context.Context, filter models.GradeFilter) (int, error) {
	where, args := r.gradeReportWhere(filter)
	var total int
	query := "SELECT COUNT(*)" + gradeReportFromClause + where
	if err := r.db.GetContext(ctx, &total, query, args...); err != nil {
		return 0, fmt.Errorf("count grades: %w", err)
	}
	return total, nil
}

// Upsert inserts or updates a grade entry.
func (r *GradeRepository) Upsert(ctx context.Context, grade *models.Grade) error {
	if grade.ID == "" {
		grade.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if grade.CreatedAt.IsZero() {
		grade.CreatedAt = now
	}
	grade.UpdatedAt = now
	const query = `INSERT INTO grades (id, enrollment_id, subject_id, component_id, grade_value, created_at, updated_at)
	        VALUES (:id, :enrollment_id, :subject_id, :component_id, :grade_value, :created_at, :updated_at)
	        ON CONFLICT (enrollment_id, subject_id, component_id)
	        DO UPDATE SET grade_value = EXCLUDED.grade_value, updated_at = EXCLUDED.updated_at, deleted_at = NULL`
	if _, err := r.db.NamedExecContext(ctx, query, grade); err != nil {
		return fmt.Errorf("upsert grade: %w", err)
	}
	return nil
}

// BulkUpsert inserts or updates multiple grades in a transaction.
func (r *GradeRepository) BulkUpsert(ctx context.Context, grades []models.Grade) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	for i := range grades {
		if grades[i].ID == "" {
			grades[i].ID = uuid.NewString()
		}
		now := time.Now().UTC()
		if grades[i].CreatedAt.IsZero() {
			grades[i].CreatedAt = now
		}
		grades[i].UpdatedAt = now
		const query = `INSERT INTO grades (id, enrollment_id, subject_id, component_id, grade_value, created_at, updated_at)
	                VALUES (:id, :enrollment_id, :subject_id, :component_id, :grade_value, :created_at, :updated_at)
	                ON CONFLICT (enrollment_id, subject_id, component_id)
	                DO UPDATE SET grade_value = EXCLUDED.grade_value, updated_at = EXCLUDED.updated_at, deleted_at = NULL`
		if _, err := tx.NamedExecContext(ctx, query, grades[i]); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("bulk upsert grade: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit grades: %w", err)
	}
	return nil
}

// FetchByEnrollments returns grades keyed by enrollment ID.
func (r *GradeRepository) FetchByEnrollments(ctx context.Context, enrollmentIDs []string, subjectID string) (map[string][]models.Grade, error) {
	if len(enrollmentIDs) == 0 {
		return map[string][]models.Grade{}, nil
	}
	placeholders := make([]string, len(enrollmentIDs))
	args := make([]interface{}, len(enrollmentIDs)+1)
	for i, id := range enrollmentIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	args[len(args)-1] = subjectID
	query := fmt.Sprintf(`SELECT g.id, g.enrollment_id, g.subject_id, g.component_id, g.grade_value, g.created_at, g.updated_at, gc.code AS component_code
	        FROM grades g
	        JOIN grade_components gc ON gc.id = g.component_id
	        WHERE g.enrollment_id IN (%s) AND g.subject_id = $%d AND g.deleted_at IS NULL`, strings.Join(placeholders, ","), len(args))
	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetch grades: %w", err)
	}
	defer rows.Close()
	result := make(map[string][]models.Grade, len(enrollmentIDs))
	for rows.Next() {
		var grade models.Grade
		if err := rows.StructScan(&grade); err != nil {
			return nil, fmt.Errorf("scan grade: %w", err)
		}
		result[grade.EnrollmentID] = append(result[grade.EnrollmentID], grade)
	}
	return result, nil
}

// Delete soft-deletes a grade entry while retaining its audit history.
func (r *GradeRepository) Delete(ctx context.Context, id string) error {
	const query = `UPDATE grades SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
	        WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete grade: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("delete grade rows: %w", err)
	} else if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteByConfig soft-deletes grades for enrollments when config cleanup is required.
func (r *GradeRepository) DeleteByConfig(ctx context.Context, enrollmentIDs []string, subjectID string) error {
	if len(enrollmentIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(enrollmentIDs))
	args := make([]interface{}, len(enrollmentIDs)+1)
	for i, id := range enrollmentIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	args[len(args)-1] = subjectID
	query := fmt.Sprintf("UPDATE grades SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE enrollment_id IN (%s) AND subject_id = $%d AND deleted_at IS NULL", strings.Join(placeholders, ","), len(args))
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("soft-delete grades: %w", err)
	}
	return nil
}

// ListByStudentAndTerm returns all grades for a student in a term.
func (r *GradeRepository) ListByStudentAndTerm(ctx context.Context, studentID, termID string) ([]models.Grade, error) {
	const query = `SELECT g.id, g.enrollment_id, g.subject_id, g.component_id, g.grade_value, g.created_at, g.updated_at, gc.code AS component_code
	        FROM grades g
	        JOIN grade_components gc ON gc.id = g.component_id
	        JOIN enrollments e ON e.id = g.enrollment_id
	        WHERE e.student_id = $1 AND e.term_id = $2 AND g.deleted_at IS NULL
	        ORDER BY g.updated_at DESC`
	var grades []models.Grade
	if err := r.db.SelectContext(ctx, &grades, query, studentID, termID); err != nil {
		return nil, fmt.Errorf("list student grades by term: %w", err)
	}
	return grades, nil
}
