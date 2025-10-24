package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// GradeFinalRepository manages final grade persistence.
type GradeFinalRepository struct {
	db *sqlx.DB
}

// NewGradeFinalRepository constructs repository.
func NewGradeFinalRepository(db *sqlx.DB) *GradeFinalRepository {
	return &GradeFinalRepository{db: db}
}

// Upsert bulk upserts final grades.
func (r *GradeFinalRepository) Upsert(ctx context.Context, finals []models.GradeFinal) error {
	if len(finals) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	const query = `INSERT INTO grade_finals (id, enrollment_id, subject_id, final_grade, finalized, calculated_at, calculation_note)
        VALUES (:id, :enrollment_id, :subject_id, :final_grade, :finalized, :calculated_at, :calculation_note)
        ON CONFLICT (enrollment_id, subject_id)
        DO UPDATE SET final_grade = EXCLUDED.final_grade, finalized = EXCLUDED.finalized, calculated_at = EXCLUDED.calculated_at, calculation_note = EXCLUDED.calculation_note`
	now := time.Now().UTC()
	for i := range finals {
		if finals[i].ID == "" {
			finals[i].ID = uuid.NewString()
		}
		if finals[i].CalculatedAt.IsZero() {
			finals[i].CalculatedAt = now
		}
		if _, err := tx.NamedExecContext(ctx, query, finals[i]); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("upsert final grade: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit final grades: %w", err)
	}
	return nil
}

// SetFinalized toggles finalized flag for finals in scope.
func (r *GradeFinalRepository) SetFinalized(ctx context.Context, enrollmentIDs []string, subjectID string, finalized bool) error {
	if len(enrollmentIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(enrollmentIDs))
	args := make([]interface{}, len(enrollmentIDs)+2)
	args[0] = finalized
	for i, id := range enrollmentIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}
	args[len(args)-1] = subjectID
	query := fmt.Sprintf("UPDATE grade_finals SET finalized = $1 WHERE enrollment_id IN (%s) AND subject_id = $%d", strings.Join(placeholders, ","), len(args))
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("set finals finalized: %w", err)
	}
	return nil
}

// FetchByEnrollments returns existing finals for provided enrollments/subject.
func (r *GradeFinalRepository) FetchByEnrollments(ctx context.Context, enrollmentIDs []string, subjectID string) (map[string]models.GradeFinal, error) {
	result := make(map[string]models.GradeFinal, len(enrollmentIDs))
	if len(enrollmentIDs) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(enrollmentIDs))
	args := make([]interface{}, len(enrollmentIDs)+1)
	for i, id := range enrollmentIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	args[len(args)-1] = subjectID
	query := fmt.Sprintf(`SELECT id, enrollment_id, subject_id, final_grade, finalized, calculated_at, calculation_note
        FROM grade_finals WHERE enrollment_id IN (%s) AND subject_id = $%d`, strings.Join(placeholders, ","), len(args))
	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetch finals: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var final models.GradeFinal
		if err := rows.StructScan(&final); err != nil {
			return nil, fmt.Errorf("scan final: %w", err)
		}
		result[final.EnrollmentID] = final
	}
	return result, nil
}

// ReportCard returns final grades per subject for a student term scope.
func (r *GradeFinalRepository) ReportCard(ctx context.Context, studentID, termID string) ([]models.GradeReportSubject, error) {
	const query = `SELECT gf.subject_id, s.name AS subject_name, gf.final_grade
        FROM grade_finals gf
        JOIN enrollments e ON e.id = gf.enrollment_id
        JOIN subjects s ON s.id = gf.subject_id
        WHERE e.student_id = $1 AND e.term_id = $2
        ORDER BY s.name`
	var subjects []models.GradeReportSubject
	if err := r.db.SelectContext(ctx, &subjects, query, studentID, termID); err != nil {
		return nil, fmt.Errorf("report card: %w", err)
	}
	return subjects, nil
}

// ClassReportRows returns per-student rows for class report.
func (r *GradeFinalRepository) ClassReportRows(ctx context.Context, classID, subjectID, termID string) ([]models.GradeFinalReportRow, error) {
	const query = `SELECT st.id AS student_id, st.full_name AS student_name, gf.final_grade,
        CASE WHEN gf.final_grade IS NULL THEN NULL ELSE RANK() OVER (ORDER BY gf.final_grade DESC) END AS rank
        FROM grade_finals gf
        JOIN enrollments e ON e.id = gf.enrollment_id
        JOIN students st ON st.id = e.student_id
        WHERE e.class_id = $1 AND e.term_id = $2 AND gf.subject_id = $3
        ORDER BY rank NULLS LAST, st.full_name`
	var rows []models.GradeFinalReportRow
	if err := r.db.SelectContext(ctx, &rows, query, classID, termID, subjectID); err != nil {
		return nil, fmt.Errorf("class report rows: %w", err)
	}
	return rows, nil
}

// ClassDistribution aggregates metrics for a class.
func (r *GradeFinalRepository) ClassDistribution(ctx context.Context, classID, subjectID, termID string) (*models.ClassGradeDistribution, error) {
	const query = `SELECT gf.subject_id, e.term_id AS term_id,
        MIN(gf.final_grade) AS min, MAX(gf.final_grade) AS max, AVG(gf.final_grade) AS average
        FROM grade_finals gf
        JOIN enrollments e ON e.id = gf.enrollment_id
        WHERE e.class_id = $1 AND e.term_id = $2 AND gf.subject_id = $3`
	var distribution models.ClassGradeDistribution
	if err := r.db.GetContext(ctx, &distribution, query, classID, termID, subjectID); err != nil {
		return nil, fmt.Errorf("class distribution: %w", err)
	}
	return &distribution, nil
}
