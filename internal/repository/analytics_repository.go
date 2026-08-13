package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// AnalyticsRepository exposes read-optimised queries for analytics endpoints.
type AnalyticsRepository struct {
	db *sqlx.DB
}

// NewAnalyticsRepository instantiates the repository.
func NewAnalyticsRepository(db *sqlx.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// AttendanceSummary retrieves aggregated attendance data with optional date filtering.
func (r *AnalyticsRepository) AttendanceSummary(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error) {
	if filter.DateFrom == nil && filter.DateTo == nil {
		var builder strings.Builder
		builder.WriteString("SELECT term_id, class_id, present_count, absent_count, percentage, updated_at FROM attendance_summary_mv WHERE 1=1")
		var args []interface{}
		if filter.TermID != "" {
			args = append(args, filter.TermID)
			builder.WriteString(fmt.Sprintf(" AND term_id = $%d", len(args)))
		}
		if filter.ClassID != "" {
			args = append(args, filter.ClassID)
			builder.WriteString(fmt.Sprintf(" AND class_id = $%d", len(args)))
		}
		builder.WriteString(" ORDER BY percentage DESC")

		var summaries []models.AnalyticsAttendanceSummary
		if err := r.db.SelectContext(ctx, &summaries, builder.String(), args...); err != nil {
			return nil, fmt.Errorf("query attendance summary mv: %w", err)
		}
		return summaries, nil
	}

	var builder strings.Builder
	builder.WriteString(`SELECT e.term_id, e.class_id,
        SUM(CASE WHEN da.status = 'H' THEN 1 ELSE 0 END) AS present_count,
        SUM(CASE WHEN da.status = 'A' THEN 1 ELSE 0 END) AS absent_count,
        CASE WHEN COUNT(*) = 0 THEN 0 ELSE (SUM(CASE WHEN da.status = 'H' THEN 1 ELSE 0 END)::DECIMAL / COUNT(*)) * 100 END AS percentage,
        MAX(da.updated_at) AS updated_at
        FROM daily_attendance da
        JOIN enrollments e ON e.id = da.enrollment_id
        WHERE 1=1`)
	var args []interface{}
	if filter.TermID != "" {
		args = append(args, filter.TermID)
		builder.WriteString(fmt.Sprintf(" AND e.term_id = $%d", len(args)))
	}
	if filter.ClassID != "" {
		args = append(args, filter.ClassID)
		builder.WriteString(fmt.Sprintf(" AND e.class_id = $%d", len(args)))
	}
	if filter.DateFrom != nil {
		args = append(args, *filter.DateFrom)
		builder.WriteString(fmt.Sprintf(" AND da.date >= $%d", len(args)))
	}
	if filter.DateTo != nil {
		args = append(args, *filter.DateTo)
		builder.WriteString(fmt.Sprintf(" AND da.date <= $%d", len(args)))
	}
	builder.WriteString(" GROUP BY e.term_id, e.class_id ORDER BY percentage DESC")

	var summaries []models.AnalyticsAttendanceSummary
	if err := r.db.SelectContext(ctx, &summaries, builder.String(), args...); err != nil {
		return nil, fmt.Errorf("query attendance summary live: %w", err)
	}
	return summaries, nil
}

// GradeSummary retrieves aggregated grade metrics from the materialized view.
func (r *AnalyticsRepository) GradeSummary(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error) {
	var builder strings.Builder
	builder.WriteString("SELECT term_id, class_id, subject_id, avg_score, median_score, rank_json, updated_at FROM grade_summary_mv WHERE 1=1")
	var args []interface{}
	if filter.TermID != "" {
		args = append(args, filter.TermID)
		builder.WriteString(fmt.Sprintf(" AND term_id = $%d", len(args)))
	}
	if filter.ClassID != "" {
		args = append(args, filter.ClassID)
		builder.WriteString(fmt.Sprintf(" AND class_id = $%d", len(args)))
	}
	if filter.SubjectID != "" {
		args = append(args, filter.SubjectID)
		builder.WriteString(fmt.Sprintf(" AND subject_id = $%d", len(args)))
	}
	builder.WriteString(" ORDER BY avg_score DESC")

	type row struct {
		TermID      string         `db:"term_id"`
		ClassID     string         `db:"class_id"`
		SubjectID   string         `db:"subject_id"`
		AvgScore    float64        `db:"avg_score"`
		MedianScore float64        `db:"median_score"`
		RankJSON    sql.NullString `db:"rank_json"`
		UpdatedAt   *time.Time     `db:"updated_at"`
	}

	var rows []row
	if err := r.db.SelectContext(ctx, &rows, builder.String(), args...); err != nil {
		return nil, fmt.Errorf("query grade summary mv: %w", err)
	}

	summaries := make([]models.AnalyticsGradeSummary, 0, len(rows))
	for _, rrow := range rows {
		summary := models.AnalyticsGradeSummary{
			TermID:       rrow.TermID,
			ClassID:      rrow.ClassID,
			SubjectID:    rrow.SubjectID,
			AverageScore: rrow.AvgScore,
			MedianScore:  rrow.MedianScore,
			UpdatedAt:    rrow.UpdatedAt,
		}
		if rrow.RankJSON.Valid && rrow.RankJSON.String != "" {
			if err := json.Unmarshal([]byte(rrow.RankJSON.String), &summary.Rank); err != nil {
				return nil, fmt.Errorf("decode rank json: %w", err)
			}
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// BehaviorSummary retrieves behaviour metrics either from the materialized view or from live aggregation when a date filter is applied.
func (r *AnalyticsRepository) BehaviorSummary(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error) {
	if filter.DateFrom == nil && filter.DateTo == nil {
		var builder strings.Builder
		builder.WriteString("SELECT s.term_id, s.student_id, s.total_positive, s.total_negative, s.balance, s.updated_at FROM behavior_summary_mv s")
		if filter.ClassID != "" {
			builder.WriteString(" JOIN enrollments e ON e.term_id = s.term_id AND e.student_id = s.student_id")
		}
		builder.WriteString(" WHERE 1=1")
		var args []interface{}
		if filter.TermID != "" {
			args = append(args, filter.TermID)
			builder.WriteString(fmt.Sprintf(" AND s.term_id = $%d", len(args)))
		}
		if filter.StudentID != "" {
			args = append(args, filter.StudentID)
			builder.WriteString(fmt.Sprintf(" AND s.student_id = $%d", len(args)))
		}
		if filter.ClassID != "" {
			args = append(args, filter.ClassID)
			builder.WriteString(fmt.Sprintf(" AND e.class_id = $%d", len(args)))
		}
		builder.WriteString(" ORDER BY s.balance DESC")

		var summaries []models.AnalyticsBehaviorSummary
		if err := r.db.SelectContext(ctx, &summaries, builder.String(), args...); err != nil {
			return nil, fmt.Errorf("query behavior summary mv: %w", err)
		}
		return summaries, nil
	}

	if filter.TermID == "" {
		return nil, fmt.Errorf("term_id is required when filtering behaviour analytics by date range")
	}

	var builder strings.Builder
	builder.WriteString(`SELECT e.term_id, bn.student_id,
        SUM(CASE WHEN bn.points > 0 THEN bn.points ELSE 0 END) AS total_positive,
        SUM(CASE WHEN bn.points < 0 THEN ABS(bn.points) ELSE 0 END) AS total_negative,
        SUM(bn.points) AS balance,
        MAX(bn.updated_at) AS updated_at
        FROM behavior_notes bn
        JOIN enrollments e ON e.student_id = bn.student_id AND e.term_id = $1
        WHERE 1=1`)

	args := []interface{}{filter.TermID}
	if filter.StudentID != "" {
		args = append(args, filter.StudentID)
		builder.WriteString(fmt.Sprintf(" AND bn.student_id = $%d", len(args)))
	}
	if filter.ClassID != "" {
		args = append(args, filter.ClassID)
		builder.WriteString(fmt.Sprintf(" AND e.class_id = $%d", len(args)))
	}
	if filter.DateFrom != nil {
		args = append(args, *filter.DateFrom)
		builder.WriteString(fmt.Sprintf(" AND bn.date >= $%d", len(args)))
	}
	if filter.DateTo != nil {
		args = append(args, *filter.DateTo)
		builder.WriteString(fmt.Sprintf(" AND bn.date <= $%d", len(args)))
	}
	builder.WriteString(" GROUP BY e.term_id, bn.student_id ORDER BY balance DESC")

	var summaries []models.AnalyticsBehaviorSummary
	if err := r.db.SelectContext(ctx, &summaries, builder.String(), args...); err != nil {
		return nil, fmt.Errorf("query behavior summary live: %w", err)
	}
	return summaries, nil
}

// ClassAnalytics returns the class drilldown for a term. The query uses the
// corrected pre-aggregated materialized views so attendance, grades, and
// behaviour rows cannot multiply one another through a cross join.
func (r *AnalyticsRepository) ClassAnalytics(ctx context.Context, classID, termID string) (*models.AnalyticsClassAnalytics, error) {
	const classQuery = `
SELECT class_id, class_name, grade, track, term_id, term_name,
       total_students, total_subjects, avg_attendance_rate, avg_grade,
       students_passed, students_failed
FROM mv_class_statistics
WHERE class_id = $1 AND term_id = $2`

	var result models.AnalyticsClassAnalytics
	if err := r.db.GetContext(ctx, &result, classQuery, classID, termID); err != nil {
		return nil, fmt.Errorf("query class analytics: %w", err)
	}

	const studentsQuery = `
SELECT student_id, full_name AS student_name, nis, COALESCE(gpa, 0) AS gpa,
       COALESCE(attendance_percentage, 0) AS attendance_percentage,
       ROW_NUMBER() OVER (ORDER BY gpa DESC NULLS LAST, student_id ASC) AS rank
FROM mv_student_performance
WHERE class_id = $1 AND term_id = $2
ORDER BY gpa DESC NULLS LAST, student_id ASC`
	if err := r.db.SelectContext(ctx, &result.Students, studentsQuery, classID, termID); err != nil {
		return nil, fmt.Errorf("query class student analytics: %w", err)
	}
	if result.Students == nil {
		result.Students = []models.AnalyticsClassStudent{}
	}

	const subjectsQuery = `
SELECT subject_id, subject_name, total_students, COALESCE(avg_grade, 0) AS avg_grade,
       COALESCE(pass_rate, 0) AS pass_rate
FROM mv_subject_statistics
WHERE class_id = $1 AND term_id = $2
ORDER BY subject_name ASC, subject_id ASC`
	if err := r.db.SelectContext(ctx, &result.SubjectPerformance, subjectsQuery, classID, termID); err != nil {
		return nil, fmt.Errorf("query class subject analytics: %w", err)
	}
	if result.SubjectPerformance == nil {
		result.SubjectPerformance = []models.AnalyticsClassSubject{}
	}
	return &result, nil
}

// StudentAnalytics returns the student drilldown for a term.
func (r *AnalyticsRepository) StudentAnalytics(ctx context.Context, studentID, termID string) (*models.AnalyticsStudentAnalytics, error) {
	const query = `
SELECT p.student_id, p.nis, p.full_name, p.class_id, p.class_name, p.term_id, p.term_name,
       COALESCE(p.total_attendance_days, 0) AS total_attendance_days,
       COALESCE(p.days_present, 0) AS days_present,
       COALESCE(p.days_sick, 0) AS days_sick,
       COALESCE(p.days_permission, 0) AS days_permission,
       COALESCE(p.days_absent, 0) AS days_absent,
       COALESCE(p.attendance_percentage, 0) AS attendance_percentage,
       COALESCE(p.subjects_enrolled, 0) AS subjects_enrolled,
       COALESCE(p.gpa, 0) AS gpa,
       COALESCE(p.lowest_grade, 0) AS lowest_grade,
       COALESCE(p.highest_grade, 0) AS highest_grade,
       COALESCE(p.subjects_passed, 0) AS subjects_passed,
       COALESCE(p.subjects_failed, 0) AS subjects_failed,
       COALESCE(p.behavior_points, 0) AS behavior_points,
       COALESCE(p.positive_notes, 0) AS positive_notes,
       COALESCE(p.negative_notes, 0) AS negative_notes,
       COALESCE(p.neutral_notes, 0) AS neutral_notes,
       COALESCE(p.subject_grades, '[]'::json) AS subject_grades,
       ROW_NUMBER() OVER (PARTITION BY p.class_id, p.term_id ORDER BY p.gpa DESC NULLS LAST, p.student_id ASC) AS rank,
       COUNT(*) OVER (PARTITION BY p.class_id, p.term_id) AS total_rank
FROM mv_student_performance p
WHERE p.student_id = $1 AND p.term_id = $2
LIMIT 1`

	type studentRow struct {
		StudentID            string  `db:"student_id"`
		NIS                  string  `db:"nis"`
		StudentName          string  `db:"full_name"`
		ClassID              string  `db:"class_id"`
		ClassName            string  `db:"class_name"`
		TermID               string  `db:"term_id"`
		TermName             string  `db:"term_name"`
		TotalAttendanceDays  int     `db:"total_attendance_days"`
		DaysPresent          int     `db:"days_present"`
		DaysSick             int     `db:"days_sick"`
		DaysPermission       int     `db:"days_permission"`
		DaysAbsent           int     `db:"days_absent"`
		AttendancePercentage float64 `db:"attendance_percentage"`
		SubjectsEnrolled     int     `db:"subjects_enrolled"`
		GPA                  float64 `db:"gpa"`
		LowestGrade          float64 `db:"lowest_grade"`
		HighestGrade         float64 `db:"highest_grade"`
		SubjectsPassed       int     `db:"subjects_passed"`
		SubjectsFailed       int     `db:"subjects_failed"`
		BehaviorPoints       int     `db:"behavior_points"`
		PositiveNotes        int     `db:"positive_notes"`
		NegativeNotes        int     `db:"negative_notes"`
		NeutralNotes         int     `db:"neutral_notes"`
		SubjectGrades        []byte  `db:"subject_grades"`
		Rank                 int     `db:"rank"`
		TotalRank            int     `db:"total_rank"`
	}
	var row studentRow
	if err := r.db.GetContext(ctx, &row, query, studentID, termID); err != nil {
		return nil, fmt.Errorf("query student analytics: %w", err)
	}

	result := &models.AnalyticsStudentAnalytics{
		StudentID:   row.StudentID,
		NIS:         row.NIS,
		StudentName: row.StudentName,
		ClassID:     row.ClassID,
		ClassName:   row.ClassName,
		TermID:      row.TermID,
		TermName:    row.TermName,
		Performance: models.AnalyticsStudentPerformance{
			GPA:              row.GPA,
			Rank:             row.Rank,
			TotalRank:        row.TotalRank,
			SubjectsEnrolled: row.SubjectsEnrolled,
			SubjectsPassed:   row.SubjectsPassed,
			SubjectsFailed:   row.SubjectsFailed,
			LowestGrade:      row.LowestGrade,
			HighestGrade:     row.HighestGrade,
		},
		Attendance: models.AnalyticsStudentAttendance{
			Percentage: row.AttendancePercentage,
			TotalDays:  row.TotalAttendanceDays,
			Present:    row.DaysPresent,
			Sick:       row.DaysSick,
			Permission: row.DaysPermission,
			Absent:     row.DaysAbsent,
		},
		Behavior: models.AnalyticsStudentBehavior{
			TotalPoints:   row.BehaviorPoints,
			PositiveNotes: row.PositiveNotes,
			NegativeNotes: row.NegativeNotes,
			NeutralNotes:  row.NeutralNotes,
		},
		SubjectBreakdown: []models.AnalyticsStudentSubject{},
	}
	if len(row.SubjectGrades) > 0 {
		if err := json.Unmarshal(row.SubjectGrades, &result.SubjectBreakdown); err != nil {
			return nil, fmt.Errorf("decode student subject analytics: %w", err)
		}
		if result.SubjectBreakdown == nil {
			result.SubjectBreakdown = []models.AnalyticsStudentSubject{}
		}
	}
	return result, nil
}

// SubjectAnalytics returns subject statistics across one or all classes.
func (r *AnalyticsRepository) SubjectAnalytics(ctx context.Context, subjectID, classID, termID string) (*models.AnalyticsSubjectAnalytics, error) {
	const overallQuery = `
SELECT sub.id AS subject_id, sub.name AS subject_name, $2 AS term_id,
       COUNT(gf.final_grade) AS total_students,
       COALESCE(AVG(gf.final_grade), 0) AS avg_grade,
       COALESCE(STDDEV_POP(gf.final_grade), 0) AS grade_stddev,
       COALESCE(MIN(gf.final_grade), 0) AS min_grade,
       COALESCE(MAX(gf.final_grade), 0) AS max_grade,
       COUNT(*) FILTER (WHERE gf.final_grade >= 75) AS passed_count,
       COUNT(*) FILTER (WHERE gf.final_grade < 75) AS failed_count,
       COALESCE(100.0 * COUNT(*) FILTER (WHERE gf.final_grade >= 75) / NULLIF(COUNT(gf.final_grade), 0), 0) AS pass_rate,
       COUNT(*) FILTER (WHERE gf.final_grade >= 90) AS grade_a_count,
       COUNT(*) FILTER (WHERE gf.final_grade >= 80 AND gf.final_grade < 90) AS grade_b_count,
       COUNT(*) FILTER (WHERE gf.final_grade >= 70 AND gf.final_grade < 80) AS grade_c_count,
       COUNT(*) FILTER (WHERE gf.final_grade >= 60 AND gf.final_grade < 70) AS grade_d_count,
       COUNT(*) FILTER (WHERE gf.final_grade < 60) AS grade_e_count
FROM subjects sub
LEFT JOIN enrollments e ON e.term_id = $2 AND e.status = 'ACTIVE' AND ($3 = '' OR e.class_id = $3)
LEFT JOIN grade_finals gf ON gf.enrollment_id = e.id AND gf.subject_id = sub.id
WHERE sub.id = $1
GROUP BY sub.id, sub.name`

	type subjectRow struct {
		SubjectID     string  `db:"subject_id"`
		SubjectName   string  `db:"subject_name"`
		TermID        string  `db:"term_id"`
		TotalStudents int     `db:"total_students"`
		AverageGrade  float64 `db:"avg_grade"`
		GradeStddev   float64 `db:"grade_stddev"`
		MinGrade      float64 `db:"min_grade"`
		MaxGrade      float64 `db:"max_grade"`
		PassedCount   int     `db:"passed_count"`
		FailedCount   int     `db:"failed_count"`
		PassRate      float64 `db:"pass_rate"`
		GradeACount   int     `db:"grade_a_count"`
		GradeBCount   int     `db:"grade_b_count"`
		GradeCCount   int     `db:"grade_c_count"`
		GradeDCount   int     `db:"grade_d_count"`
		GradeECount   int     `db:"grade_e_count"`
	}
	var overall subjectRow
	if err := r.db.GetContext(ctx, &overall, overallQuery, subjectID, termID, classID); err != nil {
		return nil, fmt.Errorf("query subject analytics: %w", err)
	}

	const classQuery = `
SELECT e.class_id, c.name AS class_name, COUNT(gf.final_grade) AS total_students,
       COALESCE(AVG(gf.final_grade), 0) AS avg_grade,
       COALESCE(100.0 * COUNT(*) FILTER (WHERE gf.final_grade >= 75) / NULLIF(COUNT(gf.final_grade), 0), 0) AS pass_rate
FROM enrollments e
JOIN classes c ON c.id = e.class_id
JOIN grade_finals gf ON gf.enrollment_id = e.id AND gf.subject_id = $1
WHERE e.term_id = $2 AND e.status = 'ACTIVE' AND ($3 = '' OR e.class_id = $3)
GROUP BY e.class_id, c.name
ORDER BY c.name ASC, e.class_id ASC`
	var byClass []models.AnalyticsSubjectClass
	if err := r.db.SelectContext(ctx, &byClass, classQuery, subjectID, termID, classID); err != nil {
		return nil, fmt.Errorf("query subject class analytics: %w", err)
	}
	if byClass == nil {
		byClass = []models.AnalyticsSubjectClass{}
	}

	const performersQuery = `
SELECT e.student_id, s.full_name AS student_name, e.class_id, c.name AS class_name,
       gf.final_grade
FROM grade_finals gf
JOIN enrollments e ON e.id = gf.enrollment_id
JOIN students s ON s.id = e.student_id
JOIN classes c ON c.id = e.class_id
WHERE gf.subject_id = $1 AND e.term_id = $2 AND e.status = 'ACTIVE' AND ($3 = '' OR e.class_id = $3)
ORDER BY gf.final_grade DESC, e.student_id ASC
LIMIT 10`
	var performers []models.AnalyticsSubjectPerformer
	if err := r.db.SelectContext(ctx, &performers, performersQuery, subjectID, termID, classID); err != nil {
		return nil, fmt.Errorf("query subject performers: %w", err)
	}
	if performers == nil {
		performers = []models.AnalyticsSubjectPerformer{}
	}

	return &models.AnalyticsSubjectAnalytics{
		SubjectID:   overall.SubjectID,
		SubjectName: overall.SubjectName,
		TermID:      overall.TermID,
		Overall: models.AnalyticsSubjectSummary{
			TotalStudents: overall.TotalStudents,
			AverageGrade:  overall.AverageGrade,
			GradeStddev:   overall.GradeStddev,
			MinGrade:      overall.MinGrade,
			MaxGrade:      overall.MaxGrade,
			PassedCount:   overall.PassedCount,
			FailedCount:   overall.FailedCount,
			PassRate:      overall.PassRate,
		},
		ByClass: byClass,
		GradeDistribution: map[string]int{
			"A": overall.GradeACount,
			"B": overall.GradeBCount,
			"C": overall.GradeCCount,
			"D": overall.GradeDCount,
			"E": overall.GradeECount,
		},
		TopPerformers: performers,
	}, nil
}

// Leaderboard returns a deterministic top-N leaderboard. Metric must be one
// of gpa, attendance, or behavior.
func (r *AnalyticsRepository) Leaderboard(ctx context.Context, metric string, filter models.AnalyticsLeaderboardFilter) ([]models.AnalyticsLeaderboardEntry, error) {
	var scoreExpression string
	switch metric {
	case "gpa":
		scoreExpression = "COALESCE(gpa, 0)"
	case "attendance":
		scoreExpression = "COALESCE(attendance_percentage, 0)"
	case "behavior":
		scoreExpression = "COALESCE(behavior_points, 0)"
	default:
		return nil, fmt.Errorf("unsupported analytics leaderboard metric %q", metric)
	}
	query := fmt.Sprintf(`
SELECT ROW_NUMBER() OVER (ORDER BY %s DESC, student_id ASC) AS rank,
       student_id, full_name AS student_name, nis, class_id, class_name,
       %s AS score, COALESCE(behavior_points, 0) AS points
FROM mv_student_performance
WHERE term_id = $1 AND ($2 = '' OR class_id = $2)
ORDER BY %s DESC, student_id ASC
LIMIT $3`, scoreExpression, scoreExpression, scoreExpression)
	rows := make([]models.AnalyticsLeaderboardEntry, 0, filter.Limit)
	if err := r.db.SelectContext(ctx, &rows, query, filter.TermID, filter.ClassID, filter.Limit); err != nil {
		return nil, fmt.Errorf("query %s leaderboard: %w", metric, err)
	}
	return rows, nil
}
