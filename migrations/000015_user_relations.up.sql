ALTER TABLE users ADD COLUMN IF NOT EXISTS teacher_id VARCHAR(36);
ALTER TABLE users ADD COLUMN IF NOT EXISTS student_id VARCHAR(36);
ALTER TABLE users ADD COLUMN IF NOT EXISTS class_id VARCHAR(36);
CREATE INDEX IF NOT EXISTS idx_users_teacher_id ON users(teacher_id);
CREATE INDEX IF NOT EXISTS idx_users_student_id ON users(student_id);
CREATE INDEX IF NOT EXISTS idx_users_class_id ON users(class_id);
