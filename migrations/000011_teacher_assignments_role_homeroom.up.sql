ALTER TABLE teacher_assignments
    ADD COLUMN IF NOT EXISTS role VARCHAR(30) NOT NULL DEFAULT 'SUBJECT_TEACHER';

CREATE UNIQUE INDEX IF NOT EXISTS uq_homeroom_class_term
    ON teacher_assignments(class_id, term_id)
    WHERE role = 'HOMEROOM';

INSERT INTO subjects (id, code, name, track, subject_group, created_at, updated_at)
SELECT 'homeroom-subject', 'HOMEROOM', 'Homeroom', 'GENERAL', 'CORE', NOW(), NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM subjects WHERE UPPER(code) = 'HOMEROOM'
);
