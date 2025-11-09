package models

import "time"

// TeacherAssignment links a teacher to a class/subject/term tuple.
type TeacherAssignment struct {
	ID        string    `db:"id" json:"id"`
	TeacherID string    `db:"teacher_id" json:"teacher_id"`
	ClassID   string    `db:"class_id" json:"class_id"`
	SubjectID string    `db:"subject_id" json:"subject_id"`
	TermID    string    `db:"term_id" json:"term_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// TeacherAssignmentDetail enriches assignments with descriptive fields.
type TeacherAssignmentDetail struct {
	TeacherAssignment
	ClassName   string  `db:"class_name" json:"class_name"`
	SubjectName string  `db:"subject_name" json:"subject_name"`
	TermName    string  `db:"term_name" json:"term_name"`
	TeacherName *string `db:"teacher_name" json:"teacher_name,omitempty"`
}
