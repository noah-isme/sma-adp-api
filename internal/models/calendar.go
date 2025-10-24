package models

import "time"

// CalendarEvent represents an academic calendar entry.
type CalendarEvent struct {
	ID            string               `db:"id" json:"id"`
	Title         string               `db:"title" json:"title"`
	Description   string               `db:"description" json:"description"`
	EventType     string               `db:"event_type" json:"event_type"`
	StartDate     time.Time            `db:"start_date" json:"start_date"`
	EndDate       time.Time            `db:"end_date" json:"end_date"`
	StartTime     *time.Time           `db:"start_time" json:"start_time,omitempty"`
	EndTime       *time.Time           `db:"end_time" json:"end_time,omitempty"`
	Audience      AnnouncementAudience `db:"audience" json:"audience"`
	TargetClassID *string              `db:"target_class_id" json:"target_class_id,omitempty"`
	Location      *string              `db:"location" json:"location,omitempty"`
	CreatedBy     string               `db:"created_by" json:"created_by"`
	CreatedAt     time.Time            `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time            `db:"updated_at" json:"updated_at"`
}

// CalendarFilter narrows down events.
type CalendarFilter struct {
	StartDate *time.Time
	EndDate   *time.Time
	Audience  []AnnouncementAudience
	ClassIDs  []string
	Page      int
	PageSize  int
}
