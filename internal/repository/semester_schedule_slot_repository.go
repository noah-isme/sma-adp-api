package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// SemesterScheduleSlotRepository manages slots for semester schedules.
type SemesterScheduleSlotRepository struct {
	db *sqlx.DB
}

// NewSemesterScheduleSlotRepository builds repository.
func NewSemesterScheduleSlotRepository(db *sqlx.DB) *SemesterScheduleSlotRepository {
	return &SemesterScheduleSlotRepository{db: db}
}

func (r *SemesterScheduleSlotRepository) exec(exec sqlx.ExtContext) sqlx.ExtContext {
	if exec != nil {
		return exec
	}
	return r.db
}

// UpsertBatch inserts or updates slots for a semester schedule.
func (r *SemesterScheduleSlotRepository) UpsertBatch(ctx context.Context, exec sqlx.ExtContext, slots []models.SemesterScheduleSlot) error {
	if len(slots) == 0 {
		return nil
	}
	target := r.exec(exec)
	now := time.Now().UTC()

	const query = `
INSERT INTO semester_schedule_slots (id, semester_schedule_id, day_of_week, time_slot, subject_id, teacher_id, room, created_at)
VALUES (:id, :semester_schedule_id, :day_of_week, :time_slot, :subject_id, :teacher_id, :room, :created_at)
ON CONFLICT (semester_schedule_id, day_of_week, time_slot) DO UPDATE
SET subject_id = EXCLUDED.subject_id,
    teacher_id = EXCLUDED.teacher_id,
    room = EXCLUDED.room`

	for i := range slots {
		slot := &slots[i]
		if slot.ID == "" {
			slot.ID = uuid.NewString()
		}
		if slot.CreatedAt.IsZero() {
			slot.CreatedAt = now
		}
		if _, err := sqlx.NamedExecContext(ctx, target, query, slot); err != nil {
			return fmt.Errorf("upsert semester schedule slot: %w", err)
		}
	}
	return nil
}

// ListBySchedule returns slots ordered by day/time for a schedule.
func (r *SemesterScheduleSlotRepository) ListBySchedule(ctx context.Context, scheduleID string) ([]models.SemesterScheduleSlot, error) {
	const query = `SELECT id, semester_schedule_id, day_of_week, time_slot, subject_id, teacher_id, room, created_at
FROM semester_schedule_slots WHERE semester_schedule_id = $1 ORDER BY day_of_week ASC, time_slot ASC`
	var slots []models.SemesterScheduleSlot
	if err := r.db.SelectContext(ctx, &slots, query, scheduleID); err != nil {
		return nil, fmt.Errorf("list semester schedule slots: %w", err)
	}
	return slots, nil
}
