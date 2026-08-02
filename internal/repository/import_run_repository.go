package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// ImportRunRepository persists CSV idempotency records and their audit events.
type ImportRunRepository struct {
	db *sqlx.DB
}

// NewImportRunRepository constructs an import run repository.
func NewImportRunRepository(db *sqlx.DB) *ImportRunRepository {
	return &ImportRunRepository{db: db}
}

// Begin claims an idempotency key. created is false when a prior request used
// the same key, allowing handlers to replay or reject it safely.
func (r *ImportRunRepository) Begin(ctx context.Context, importType, key, requestHash string, userID *string) (*models.ImportRun, bool, error) {
	run := &models.ImportRun{
		ID:             uuid.NewString(),
		ImportType:     importType,
		IdempotencyKey: key,
		RequestHash:    requestHash,
		UserID:         userID,
		Status:         "PROCESSING",
		CreatedAt:      time.Now().UTC(),
	}
	const insert = `INSERT INTO import_runs
        (id, import_type, idempotency_key, request_hash, user_id, status, created_at)
        VALUES (:id, :import_type, :idempotency_key, :request_hash, :user_id, :status, :created_at)
        ON CONFLICT (import_type, idempotency_key) DO NOTHING`
	result, err := r.db.NamedExecContext(ctx, insert, run)
	if err != nil {
		return nil, false, fmt.Errorf("begin import run: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 1 {
		return run, true, nil
	}
	var existing models.ImportRun
	if err := r.db.GetContext(ctx, &existing, `SELECT id, import_type, idempotency_key, request_hash, user_id, status, created_count, failed_count, result, created_at, completed_at
        FROM import_runs WHERE import_type = $1 AND idempotency_key = $2`, importType, key); err != nil {
		return nil, false, fmt.Errorf("load import run: %w", err)
	}
	return &existing, false, nil
}

// Complete stores the result and creates an audit log in the same transaction.
func (r *ImportRunRepository) Complete(ctx context.Context, runID, importType string, created, failed int, result []byte, userID *string, ipAddress, userAgent string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin import completion: %w", err)
	}
	const update = `UPDATE import_runs SET status = 'COMPLETED', created_count = $2, failed_count = $3, result = $4, completed_at = CURRENT_TIMESTAMP WHERE id = $1`
	if _, err := tx.ExecContext(ctx, update, runID, created, failed, result); err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("complete import run: %w", err)
	}
	const audit = `INSERT INTO audit_logs
        (id, user_id, action, resource, resource_id, new_values, ip_address, user_agent, created_at)
        VALUES (:id, :user_id, :action, :resource, :resource_id, :new_values, :ip_address, :user_agent, :created_at)`
	auditLog := models.AuditLog{
		ID:         uuid.NewString(),
		UserID:     userID,
		Action:     models.AuditActionCSVImport,
		Resource:   importType,
		ResourceID: &runID,
		NewValues:  result,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		CreatedAt:  time.Now().UTC(),
	}
	if _, err := tx.NamedExecContext(ctx, audit, auditLog); err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("audit import run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit import run: %w", err)
	}
	return nil
}
