package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// ScheduleRepository provides persistence for schedules.
type ScheduleRepository struct {
	db *sqlx.DB
}

// NewScheduleRepository creates a new schedule repository.
func NewScheduleRepository(db *sqlx.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

// List returns schedules with optional filtering and pagination.
func (r *ScheduleRepository) List(ctx context.Context, filter models.ScheduleFilter) ([]models.Schedule, int, error) {
	base := "FROM schedules WHERE 1=1"
	var conditions []string
	var args []interface{}

	if filter.TermID != "" {
		conditions = append(conditions, fmt.Sprintf("term_id = $%d", len(args)+1))
		args = append(args, filter.TermID)
	}
	if filter.ClassID != "" {
		conditions = append(conditions, fmt.Sprintf("class_id = $%d", len(args)+1))
		args = append(args, filter.ClassID)
	}
	if filter.TeacherID != "" {
		conditions = append(conditions, fmt.Sprintf("teacher_id = $%d", len(args)+1))
		args = append(args, filter.TeacherID)
	}
	if filter.DayOfWeek != "" {
		conditions = append(conditions, fmt.Sprintf("day_of_week = $%d", len(args)+1))
		args = append(args, filter.DayOfWeek)
	}
	if filter.TimeSlot != "" {
		conditions = append(conditions, fmt.Sprintf("time_slot = $%d", len(args)+1))
		args = append(args, filter.TimeSlot)
	}
	if filter.Room != "" {
		conditions = append(conditions, fmt.Sprintf("room = $%d", len(args)+1))
		args = append(args, filter.Room)
	}

	if len(conditions) > 0 {
		base += " AND " + strings.Join(conditions, " AND ")
	}

	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "day_of_week"
	}
	allowedSorts := map[string]bool{
		"day_of_week": true,
		"time_slot":   true,
		"room":        true,
		"created_at":  true,
	}
	if !allowedSorts[sortBy] {
		sortBy = "day_of_week"
	}
	order := strings.ToUpper(filter.SortOrder)
	if order != "ASC" && order != "DESC" {
		order = "ASC"
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

	query := fmt.Sprintf("SELECT id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room, created_at, updated_at %s ORDER BY %s %s LIMIT %d OFFSET %d", base, sortBy, order, size, offset)
	var schedules []models.Schedule
	if err := r.db.SelectContext(ctx, &schedules, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list schedules: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s", base)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count schedules: %w", err)
	}

	return schedules, total, nil
}

// FindByID loads a schedule by id.
func (r *ScheduleRepository) FindByID(ctx context.Context, id string) (*models.Schedule, error) {
	const query = `SELECT id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room, created_at, updated_at FROM schedules WHERE id = $1`
	var sched models.Schedule
	if err := r.db.GetContext(ctx, &sched, query, id); err != nil {
		return nil, err
	}
	return &sched, nil
}

// FindConflicts returns schedules that overlap on term/day/time slot for validation.
func (r *ScheduleRepository) FindConflicts(ctx context.Context, termID, dayOfWeek, timeSlot string) ([]models.Schedule, error) {
	const query = `SELECT id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room, created_at, updated_at FROM schedules WHERE term_id = $1 AND day_of_week = $2 AND time_slot = $3`
	var schedules []models.Schedule
	if err := r.db.SelectContext(ctx, &schedules, query, termID, dayOfWeek, timeSlot); err != nil {
		return nil, fmt.Errorf("find schedule conflicts: %w", err)
	}
	return schedules, nil
}

// ListByClass returns schedules for a class ordered by day/time.
func (r *ScheduleRepository) ListByClass(ctx context.Context, classID string) ([]models.Schedule, error) {
	const query = `SELECT id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room, created_at, updated_at FROM schedules WHERE class_id = $1 ORDER BY day_of_week ASC, time_slot ASC`
	var schedules []models.Schedule
	if err := r.db.SelectContext(ctx, &schedules, query, classID); err != nil {
		return nil, fmt.Errorf("list schedules by class: %w", err)
	}
	return schedules, nil
}

// ListByTeacher returns schedules taught by a teacher.
func (r *ScheduleRepository) ListByTeacher(ctx context.Context, teacherID string) ([]models.Schedule, error) {
	const query = `SELECT id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room, created_at, updated_at FROM schedules WHERE teacher_id = $1 ORDER BY day_of_week ASC, time_slot ASC`
	var schedules []models.Schedule
	if err := r.db.SelectContext(ctx, &schedules, query, teacherID); err != nil {
		return nil, fmt.Errorf("list schedules by teacher: %w", err)
	}
	return schedules, nil
}

// Create stores a new schedule record.
func (r *ScheduleRepository) Create(ctx context.Context, schedule *models.Schedule) error {
	if schedule.ID == "" {
		schedule.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if schedule.CreatedAt.IsZero() {
		schedule.CreatedAt = now
	}
	schedule.UpdatedAt = now

	const query = `INSERT INTO schedules (id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room, created_at, updated_at) VALUES (:id, :term_id, :class_id, :subject_id, :teacher_id, :day_of_week, :time_slot, :room, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, schedule); err != nil {
		return fmt.Errorf("create schedule: %w", err)
	}
	return nil
}

// BulkCreate inserts many schedules within a transaction.
func (r *ScheduleRepository) BulkCreate(ctx context.Context, schedules []models.Schedule) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bulk create schedules: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = r.bulkInsertSchedules(ctx, tx, schedules); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit bulk create schedules: %w", err)
	}
	return nil
}

// BulkCreateWithTx inserts schedules using an existing transaction.
func (r *ScheduleRepository) BulkCreateWithTx(ctx context.Context, tx *sqlx.Tx, schedules []models.Schedule) error {
	if tx == nil {
		return fmt.Errorf("nil transaction provided")
	}
	return r.bulkInsertSchedules(ctx, tx, schedules)
}

func (r *ScheduleRepository) bulkInsertSchedules(ctx context.Context, exec sqlx.ExtContext, schedules []models.Schedule) error {
	now := time.Now().UTC()
	for i := range schedules {
		payload := schedules[i]
		if payload.ID == "" {
			payload.ID = uuid.NewString()
		}
		if payload.CreatedAt.IsZero() {
			payload.CreatedAt = now
		}
		payload.UpdatedAt = now

		if _, err := sqlx.NamedExecContext(ctx, exec, `INSERT INTO schedules (id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room, created_at, updated_at) VALUES (:id, :term_id, :class_id, :subject_id, :teacher_id, :day_of_week, :time_slot, :room, :created_at, :updated_at)`, &payload); err != nil {
			return fmt.Errorf("bulk insert schedule: %w", err)
		}
		schedules[i] = payload
	}
	return nil
}

// Update modifies a schedule record.
func (r *ScheduleRepository) Update(ctx context.Context, schedule *models.Schedule) error {
	schedule.UpdatedAt = time.Now().UTC()
	const query = `UPDATE schedules SET term_id = :term_id, class_id = :class_id, subject_id = :subject_id, teacher_id = :teacher_id, day_of_week = :day_of_week, time_slot = :time_slot, room = :room, updated_at = :updated_at WHERE id = :id`
	if _, err := r.db.NamedExecContext(ctx, query, schedule); err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	return nil
}

// Delete removes a schedule by id.
func (r *ScheduleRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM schedules WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete schedule: %w", err)
	}
	return nil
}
