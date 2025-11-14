CREATE TABLE IF NOT EXISTS configurations (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL,
    type VARCHAR(20) NOT NULL,
    description TEXT,
    updated_by VARCHAR(36),
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
