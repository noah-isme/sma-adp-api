package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// TeacherAssignmentRepository persists teacher-class assignments.
type TeacherAssignmentRepository struct {
	db *sqlx.DB
}

// NewTeacherAssignmentRepository constructs the repository.
func NewTeacherAssignmentRepository(db *sqlx.DB) *TeacherAssignmentRepository {
	return &TeacherAssignmentRepository{db: db}
}

// ListByTeacher returns assignments owned by teacher.
func (r *TeacherAssignmentRepository) ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error) {
	const query = `
SELECT ta.id, ta.teacher_id, ta.class_id, ta.subject_id, ta.term_id, ta.created_at,
       c.name AS class_name, s.name AS subject_name, t.name AS term_name, tr.full_name AS teacher_name
FROM teacher_assignments ta
JOIN classes c ON c.id = ta.class_id
JOIN subjects s ON s.id = ta.subject_id
JOIN terms t ON t.id = ta.term_id
JOIN teachers tr ON tr.id = ta.teacher_id
WHERE ta.teacher_id = $1
ORDER BY t.start_date DESC, c.name ASC`
	var assignments []models.TeacherAssignmentDetail
	if err := r.db.SelectContext(ctx, &assignments, query, teacherID); err != nil {
		return nil, fmt.Errorf("list teacher assignments: %w", err)
	}
	return assignments, nil
}

// Exists checks if the teacher-class-subject-term tuple already exists.
func (r *TeacherAssignmentRepository) Exists(ctx context.Context, teacherID, classID, subjectID, termID string) (bool, error) {
	const query = `SELECT 1 FROM teacher_assignments WHERE teacher_id = $1 AND class_id = $2 AND subject_id = $3 AND term_id = $4 LIMIT 1`
	var exists int
	if err := r.db.GetContext(ctx, &exists, query, teacherID, classID, subjectID, termID); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check teacher assignment: %w", err)
	}
	return true, nil
}

// Create inserts a new assignment.
func (r *TeacherAssignmentRepository) Create(ctx context.Context, assignment *models.TeacherAssignment) error {
	if assignment.ID == "" {
		assignment.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if assignment.CreatedAt.IsZero() {
		assignment.CreatedAt = now
	}
	const query = `INSERT INTO teacher_assignments (id, teacher_id, class_id, subject_id, term_id, created_at)
		VALUES (:id, :teacher_id, :class_id, :subject_id, :term_id, :created_at)`
	if _, err := r.db.NamedExecContext(ctx, query, assignment); err != nil {
		return fmt.Errorf("create teacher assignment: %w", err)
	}
	return nil
}

// Delete removes an assignment verifying ownership.
func (r *TeacherAssignmentRepository) Delete(ctx context.Context, teacherID, assignmentID string) error {
	const query = `DELETE FROM teacher_assignments WHERE id = $1 AND teacher_id = $2`
	result, err := r.db.ExecContext(ctx, query, assignmentID, teacherID)
	if err != nil {
		return fmt.Errorf("delete teacher assignment: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted assignment rows: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CountByTeacherAndTerm returns number of assignments for teacher in a term.
func (r *TeacherAssignmentRepository) CountByTeacherAndTerm(ctx context.Context, teacherID, termID string) (int, error) {
	const query = `SELECT COUNT(*) FROM teacher_assignments WHERE teacher_id = $1 AND term_id = $2`
	var count int
	if err := r.db.GetContext(ctx, &count, query, teacherID, termID); err != nil {
		return 0, fmt.Errorf("count teacher assignments: %w", err)
	}
	return count, nil
}
