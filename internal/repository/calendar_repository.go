package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// CalendarRepository persists calendar events.
type CalendarRepository struct {
	db *sqlx.DB
}

// NewCalendarRepository constructs a calendar repository.
func NewCalendarRepository(db *sqlx.DB) *CalendarRepository {
	return &CalendarRepository{db: db}
}

// List returns calendar events matching filters.
func (r *CalendarRepository) List(ctx context.Context, filter models.CalendarFilter) ([]models.CalendarEvent, int, error) {
	base := "FROM calendar_events"
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.StartDate != nil {
		where = append(where, fmt.Sprintf("end_date >= $%d", len(args)+1))
		args = append(args, *filter.StartDate)
	}
	if filter.EndDate != nil {
		where = append(where, fmt.Sprintf("start_date <= $%d", len(args)+1))
		args = append(args, *filter.EndDate)
	}
	if len(filter.Audience) > 0 {
		audiences := make([]string, len(filter.Audience))
		for i, a := range filter.Audience {
			audiences[i] = string(a)
		}
		where = append(where, fmt.Sprintf("audience = ANY($%d)", len(args)+1))
		args = append(args, pq.Array(audiences))
	}
	if len(filter.ClassIDs) > 0 {
		where = append(where, fmt.Sprintf("(audience <> 'CLASS' OR target_class_id = ANY($%d))", len(args)+1))
		args = append(args, pq.Array(filter.ClassIDs))
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

	query := fmt.Sprintf(`SELECT id, title, description, event_type, start_date, end_date, start_time, end_time, audience, target_class_id, location, created_by, created_at, updated_at
%s WHERE %s ORDER BY start_date ASC, start_time ASC NULLS FIRST LIMIT %d OFFSET %d`, base, whereClause, size, offset)
	var events []models.CalendarEvent
	if err := r.db.SelectContext(ctx, &events, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list calendar events: %w", err)
	}
	countQuery := fmt.Sprintf("SELECT COUNT(*) %s WHERE %s", base, whereClause)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count calendar events: %w", err)
	}
	return events, total, nil
}

// GetByID fetches a calendar event.
func (r *CalendarRepository) GetByID(ctx context.Context, id string) (*models.CalendarEvent, error) {
	const query = `SELECT id, title, description, event_type, start_date, end_date, start_time, end_time, audience, target_class_id, location, created_by, created_at, updated_at
FROM calendar_events WHERE id = $1`
	var event models.CalendarEvent
	if err := r.db.GetContext(ctx, &event, query, id); err != nil {
		return nil, err
	}
	return &event, nil
}

// Create inserts a calendar event.
func (r *CalendarRepository) Create(ctx context.Context, event *models.CalendarEvent) error {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	event.UpdatedAt = now
	query := `INSERT INTO calendar_events (id, title, description, event_type, start_date, end_date, start_time, end_time, audience, target_class_id, location, created_by, created_at, updated_at)
VALUES (:id, :title, :description, :event_type, :start_date, :end_date, :start_time, :end_time, :audience, :target_class_id, :location, :created_by, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, event); err != nil {
		return fmt.Errorf("create calendar event: %w", err)
	}
	return nil
}

// Update modifies an event.
func (r *CalendarRepository) Update(ctx context.Context, event *models.CalendarEvent) error {
	event.UpdatedAt = time.Now().UTC()
	query := `UPDATE calendar_events SET title = :title, description = :description, event_type = :event_type, start_date = :start_date,
end_date = :end_date, start_time = :start_time, end_time = :end_time, audience = :audience, target_class_id = :target_class_id, location = :location, updated_at = :updated_at
WHERE id = :id`
	if _, err := r.db.NamedExecContext(ctx, query, event); err != nil {
		return fmt.Errorf("update calendar event: %w", err)
	}
	return nil
}

// Delete removes an event.
func (r *CalendarRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM calendar_events WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete calendar event: %w", err)
	}
	return nil
}
