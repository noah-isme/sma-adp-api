package dto

// AdminDashboardResponse captures the aggregated admin dashboard payload.
type AdminDashboardResponse struct {
	TermID     string                   `json:"termId"`
	Attendance AdminAttendanceSection   `json:"attendance"`
	Grades     AdminGradesSection       `json:"grades"`
	Behavior   AdminBehaviorSection     `json:"behavior"`
	Ops        AdminOperationsHighlight `json:"ops"`
}

// AdminAttendanceSection summarises attendance for admin dashboard.
type AdminAttendanceSection struct {
	OverallRate float64             `json:"overallRate"`
	ByClass     []AttendanceByClass `json:"byClass"`
}

// AttendanceByClass denotes per-class attendance rate.
type AttendanceByClass struct {
	ClassID string  `json:"classId"`
	Rate    float64 `json:"rate"`
}

// AdminGradesSection summarises grade analytics.
type AdminGradesSection struct {
	AverageByClass []ClassAverageGrade    `json:"avgByClass"`
	Distribution   []GradeDistributionBin `json:"distribution"`
}

// ClassAverageGrade contains the aggregated grade per class.
type ClassAverageGrade struct {
	ClassID string  `json:"classId"`
	Average float64 `json:"avg"`
}

// GradeDistributionBin captures grade bucket counts.
type GradeDistributionBin struct {
	Bucket string `json:"bucket"`
	Count  int    `json:"count"`
}

// AdminBehaviorSection highlights positive/negative behavior.
type AdminBehaviorSection struct {
	TopPositive []BehaviorLeaderboardEntry `json:"topPositive"`
	TopNegative []BehaviorLeaderboardEntry `json:"topNegative"`
}

// BehaviorLeaderboardEntry ranks students by points.
type BehaviorLeaderboardEntry struct {
	StudentID string `json:"studentId"`
	Points    int    `json:"points"`
}

// AdminOperationsHighlight gathers operational signals.
type AdminOperationsHighlight struct {
	UpcomingEvents    []OpsEvent `json:"upcomingEvents"`
	OpenAnnouncements int        `json:"openAnnouncements"`
}

// OpsEvent is a simplified calendar event for the dashboard.
type OpsEvent struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

// TeacherDashboardResponse captures personalised teacher dashboard data.
type TeacherDashboardResponse struct {
	TeacherID string                 `json:"teacherId"`
	Today     TeacherScheduleSummary `json:"today"`
	Classes   []TeacherClassSummary  `json:"classes"`
	Alerts    TeacherAlerts          `json:"alerts"`
}

// TeacherScheduleSummary outlines today's schedule.
type TeacherScheduleSummary struct {
	Date      string                `json:"date"`
	Schedules []TeacherScheduleSlot `json:"schedules"`
}

// TeacherScheduleSlot represents one schedule entry.
type TeacherScheduleSlot struct {
	ClassID   string  `json:"classId"`
	SubjectID string  `json:"subjectId"`
	TimeSlot  int     `json:"timeSlot"`
	Room      *string `json:"room"`
}

// TeacherClassSummary aggregates per-class indicators.
type TeacherClassSummary struct {
	ClassID        string  `json:"classId"`
	AttendanceRate float64 `json:"attendanceRate"`
	AverageGrade   float64 `json:"avgGrade"`
}

// TeacherAlerts contains alerting identifiers.
type TeacherAlerts struct {
	LowAttendanceClasses []string `json:"lowAttendanceClasses"`
	GradeOutliers        []string `json:"gradeOutliers"`
}
