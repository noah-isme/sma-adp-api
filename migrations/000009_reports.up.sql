CREATE TABLE IF NOT EXISTS report_jobs (
    id VARCHAR(36) PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    params JSONB DEFAULT '{}'::jsonb,
    status VARCHAR(20) DEFAULT 'QUEUED',
    progress INT DEFAULT 0,
    result_url TEXT,
    created_by VARCHAR(36) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_report_jobs_status ON report_jobs(status);
