ALTER TABLE grades
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_grades_deleted_at ON grades (deleted_at);
