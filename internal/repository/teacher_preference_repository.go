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
