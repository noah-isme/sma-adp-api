package models

import "time"

// TermType represents the type of academic term (e.g. semester, trimester).
type TermType string

const (
	TermTypeSemester  TermType = "SEMESTER"
	TermTypeTrimester TermType = "TRIMESTER"
	TermTypeQuarter   TermType = "QUARTER"
)

// Term models an academic term within the institution calendar.
type Term struct {
	ID           string    `db:"id" json:"id"`
	Name         string    `db:"name" json:"name"`
	Type         TermType  `db:"type" json:"type"`
	AcademicYear string    `db:"academic_year" json:"academic_year"`
	StartDate    time.Time `db:"start_date" json:"start_date"`
	EndDate      time.Time `db:"end_date" json:"end_date"`
	IsActive     bool      `db:"is_active" json:"is_active"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// TermFilter defines filters supported by list endpoints.
type TermFilter struct {
	AcademicYear string
	Type         TermType
	IsActive     *bool
	Page         int
	PageSize     int
	SortBy       string
	SortOrder    string
}
