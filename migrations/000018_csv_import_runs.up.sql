CREATE TABLE IF NOT EXISTS import_runs (
    id VARCHAR(36) PRIMARY KEY,
    import_type VARCHAR(50) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PROCESSING',
    created_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    result JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    UNIQUE(import_type, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_import_runs_created_at ON import_runs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_import_runs_user_id ON import_runs(user_id);
