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
// deliberately return unfiltered CSV directly; asynchronous report jobs remain
// available separately under /reports/generate and /export/{token}.
type ExportCompatibilityHandler struct{ db *sqlx.DB }

func NewExportCompatibilityHandler(db *sqlx.DB) *ExportCompatibilityHandler {
	return &ExportCompatibilityHandler{db: db}
}

// Students exports the complete students table as an unfiltered CSV download.
// @Summary Export students as unfiltered CSV
// @Tags Exports
// @Description Streams the complete students table as text/csv. Query filters and XLSX format are unsupported.
// @Produce text/csv
// @Success 200 {file} binary
// @Router /export/students [get]
func (h *ExportCompatibilityHandler) Students(c *gin.Context) {
	h.write(c, "students.csv", []string{"id", "nis", "full_name", "gender", "active"}, `SELECT id, nis, full_name, gender, active FROM students ORDER BY full_name`)
}

// Grades exports the complete grades table as an unfiltered CSV download.
// @Summary Export grades as unfiltered CSV
// @Tags Exports
// @Description Streams the complete grades table as text/csv. Query filters and XLSX format are unsupported.
// @Produce text/csv
// @Success 200 {file} binary
// @Router /export/grades [get]
func (h *ExportCompatibilityHandler) Grades(c *gin.Context) {
	h.write(c, "grades.csv", []string{"id", "enrollment_id", "subject_id", "component_id", "grade_value", "updated_at"}, `SELECT id, enrollment_id, subject_id, component_id, grade_value, updated_at FROM grades ORDER BY updated_at DESC`)
}

// Attendance exports the complete daily attendance table as an unfiltered CSV download.
// @Summary Export attendance as unfiltered CSV
// @Tags Exports
// @Description Streams the complete daily attendance table as text/csv. Query filters and XLSX format are unsupported.
// @Produce text/csv
// @Success 200 {file} binary
// @Router /export/attendance [get]
func (h *ExportCompatibilityHandler) Attendance(c *gin.Context) {
	h.write(c, "attendance.csv", []string{"id", "enrollment_id", "date", "status", "notes", "updated_at"}, `SELECT id, enrollment_id, date, status, COALESCE(notes, ''), updated_at FROM daily_attendance ORDER BY date DESC`)
}

func (h *ExportCompatibilityHandler) write(c *gin.Context, filename string, header []string, query string) {
	rows, err := h.db.QueryxContext(context.Background(), query)
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
