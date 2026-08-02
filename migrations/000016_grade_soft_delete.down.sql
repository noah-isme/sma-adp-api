DROP INDEX IF EXISTS idx_grades_deleted_at;
ALTER TABLE grades
    DROP COLUMN IF EXISTS deleted_at;
