package models

import "time"

// Class represents an academic class or section.
type Class struct {
	ID                string    `db:"id" json:"id"`
	Name              string    `db:"name" json:"name"`
	Grade             string    `db:"grade" json:"grade"`
	Track             string    `db:"track" json:"track"`
	HomeroomTeacherID *string   `db:"homeroom_teacher_id" json:"homeroom_teacher_id,omitempty"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
}

// ClassDetail extends Class with optional homeroom teacher information.
type ClassDetail struct {
	Class
	HomeroomTeacherName *string `db:"homeroom_teacher_name" json:"homeroom_teacher_name,omitempty"`
}

// ClassFilter defines filter criteria for listing classes.
type ClassFilter struct {
	Grade     string
	Track     string
	Search    string
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

// ClassSubject represents the mapping between a class and a subject with an optional teacher.
type ClassSubject struct {
	ID        string    `db:"id" json:"id"`
	ClassID   string    `db:"class_id" json:"class_id"`
	SubjectID string    `db:"subject_id" json:"subject_id"`
	TeacherID *string   `db:"teacher_id" json:"teacher_id,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ClassSubjectAssignment is a view that includes subject and teacher info for responses.
type ClassSubjectAssignment struct {
	ClassSubject
	SubjectName string  `db:"subject_name" json:"subject_name"`
	SubjectCode string  `db:"subject_code" json:"subject_code"`
	TeacherName *string `db:"teacher_name" json:"teacher_name,omitempty"`
}
