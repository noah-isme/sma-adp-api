package models

import "time"

// AnalyticsAttendanceFilter scopes attendance analytics queries.
type AnalyticsAttendanceFilter struct {
	TermID   string
	ClassID  string
	DateFrom *time.Time
	DateTo   *time.Time
}

// AnalyticsAttendanceSummary represents aggregated attendance metrics.
type AnalyticsAttendanceSummary struct {
	TermID       string     `db:"term_id" json:"term_id"`
	ClassID      string     `db:"class_id" json:"class_id"`
	PresentCount int        `db:"present_count" json:"present_count"`
	AbsentCount  int        `db:"absent_count" json:"absent_count"`
	Percentage   float64    `db:"percentage" json:"percentage"`
	UpdatedAt    *time.Time `db:"updated_at" json:"updated_at,omitempty"`
}

// AnalyticsGradeFilter scopes grade analytics queries.
type AnalyticsGradeFilter struct {
	TermID    string
	ClassID   string
	SubjectID string
}

// AnalyticsGradeSummary represents aggregated grade metrics per class/subject.
type AnalyticsGradeSummary struct {
	TermID       string               `db:"term_id" json:"term_id"`
	ClassID      string               `db:"class_id" json:"class_id"`
	SubjectID    string               `db:"subject_id" json:"subject_id"`
	AverageScore float64              `db:"avg_score" json:"average_score"`
	MedianScore  float64              `db:"median_score" json:"median_score"`
	Rank         []AnalyticsGradeRank `json:"rank"`
	UpdatedAt    *time.Time           `db:"updated_at" json:"updated_at,omitempty"`
}

// AnalyticsGradeRank captures rank ordering with scores.
type AnalyticsGradeRank struct {
	StudentID string  `json:"student_id"`
	Score     float64 `json:"score"`
	Rank      int     `json:"rank"`
}

// AnalyticsBehaviorFilter scopes behaviour analytics queries.
type AnalyticsBehaviorFilter struct {
	TermID    string
	StudentID string
	ClassID   string
	DateFrom  *time.Time
	DateTo    *time.Time
}

// AnalyticsBehaviorSummary provides aggregated behaviour statistics.
type AnalyticsBehaviorSummary struct {
	TermID        string     `db:"term_id" json:"term_id"`
	StudentID     string     `db:"student_id" json:"student_id"`
	TotalPositive int        `db:"total_positive" json:"total_positive"`
	TotalNegative int        `db:"total_negative" json:"total_negative"`
	Balance       int        `db:"balance" json:"balance"`
	UpdatedAt     *time.Time `db:"updated_at" json:"updated_at,omitempty"`
}

// AnalyticsSystemMetrics represents system level analytics captured from instrumentation.
type AnalyticsSystemMetrics struct {
	CacheHitRatio            float64   `json:"cache_hit_ratio"`
	CacheHits                uint64    `json:"cache_hits"`
	CacheMisses              uint64    `json:"cache_misses"`
	RequestsTotal            uint64    `json:"requests_total"`
	AverageRequestDurationMs float64   `json:"average_request_duration_ms"`
	DBQueryCount             uint64    `json:"db_query_count"`
	AverageDBQueryDurationMs float64   `json:"average_db_query_duration_ms"`
	Goroutines               int       `json:"goroutines"`
	GeneratedAt              time.Time `json:"generated_at"`
}

// AnalyticsClassStudent is a ranked student row in a class analytics response.
// All response fields intentionally use snake_case to match the API contract.
type AnalyticsClassStudent struct {
	StudentID            string  `db:"student_id" json:"student_id"`
	StudentName          string  `db:"student_name" json:"student_name"`
	NIS                  string  `db:"nis" json:"nis"`
	GPA                  float64 `db:"gpa" json:"gpa"`
	AttendancePercentage float64 `db:"attendance_percentage" json:"attendance_percentage"`
	Rank                 int     `db:"rank" json:"rank"`
}

// AnalyticsClassSubject is a subject aggregate for a class analytics response.
type AnalyticsClassSubject struct {
	SubjectID     string  `db:"subject_id" json:"subject_id"`
	SubjectName   string  `db:"subject_name" json:"subject_name"`
	TotalStudents int     `db:"total_students" json:"total_students"`
	AverageGrade  float64 `db:"avg_grade" json:"average_grade"`
	PassRate      float64 `db:"pass_rate" json:"pass_rate"`
}

// AnalyticsClassAnalytics contains the class-level drilldown and its detail rows.
type AnalyticsClassAnalytics struct {
	ClassID            string                  `db:"class_id" json:"class_id"`
	ClassName          string                  `db:"class_name" json:"class_name"`
	Grade              string                  `db:"grade" json:"grade"`
	Track              string                  `db:"track" json:"track"`
	TermID             string                  `db:"term_id" json:"term_id"`
	TermName           string                  `db:"term_name" json:"term_name"`
	TotalStudents      int                     `db:"total_students" json:"total_students"`
	TotalSubjects      int                     `db:"total_subjects" json:"total_subjects"`
	AverageAttendance  float64                 `db:"avg_attendance_rate" json:"average_attendance_rate"`
	AverageGrade       float64                 `db:"avg_grade" json:"average_grade"`
	StudentsPassed     int                     `db:"students_passed" json:"students_passed"`
	StudentsFailed     int                     `db:"students_failed" json:"students_failed"`
	Students           []AnalyticsClassStudent `json:"students"`
	SubjectPerformance []AnalyticsClassSubject `json:"subject_performance"`
}

