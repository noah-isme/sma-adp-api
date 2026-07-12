-- Add created_at column to grade_config_components.
-- The GradeConfigComponent model and loadComponents query reference gcc.created_at,
-- but the initial migration (000001) never created this column.
-- This fixes the 500 error on GET /grade-configs and GET /grade-configs/{id}.
ALTER TABLE grade_config_components
    ADD COLUMN IF NOT EXISTS created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP;
