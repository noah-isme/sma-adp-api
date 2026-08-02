ALTER TABLE grade_components
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_grade_components_deleted_at ON grade_components (deleted_at);
