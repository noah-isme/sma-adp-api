-- Reverse migration 000014: drop user_id from teachers.
DROP INDEX IF EXISTS idx_teachers_user_id;
ALTER TABLE teachers
    DROP COLUMN IF EXISTS user_id;
