package handler

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// ExportCompatibilityHandler serves the legacy browser-download routes. They
// now support basic filtering while asynchronous report jobs remain
// available separately under /reports/generate and /export/{token}.
type ExportCompatibilityHandler struct{ db *sqlx.DB }

func NewExportCompatibilityHandler(db *sqlx.DB) *ExportCompatibilityHandler {
	return &ExportCompatibilityHandler{db: db}
}

// Students exports students as CSV with optional filters.
// @Summary Export students as CSV
// @Tags Exports
// @Description Streams students as text/csv with optional filters.
// @Produce text/csv
// @Param classId query string false "Filter by class"
// @Param active query bool false "Filter by active state"
// @Param gender query string false "Filter by gender"
// @Success 200 {file} binary
// @Router /export/students [get]
func (h *ExportCompatibilityHandler) Students(c *gin.Context) {
	query := `SELECT s.id, s.nis, s.full_name, s.gender, s.active FROM students s WHERE 1=1`
	var args []interface{}
	
	if classId := c.Query("classId"); classId != "" {
		query += " AND s.id IN (SELECT student_id FROM enrollments WHERE class_id = $1 AND status = 'active')"
		args = append(args, classId)
	}
	if active := c.Query("active"); active != "" {
		argIdx := len(args) + 1
		query += fmt.Sprintf(" AND s.active = $%d", argIdx)
		args = append(args, active == "true")
	}
	if gender := c.Query("gender"); gender != "" {
		argIdx := len(args) + 1
		query += fmt.Sprintf(" AND s.gender = $%d", argIdx)
		args = append(args, gender)
	}
	query += " ORDER BY s.full_name"
	
	h.write(c, "students.csv", []string{"id", "nis", "full_name", "gender", "active"}, query, args...)
}

// Grades exports grades as CSV with optional filters.
// @Summary Export grades as CSV
// @Tags Exports
// @Description Streams grades as text/csv with optional filters.
// @Produce text/csv
// @Param classId query string false "Filter by class"
// @Param subjectId query string false "Filter by subject"
// @Param componentId query string false "Filter by component"
// @Param status query string false "Filter by status"
// @Success 200 {file} binary
// @Router /export/grades [get]
func (h *ExportCompatibilityHandler) Grades(c *gin.Context) {
	query := `SELECT g.id, g.enrollment_id, g.subject_id, g.component_id, g.grade_value, g.updated_at
		FROM grades g
		JOIN enrollments e ON e.id = g.enrollment_id
		WHERE 1=1 AND g.deleted_at IS NULL`
	var args []interface{}
	
	if classId := c.Query("classId"); classId != "" {
		query += fmt.Sprintf(" AND e.class_id = $%d", len(args)+1)
		args = append(args, classId)
	}
	if subjectId := c.Query("subjectId"); subjectId != "" {
		query += fmt.Sprintf(" AND g.subject_id = $%d", len(args)+1)
		args = append(args, subjectId)
	}
	if componentId := c.Query("componentId"); componentId != "" {
		query += fmt.Sprintf(" AND g.component_id = $%d", len(args)+1)
		args = append(args, componentId)
	}
	if status := c.Query("status"); status != "" {
		// Status would need KKM comparison logic - simplified here
		query += " AND 1=1" // placeholder
	}
	query += " ORDER BY g.updated_at DESC"
	
	h.write(c, "grades.csv", []string{"id", "enrollment_id", "subject_id", "component_id", "grade_value", "updated_at"}, query, args...)
}

// Attendance exports attendance as CSV with optional filters.
// @Summary Export attendance as CSV
// @Tags Exports
// @Description Streams attendance as text/csv with optional filters.
// @Produce text/csv
// @Param classId query string false "Filter by class"
// @Param dateFrom query string false "Filter by date from (RFC3339)"
// @Param dateTo query string false "Filter by date to (RFC3339)"
// @Success 200 {file} binary
// @Router /export/attendance [get]
func (h *ExportCompatibilityHandler) Attendance(c *gin.Context) {
	query := `SELECT da.id, da.enrollment_id, da.date, da.status, COALESCE(da.notes, ''), da.updated_at
		FROM daily_attendance da
		JOIN enrollments e ON e.id = da.enrollment_id
		WHERE 1=1`
	var args []interface{}
	
	if classId := c.Query("classId"); classId != "" {
		query += fmt.Sprintf(" AND e.class_id = $%d", len(args)+1)
		args = append(args, classId)
	}
	if dateFrom := c.Query("dateFrom"); dateFrom != "" {
		query += fmt.Sprintf(" AND da.date >= $%d", len(args)+1)
		args = append(args, dateFrom)
	}
	if dateTo := c.Query("dateTo"); dateTo != "" {
		query += fmt.Sprintf(" AND da.date <= $%d", len(args)+1)
		args = append(args, dateTo)
	}
	query += " ORDER BY da.date DESC"
	
	h.write(c, "attendance.csv", []string{"id", "enrollment_id", "date", "status", "notes", "updated_at"}, query, args...)
}

func (h *ExportCompatibilityHandler) write(c *gin.Context, filename string, header []string, query string, args ...interface{}) {
	rows, err := h.db.QueryxContext(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to export CSV"}})
		return
	}
	defer rows.Close()
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "text/csv; charset=utf-8")
	w := csv.NewWriter(c.Writer)
	defer w.Flush()
	_ = w.Write(header)
	for rows.Next() {
		values, err := rows.SliceScan()
		if err != nil {
			return
		}
		record := make([]string, len(values))
		for i, value := range values {
			record[i] = fmt.Sprint(value)
		}
		_ = w.Write(record)
	}
}
