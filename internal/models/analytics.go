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
