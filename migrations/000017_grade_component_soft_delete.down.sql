DROP INDEX IF EXISTS idx_grade_components_deleted_at;
ALTER TABLE grade_components
    DROP COLUMN IF EXISTS deleted_at;
