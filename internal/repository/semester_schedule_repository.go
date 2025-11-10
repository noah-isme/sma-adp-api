package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// SemesterScheduleRepository persists versioned semester timetables.
type SemesterScheduleRepository struct {
	db *sqlx.DB
}

// NewSemesterScheduleRepository constructs repository.
func NewSemesterScheduleRepository(db *sqlx.DB) *SemesterScheduleRepository {
	return &SemesterScheduleRepository{db: db}
}

func (r *SemesterScheduleRepository) exec(exec sqlx.ExtContext) sqlx.ExtContext {
	if exec != nil {
		return exec
	}
	return r.db
}

// CreateVersioned inserts a schedule assigning the next version for the class-term tuple.
func (r *SemesterScheduleRepository) CreateVersioned(ctx context.Context, exec sqlx.ExtContext, schedule *models.SemesterSchedule) error {
	if schedule == nil {
		return fmt.Errorf("schedule payload is nil")
	}
	if schedule.TermID == "" || schedule.ClassID == "" {
		return fmt.Errorf("term_id and class_id are required")
	}
	if schedule.ID == "" {
		schedule.ID = uuid.NewString()
	}
	if schedule.Status == "" {
		schedule.Status = models.SemesterScheduleStatusDraft
	}
	if len(schedule.Meta) == 0 {
		schedule.Meta = types.JSONText(`{}`)
	}
	now := time.Now().UTC()
	if schedule.CreatedAt.IsZero() {
		schedule.CreatedAt = now
	}
	schedule.UpdatedAt = now

	target := r.exec(exec)

	const nextVersionQuery = `SELECT COALESCE(MAX(version), 0) + 1 FROM semester_schedules WHERE term_id = $1 AND class_id = $2`
	if err := sqlx.GetContext(ctx, target, &schedule.Version, nextVersionQuery, schedule.TermID, schedule.ClassID); err != nil {
		return fmt.Errorf("compute next semester schedule version: %w", err)
	}

	const insertQuery = `
INSERT INTO semester_schedules (id, term_id, class_id, version, status, meta, created_at, updated_at)
VALUES (:id, :term_id, :class_id, :version, :status, :meta, :created_at, :updated_at)`
	if _, err := sqlx.NamedExecContext(ctx, target, insertQuery, schedule); err != nil {
		return fmt.Errorf("insert semester schedule: %w", err)
	}
	return nil
}

// ListByTermClass returns all versions for the provided class-term tuple.
func (r *SemesterScheduleRepository) ListByTermClass(ctx context.Context, termID, classID string) ([]models.SemesterSchedule, error) {
	const query = `SELECT id, term_id, class_id, version, status, meta, created_at, updated_at
FROM semester_schedules WHERE term_id = $1 AND class_id = $2 ORDER BY version DESC`
	var schedules []models.SemesterSchedule
	if err := r.db.SelectContext(ctx, &schedules, query, termID, classID); err != nil {
		return nil, fmt.Errorf("list semester schedules: %w", err)
	}
	return schedules, nil
}

// FindByID loads a schedule by its identifier.
func (r *SemesterScheduleRepository) FindByID(ctx context.Context, id string) (*models.SemesterSchedule, error) {
	const query = `SELECT id, term_id, class_id, version, status, meta, created_at, updated_at FROM semester_schedules WHERE id = $1`
	var schedule models.SemesterSchedule
	if err := r.db.GetContext(ctx, &schedule, query, id); err != nil {
		return nil, err
	}
	return &schedule, nil
}

// Delete removes a stored schedule version.
func (r *SemesterScheduleRepository) Delete(ctx context.Context, id string) error {
	const query = `DELETE FROM semester_schedules WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete semester schedule: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("semester schedule rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateStatus updates the status (and optionally meta) of a schedule.
func (r *SemesterScheduleRepository) UpdateStatus(ctx context.Context, exec sqlx.ExtContext, id string, status models.SemesterScheduleStatus, meta types.JSONText) error {
	target := r.exec(exec)
	now := time.Now().UTC()

	var (
		query string
		args  []interface{}
	)
	if len(meta) > 0 {
		query = `UPDATE semester_schedules SET status = $1, meta = $2, updated_at = $3 WHERE id = $4`
		args = []interface{}{status, meta, now, id}
	} else {
		query = `UPDATE semester_schedules SET status = $1, updated_at = $2 WHERE id = $3`
		args = []interface{}{status, now, id}
	}
	result, err := target.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update semester schedule status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("semester schedule status rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
