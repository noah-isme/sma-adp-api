package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// ClassSubjectRepository manages class-subject mappings.
type ClassSubjectRepository struct {
	db *sqlx.DB
}

// NewClassSubjectRepository creates a new repository.
func NewClassSubjectRepository(db *sqlx.DB) *ClassSubjectRepository {
	return &ClassSubjectRepository{db: db}
}

// ListByClass returns subject assignments for a class.
func (r *ClassSubjectRepository) ListByClass(ctx context.Context, classID string) ([]models.ClassSubjectAssignment, error) {
	const query = `
SELECT cs.id, cs.class_id, cs.subject_id, cs.teacher_id, cs.created_at,
       s.name AS subject_name, s.code AS subject_code,
       u.full_name AS teacher_name
FROM class_subjects cs
JOIN subjects s ON s.id = cs.subject_id
LEFT JOIN users u ON u.id = cs.teacher_id
WHERE cs.class_id = $1
ORDER BY s.name ASC`
	var assignments []models.ClassSubjectAssignment
	if err := r.db.SelectContext(ctx, &assignments, query, classID); err != nil {
		return nil, fmt.Errorf("list class subjects: %w", err)
	}
	return assignments, nil
}

// ReplaceAssignments replaces the mapping for the class with provided assignments within a transaction.
func (r *ClassSubjectRepository) ReplaceAssignments(ctx context.Context, classID string, assignments []models.ClassSubject) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace class subjects: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `DELETE FROM class_subjects WHERE class_id = $1`, classID); err != nil {
		return fmt.Errorf("clear existing class subjects: %w", err)
	}

	if len(assignments) == 0 {
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("commit replace class subjects: %w", err)
		}
		return nil
	}

	now := time.Now().UTC()
	for _, assignment := range assignments {
		payload := assignment
		if payload.ID == "" {
			payload.ID = uuid.NewString()
		}
		if payload.CreatedAt.IsZero() {
			payload.CreatedAt = now
		}
		if _, err = tx.NamedExecContext(ctx, `INSERT INTO class_subjects (id, class_id, subject_id, teacher_id, created_at) VALUES (:id, :class_id, :subject_id, :teacher_id, :created_at)`, &payload); err != nil {
			return fmt.Errorf("insert class subject: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit replace class subjects: %w", err)
	}
	return nil
}

// CountByTeacher returns how many classes a teacher is mapped to via class subjects.
func (r *ClassSubjectRepository) CountByTeacher(ctx context.Context, teacherID string) (int, error) {
	const query = `SELECT COUNT(*) FROM class_subjects WHERE teacher_id = $1`
	var count int
	if err := r.db.GetContext(ctx, &count, query, teacherID); err != nil {
		return 0, fmt.Errorf("count class subject by teacher: %w", err)
	}
	return count, nil
}
