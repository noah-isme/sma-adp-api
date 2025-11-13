DROP INDEX IF EXISTS uq_homeroom_class_term;

ALTER TABLE teacher_assignments
    DROP COLUMN IF EXISTS role;

DELETE FROM subjects WHERE UPPER(code) = 'HOMEROOM';
