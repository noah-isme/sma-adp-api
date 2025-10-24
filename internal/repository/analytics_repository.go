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
        FROM daily_attendances da
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
