package models

import "time"

// BehaviorNoteType represents the nature of a note.
type BehaviorNoteType string

const (
	BehaviorNotePositive BehaviorNoteType = "+"
	BehaviorNoteNegative BehaviorNoteType = "-"
	BehaviorNoteNeutral  BehaviorNoteType = "0"
)

// BehaviorNote captures behavioural information for a student.
type BehaviorNote struct {
	ID          string           `db:"id" json:"id"`
	StudentID   string           `db:"student_id" json:"student_id"`
	NoteDate    time.Time        `db:"date" json:"date"`
	NoteType    BehaviorNoteType `db:"note_type" json:"note_type"`
	Points      int              `db:"points" json:"points"`
	Description string           `db:"description" json:"description"`
	CreatedBy   string           `db:"created_by" json:"created_by"`
	CreatedAt   time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time        `db:"updated_at" json:"updated_at"`
}

// BehaviorNoteFilter allows listing notes.
type BehaviorNoteFilter struct {
	StudentID string
	DateFrom  *time.Time
	DateTo    *time.Time
	NoteTypes []BehaviorNoteType
	Page      int
	PageSize  int
}

// BehaviorSummary aggregates point information for a student.
type BehaviorSummary struct {
	StudentID     string     `json:"student_id"`
	TotalPoints   int        `json:"total_points"`
	PositiveCount int        `json:"positive_count"`
	NegativeCount int        `json:"negative_count"`
	NeutralCount  int        `json:"neutral_count"`
	LastUpdatedAt *time.Time `json:"last_updated_at,omitempty"`
}
