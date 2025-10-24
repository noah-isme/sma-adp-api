package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// GradeConfigRepository manages grade configuration persistence.
type GradeConfigRepository struct {
	db *sqlx.DB
}

// NewGradeConfigRepository creates a new repository instance.
func NewGradeConfigRepository(db *sqlx.DB) *GradeConfigRepository {
	return &GradeConfigRepository{db: db}
}

// List returns grade configs matching the provided filters.
func (r *GradeConfigRepository) List(ctx context.Context, filter models.FinalGradeFilter) ([]models.GradeConfig, error) {
	query := `SELECT id, class_id, subject_id, term_id, calculation_scheme, finalized, created_at, updated_at
        FROM grade_configs WHERE 1=1`
	args := []interface{}{}
	if filter.ClassID != "" {
		query += fmt.Sprintf(" AND class_id = $%d", len(args)+1)
		args = append(args, filter.ClassID)
	}
	if filter.SubjectID != "" {
		query += fmt.Sprintf(" AND subject_id = $%d", len(args)+1)
		args = append(args, filter.SubjectID)
	}
	if filter.TermID != "" {
		query += fmt.Sprintf(" AND term_id = $%d", len(args)+1)
		args = append(args, filter.TermID)
	}
	query += " ORDER BY created_at DESC"

	var configs []models.GradeConfig
	if err := r.db.SelectContext(ctx, &configs, query, args...); err != nil {
		return nil, fmt.Errorf("list grade configs: %w", err)
	}
	for i := range configs {
		comps, err := r.loadComponents(ctx, configs[i].ID)
		if err != nil {
			return nil, err
		}
		configs[i].Components = comps
	}
	return configs, nil
}

// FindByID returns a grade config by ID with components.
func (r *GradeConfigRepository) FindByID(ctx context.Context, id string) (*models.GradeConfig, error) {
	const query = `SELECT id, class_id, subject_id, term_id, calculation_scheme, finalized, created_at, updated_at FROM grade_configs WHERE id = $1`
	var config models.GradeConfig
	if err := r.db.GetContext(ctx, &config, query, id); err != nil {
		return nil, err
	}
	components, err := r.loadComponents(ctx, id)
	if err != nil {
		return nil, err
	}
	config.Components = components
	return &config, nil
}

// FindByScope retrieves a config using class+subject+term combination.
func (r *GradeConfigRepository) FindByScope(ctx context.Context, classID, subjectID, termID string) (*models.GradeConfig, error) {
	const query = `SELECT id, class_id, subject_id, term_id, calculation_scheme, finalized, created_at, updated_at FROM grade_configs WHERE class_id = $1 AND subject_id = $2 AND term_id = $3`
	var config models.GradeConfig
	if err := r.db.GetContext(ctx, &config, query, classID, subjectID, termID); err != nil {
		return nil, err
	}
	components, err := r.loadComponents(ctx, config.ID)
	if err != nil {
		return nil, err
	}
	config.Components = components
	return &config, nil
}

// Exists checks if config exists for scope, excluding optional ID.
func (r *GradeConfigRepository) Exists(ctx context.Context, classID, subjectID, termID, excludeID string) (bool, error) {
	query := "SELECT 1 FROM grade_configs WHERE class_id = $1 AND subject_id = $2 AND term_id = $3"
	args := []interface{}{classID, subjectID, termID}
	if excludeID != "" {
		query += " AND id <> $4"
		args = append(args, excludeID)
	}
	var exists int
	if err := r.db.GetContext(ctx, &exists, query+" LIMIT 1", args...); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check grade config: %w", err)
	}
	return true, nil
}

// Create inserts a config with its components.
func (r *GradeConfigRepository) Create(ctx context.Context, config *models.GradeConfig) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	if err := r.createTx(ctx, tx, config); err != nil {
		tx.Rollback() //nolint:errcheck
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit grade config: %w", err)
	}
	return nil
}

func (r *GradeConfigRepository) createTx(ctx context.Context, tx *sqlx.Tx, config *models.GradeConfig) error {
	if config.ID == "" {
		config.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if config.CreatedAt.IsZero() {
		config.CreatedAt = now
	}
	config.UpdatedAt = now
	const insertConfig = `INSERT INTO grade_configs (id, class_id, subject_id, term_id, calculation_scheme, finalized, created_at, updated_at)
        VALUES (:id, :class_id, :subject_id, :term_id, :calculation_scheme, :finalized, :created_at, :updated_at)`
	if _, err := tx.NamedExecContext(ctx, insertConfig, config); err != nil {
		return fmt.Errorf("insert grade config: %w", err)
	}
	if err := r.replaceComponentsTx(ctx, tx, config.ID, config.Components); err != nil {
		return err
	}
	return nil
}

// Update applies changes to config metadata and components.
func (r *GradeConfigRepository) Update(ctx context.Context, config *models.GradeConfig) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	config.UpdatedAt = time.Now().UTC()
	const updateQuery = `UPDATE grade_configs SET calculation_scheme = :calculation_scheme, finalized = :finalized, updated_at = :updated_at WHERE id = :id`
	if _, err := tx.NamedExecContext(ctx, updateQuery, config); err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("update grade config: %w", err)
	}
	if err := r.replaceComponentsTx(ctx, tx, config.ID, config.Components); err != nil {
		tx.Rollback() //nolint:errcheck
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit grade config: %w", err)
	}
	return nil
}

// Finalize toggles finalized flag for config.
func (r *GradeConfigRepository) Finalize(ctx context.Context, id string, finalized bool) error {
	const query = `UPDATE grade_configs SET finalized = $2, updated_at = $3 WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id, finalized, time.Now().UTC()); err != nil {
		return fmt.Errorf("finalize grade config: %w", err)
	}
	return nil
}

// replaceComponentsTx rewrites config components in a transaction.
func (r *GradeConfigRepository) replaceComponentsTx(ctx context.Context, tx *sqlx.Tx, configID string, components []models.GradeConfigComponent) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM grade_config_components WHERE grade_config_id = $1", configID); err != nil {
		return fmt.Errorf("clear grade config components: %w", err)
	}
	if len(components) == 0 {
		return nil
	}
	const insertComponent = `INSERT INTO grade_config_components (id, grade_config_id, component_id, weight)
        VALUES (:id, :grade_config_id, :component_id, :weight)`
	for i := range components {
		if components[i].ID == "" {
			components[i].ID = uuid.NewString()
		}
		components[i].GradeConfigID = configID
		if _, err := tx.NamedExecContext(ctx, insertComponent, components[i]); err != nil {
			return fmt.Errorf("insert grade config component: %w", err)
		}
	}
	return nil
}

func (r *GradeConfigRepository) loadComponents(ctx context.Context, configID string) ([]models.GradeConfigComponent, error) {
	const query = `SELECT gcc.id, gcc.grade_config_id, gcc.component_id, gcc.weight, gc.code AS component_code, gc.name AS component_name, gcc.created_at
        FROM grade_config_components gcc
        JOIN grade_components gc ON gc.id = gcc.component_id
        WHERE gcc.grade_config_id = $1 ORDER BY gc.code`
	var components []models.GradeConfigComponent
	if err := r.db.SelectContext(ctx, &components, query, configID); err != nil {
		return nil, fmt.Errorf("load grade config components: %w", err)
	}
	return components, nil
}
