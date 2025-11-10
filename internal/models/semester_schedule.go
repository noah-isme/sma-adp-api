package models

import (
	"time"

	"github.com/jmoiron/sqlx/types"
)

// SemesterScheduleStatus represents lifecycle phases for generated schedules.
type SemesterScheduleStatus string

const (
	SemesterScheduleStatusDraft     SemesterScheduleStatus = "DRAFT"
	SemesterScheduleStatusPublished SemesterScheduleStatus = "PUBLISHED"
	SemesterScheduleStatusArchived  SemesterScheduleStatus = "ARCHIVED"
)

// SemesterSchedule captures a versioned timetable proposal for a class-term pair.
type SemesterSchedule struct {
	ID        string                 `db:"id" json:"id"`
	TermID    string                 `db:"term_id" json:"term_id"`
	ClassID   string                 `db:"class_id" json:"class_id"`
	Version   int                    `db:"version" json:"version"`
	Status    SemesterScheduleStatus `db:"status" json:"status"`
	Meta      types.JSONText         `db:"meta" json:"meta"`
	CreatedAt time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt time.Time              `db:"updated_at" json:"updated_at"`
}

// SemesterScheduleSlot is a concrete slot inside a semester schedule.
type SemesterScheduleSlot struct {
	ID                 string    `db:"id" json:"id"`
	SemesterScheduleID string    `db:"semester_schedule_id" json:"semester_schedule_id"`
	DayOfWeek          int       `db:"day_of_week" json:"day_of_week"`
	TimeSlot           int       `db:"time_slot" json:"time_slot"`
	SubjectID          string    `db:"subject_id" json:"subject_id"`
	TeacherID          string    `db:"teacher_id" json:"teacher_id"`
	Room               *string   `db:"room" json:"room,omitempty"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
}

// SemesterScheduleSummary aggregates versions available for a term/class pair.
type SemesterScheduleSummary struct {
	TermID    string                 `json:"term_id"`
	ClassID   string                 `json:"class_id"`
	ActiveID  *string                `json:"active_id,omitempty"`
	Versions  []SemesterScheduleMeta `json:"versions"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// SemesterScheduleMeta represents lightweight metadata for list views.
type SemesterScheduleMeta struct {
	ID        string                 `json:"id"`
	Version   int                    `json:"version"`
	Status    SemesterScheduleStatus `json:"status"`
	Score     float64                `json:"score"`
	CreatedAt time.Time              `json:"created_at"`
}
