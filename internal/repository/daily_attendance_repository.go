package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// DailyAttendanceRepository handles persistence for daily attendance records.
type DailyAttendanceRepository struct {
	db *sqlx.DB
}

// NewDailyAttendanceRepository constructs the repository.
func NewDailyAttendanceRepository(db *sqlx.DB) *DailyAttendanceRepository {
	return &DailyAttendanceRepository{db: db}
}

// List returns daily attendance rows matching the provided filter.
func (r *DailyAttendanceRepository) List(ctx context.Context, filter models.DailyAttendanceFilter) ([]models.DailyAttendanceRecord, int, error) {
	base := `FROM daily_attendance da
JOIN enrollments e ON e.id = da.enrollment_id
JOIN students s ON s.id = e.student_id
LEFT JOIN classes c ON c.id = e.class_id`
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.ClassID != "" {
		where = append(where, fmt.Sprintf("e.class_id = $%d", len(args)+1))
		args = append(args, filter.ClassID)
	}
	if filter.TermID != "" {
		where = append(where, fmt.Sprintf("e.term_id = $%d", len(args)+1))
		args = append(args, filter.TermID)
	}
	if filter.StudentID != "" {
		where = append(where, fmt.Sprintf("e.student_id = $%d", len(args)+1))
		args = append(args, filter.StudentID)
	}
	if filter.Status != nil && filter.Status.Valid() {
		where = append(where, fmt.Sprintf("da.status = $%d", len(args)+1))
		args = append(args, *filter.Status)
	}
	if filter.DateFrom != nil {
		where = append(where, fmt.Sprintf("da.date >= $%d", len(args)+1))
		args = append(args, *filter.DateFrom)
	}
	if filter.DateTo != nil {
		where = append(where, fmt.Sprintf("da.date <= $%d", len(args)+1))
		args = append(args, *filter.DateTo)
	}
	whereClause := strings.Join(where, " AND ")
	sortBy := filter.SortBy
	allowedSort := map[string]string{
		"date":       "da.date",
		"status":     "da.status",
		"created_at": "da.created_at",
	}
	if sortBy == "" {
		sortBy = "date"
	}
	sortColumn, ok := allowedSort[sortBy]
	if !ok {
		sortColumn = "da.date"
	}
	order := strings.ToUpper(filter.SortOrder)
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 || size > 200 {
		size = 50
	}
	offset := (page - 1) * size

	query := fmt.Sprintf(`SELECT da.id, da.enrollment_id, da.date, da.status, da.notes, da.created_at, da.updated_at,
        e.student_id, s.full_name AS student_name, e.class_id, c.name AS class_name, e.term_id
        %s WHERE %s
        ORDER BY %s %s
        LIMIT %d OFFSET %d`, base, whereClause, sortColumn, order, size, offset)

	var rows []models.DailyAttendanceRecord
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list daily attendance: %w", err)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s WHERE %s", base, whereClause)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count daily attendance: %w", err)
	}
	return rows, total, nil
}

// Upsert inserts or updates a daily attendance record.
func (r *DailyAttendanceRepository) Upsert(ctx context.Context, record *models.DailyAttendance) (*models.DailyAttendance, error) {
	now := time.Now().UTC()
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	query := `INSERT INTO daily_attendance (id, enrollment_id, date, status, notes, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (enrollment_id, date)
DO UPDATE SET status = EXCLUDED.status, notes = EXCLUDED.notes, updated_at = EXCLUDED.updated_at
RETURNING id, enrollment_id, date, status, notes, created_at, updated_at`
	var stored models.DailyAttendance
	if err := r.db.GetContext(ctx, &stored, query, record.ID, record.EnrollmentID, record.Date, record.Status, record.Notes, record.CreatedAt, record.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert daily attendance: %w", err)
	}
	return &stored, nil
}

