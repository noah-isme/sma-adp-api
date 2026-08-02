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

// GradeComponentRepository manages grade component persistence.
type GradeComponentRepository struct {
	db *sqlx.DB
}

// NewGradeComponentRepository creates a repository instance.
func NewGradeComponentRepository(db *sqlx.DB) *GradeComponentRepository {
	return &GradeComponentRepository{db: db}
}

// List returns all grade components optionally filtered by search query.
func (r *GradeComponentRepository) List(ctx context.Context, search string) ([]models.GradeComponent, error) {
	query := "SELECT id, code, name, description, created_at, updated_at FROM grade_components WHERE deleted_at IS NULL"
	var args []interface{}
	if search != "" {
		query += " AND (LOWER(name) LIKE $1 OR LOWER(code) LIKE $1)"
		args = append(args, "%"+strings.ToLower(search)+"%")
	}
	query += " ORDER BY created_at"
	var components []models.GradeComponent
	if err := r.db.SelectContext(ctx, &components, query, args...); err != nil {
		return nil, fmt.Errorf("list grade components: %w", err)
	}
	return components, nil
}

// FindByID returns a component by its ID.
func (r *GradeComponentRepository) FindByID(ctx context.Context, id string) (*models.GradeComponent, error) {
	const query = `SELECT id, code, name, description, created_at, updated_at FROM grade_components WHERE id = $1 AND deleted_at IS NULL`
	var component models.GradeComponent
	if err := r.db.GetContext(ctx, &component, query, id); err != nil {
		return nil, err
	}
	return &component, nil
}

// FindByCode returns a component by its code.
func (r *GradeComponentRepository) FindByCode(ctx context.Context, code string) (*models.GradeComponent, error) {
	const query = `SELECT id, code, name, description, created_at, updated_at FROM grade_components WHERE code = $1 AND deleted_at IS NULL`
	var component models.GradeComponent
	if err := r.db.GetContext(ctx, &component, query, code); err != nil {
		return nil, err
	}
	return &component, nil
}

// ExistsByCode checks whether a component code is already used.
func (r *GradeComponentRepository) ExistsByCode(ctx context.Context, code string, excludeID string) (bool, error) {
	query := "SELECT 1 FROM grade_components WHERE code = $1 AND deleted_at IS NULL"
	args := []interface{}{code}
	if excludeID != "" {
		query += " AND id <> $2"
		args = append(args, excludeID)
	}
	var exists int
	if err := r.db.GetContext(ctx, &exists, query+" LIMIT 1", args...); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check grade component: %w", err)
	}
	return true, nil
}

// Create inserts a new grade component.
func (r *GradeComponentRepository) Create(ctx context.Context, component *models.GradeComponent) error {
	if component.ID == "" {
		component.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if component.CreatedAt.IsZero() {
		component.CreatedAt = now
	}
	component.UpdatedAt = now
	const query = `INSERT INTO grade_components (id, code, name, description, created_at, updated_at)
        VALUES (:id, :code, :name, :description, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, component); err != nil {
		return fmt.Errorf("create grade component: %w", err)
	}
	return nil
}

// Update modifies an active grade component.
func (r *GradeComponentRepository) Update(ctx context.Context, component *models.GradeComponent) error {
	component.UpdatedAt = time.Now().UTC()
	const query = `UPDATE grade_components
        SET code = :code, name = :name, description = :description, updated_at = :updated_at
        WHERE id = :id AND deleted_at IS NULL`
	result, err := r.db.NamedExecContext(ctx, query, component)
	if err != nil {
		return fmt.Errorf("update grade component: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("update grade component rows: %w", err)
	} else if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete soft-deletes a grade component while retaining references from old grades/configs.
func (r *GradeComponentRepository) Delete(ctx context.Context, id string) error {
	const query = `UPDATE grade_components
        SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
        WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete grade component: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("delete grade component rows: %w", err)
	} else if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
