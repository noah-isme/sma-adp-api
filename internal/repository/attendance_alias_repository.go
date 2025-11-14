package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// AttendanceAliasFilter scopes aggregation queries for attendance aliases.
type AttendanceAliasFilter struct {
	TermID    string
	ClassID   string
	ClassIDs  []string
	StudentID string
	DateFrom  *time.Time
	DateTo    *time.Time
}

// AttendanceAliasStudentRow represents per-student aggregates.
type AttendanceAliasStudentRow struct {
	StudentID   string  `db:"student_id"`
	StudentName string  `db:"student_name"`
	ClassID     string  `db:"class_id"`
	Present     int     `db:"present"`
	Sick        int     `db:"sick"`
	Excused     int     `db:"excused"`
	Absent      int     `db:"absent"`
	Total       int     `db:"total"`
	Rate        float64 `db:"rate"`
}

// AttendanceAliasAggregate groups summary totals and student rows.
type AttendanceAliasAggregate struct {
	TotalDays int
	Present   int
	Sick      int
	Excused   int
	Absent    int
	Students  []AttendanceAliasStudentRow
}

// AttendanceAliasRepository exposes read-only aggregate helpers for attendance aliases.
type AttendanceAliasRepository struct {
	db *sqlx.DB
}

// NewAttendanceAliasRepository builds the repository.
func NewAttendanceAliasRepository(db *sqlx.DB) *AttendanceAliasRepository {
	return &AttendanceAliasRepository{db: db}
}

// Aggregate returns overall summary counts and per-student aggregates.
func (r *AttendanceAliasRepository) Aggregate(ctx context.Context, filter AttendanceAliasFilter) (*AttendanceAliasAggregate, error) {
	if filter.TermID == "" {
		return nil, fmt.Errorf("termId is required")
	}
	where, args := buildAttendanceAliasConditions(filter)
	whereClause := strings.Join(where, " AND ")

	totalSQL := fmt.Sprintf(`SELECT
    COALESCE(COUNT(DISTINCT da.date), 0) AS total_days,
    COALESCE(SUM(CASE WHEN da.status = 'H' THEN 1 ELSE 0 END), 0) AS present,
    COALESCE(SUM(CASE WHEN da.status = 'S' THEN 1 ELSE 0 END), 0) AS sick,
    COALESCE(SUM(CASE WHEN da.status = 'I' THEN 1 ELSE 0 END), 0) AS excused,
    COALESCE(SUM(CASE WHEN da.status = 'A' THEN 1 ELSE 0 END), 0) AS absent
FROM daily_attendance da
JOIN enrollments e ON e.id = da.enrollment_id
WHERE %s`, whereClause)
	totalRow := struct {
		TotalDays int `db:"total_days"`
		Present   int `db:"present"`
		Sick      int `db:"sick"`
		Excused   int `db:"excused"`
		Absent    int `db:"absent"`
	}{}
	if err := r.db.GetContext(ctx, &totalRow, totalSQL, append([]interface{}{}, args...)...); err != nil {
		return nil, fmt.Errorf("attendance alias totals: %w", err)
	}

	studentsSQL := fmt.Sprintf(`SELECT
    e.student_id,
    s.full_name AS student_name,
    e.class_id,
    SUM(CASE WHEN da.status = 'H' THEN 1 ELSE 0 END) AS present,
    SUM(CASE WHEN da.status = 'S' THEN 1 ELSE 0 END) AS sick,
    SUM(CASE WHEN da.status = 'I' THEN 1 ELSE 0 END) AS excused,
    SUM(CASE WHEN da.status = 'A' THEN 1 ELSE 0 END) AS absent,
    COUNT(*) AS total,
    CASE WHEN COUNT(*) = 0 THEN 0 ELSE (SUM(CASE WHEN da.status = 'H' THEN 1 ELSE 0 END)::DECIMAL / COUNT(*)) * 100 END AS rate
FROM daily_attendance da
JOIN enrollments e ON e.id = da.enrollment_id
JOIN students s ON s.id = e.student_id
WHERE %s
GROUP BY e.student_id, s.full_name, e.class_id
ORDER BY s.full_name ASC`, whereClause)
	var rows []AttendanceAliasStudentRow
	if err := r.db.SelectContext(ctx, &rows, studentsSQL, append([]interface{}{}, args...)...); err != nil {
		return nil, fmt.Errorf("attendance alias per-student summary: %w", err)
	}

	return &AttendanceAliasAggregate{
		TotalDays: totalRow.TotalDays,
		Present:   totalRow.Present,
		Sick:      totalRow.Sick,
		Excused:   totalRow.Excused,
		Absent:    totalRow.Absent,
		Students:  rows,
	}, nil
}

func buildAttendanceAliasConditions(filter AttendanceAliasFilter) ([]string, []interface{}) {
	conditions := []string{"e.status = 'ACTIVE'"}
	args := []interface{}{}

	args = append(args, filter.TermID)
	conditions = append(conditions, fmt.Sprintf("e.term_id = $%d", len(args)))

	if filter.ClassID != "" {
		args = append(args, filter.ClassID)
		conditions = append(conditions, fmt.Sprintf("e.class_id = $%d", len(args)))
	} else if len(filter.ClassIDs) > 0 {
		placeholders := make([]string, len(filter.ClassIDs))
		for i, classID := range filter.ClassIDs {
			args = append(args, classID)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		conditions = append(conditions, fmt.Sprintf("e.class_id IN (%s)", strings.Join(placeholders, ",")))
	}

	if filter.StudentID != "" {
		args = append(args, filter.StudentID)
		conditions = append(conditions, fmt.Sprintf("e.student_id = $%d", len(args)))
	}

	if filter.DateFrom != nil {
		args = append(args, *filter.DateFrom)
		conditions = append(conditions, fmt.Sprintf("da.date >= $%d", len(args)))
	}

	if filter.DateTo != nil {
		args = append(args, *filter.DateTo)
		conditions = append(conditions, fmt.Sprintf("da.date <= $%d", len(args)))
	}

	return conditions, args
}
