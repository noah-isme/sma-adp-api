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

// SubjectAttendanceRepository persists subject session attendance.
type SubjectAttendanceRepository struct {
	db *sqlx.DB
}

// NewSubjectAttendanceRepository constructs the repository.
func NewSubjectAttendanceRepository(db *sqlx.DB) *SubjectAttendanceRepository {
	return &SubjectAttendanceRepository{db: db}
}

// List returns session attendance filtered by provided criteria.
func (r *SubjectAttendanceRepository) List(ctx context.Context, filter models.SubjectAttendanceFilter) ([]models.SubjectAttendanceRecord, int, error) {
	base := `FROM subject_attendance sa
JOIN enrollments e ON e.id = sa.enrollment_id
JOIN students s ON s.id = e.student_id
LEFT JOIN schedules sch ON sch.id = sa.schedule_id
LEFT JOIN classes c ON c.id = e.class_id
LEFT JOIN subjects sub ON sub.id = sch.subject_id`
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.ScheduleID != "" {
		where = append(where, fmt.Sprintf("sa.schedule_id = $%d", len(args)+1))
		args = append(args, filter.ScheduleID)
	}
	if filter.Status != nil && filter.Status.Valid() {
		where = append(where, fmt.Sprintf("sa.status = $%d", len(args)+1))
		args = append(args, *filter.Status)
	}
	if filter.Date != nil {
		where = append(where, fmt.Sprintf("sa.date = $%d", len(args)+1))
		args = append(args, *filter.Date)
	}
	whereClause := strings.Join(where, " AND ")
	sortBy := filter.SortBy
	allowedSort := map[string]string{
		"date":       "sa.date",
		"created_at": "sa.created_at",
	}
	if sortBy == "" {
		sortBy = "date"
	}
	column, ok := allowedSort[sortBy]
	if !ok {
		column = "sa.date"
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

	query := fmt.Sprintf(`SELECT sa.id, sa.enrollment_id, sa.schedule_id, sa.date, sa.status, sa.notes, sa.created_at, sa.updated_at,
        e.student_id, s.full_name AS student_name, e.class_id, c.name AS class_name, sch.subject_id, sub.name AS subject_name
        %s WHERE %s ORDER BY %s %s LIMIT %d OFFSET %d`, base, whereClause, column, order, size, offset)
	var rows []models.SubjectAttendanceRecord
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list subject attendance: %w", err)
	}
	countQuery := fmt.Sprintf("SELECT COUNT(*) %s WHERE %s", base, whereClause)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count subject attendance: %w", err)
	}
	return rows, total, nil
}

// Upsert inserts or updates a subject attendance record.
func (r *SubjectAttendanceRepository) Upsert(ctx context.Context, record *models.SubjectAttendance) (*models.SubjectAttendance, error) {
	now := time.Now().UTC()
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	query := `INSERT INTO subject_attendance (id, enrollment_id, schedule_id, date, status, notes, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (enrollment_id, schedule_id, date)
DO UPDATE SET status = EXCLUDED.status, notes = EXCLUDED.notes, updated_at = EXCLUDED.updated_at
RETURNING id, enrollment_id, schedule_id, date, status, notes, created_at, updated_at`
	var stored models.SubjectAttendance
	if err := r.db.GetContext(ctx, &stored, query, record.ID, record.EnrollmentID, record.ScheduleID, record.Date, record.Status, record.Notes, record.CreatedAt, record.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert subject attendance: %w", err)
	}
	return &stored, nil
}

// BulkInsert inserts multiple subject attendance entries.
func (r *SubjectAttendanceRepository) BulkInsert(ctx context.Context, records []models.SubjectAttendance, atomic bool) ([]models.SubjectAttendance, error) {
	if len(records) == 0 {
		return nil, nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin bulk subject attendance: %w", err)
	}
	conflicts := make([]models.SubjectAttendance, 0)
	commit := false
	defer func() {
		if !commit {
			tx.Rollback()
		}
	}()
	query := `INSERT INTO subject_attendance (id, enrollment_id, schedule_id, date, status, notes, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (enrollment_id, schedule_id, date) DO NOTHING RETURNING id`
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
		if err := tx.QueryRowxContext(ctx, query, rec.ID, rec.EnrollmentID, rec.ScheduleID, rec.Date, rec.Status, rec.Notes, rec.CreatedAt, rec.UpdatedAt).Scan(&insertedID); err != nil {
			if err == sql.ErrNoRows {
				conflicts = append(conflicts, *rec)
				if atomic {
					return nil, fmt.Errorf("bulk insert subject attendance: duplicate for enrollment %s schedule %s on %s", rec.EnrollmentID, rec.ScheduleID, rec.Date.Format(time.RFC3339))
				}
				continue
			}
			return nil, fmt.Errorf("bulk insert subject attendance: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit bulk subject attendance: %w", err)
	}
	commit = true
	return conflicts, nil
}

// SessionReport lists the attendance for a schedule session.
func (r *SubjectAttendanceRepository) SessionReport(ctx context.Context, scheduleID string, date time.Time) ([]models.SubjectAttendanceReportRow, error) {
	query := `SELECT sa.enrollment_id, e.student_id, s.full_name AS student_name, sa.status, sa.notes
FROM subject_attendance sa
JOIN enrollments e ON e.id = sa.enrollment_id
JOIN students s ON s.id = e.student_id
WHERE sa.schedule_id = $1 AND sa.date = $2`
	var rows []models.SubjectAttendanceReportRow
	if err := r.db.SelectContext(ctx, &rows, query, scheduleID, date); err != nil {
		return nil, fmt.Errorf("subject attendance session report: %w", err)
	}
	return rows, nil
}
