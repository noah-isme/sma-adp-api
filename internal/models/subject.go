package models

import "time"

// Subject represents an academic subject.
type Subject struct {
	ID           string    `db:"id" json:"id"`
	Code         string    `db:"code" json:"code"`
	Name         string    `db:"name" json:"name"`
	Track        string    `db:"track" json:"track"`
	SubjectGroup string    `db:"subject_group" json:"subject_group"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// SubjectFilter captures supported filters for listing subjects.
type SubjectFilter struct {
	Track     string
	Group     string
	Search    string
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}
