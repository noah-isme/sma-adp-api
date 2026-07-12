-- Reverse migration 000013: drop created_at from grade_config_components.
ALTER TABLE grade_config_components
    DROP COLUMN IF EXISTS created_at;
