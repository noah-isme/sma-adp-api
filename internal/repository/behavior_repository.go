package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// BehaviorRepository manages persistence for behaviour notes.
type BehaviorRepository struct {
	db *sqlx.DB
}

// NewBehaviorRepository constructs a new repository.
func NewBehaviorRepository(db *sqlx.DB) *BehaviorRepository {
	return &BehaviorRepository{db: db}
}

// List returns behaviour notes per provided filter.
func (r *BehaviorRepository) List(ctx context.Context, filter models.BehaviorNoteFilter) ([]models.BehaviorNote, int, error) {
	base := "FROM behavior_notes"
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.StudentID != "" {
		where = append(where, fmt.Sprintf("student_id = $%d", len(args)+1))
		args = append(args, filter.StudentID)
	}
	if filter.DateFrom != nil {
		where = append(where, fmt.Sprintf("date >= $%d", len(args)+1))
		args = append(args, *filter.DateFrom)
	}
	if filter.DateTo != nil {
		where = append(where, fmt.Sprintf("date <= $%d", len(args)+1))
		args = append(args, *filter.DateTo)
	}
	if len(filter.NoteTypes) > 0 {
		placeholders := fmt.Sprintf("$%d", len(args)+1)
		values := make([]string, len(filter.NoteTypes))
		for i, t := range filter.NoteTypes {
			values[i] = string(t)
		}
		args = append(args, pq.Array(values))
		where = append(where, fmt.Sprintf("note_type = ANY(%s)", placeholders))
	}
	whereClause := strings.Join(where, " AND ")
	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 || size > 200 {
		size = 50
	}
	offset := (page - 1) * size
	query := fmt.Sprintf(`SELECT id, student_id, date, note_type, points, description, created_by, created_at, updated_at
%s WHERE %s ORDER BY date DESC, created_at DESC LIMIT %d OFFSET %d`, base, whereClause, size, offset)
	var notes []models.BehaviorNote
	if err := r.db.SelectContext(ctx, &notes, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list behavior notes: %w", err)
	}
	countQuery := fmt.Sprintf("SELECT COUNT(*) %s WHERE %s", base, whereClause)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count behavior notes: %w", err)
	}
	return notes, total, nil
}

// Create inserts a new behaviour note.
func (r *BehaviorRepository) Create(ctx context.Context, note *models.BehaviorNote) error {
	if note.ID == "" {
		note.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if note.CreatedAt.IsZero() {
		note.CreatedAt = now
	}
	note.UpdatedAt = now
	query := `INSERT INTO behavior_notes (id, student_id, date, note_type, points, description, created_by, created_at, updated_at)
VALUES (:id, :student_id, :date, :note_type, :points, :description, :created_by, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, note); err != nil {
		return fmt.Errorf("create behavior note: %w", err)
	}
	return nil
}

// Update modifies an existing behaviour note.
func (r *BehaviorRepository) Update(ctx context.Context, note *models.BehaviorNote) error {
	note.UpdatedAt = time.Now().UTC()
	query := `UPDATE behavior_notes SET student_id = :student_id, date = :date, note_type = :note_type, points = :points, description = :description, updated_at = :updated_at
WHERE id = :id`
	if _, err := r.db.NamedExecContext(ctx, query, note); err != nil {
		return fmt.Errorf("update behavior note: %w", err)
	}
	return nil
}

// Delete removes a behavior note.
func (r *BehaviorRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM behavior_notes WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete behavior note: %w", err)
	}
	return nil
}

// Summary aggregates behaviour metrics for a student.
func (r *BehaviorRepository) Summary(ctx context.Context, studentID string) (*models.BehaviorSummary, error) {
	query := `SELECT COALESCE(SUM(points),0) AS total_points,
        COALESCE(SUM(CASE WHEN note_type = '+' THEN 1 ELSE 0 END),0) AS positive_count,
        COALESCE(SUM(CASE WHEN note_type = '-' THEN 1 ELSE 0 END),0) AS negative_count,
        COALESCE(SUM(CASE WHEN note_type = '0' THEN 1 ELSE 0 END),0) AS neutral_count,
        MAX(updated_at) AS last_updated_at
FROM behavior_notes
WHERE student_id = $1`
	var summary models.BehaviorSummary
	summary.StudentID = studentID
	var lastUpdated sql.NullTime
	if err := r.db.QueryRowxContext(ctx, query, studentID).Scan(&summary.TotalPoints, &summary.PositiveCount, &summary.NegativeCount, &summary.NeutralCount, &lastUpdated); err != nil {
		return nil, fmt.Errorf("behavior summary: %w", err)
	}
	if lastUpdated.Valid {
		summary.LastUpdatedAt = &lastUpdated.Time
	}
	return &summary, nil
}
