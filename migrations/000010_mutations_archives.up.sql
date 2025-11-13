CREATE TABLE IF NOT EXISTS mutations (
    id VARCHAR(36) PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    entity VARCHAR(50) NOT NULL,
    entity_id VARCHAR(36) NOT NULL,
    current_snapshot JSONB NOT NULL,
    requested_changes JSONB NOT NULL,
    status VARCHAR(20) DEFAULT 'PENDING',
    reason TEXT,
    requested_by VARCHAR(36) NOT NULL,
    reviewed_by VARCHAR(36),
    requested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    reviewed_at TIMESTAMP,
    note TEXT
);
CREATE INDEX IF NOT EXISTS idx_mutations_status ON mutations(status);

CREATE TABLE IF NOT EXISTS archives (
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    category VARCHAR(50) NOT NULL,
    scope VARCHAR(50) NOT NULL,
    ref_term_id VARCHAR(36),
    ref_class_id VARCHAR(36),
    ref_student_id VARCHAR(36),
    file_path TEXT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    uploaded_by VARCHAR(36) NOT NULL,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_archives_scope ON archives(scope, category);
