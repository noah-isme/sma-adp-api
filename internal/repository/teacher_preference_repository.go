package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// TeacherPreferenceRepository persists teacher preferences.
type TeacherPreferenceRepository struct {
	db *sqlx.DB
}

// NewTeacherPreferenceRepository constructs the repository.
func NewTeacherPreferenceRepository(db *sqlx.DB) *TeacherPreferenceRepository {
	return &TeacherPreferenceRepository{db: db}
}

// GetByTeacher returns stored preferences for a teacher.
func (r *TeacherPreferenceRepository) GetByTeacher(ctx context.Context, teacherID string) (*models.TeacherPreference, error) {
	const query = `SELECT id, teacher_id, max_load_per_day, max_load_per_week, unavailable, created_at, updated_at FROM teacher_preferences WHERE teacher_id = $1`
	var pref models.TeacherPreference
	if err := r.db.GetContext(ctx, &pref, query, teacherID); err != nil {
		return nil, err
	}
	return &pref, nil
}

// Upsert creates or updates teacher preferences.
func (r *TeacherPreferenceRepository) Upsert(ctx context.Context, pref *models.TeacherPreference) error {
	if pref.ID == "" {
		pref.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if pref.CreatedAt.IsZero() {
		pref.CreatedAt = now
	}
	pref.UpdatedAt = now
	if len(pref.Unavailable) == 0 {
		pref.Unavailable = []byte("[]")
	}

	const query = `INSERT INTO teacher_preferences (id, teacher_id, max_load_per_day, max_load_per_week, unavailable, created_at, updated_at)
		VALUES (:id, :teacher_id, :max_load_per_day, :max_load_per_week, :unavailable, :created_at, :updated_at)
		ON CONFLICT (teacher_id) DO UPDATE
		SET max_load_per_day = EXCLUDED.max_load_per_day,
		    max_load_per_week = EXCLUDED.max_load_per_week,
		    unavailable = EXCLUDED.unavailable,
		    updated_at = EXCLUDED.updated_at`
	if _, err := r.db.NamedExecContext(ctx, query, pref); err != nil {
		return fmt.Errorf("upsert teacher preference: %w", err)
	}
	return nil
}

// ListAll returns all teacher preferences with pagination.
func (r *TeacherPreferenceRepository) ListAll(ctx context.Context, filter models.TeacherPreferenceFilter) ([]models.TeacherPreference, int, error) {
	conditions := []string{}
	args := []interface{}{}
	argIndex := 1

	if filter.TeacherID != "" {
		conditions = append(conditions, fmt.Sprintf("teacher_id = $%d", argIndex))
		args = append(args, filter.TeacherID)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM teacher_preferences %s`, whereClause)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count all teacher preferences: %w", err)
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.PageSize
	if size <= 0 {
		size = 20
	}
	if size > 1000 {
		size = 1000
	}

	offset := (page - 1) * size

	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "teacher_id"
	}
	sortOrder := filter.SortOrder
	if sortOrder == "" {
		sortOrder = "ASC"
	}

	query := fmt.Sprintf(`
SELECT id, teacher_id, max_load_per_day, max_load_per_week, unavailable, created_at, updated_at
FROM teacher_preferences
%s
ORDER BY %s %s
LIMIT $%d OFFSET $%d`, whereClause, sortBy, sortOrder, argIndex, argIndex+1)
	args = append(args, size, offset)

	var prefs []models.TeacherPreference
	if err := r.db.SelectContext(ctx, &prefs, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list all teacher preferences: %w", err)
	}
	return prefs, total, nil
}
