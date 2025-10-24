package models

import "time"

// Student represents a learner registered in the institution.
type Student struct {
	ID        string    `db:"id" json:"id"`
	NIS       string    `db:"nis" json:"nis"`
	FullName  string    `db:"full_name" json:"full_name"`
	Gender    string    `db:"gender" json:"gender"`
	BirthDate time.Time `db:"birth_date" json:"birth_date"`
	Address   string    `db:"address" json:"address"`
	Phone     string    `db:"phone" json:"phone"`
	Active    bool      `db:"active" json:"active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// StudentFilter encapsulates allowed search parameters for listing students.
type StudentFilter struct {
	Search    string
	ClassID   string
	Active    *bool
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

// StudentDetail contains student information with enrollment context.
type StudentDetail struct {
	Student
	CurrentClassID   *string    `db:"current_class_id" json:"current_class_id,omitempty"`
	CurrentClassName *string    `db:"current_class_name" json:"current_class_name,omitempty"`
	CurrentTermID    *string    `db:"current_term_id" json:"current_term_id,omitempty"`
	JoinedAt         *time.Time `db:"joined_at" json:"joined_at,omitempty"`
}
