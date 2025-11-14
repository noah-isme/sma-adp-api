package dto

import (
	"time"
)

// AttendanceSummaryRequest captures query parameters for /attendance.
type AttendanceSummaryRequest struct {
	TermID    string
	ClassID   string
	StudentID string
	FromDate  *time.Time
	ToDate    *time.Time
}

// AttendanceDailyRequest captures filters for /attendance/daily alias.
type AttendanceDailyRequest struct {
	TermID    string
	ClassID   string
	StudentID string
	Status    *string
	DateFrom  *time.Time
	DateTo    *time.Time
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

// AttendanceSummaryResponse represents the /attendance payload.
type AttendanceSummaryResponse struct {
	Scope      AttendanceSummaryScope     `json:"scope"`
	Summary    AttendanceSummaryStats     `json:"summary"`
	PerStudent []AttendanceSummaryStudent `json:"perStudent"`
}

// AttendanceSummaryScope echoes the applied filters.
type AttendanceSummaryScope struct {
	TermID    string  `json:"termId"`
	ClassID   *string `json:"classId,omitempty"`
	StudentID *string `json:"studentId,omitempty"`
}

// AttendanceSummaryStats aggregates status counts.
type AttendanceSummaryStats struct {
	TotalDays      int     `json:"totalDays"`
	Present        int     `json:"present"`
	Sick           int     `json:"sick"`
	Excused        int     `json:"excused"`
	Absent         int     `json:"absent"`
	AttendanceRate float64 `json:"attendanceRate"`
}

// AttendanceSummaryStudent represents per-student breakdown.
type AttendanceSummaryStudent struct {
	StudentID      string  `json:"studentId"`
	StudentName    string  `json:"studentName"`
	ClassID        string  `json:"classId"`
	Present        int     `json:"present"`
	Sick           int     `json:"sick"`
	Excused        int     `json:"excused"`
	Absent         int     `json:"absent"`
	AttendanceRate float64 `json:"attendanceRate"`
}
