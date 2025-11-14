package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// ConfigurationRepository persists configuration entries.
type ConfigurationRepository struct {
	db *sqlx.DB
}

// NewConfigurationRepository constructs the repository.
func NewConfigurationRepository(db *sqlx.DB) *ConfigurationRepository {
	return &ConfigurationRepository{db: db}
}

// ListByKeys returns configurations whose key is in the provided slice.
func (r *ConfigurationRepository) ListByKeys(ctx context.Context, keys []string) ([]models.Configuration, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	query := fmt.Sprintf(`SELECT key, value, type, description, updated_by, updated_at
FROM configurations WHERE key IN (%s) ORDER BY key ASC`, placeholders(len(keys)))
	args := make([]interface{}, len(keys))
	for i, key := range keys {
		args[i] = key
	}
	var configs []models.Configuration
	if err := r.db.SelectContext(ctx, &configs, query, args...); err != nil {
		return nil, fmt.Errorf("list configurations: %w", err)
	}
	return configs, nil
}

// Get fetches a single configuration by key.
func (r *ConfigurationRepository) Get(ctx context.Context, key string) (*models.Configuration, error) {
	const query = `SELECT key, value, type, description, updated_by, updated_at FROM configurations WHERE key = $1`
	var cfg models.Configuration
	if err := r.db.GetContext(ctx, &cfg, query, key); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Upsert inserts or updates a configuration entry.
func (r *ConfigurationRepository) Upsert(ctx context.Context, cfg *models.Configuration) error {
	const query = `INSERT INTO configurations (key, value, type, description, updated_by, updated_at)
VALUES (:key, :value, :type, :description, :updated_by, :updated_at)
ON CONFLICT (key)
DO UPDATE SET value = EXCLUDED.value, type = EXCLUDED.type, description = EXCLUDED.description,
              updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at`
	cfg.UpdatedAt = time.Now().UTC()
	if _, err := r.db.NamedExecContext(ctx, query, cfg); err != nil {
		return fmt.Errorf("upsert configuration: %w", err)
	}
	return nil
}

// BulkUpsert performs upserts within a transaction.
func (r *ConfigurationRepository) BulkUpsert(ctx context.Context, cfgs []models.Configuration) error {
	if len(cfgs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bulk configuration tx: %w", err)
	}
	const query = `INSERT INTO configurations (key, value, type, description, updated_by, updated_at)
VALUES (:key, :value, :type, :description, :updated_by, :updated_at)
ON CONFLICT (key)
DO UPDATE SET value = EXCLUDED.value, type = EXCLUDED.type, description = EXCLUDED.description,
              updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at`
	for i := range cfgs {
		cfgs[i].UpdatedAt = time.Now().UTC()
		if _, err := tx.NamedExecContext(ctx, query, cfgs[i]); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("bulk upsert configuration: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bulk configuration tx: %w", err)
	}
	return nil
}

func placeholders(n int) string {
	values := make([]string, n)
	for i := 1; i <= n; i++ {
		values[i-1] = fmt.Sprintf("$%d", i)
	}
	return strings.Join(values, ",")
}