// AnalyticsStudentSubject is a subject-level row in a student drilldown.
type AnalyticsStudentSubject struct {
	SubjectID   string  `db:"subject_id" json:"subject_id"`
	SubjectName string  `db:"subject_name" json:"subject_name"`
	SubjectCode string  `db:"subject_code" json:"subject_code"`
	FinalGrade  float64 `db:"final_grade" json:"final_grade"`
}

// AnalyticsStudentPerformance contains grade metrics for one student.
type AnalyticsStudentPerformance struct {
	GPA              float64 `json:"gpa"`
	Rank             int     `json:"rank"`
	TotalRank        int     `json:"total_rank"`
	SubjectsEnrolled int     `json:"subjects_enrolled"`
	SubjectsPassed   int     `json:"subjects_passed"`
	SubjectsFailed   int     `json:"subjects_failed"`
	LowestGrade      float64 `json:"lowest_grade"`
	HighestGrade     float64 `json:"highest_grade"`
}

// AnalyticsStudentAttendance contains attendance metrics for one student.
type AnalyticsStudentAttendance struct {
	Percentage float64 `json:"percentage"`
	TotalDays  int     `json:"total_days"`
	Present    int     `json:"present"`
	Sick       int     `json:"sick"`
	Permission int     `json:"permission"`
	Absent     int     `json:"absent"`
}

// AnalyticsStudentBehavior contains behaviour metrics for one student.
type AnalyticsStudentBehavior struct {
	TotalPoints   int `json:"total_points"`
	PositiveNotes int `json:"positive_notes"`
	NegativeNotes int `json:"negative_notes"`
	NeutralNotes  int `json:"neutral_notes"`
}

// AnalyticsStudentAnalytics contains the student-level drilldown.
type AnalyticsStudentAnalytics struct {
	StudentID        string                      `db:"student_id" json:"student_id"`
	NIS              string                      `db:"nis" json:"nis"`
	StudentName      string                      `db:"full_name" json:"student_name"`
	ClassID          string                      `db:"class_id" json:"class_id"`
	ClassName        string                      `db:"class_name" json:"class_name"`
	TermID           string                      `db:"term_id" json:"term_id"`
	TermName         string                      `db:"term_name" json:"term_name"`
	Performance      AnalyticsStudentPerformance `json:"performance"`
	Attendance       AnalyticsStudentAttendance  `json:"attendance"`
	Behavior         AnalyticsStudentBehavior    `json:"behavior"`
	SubjectBreakdown []AnalyticsStudentSubject   `json:"subject_breakdown"`
}

// AnalyticsSubjectSummary contains overall statistics for one subject scope.
type AnalyticsSubjectSummary struct {
	TotalStudents int     `json:"total_students"`
	AverageGrade  float64 `json:"average_grade"`
	GradeStddev   float64 `json:"grade_stddev"`
	MinGrade      float64 `json:"min_grade"`
	MaxGrade      float64 `json:"max_grade"`
	PassedCount   int     `json:"passed_count"`
	FailedCount   int     `json:"failed_count"`
	PassRate      float64 `json:"pass_rate"`
}

// AnalyticsSubjectClass is a subject aggregate grouped by class.
type AnalyticsSubjectClass struct {
	ClassID       string  `db:"class_id" json:"class_id"`
	ClassName     string  `db:"class_name" json:"class_name"`
	TotalStudents int     `db:"total_students" json:"total_students"`
	AverageGrade  float64 `db:"avg_grade" json:"average_grade"`
	PassRate      float64 `db:"pass_rate" json:"pass_rate"`
}

// AnalyticsSubjectPerformer is a top student row for a subject.
type AnalyticsSubjectPerformer struct {
	StudentID   string  `db:"student_id" json:"student_id"`
	StudentName string  `db:"student_name" json:"student_name"`
	ClassID     string  `db:"class_id" json:"class_id"`
	ClassName   string  `db:"class_name" json:"class_name"`
	Grade       float64 `db:"final_grade" json:"grade"`
}

// AnalyticsSubjectAnalytics contains subject-level statistics and drilldown rows.
type AnalyticsSubjectAnalytics struct {
	SubjectID         string                      `db:"subject_id" json:"subject_id"`
	SubjectName       string                      `db:"subject_name" json:"subject_name"`
	TermID            string                      `db:"term_id" json:"term_id"`
	Overall           AnalyticsSubjectSummary     `json:"overall"`
	ByClass           []AnalyticsSubjectClass     `json:"by_class"`
	GradeDistribution map[string]int              `json:"grade_distribution"`
	TopPerformers     []AnalyticsSubjectPerformer `json:"top_performers"`
}

// AnalyticsLeaderboardFilter scopes a leaderboard query.
type AnalyticsLeaderboardFilter struct {
	TermID  string
	ClassID string
	Limit   int
}

// AnalyticsLeaderboardEntry is shared by GPA, attendance, and behaviour leaderboards.
type AnalyticsLeaderboardEntry struct {
	Rank        int     `db:"rank" json:"rank"`
	StudentID   string  `db:"student_id" json:"student_id"`
	StudentName string  `db:"student_name" json:"student_name"`
	NIS         string  `db:"nis" json:"nis"`
	ClassID     string  `db:"class_id" json:"class_id"`
	ClassName   string  `db:"class_name" json:"class_name"`
	Score       float64 `db:"score" json:"score"`
	Points      int     `db:"points" json:"points,omitempty"`
}

// AnalyticsLeaderboard is the response envelope data for a leaderboard endpoint.
type AnalyticsLeaderboard struct {
	TermID      string                      `json:"term_id"`
	ClassID     string                      `json:"class_id,omitempty"`
	Metric      string                      `json:"metric"`
	Leaderboard []AnalyticsLeaderboardEntry `json:"leaderboard"`
}
