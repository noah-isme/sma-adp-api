package models

import "time"

// AttendanceStatus represents the status for attendance records.
type AttendanceStatus string

const (
	AttendanceStatusPresent AttendanceStatus = "H"
	AttendanceStatusSick    AttendanceStatus = "S"
	AttendanceStatusExcused AttendanceStatus = "I"
	AttendanceStatusAbsent  AttendanceStatus = "A"
)

// Valid returns true when the status is a supported value.
func (s AttendanceStatus) Valid() bool {
	switch s {
	case AttendanceStatusPresent, AttendanceStatusSick, AttendanceStatusExcused, AttendanceStatusAbsent:
		return true
	default:
		return false
	}
}

// BulkOperationMode controls how bulk writes behave on errors.
type BulkOperationMode string

const (
	BulkModeAtomic         BulkOperationMode = "atomic"
	BulkModePartialOnError BulkOperationMode = "partialOnError"
)

// DailyAttendance represents a single daily attendance row.
type DailyAttendance struct {
	ID           string           `db:"id" json:"id"`
	EnrollmentID string           `db:"enrollment_id" json:"enrollment_id"`
	Date         time.Time        `db:"date" json:"date"`
	Status       AttendanceStatus `db:"status" json:"status"`
	Notes        *string          `db:"notes" json:"notes,omitempty"`
	CreatedAt    time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at" json:"updated_at"`
}

// DailyAttendanceRecord extends the model with student metadata.
type DailyAttendanceRecord struct {
	DailyAttendance
	StudentID   string  `db:"student_id" json:"student_id"`
	StudentName string  `db:"student_name" json:"student_name"`
	ClassID     string  `db:"class_id" json:"class_id"`
	ClassName   *string `db:"class_name" json:"class_name,omitempty"`
	TermID      *string `db:"term_id" json:"term_id,omitempty"`
}

// DailyAttendanceFilter defines query filters.
type DailyAttendanceFilter struct {
	ClassID   string
	TermID    string
	Status    *AttendanceStatus
	DateFrom  *time.Time
	DateTo    *time.Time
	StudentID string
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

// DailyAttendanceReportRow captures report rows for a class/date.
type DailyAttendanceReportRow struct {
	StudentID   string           `db:"student_id" json:"student_id"`
	StudentName string           `db:"student_name" json:"student_name"`
	Status      AttendanceStatus `db:"status" json:"status"`
	Notes       *string          `db:"notes" json:"notes,omitempty"`
}

// DailyAttendanceSummary summarises counts for a student.
type DailyAttendanceSummary struct {
	Present int     `json:"present"`
	Sick    int     `json:"sick"`
	Excused int     `json:"excused"`
	Absent  int     `json:"absent"`
	Total   int     `json:"total"`
	Percent float64 `json:"percent"`
}

// DailyAttendanceHistoryRow captures attendance history entries.
type DailyAttendanceHistoryRow struct {
	Date   time.Time        `db:"date" json:"date"`
	Status AttendanceStatus `db:"status" json:"status"`
	Notes  *string          `db:"notes" json:"notes,omitempty"`
}

// SubjectAttendance represents attendance per subject session.
type SubjectAttendance struct {
	ID           string           `db:"id" json:"id"`
	EnrollmentID string           `db:"enrollment_id" json:"enrollment_id"`
	ScheduleID   string           `db:"schedule_id" json:"schedule_id"`
	Date         time.Time        `db:"date" json:"date"`
	Status       AttendanceStatus `db:"status" json:"status"`
	Notes        *string          `db:"notes" json:"notes,omitempty"`
	CreatedAt    time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at" json:"updated_at"`
}

// SubjectAttendanceRecord extends the session attendance row with metadata.
type SubjectAttendanceRecord struct {
	SubjectAttendance
	StudentID   string  `db:"student_id" json:"student_id"`
	StudentName string  `db:"student_name" json:"student_name"`
	ClassID     string  `db:"class_id" json:"class_id"`
	ClassName   *string `db:"class_name" json:"class_name,omitempty"`
	SubjectID   string  `db:"subject_id" json:"subject_id"`
	SubjectName *string `db:"subject_name" json:"subject_name,omitempty"`
}

// SubjectAttendanceFilter scopes listing queries.
type SubjectAttendanceFilter struct {
	ScheduleID string
	Date       *time.Time
	Status     *AttendanceStatus
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

// SubjectAttendanceReportRow summarises a session.
type SubjectAttendanceReportRow struct {
	EnrollmentID string           `db:"enrollment_id" json:"enrollment_id"`
	StudentID    string           `db:"student_id" json:"student_id"`
	StudentName  string           `db:"student_name" json:"student_name"`
	Status       AttendanceStatus `db:"status" json:"status"`
	Notes        *string          `db:"notes" json:"notes,omitempty"`
}

// AttendanceBulkConflict captures failed bulk operations.
type AttendanceBulkConflict struct {
	EnrollmentID string    `json:"enrollment_id"`
	ScheduleID   *string   `json:"schedule_id,omitempty"`
	Date         time.Time `json:"date"`
	Reason       string    `json:"reason"`
}
