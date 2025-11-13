package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/dto"
)

// HomeroomRepository provides persistence helpers for homeroom projections.
type HomeroomRepository struct {
	db *sqlx.DB
}

// NewHomeroomRepository constructs the repository.
func NewHomeroomRepository(db *sqlx.DB) *HomeroomRepository {
	return &HomeroomRepository{db: db}
}

// List returns homeroom rows for the specified term (optionally filtering class).
func (r *HomeroomRepository) List(ctx context.Context, filter dto.HomeroomFilter) ([]dto.HomeroomItem, error) {
	return r.list(ctx, filter, "")
}

// ListForTeacher returns homerooms restricted to classes owned by the teacher.
func (r *HomeroomRepository) ListForTeacher(ctx context.Context, teacherID string, filter dto.HomeroomFilter) ([]dto.HomeroomItem, error) {
	return r.list(ctx, filter, teacherID)
}

func (r *HomeroomRepository) list(ctx context.Context, filter dto.HomeroomFilter, teacherID string) ([]dto.HomeroomItem, error) {
	if filter.TermID == "" {
		return nil, fmt.Errorf("termId is required")
	}

	query := strings.Builder{}
	query.WriteString(`
SELECT
	c.id AS class_id,
	c.name AS class_name,
	t.id AS term_id,
	t.name AS term_name,
	ta.teacher_id AS homeroom_teacher_id,
	tr.full_name AS homeroom_teacher_name
FROM classes c
JOIN terms t ON t.id = $1
LEFT JOIN teacher_assignments ta
	ON ta.class_id = c.id
	AND ta.term_id = t.id
	AND ta.role = 'HOMEROOM'
LEFT JOIN teachers tr ON tr.id = ta.teacher_id
WHERE 1=1`)

	args := []interface{}{filter.TermID}
	if filter.ClassID != "" {
		args = append(args, filter.ClassID)
		fmt.Fprintf(&query, " AND c.id = $%d", len(args))
	}
	if teacherID != "" {
		args = append(args, teacherID)
		fmt.Fprintf(&query, `
AND EXISTS (
	SELECT 1 FROM teacher_assignments ta_scope
	WHERE ta_scope.class_id = c.id
		AND ta_scope.term_id = t.id
		AND ta_scope.teacher_id = $%d
)`, len(args))
	}
	query.WriteString("\nORDER BY c.name ASC")

	var items []dto.HomeroomItem
	if err := r.db.SelectContext(ctx, &items, query.String(), args...); err != nil {
		return nil, fmt.Errorf("list homerooms: %w", err)
	}
	return items, nil
}

// Get fetches a single class homeroom entry.
func (r *HomeroomRepository) Get(ctx context.Context, classID, termID string) (*dto.HomeroomItem, error) {
	const query = `
SELECT
	c.id AS class_id,
	c.name AS class_name,
	t.id AS term_id,
	t.name AS term_name,
	ta.teacher_id AS homeroom_teacher_id,
	tr.full_name AS homeroom_teacher_name
FROM classes c
JOIN terms t ON t.id = $2
LEFT JOIN teacher_assignments ta
	ON ta.class_id = c.id
	AND ta.term_id = t.id
	AND ta.role = 'HOMEROOM'
LEFT JOIN teachers tr ON tr.id = ta.teacher_id
WHERE c.id = $1`

	var item dto.HomeroomItem
	if err := r.db.GetContext(ctx, &item, query, classID, termID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get homeroom: %w", err)
	}
	return &item, nil
}

// HomeroomAssignmentParams holds values required to upsert homeroom assignments.
type HomeroomAssignmentParams struct {
	ClassID   string
	TermID    string
	TeacherID string
	SubjectID string
}

// Upsert ensures a single homeroom assignment for the class-term combination.
func (r *HomeroomRepository) Upsert(ctx context.Context, params HomeroomAssignmentParams) (prevTeacherID *string, err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin homeroom transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var current struct {
		ID        string `db:"id"`
		TeacherID string `db:"teacher_id"`
	}
	const selectQuery = `SELECT id, teacher_id FROM teacher_assignments WHERE class_id = $1 AND term_id = $2 AND role = 'HOMEROOM' FOR UPDATE`
	if err = tx.GetContext(ctx, &current, selectQuery, params.ClassID, params.TermID); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("lock homeroom assignment: %w", err)
		}
		now := time.Now().UTC()
		const insertQuery = `INSERT INTO teacher_assignments (id, teacher_id, class_id, subject_id, term_id, role, created_at)
VALUES ($1, $2, $3, $4, $5, 'HOMEROOM', $6)`
		if _, err = tx.ExecContext(ctx, insertQuery, uuid.NewString(), params.TeacherID, params.ClassID, params.SubjectID, params.TermID, now); err != nil {
			return nil, fmt.Errorf("insert homeroom assignment: %w", err)
		}
	} else {
		prevTeacherID = &current.TeacherID
		const updateQuery = `UPDATE teacher_assignments SET teacher_id = $1, subject_id = $2, role = 'HOMEROOM' WHERE id = $3`
		if _, err = tx.ExecContext(ctx, updateQuery, params.TeacherID, params.SubjectID, current.ID); err != nil {
			return nil, fmt.Errorf("update homeroom assignment: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit homeroom assignment: %w", err)
	}
	return prevTeacherID, nil
}
