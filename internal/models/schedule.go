package models

import "time"

// Schedule represents a scheduled subject for a class within a term.
type Schedule struct {
	ID        string    `db:"id" json:"id"`
	TermID    string    `db:"term_id" json:"term_id"`
	ClassID   string    `db:"class_id" json:"class_id"`
	SubjectID string    `db:"subject_id" json:"subject_id"`
	TeacherID string    `db:"teacher_id" json:"teacher_id"`
	DayOfWeek string    `db:"day_of_week" json:"day_of_week"`
	TimeSlot  string    `db:"time_slot" json:"time_slot"`
	Room      string    `db:"room" json:"room"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ScheduleFilter describes query params for listing schedules.
type ScheduleFilter struct {
	TermID    string
	ClassID   string
	TeacherID string
	DayOfWeek string
	TimeSlot  string
	Room      string
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

// ScheduleConflict describes an existing schedule that causes a conflict.
type ScheduleConflict struct {
	ScheduleID string `json:"schedule_id"`
	TermID     string `json:"term_id"`
	ClassID    string `json:"class_id"`
	SubjectID  string `json:"subject_id"`
	TeacherID  string `json:"teacher_id"`
	DayOfWeek  string `json:"day_of_week"`
	TimeSlot   string `json:"time_slot"`
	Room       string `json:"room"`
	Dimension  string `json:"dimension"`
}

// ScheduleConflictError is returned when a schedule collides with an existing one.
type ScheduleConflictError struct {
	Type     string             `json:"type"`
	Message  string             `json:"message"`
	Conflict ScheduleConflict   `json:"conflict"`
	Errors   []ScheduleConflict `json:"errors,omitempty"`
}

// Error implements the error interface for conflict errors.
func (e *ScheduleConflictError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Message
}