// BulkInsert inserts many records best-effort; returns conflicting entries when partial.
func (r *DailyAttendanceRepository) BulkInsert(ctx context.Context, records []models.DailyAttendance, atomic bool) ([]models.DailyAttendance, error) {
	if len(records) == 0 {
		return nil, nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin bulk daily attendance: %w", err)
	}
	conflicts := make([]models.DailyAttendance, 0)
	commit := false
	defer func() {
		if !commit {
			tx.Rollback()
		}
	}()
	query := `INSERT INTO daily_attendance (id, enrollment_id, date, status, notes, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (enrollment_id, date) DO NOTHING RETURNING id`
	now := time.Now().UTC()
	for i := range records {
		rec := &records[i]
		if rec.ID == "" {
			rec.ID = uuid.NewString()
		}
		if rec.CreatedAt.IsZero() {
			rec.CreatedAt = now
		}
		rec.UpdatedAt = now
		var insertedID string
		if err := tx.QueryRowxContext(ctx, query, rec.ID, rec.EnrollmentID, rec.Date, rec.Status, rec.Notes, rec.CreatedAt, rec.UpdatedAt).Scan(&insertedID); err != nil {
			if err == sql.ErrNoRows {
				conflicts = append(conflicts, *rec)
				if atomic {
					return nil, fmt.Errorf("bulk insert daily attendance: duplicate for enrollment %s on %s", rec.EnrollmentID, rec.Date.Format(time.RFC3339))
				}
				continue
			}
			return nil, fmt.Errorf("bulk insert daily attendance: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit bulk daily attendance: %w", err)
	}
	commit = true
	return conflicts, nil
}

// ClassReport summarises attendance for a class on a given date.
func (r *DailyAttendanceRepository) ClassReport(ctx context.Context, classID string, date time.Time) ([]models.DailyAttendanceReportRow, error) {
	query := `SELECT s.id AS student_id, s.full_name AS student_name, da.status, da.notes
FROM daily_attendance da
JOIN enrollments e ON e.id = da.enrollment_id
JOIN students s ON s.id = e.student_id
WHERE e.class_id = $1 AND da.date = $2`
	var rows []models.DailyAttendanceReportRow
	if err := r.db.SelectContext(ctx, &rows, query, classID, date); err != nil {
		return nil, fmt.Errorf("class report: %w", err)
	}
	return rows, nil
}

// StudentHistory returns attendance history for a student.
func (r *DailyAttendanceRepository) StudentHistory(ctx context.Context, studentID string, from, to *time.Time) ([]models.DailyAttendanceHistoryRow, error) {
	where := []string{"e.student_id = $1"}
	args := []interface{}{studentID}
	if from != nil {
		where = append(where, fmt.Sprintf("da.date >= $%d", len(args)+1))
		args = append(args, *from)
	}
	if to != nil {
		where = append(where, fmt.Sprintf("da.date <= $%d", len(args)+1))
		args = append(args, *to)
	}
	query := fmt.Sprintf(`SELECT da.date, da.status, da.notes
FROM daily_attendance da
JOIN enrollments e ON e.id = da.enrollment_id
WHERE %s
ORDER BY da.date DESC`, strings.Join(where, " AND "))
	var rows []models.DailyAttendanceHistoryRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("student attendance history: %w", err)
	}
	return rows, nil
}

// StudentSummary aggregates counts for a student within a term.
func (r *DailyAttendanceRepository) StudentSummary(ctx context.Context, studentID string, termID string) (*models.DailyAttendanceSummary, error) {
	query := `SELECT da.status, COUNT(*) AS cnt
FROM daily_attendance da
JOIN enrollments e ON e.id = da.enrollment_id
WHERE e.student_id = $1 AND ($2 = '' OR e.term_id = $2)
GROUP BY da.status`
	rows := []struct {
		Status string `db:"status"`
		Count  int    `db:"cnt"`
	}{}
	if err := r.db.SelectContext(ctx, &rows, query, studentID, termID); err != nil {
		return nil, fmt.Errorf("student attendance summary: %w", err)
	}
	summary := &models.DailyAttendanceSummary{}
	for _, row := range rows {
		switch models.AttendanceStatus(row.Status) {
		case models.AttendanceStatusPresent:
			summary.Present += row.Count
		case models.AttendanceStatusSick:
			summary.Sick += row.Count
		case models.AttendanceStatusExcused:
			summary.Excused += row.Count
		case models.AttendanceStatusAbsent:
			summary.Absent += row.Count
		}
		summary.Total += row.Count
	}
	if summary.Total > 0 {
		summary.Percent = float64(summary.Present) / float64(summary.Total) * 100
	}
	return summary, nil
}
