package models

import "time"

// Teacher represents an instructor record.
type Teacher struct {
	ID        string    `db:"id" json:"id"`
	NIP       *string   `db:"nip" json:"nip,omitempty"`
	Email     string    `db:"email" json:"email"`
	FullName  string    `db:"full_name" json:"full_name"`
	Phone     *string   `db:"phone" json:"phone,omitempty"`
	Expertise *string   `db:"expertise" json:"expertise,omitempty"`
	Active    bool      `db:"active" json:"active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// TeacherFilter captures filtering options for listing teachers.
type TeacherFilter struct {
	Search    string
	Active    *bool
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}
