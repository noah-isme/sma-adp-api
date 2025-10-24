package models

import "time"

// EnrollmentStatus represents the lifecycle of an enrollment.
type EnrollmentStatus string

// Possible enrollment statuses.
const (
	EnrollmentStatusActive      EnrollmentStatus = "ACTIVE"
	EnrollmentStatusTransferred EnrollmentStatus = "TRANSFERRED"
	EnrollmentStatusLeft        EnrollmentStatus = "LEFT"
)

// Enrollment captures a student's registration to a class within a term.
type Enrollment struct {
	ID        string           `db:"id" json:"id"`
	StudentID string           `db:"student_id" json:"student_id"`
	ClassID   string           `db:"class_id" json:"class_id"`
	TermID    string           `db:"term_id" json:"term_id"`
	JoinedAt  time.Time        `db:"joined_at" json:"joined_at"`
	LeftAt    *time.Time       `db:"left_at" json:"left_at,omitempty"`
	Status    EnrollmentStatus `db:"status" json:"status"`
}

// EnrollmentDetail enriches Enrollment with student and class info.
type EnrollmentDetail struct {
	Enrollment
	StudentName string `db:"student_name" json:"student_name"`
	StudentNIS  string `db:"student_nis" json:"student_nis"`
	ClassName   string `db:"class_name" json:"class_name"`
	TermName    string `db:"term_name" json:"term_name"`
}

// EnrollmentFilter provides filters for listing enrollments.
type EnrollmentFilter struct {
	StudentID string
	ClassID   string
	TermID    string
	Status    EnrollmentStatus
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}
