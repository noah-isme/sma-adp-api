-- Seed data for SMA ADP API contract tests.
-- Run AFTER all migrations (000001-000012) are applied.
-- Password for all users: "admin123"
-- Bcrypt hash: $2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS
--
-- Contract test variables:
--   classId        = cls-001
--   studentId      = std-001
--   teacherId      = tch-001
--   gradeConfigId  = gcfg-001

-- ============================================================
-- 1. Users (3 roles for RBAC testing)
-- ============================================================
INSERT INTO users (id, email, password_hash, full_name, role, active, created_at, updated_at)
VALUES ('usr-sa-001', 'superadmin@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Super Admin', 'SUPERADMIN', TRUE, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (id, email, password_hash, full_name, role, active, created_at, updated_at)
VALUES ('usr-adm-001', 'admin@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Admin Sekolah', 'ADMIN', TRUE, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (id, email, password_hash, full_name, role, active, created_at, updated_at)
VALUES ('usr-tch-001', 'teacher@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Guru Matematika', 'TEACHER', TRUE, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 2. Terms
-- ============================================================
INSERT INTO terms (id, name, type, academic_year, start_date, end_date, is_active, created_at, updated_at)
VALUES ('term-001', 'Ganjil 2025/2026', 'SEMESTER', '2025/2026', '2025-07-01', '2025-12-31', TRUE, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 3. Subjects
-- ============================================================
INSERT INTO subjects (id, code, name, track, subject_group, created_at, updated_at)
VALUES ('subj-001', 'MTK', 'Matematika', 'IPA', 'CORE', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO subjects (id, code, name, track, subject_group, created_at, updated_at)
VALUES ('subj-002', 'FIS', 'Fisika', 'IPA', 'CORE', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 4. Classes
-- ============================================================
INSERT INTO classes (id, name, grade, track, homeroom_teacher_id, created_at, updated_at)
VALUES ('cls-001', 'X IPA 1', '10', 'IPA', NULL, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 5. Teachers (table created in migration 000007)
-- ============================================================
INSERT INTO teachers (id, user_id, nip, email, full_name, phone, expertise, active, created_at, updated_at)
VALUES ('tch-001', 'usr-tch-001', '198001012005011001', 'teacher@sma.test', 'Guru Matematika', '081234567890', 'Matematika', TRUE, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Set homeroom teacher for class
UPDATE classes SET homeroom_teacher_id = 'tch-001', updated_at = NOW() WHERE id = 'cls-001';

-- ============================================================
-- 6. Students
-- ============================================================
INSERT INTO students (id, nis, full_name, gender, birth_date, address, phone, active, created_at, updated_at)
VALUES ('std-001', '2025001', 'Ahmad Test', 'L', '2008-01-15', 'Jl. Test No. 1', '08123456789', TRUE, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO students (id, nis, full_name, gender, birth_date, address, phone, active, created_at, updated_at)
VALUES ('std-002', '2025002', 'Siti Test', 'P', '2008-03-20', 'Jl. Test No. 2', '08123456788', TRUE, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 7. Enrollments
-- ============================================================
INSERT INTO enrollments (id, student_id, class_id, term_id, joined_at, left_at, status)
VALUES ('enr-001', 'std-001', 'cls-001', 'term-001', NOW(), NULL, 'ACTIVE')
ON CONFLICT (id) DO NOTHING;

INSERT INTO enrollments (id, student_id, class_id, term_id, joined_at, left_at, status)
VALUES ('enr-002', 'std-002', 'cls-001', 'term-001', NOW(), NULL, 'ACTIVE')
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 8. Class Subjects
-- ============================================================
INSERT INTO class_subjects (id, class_id, subject_id, teacher_id, created_at)
VALUES ('cs-001', 'cls-001', 'subj-001', 'tch-001', NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO class_subjects (id, class_id, subject_id, teacher_id, created_at)
VALUES ('cs-002', 'cls-001', 'subj-002', 'tch-001', NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 9. Schedules
-- ============================================================
INSERT INTO schedules (id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room, created_at, updated_at)
VALUES ('sched-001', 'term-001', 'cls-001', 'subj-001', 'tch-001', 'MONDAY', '07:00-08:30', 'R-101', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO schedules (id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room, created_at, updated_at)
VALUES ('sched-002', 'term-001', 'cls-001', 'subj-001', 'tch-001', 'TUESDAY', '07:00-08:30', 'R-101', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 10. Grade Components
-- ============================================================
INSERT INTO grade_components (id, code, name, description, created_at, updated_at)
VALUES ('gcomp-001', 'UH', 'Ulangan Harian', 'Ulangan harian rutin', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO grade_components (id, code, name, description, created_at, updated_at)
VALUES ('gcomp-002', 'UTS', 'Ujian Tengah Semester', 'Ujian tengah semester', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO grade_components (id, code, name, description, created_at, updated_at)
VALUES ('gcomp-003', 'UAS', 'Ujian Akhir Semester', 'Ujian akhir semester', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 11. Grade Configs
-- ============================================================
INSERT INTO grade_configs (id, class_id, subject_id, term_id, calculation_scheme, finalized, created_at, updated_at)
VALUES ('gcfg-001', 'cls-001', 'subj-001', 'term-001', 'WEIGHTED', FALSE, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 12. Grade Config Components (link config to components with weights)
-- ============================================================
INSERT INTO grade_config_components (id, grade_config_id, component_id, weight)
VALUES ('gcc-001', 'gcfg-001', 'gcomp-001', 0.30)
ON CONFLICT (id) DO NOTHING;

INSERT INTO grade_config_components (id, grade_config_id, component_id, weight)
VALUES ('gcc-002', 'gcfg-001', 'gcomp-002', 0.30)
ON CONFLICT (id) DO NOTHING;

INSERT INTO grade_config_components (id, grade_config_id, component_id, weight)
VALUES ('gcc-003', 'gcfg-001', 'gcomp-003', 0.40)
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 13. Teacher Assignments (table created in migration 000007)
-- ============================================================
INSERT INTO teacher_assignments (id, teacher_id, class_id, subject_id, term_id, created_at)
VALUES ('ta-001', 'tch-001', 'cls-001', 'subj-001', 'term-001', NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 14. Teacher Preferences (table created in migration 000007)
-- ============================================================
INSERT INTO teacher_preferences (id, teacher_id, max_load_per_day, max_load_per_week, unavailable, created_at, updated_at)
VALUES ('tp-001', 'tch-001', 6, 30, '[]'::jsonb, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 15. Behavior Notes
-- ============================================================
INSERT INTO behavior_notes (id, student_id, date, note_type, points, description, created_by, created_at, updated_at)
VALUES ('bn-001', 'std-001', '2025-08-15', '+', 2, 'Aktif bertanya di kelas', 'usr-tch-001', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO behavior_notes (id, student_id, date, note_type, points, description, created_by, created_at, updated_at)
VALUES ('bn-002', 'std-001', '2025-08-20', '-', -1, 'Terlambat masuk kelas', 'usr-tch-001', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 16. Announcements
-- ============================================================
INSERT INTO announcements (id, title, content, audience, target_class_id, priority, is_pinned, published_at, expires_at, created_by, created_at, updated_at)
VALUES ('ann-001', 'Selamat Datang', 'Selamat datang di semester ganjil 2025/2026', 'ALL', NULL, 'NORMAL', FALSE, NOW(), NULL, 'usr-adm-001', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 17. Calendar Events
-- ============================================================
INSERT INTO calendar_events (id, title, description, event_type, start_date, end_date, start_time, end_time, audience, target_class_id, location, created_by, created_at, updated_at)
VALUES ('cal-001', 'Ulangan Harian Matematika', 'Ulangan harian bab 1', 'EXAM', '2025-08-25', '2025-08-25', NULL, NULL, 'CLASS', 'cls-001', 'R-101', 'usr-tch-001', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 18. Configurations (table created in migration 000012)
-- ============================================================
INSERT INTO configurations (key, value, type, description, updated_by, updated_at)
VALUES ('active_term_id', 'term-001', 'STRING', 'Active term for the current academic year', 'usr-sa-001', NOW())
ON CONFLICT (key) DO NOTHING;

INSERT INTO configurations (key, value, type, description, updated_by, updated_at)
VALUES ('school_name', 'SMA Negeri Test', 'STRING', 'School name', 'usr-sa-001', NOW())
ON CONFLICT (key) DO NOTHING;
