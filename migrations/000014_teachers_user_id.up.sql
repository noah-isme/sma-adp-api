-- Add user_id column to teachers table.
-- Links teacher records to user accounts for role-based resolution.
-- Used by DashboardHandler.Teacher to resolve claims.UserID -> teacher.ID.
-- Nullable for backward compatibility: existing teachers without a user account remain valid.
ALTER TABLE teachers
    ADD COLUMN IF NOT EXISTS user_id varchar(36);

-- Optional: add a unique index so one user maps to at most one teacher.
CREATE UNIQUE INDEX IF NOT EXISTS idx_teachers_user_id ON teachers (user_id) WHERE user_id IS NOT NULL;
