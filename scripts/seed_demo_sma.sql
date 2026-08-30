-- ==============================================================================
-- Realistic Demo Seed Data for SMA Admin Panel API
-- Sekolah: SMA Negeri 1 Nusantara
-- Tahun Ajaran: 2025/2026 Semester Ganjil
-- All accounts password: "admin123"
-- Bcrypt: $2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS
-- ==============================================================================

-- ------------------------------------------------------------------------------
-- 1. Academic Terms
-- ------------------------------------------------------------------------------
INSERT INTO terms (id, name, type, academic_year, start_date, end_date, is_active, created_at, updated_at)
VALUES 
    ('term-2025-1', 'Tahun Ajaran 2025/2026 - Semester Ganjil', 'SEMESTER', '2025/2026', '2025-07-14 00:00:00', '2025-12-20 23:59:59', TRUE, NOW(), NOW()),
    ('term-2024-2', 'Tahun Ajaran 2024/2025 - Semester Genap', 'SEMESTER', '2024/2025', '2025-01-06 00:00:00', '2025-06-21 23:59:59', FALSE, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    name = EXCLUDED.name, 
    is_active = EXCLUDED.is_active,
    start_date = EXCLUDED.start_date,
    end_date = EXCLUDED.end_date;

-- ------------------------------------------------------------------------------
-- 2. School Configurations
-- ------------------------------------------------------------------------------
INSERT INTO configurations (key, value, type, description, updated_at)
VALUES
    ('school_name', 'SMA Negeri 1 Nusantara', 'STRING', 'Nama resmi sekolah', NOW()),
    ('school_npsn', '20108942', 'STRING', 'Nomor Pokok Sekolah Nasional', NOW()),
    ('school_address', 'Jl. Pendidikan No. 45, Kebayoran Baru, Jakarta Selatan', 'STRING', 'Alamat sekolah', NOW()),
    ('school_phone', '(021) 7201234', 'STRING', 'Nomor telepon kantor tata usaha', NOW()),
    ('school_email', 'info@sman1nusantara.sch.id', 'STRING', 'Email resmi sekolah', NOW()),
    ('headmaster_name', 'Drs. H. Bambang Sudirman, M.Pd.', 'STRING', 'Kepala Sekolah aktif', NOW()),
    ('headmaster_nip', '196805121993031004', 'STRING', 'NIP Kepala Sekolah', NOW()),
    ('active_term_id', 'term-2025-1', 'STRING', 'Term semester aktif saat ini', NOW()),
    ('kkm_default', '75.0', 'FLOAT', 'Kriteria Ketuntasan Minimal standar', NOW()),
    ('curriculum_type', 'Kurikulum Merdeka & K13', 'STRING', 'Kurikulum operasional sekolah', NOW())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();

-- ------------------------------------------------------------------------------
-- 3. Core Users (Admin, Staff, Teachers)
-- ------------------------------------------------------------------------------
INSERT INTO users (id, email, password_hash, full_name, role, active, created_at, updated_at)
VALUES 
    -- Superadmin & Admin
    ('usr-sa-001', 'superadmin@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Drs. H. Bambang Sudirman, M.Pd.', 'SUPERADMIN', TRUE, NOW(), NOW()),
    ('usr-adm-001', 'admin@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Siti Rahmawati, S.Kom. (Admin TU)', 'ADMIN', TRUE, NOW(), NOW()),
    ('usr-adm-002', 'kurikulum@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Dra. Endang Purwanti (Waka Kurikulum)', 'ADMIN', TRUE, NOW(), NOW()),
    
    -- Teachers
    ('usr-tch-001', 'teacher@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Budi Santoso, S.Pd., M.Si.', 'TEACHER', TRUE, NOW(), NOW()),
    ('usr-tch-002', 'ratna.sari@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Ratna Sari Dewi, M.Pd.', 'TEACHER', TRUE, NOW(), NOW()),
    ('usr-tch-003', 'hendra.wijaya@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Hendra Wijaya, S.Si.', 'TEACHER', TRUE, NOW(), NOW()),
    ('usr-tch-004', 'dewi.lestari@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Dewi Lestari, S.Pd.', 'TEACHER', TRUE, NOW(), NOW()),
    ('usr-tch-005', 'agus.setiawan@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Agus Setiawan, S.Pd., M.Hum.', 'TEACHER', TRUE, NOW(), NOW()),
    ('usr-tch-006', 'sri.wahyuni@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Sri Wahyuni, M.A.', 'TEACHER', TRUE, NOW(), NOW()),
    ('usr-tch-007', 'ahmad.fauzi@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Ahmad Fauzi, S.E., M.M.', 'TEACHER', TRUE, NOW(), NOW()),
    ('usr-tch-008', 'rini.astuti@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Rini Astuti, S.Sos.', 'TEACHER', TRUE, NOW(), NOW()),
    ('usr-tch-009', 'eko.prasetyo@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Eko Prasetyo, S.Pd.', 'TEACHER', TRUE, NOW(), NOW()),
    ('usr-tch-010', 'nurul.hidayah@sma.test', '$2a$10$qPwguS/gOZMQVrf./XsWSOmvK86wCIt7kj4H5rg7xyPbjwfHv4VjS', 'Nurul Hidayah, S.Ag.', 'TEACHER', TRUE, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    email = EXCLUDED.email, 
    full_name = EXCLUDED.full_name, 
    role = EXCLUDED.role,
    active = TRUE;

-- ------------------------------------------------------------------------------
-- 4. Teachers Entities
-- ------------------------------------------------------------------------------
INSERT INTO teachers (id, user_id, nip, email, full_name, phone, expertise, active, created_at, updated_at)
VALUES 
    ('tch-001', 'usr-tch-001', '198203152006041008', 'teacher@sma.test', 'Budi Santoso, S.Pd., M.Si.', '081234567801', 'Matematika Peminatan & Wajib', TRUE, NOW(), NOW()),
    ('tch-002', 'usr-tch-002', '198507202009022004', 'ratna.sari@sma.test', 'Ratna Sari Dewi, M.Pd.', '081234567802', 'Fisika & Olimpiade Sains', TRUE, NOW(), NOW()),
    ('tch-003', 'usr-tch-003', '198009112005011003', 'hendra.wijaya@sma.test', 'Hendra Wijaya, S.Si.', '081234567803', 'Kimia Analitik & Organik', TRUE, NOW(), NOW()),
    ('tch-004', 'usr-tch-004', '198812042011012009', 'dewi.lestari@sma.test', 'Dewi Lestari, S.Pd.', '081234567804', 'Biologi & Genetika', TRUE, NOW(), NOW()),
    ('tch-005', 'usr-tch-005', '197906182003121002', 'agus.setiawan@sma.test', 'Agus Setiawan, S.Pd., M.Hum.', '081234567805', 'Bahasa & Sastra Indonesia', TRUE, NOW(), NOW()),
    ('tch-006', 'usr-tch-006', '198604252010012011', 'sri.wahyuni@sma.test', 'Sri Wahyuni, M.A.', '081234567806', 'Bahasa Inggris & TOEFL Prep', TRUE, NOW(), NOW()),
    ('tch-007', 'usr-tch-007', '198301302008011006', 'ahmad.fauzi@sma.test', 'Ahmad Fauzi, S.E., M.M.', '081234567807', 'Ekonomi & Akuntansi', TRUE, NOW(), NOW()),
    ('tch-008', 'usr-tch-008', '198711152012022003', 'rini.astuti@sma.test', 'Rini Astuti, S.Sos.', '081234567808', 'Sosiologi & Antropologi', TRUE, NOW(), NOW()),
    ('tch-009', 'usr-tch-009', '199008222015031005', 'eko.prasetyo@sma.test', 'Eko Prasetyo, S.Pd.', '081234567809', 'Sejarah Indonesia & Pkn', TRUE, NOW(), NOW()),
    ('tch-010', 'usr-tch-010', '198402172007012007', 'nurul.hidayah@sma.test', 'Nurul Hidayah, S.Ag.', '081234567810', 'Pendidikan Agama & Budi Pekerti', TRUE, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    user_id = EXCLUDED.user_id,
    nip = EXCLUDED.nip,
    full_name = EXCLUDED.full_name,
    expertise = EXCLUDED.expertise;

-- Link teacher_id back to users table
UPDATE users SET teacher_id = 'tch-001' WHERE id = 'usr-tch-001';
UPDATE users SET teacher_id = 'tch-002' WHERE id = 'usr-tch-002';
UPDATE users SET teacher_id = 'tch-003' WHERE id = 'usr-tch-003';
UPDATE users SET teacher_id = 'tch-004' WHERE id = 'usr-tch-004';
UPDATE users SET teacher_id = 'tch-005' WHERE id = 'usr-tch-005';
UPDATE users SET teacher_id = 'tch-006' WHERE id = 'usr-tch-006';
UPDATE users SET teacher_id = 'tch-007' WHERE id = 'usr-tch-007';
UPDATE users SET teacher_id = 'tch-008' WHERE id = 'usr-tch-008';
UPDATE users SET teacher_id = 'tch-009' WHERE id = 'usr-tch-009';
UPDATE users SET teacher_id = 'tch-010' WHERE id = 'usr-tch-010';

-- ------------------------------------------------------------------------------
-- 5. Classes
-- ------------------------------------------------------------------------------
INSERT INTO classes (id, name, grade, track, homeroom_teacher_id, created_at, updated_at)
VALUES 
    ('cls-x-ipa-1', 'X MIPA 1', '10', 'IPA', 'tch-001', NOW(), NOW()),
    ('cls-x-ips-1', 'X IPS 1', '10', 'IPS', 'tch-006', NOW(), NOW()),
    ('cls-xi-ipa-1', 'XI MIPA 1', '11', 'IPA', 'tch-003', NOW(), NOW()),
    ('cls-xi-ips-1', 'XI IPS 1', '11', 'IPS', 'tch-007', NOW(), NOW()),
    ('cls-xii-ipa-1', 'XII MIPA 1', '12', 'IPA', 'tch-005', NOW(), NOW()),
    ('cls-xii-ips-1', 'XII IPS 1', '12', 'IPS', 'tch-008', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    name = EXCLUDED.name, 
    grade = EXCLUDED.grade, 
    track = EXCLUDED.track, 
    homeroom_teacher_id = EXCLUDED.homeroom_teacher_id;

-- ------------------------------------------------------------------------------
-- 6. Subjects (Kurikulum Merdeka SMA)
-- ------------------------------------------------------------------------------
INSERT INTO subjects (id, code, name, track, subject_group, created_at, updated_at)
VALUES 
    ('subj-001', 'MTK-W', 'Matematika Wajib', 'GENERAL', 'CORE', NOW(), NOW()),
    ('subj-002', 'FIS', 'Fisika', 'IPA', 'CORE', NOW(), NOW()),
    ('subj-mtk-p', 'MTK-P', 'Matematika Peminatan', 'IPA', 'ELECTIVE', NOW(), NOW()),
    ('subj-kim', 'KIM', 'Kimia', 'IPA', 'CORE', NOW(), NOW()),
    ('subj-bio', 'BIO', 'Biologi', 'IPA', 'CORE', NOW(), NOW()),
    ('subj-eko', 'EKO', 'Ekonomi', 'IPS', 'CORE', NOW(), NOW()),
    ('subj-sos', 'SOS', 'Sosiologi', 'IPS', 'CORE', NOW(), NOW()),
    ('subj-geo', 'GEO', 'Geografi', 'IPS', 'CORE', NOW(), NOW()),
    ('subj-ind', 'BINDO', 'Bahasa Indonesia', 'GENERAL', 'CORE', NOW(), NOW()),
    ('subj-ing', 'BING', 'Bahasa Inggris', 'GENERAL', 'CORE', NOW(), NOW()),
    ('subj-sej', 'SEJ', 'Sejarah Indonesia', 'GENERAL', 'CORE', NOW(), NOW()),
    ('subj-pai', 'PAI', 'Pendidikan Agama & Budi Pekerti', 'GENERAL', 'CORE', NOW(), NOW()),
    ('subj-pjok', 'PJOK', 'Pendidikan Jasmani & Olahraga', 'GENERAL', 'CORE', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    code = EXCLUDED.code,
    name = EXCLUDED.name, 
    track = EXCLUDED.track, 
    subject_group = EXCLUDED.subject_group;

-- ------------------------------------------------------------------------------
-- 7. Class Subjects Association
-- ------------------------------------------------------------------------------
INSERT INTO class_subjects (id, class_id, subject_id, teacher_id, created_at)
VALUES
    -- X MIPA 1
    ('cs-x1-mtk', 'cls-x-ipa-1', 'subj-001', 'tch-001', NOW()),
    ('cs-x1-fis', 'cls-x-ipa-1', 'subj-002', 'tch-002', NOW()),
    ('cs-x1-kim', 'cls-x-ipa-1', 'subj-kim', 'tch-003', NOW()),
    ('cs-x1-bio', 'cls-x-ipa-1', 'subj-bio', 'tch-004', NOW()),
    ('cs-x1-ind', 'cls-x-ipa-1', 'subj-ind', 'tch-005', NOW()),
    ('cs-x1-ing', 'cls-x-ipa-1', 'subj-ing', 'tch-006', NOW()),
    
    -- X IPS 1
    ('cs-x2-mtk', 'cls-x-ips-1', 'subj-001', 'tch-001', NOW()),
    ('cs-x2-eko', 'cls-x-ips-1', 'subj-eko', 'tch-007', NOW()),
    ('cs-x2-sos', 'cls-x-ips-1', 'subj-sos', 'tch-008', NOW()),
    ('cs-x2-geo', 'cls-x-ips-1', 'subj-geo', 'tch-007', NOW()),
    ('cs-x2-ind', 'cls-x-ips-1', 'subj-ind', 'tch-005', NOW()),
    ('cs-x2-ing', 'cls-x-ips-1', 'subj-ing', 'tch-006', NOW()),

    -- XI MIPA 1
    ('cs-xi1-mtk', 'cls-xi-ipa-1', 'subj-001', 'tch-001', NOW()),
    ('cs-xi1-mtkp', 'cls-xi-ipa-1', 'subj-mtk-p', 'tch-001', NOW()),
    ('cs-xi1-fis', 'cls-xi-ipa-1', 'subj-002', 'tch-002', NOW()),
    ('cs-xi1-kim', 'cls-xi-ipa-1', 'subj-kim', 'tch-003', NOW()),
    ('cs-xi1-bio', 'cls-xi-ipa-1', 'subj-bio', 'tch-004', NOW()),
    ('cs-xi1-ing', 'cls-xi-ipa-1', 'subj-ing', 'tch-006', NOW()),

    -- XII MIPA 1
    ('cs-xii1-mtk', 'cls-xii-ipa-1', 'subj-001', 'tch-001', NOW()),
    ('cs-xii1-fis', 'cls-xii-ipa-1', 'subj-002', 'tch-002', NOW()),
    ('cs-xii1-kim', 'cls-xii-ipa-1', 'subj-kim', 'tch-003', NOW()),
    ('cs-xii1-bio', 'cls-xii-ipa-1', 'subj-bio', 'tch-004', NOW()),
    ('cs-xii1-ind', 'cls-xii-ipa-1', 'subj-ind', 'tch-005', NOW()),
    ('cs-xii1-sej', 'cls-xii-ipa-1', 'subj-sej', 'tch-009', NOW())
ON CONFLICT (id) DO UPDATE SET teacher_id = EXCLUDED.teacher_id;

-- ------------------------------------------------------------------------------
-- 8. Teacher Assignments
-- ------------------------------------------------------------------------------
INSERT INTO teacher_assignments (id, teacher_id, class_id, subject_id, term_id, role, created_at)
VALUES 
    -- Homeroom Assignments (using homeroom-subject)
    ('ta-hr-001', 'tch-001', 'cls-x-ipa-1', 'homeroom-subject', 'term-2025-1', 'HOMEROOM', NOW()),
    ('ta-hr-002', 'tch-006', 'cls-x-ips-1', 'homeroom-subject', 'term-2025-1', 'HOMEROOM', NOW()),
    ('ta-hr-003', 'tch-003', 'cls-xi-ipa-1', 'homeroom-subject', 'term-2025-1', 'HOMEROOM', NOW()),
    ('ta-hr-004', 'tch-007', 'cls-xi-ips-1', 'homeroom-subject', 'term-2025-1', 'HOMEROOM', NOW()),
    ('ta-hr-005', 'tch-005', 'cls-xii-ipa-1', 'homeroom-subject', 'term-2025-1', 'HOMEROOM', NOW()),
    ('ta-hr-006', 'tch-008', 'cls-xii-ips-1', 'homeroom-subject', 'term-2025-1', 'HOMEROOM', NOW()),
    
    -- Subject Teacher Assignments
    ('ta-sub-001', 'tch-001', 'cls-x-ipa-1', 'subj-001', 'term-2025-1', 'SUBJECT_TEACHER', NOW()),
    ('ta-sub-002', 'tch-002', 'cls-x-ipa-1', 'subj-002', 'term-2025-1', 'SUBJECT_TEACHER', NOW()),
    ('ta-sub-003', 'tch-003', 'cls-x-ipa-1', 'subj-kim', 'term-2025-1', 'SUBJECT_TEACHER', NOW()),
    ('ta-sub-004', 'tch-004', 'cls-x-ipa-1', 'subj-bio', 'term-2025-1', 'SUBJECT_TEACHER', NOW()),
    ('ta-sub-005', 'tch-005', 'cls-x-ipa-1', 'subj-ind', 'term-2025-1', 'SUBJECT_TEACHER', NOW()),
    ('ta-sub-006', 'tch-006', 'cls-x-ipa-1', 'subj-ing', 'term-2025-1', 'SUBJECT_TEACHER', NOW()),
    ('ta-sub-007', 'tch-007', 'cls-x-ips-1', 'subj-eko', 'term-2025-1', 'SUBJECT_TEACHER', NOW()),
    ('ta-sub-008', 'tch-008', 'cls-x-ips-1', 'subj-sos', 'term-2025-1', 'SUBJECT_TEACHER', NOW())
ON CONFLICT (id) DO UPDATE SET role = EXCLUDED.role;

-- ------------------------------------------------------------------------------
-- 9. Weekly Schedules
-- ------------------------------------------------------------------------------
INSERT INTO schedules (id, term_id, class_id, subject_id, teacher_id, day_of_week, time_slot, room, created_at, updated_at)
VALUES
    -- X MIPA 1 (Senin s/d Jumat)
    ('sch-x1-mon1', 'term-2025-1', 'cls-x-ipa-1', 'subj-001', 'tch-001', 'MONDAY', '07:30-09:00', 'R-101', NOW(), NOW()),
    ('sch-x1-mon2', 'term-2025-1', 'cls-x-ipa-1', 'subj-002', 'tch-002', 'MONDAY', '09:15-10:45', 'Lab Fisika', NOW(), NOW()),
    ('sch-x1-tue1', 'term-2025-1', 'cls-x-ipa-1', 'subj-kim', 'tch-003', 'TUESDAY', '07:30-09:00', 'Lab Kimia', NOW(), NOW()),
    ('sch-x1-tue2', 'term-2025-1', 'cls-x-ipa-1', 'subj-bio', 'tch-004', 'TUESDAY', '09:15-10:45', 'Lab Biologi', NOW(), NOW()),
    ('sch-x1-wed1', 'term-2025-1', 'cls-x-ipa-1', 'subj-ind', 'tch-005', 'WEDNESDAY', '07:30-09:00', 'R-101', NOW(), NOW()),
    ('sch-x1-wed2', 'term-2025-1', 'cls-x-ipa-1', 'subj-ing', 'tch-006', 'WEDNESDAY', '09:15-10:45', 'Lab Bahasa', NOW(), NOW()),
    ('sch-x1-thu1', 'term-2025-1', 'cls-x-ipa-1', 'subj-pai', 'tch-010', 'THURSDAY', '07:30-09:00', 'R-101', NOW(), NOW()),
    ('sch-x1-fri1', 'term-2025-1', 'cls-x-ipa-1', 'subj-pjok', 'tch-009', 'FRIDAY', '07:00-08:30', 'Lapangan Olahraga', NOW(), NOW()),

    -- X IPS 1
    ('sch-x2-mon1', 'term-2025-1', 'cls-x-ips-1', 'subj-eko', 'tch-007', 'MONDAY', '07:30-09:00', 'R-102', NOW(), NOW()),
    ('sch-x2-mon2', 'term-2025-1', 'cls-x-ips-1', 'subj-sos', 'tch-008', 'MONDAY', '09:15-10:45', 'R-102', NOW(), NOW()),
    ('sch-x2-tue1', 'term-2025-1', 'cls-x-ips-1', 'subj-geo', 'tch-007', 'TUESDAY', '07:30-09:00', 'R-102', NOW(), NOW()),
    ('sch-x2-wed1', 'term-2025-1', 'cls-x-ips-1', 'subj-ind', 'tch-005', 'WEDNESDAY', '07:30-09:00', 'R-102', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    day_of_week = EXCLUDED.day_of_week, 
    time_slot = EXCLUDED.time_slot, 
    room = EXCLUDED.room;

-- ------------------------------------------------------------------------------
-- 10. Students Data
-- ------------------------------------------------------------------------------
INSERT INTO students (id, nis, full_name, gender, birth_date, address, phone, active, created_at, updated_at)
VALUES 
    -- Kelas X MIPA 1 Students
    ('std-001', '20250101', 'Muhammad Rizky Pratama', 'L', '2009-02-14 00:00:00', 'Jl. Gandaria I No. 12, Kebayoran Baru, Jakarta Selatan', '081311223301', TRUE, NOW(), NOW()),
    ('std-002', '20250102', 'Siti Nurhaliza Putri', 'P', '2009-05-20 00:00:00', 'Jl. Radio Dalam No. 45, Gandaria Utara, Jakarta Selatan', '081311223302', TRUE, NOW(), NOW()),
    ('std-003', '20250103', 'Dimas Arya Wijaya', 'L', '2009-03-10 00:00:00', 'Jl. Cipete Raya No. 8, Cilandak, Jakarta Selatan', '081311223303', TRUE, NOW(), NOW()),
    ('std-004', '20250104', 'Anisa Rahmawati Zahra', 'P', '2009-08-25 00:00:00', 'Jl. Panglima Polim IX No. 22, Kebayoran Baru, Jakarta Selatan', '081311223304', TRUE, NOW(), NOW()),
    ('std-005', '20250105', 'Kevin Sanjaya Sukamuljo', 'L', '2009-01-18 00:00:00', 'Jl. Fatmawati No. 99, Pondok Labu, Jakarta Selatan', '081311223305', TRUE, NOW(), NOW()),
    ('std-006', '20250106', 'Aurelia Dinda Kirana', 'P', '2009-07-30 00:00:00', 'Jl. Wijaya Timur No. 15, Petogogan, Jakarta Selatan', '081311223306', TRUE, NOW(), NOW()),
    ('std-007', '20250107', 'Farhan Ramadhan Kusuma', 'L', '2009-09-12 00:00:00', 'Jl. Melawai No. 34, Kebayoran Baru, Jakarta Selatan', '081311223307', TRUE, NOW(), NOW()),
    ('std-008', '20250108', 'Nabila Shafa Maharani', 'P', '2009-11-05 00:00:00', 'Jl. Senopati No. 88, Selong, Jakarta Selatan', '081311223308', TRUE, NOW(), NOW()),

    -- Kelas X IPS 1 Students
    ('std-009', '20250201', 'Bryan Aditya Nugraha', 'L', '2009-04-16 00:00:00', 'Jl. Brawijaya Raya No. 20, Pulo, Jakarta Selatan', '081311223309', TRUE, NOW(), NOW()),
    ('std-010', '20250202', 'Clarissa Amanda Permata', 'P', '2009-06-22 00:00:00', 'Jl. Kemang Timur No. 10, Bangka, Jakarta Selatan', '081311223310', TRUE, NOW(), NOW()),
    ('std-011', '20250203', 'Daffa Ihsan Al-Ghifari', 'L', '2009-10-08 00:00:00', 'Jl. Antasari No. 70, Cilandak Barat, Jakarta Selatan', '081311223311', TRUE, NOW(), NOW()),
    ('std-012', '20250204', 'Fathia Salsa Bella', 'P', '2009-12-19 00:00:00', 'Jl. Dharmawangsa VI No. 5, Pulo, Jakarta Selatan', '081311223312', TRUE, NOW(), NOW()),

    -- Kelas XI MIPA 1 Students
    ('std-013', '20240101', 'Gilang Akbar Prasetya', 'L', '2008-01-28 00:00:00', 'Jl. Tebet Barat Dalam No. 14, Jakarta Selatan', '081311223313', TRUE, NOW(), NOW()),
    ('std-014', '20240102', 'Hani Amelia Safitri', 'P', '2008-04-03 00:00:00', 'Jl. Bukit Duri Selatan No. 50, Jakarta Selatan', '081311223314', TRUE, NOW(), NOW()),
    ('std-015', '20240103', 'Irfan Maulana Hakim', 'L', '2008-08-14 00:00:00', 'Jl. Saharjo No. 120, Manggarai, Jakarta Selatan', '081311223315', TRUE, NOW(), NOW()),
    ('std-016', '20240104', 'Jessica Aurel Tambunan', 'P', '2008-11-29 00:00:00', 'Jl. Prof. Supomo No. 33, Tebet, Jakarta Selatan', '081311223316', TRUE, NOW(), NOW()),

    -- Kelas XII MIPA 1 Students
    ('std-017', '20230101', 'Kenzo Raditya Ardhani', 'L', '2007-03-05 00:00:00', 'Jl. Wolter Monginsidi No. 64, Rawa Barat, Jakarta Selatan', '081311223317', TRUE, NOW(), NOW()),
    ('std-018', '20230102', 'Larasati Ayu Wulandari', 'P', '2007-07-17 00:00:00', 'Jl. Suryo No. 40, Senopati, Jakarta Selatan', '081311223318', TRUE, NOW(), NOW()),
    ('std-019', '20230103', 'Mahendra Bintang Samudra', 'L', '2007-09-21 00:00:00', 'Jl. Gunawarman No. 25, Selong, Jakarta Selatan', '081311223319', TRUE, NOW(), NOW()),
    ('std-020', '20230104', 'Nadya Putri Karima', 'P', '2007-12-10 00:00:00', 'Jl. Cikajang No. 18, Petogogan, Jakarta Selatan', '081311223320', TRUE, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    nis = EXCLUDED.nis,
    full_name = EXCLUDED.full_name,
    gender = EXCLUDED.gender,
    phone = EXCLUDED.phone,
    address = EXCLUDED.address;

-- ------------------------------------------------------------------------------
-- 11. Student Enrollments
-- ------------------------------------------------------------------------------
INSERT INTO enrollments (id, student_id, class_id, term_id, joined_at, status)
VALUES
    -- X MIPA 1
    ('enr-001', 'std-001', 'cls-x-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-002', 'std-002', 'cls-x-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-003', 'std-003', 'cls-x-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-004', 'std-004', 'cls-x-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-005', 'std-005', 'cls-x-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-006', 'std-006', 'cls-x-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-007', 'std-007', 'cls-x-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-008', 'std-008', 'cls-x-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),

    -- X IPS 1
    ('enr-009', 'std-009', 'cls-x-ips-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-010', 'std-010', 'cls-x-ips-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-011', 'std-011', 'cls-x-ips-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-012', 'std-012', 'cls-x-ips-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),

    -- XI MIPA 1
    ('enr-013', 'std-013', 'cls-xi-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-014', 'std-014', 'cls-xi-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-015', 'std-015', 'cls-xi-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-016', 'std-016', 'cls-xi-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),

    -- XII MIPA 1
    ('enr-017', 'std-017', 'cls-xii-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-018', 'std-018', 'cls-xii-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-019', 'std-019', 'cls-xii-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE'),
    ('enr-020', 'std-020', 'cls-xii-ipa-1', 'term-2025-1', '2025-07-14 07:00:00', 'ACTIVE')
ON CONFLICT (id) DO UPDATE SET status = 'ACTIVE';

-- ------------------------------------------------------------------------------
-- 12. Grade Components & Weights
-- ------------------------------------------------------------------------------
INSERT INTO grade_components (id, code, name, description, created_at, updated_at)
VALUES 
    ('gc-tgs', 'TGS', 'Tugas Mandiri & Kelompok', 'Penugasan terstruktur dan tugas mandiri siswa', NOW(), NOW()),
    ('gc-uh1', 'UH-1', 'Ulangan Harian 1', 'Penilaian harian materi bab pertama', NOW(), NOW()),
    ('gc-uh2', 'UH-2', 'Ulangan Harian 2', 'Penilaian harian materi bab kedua', NOW(), NOW()),
    ('gc-pts', 'PTS', 'Penilaian Tengah Semester', 'Ujian evaluasi capaian tengah semester', NOW(), NOW()),
    ('gc-pas', 'PAS', 'Penilaian Akhir Semester', 'Ujian komprehensif akhir semester', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description;

-- Grade Configs
INSERT INTO grade_configs (id, class_id, subject_id, term_id, calculation_scheme, finalized, created_at, updated_at)
VALUES
    ('gcfg-x1-mtk', 'cls-x-ipa-1', 'subj-001', 'term-2025-1', 'WEIGHTED', FALSE, NOW(), NOW()),
    ('gcfg-x1-fis', 'cls-x-ipa-1', 'subj-002', 'term-2025-1', 'WEIGHTED', FALSE, NOW(), NOW()),
    ('gcfg-x1-kim', 'cls-x-ipa-1', 'subj-kim', 'term-2025-1', 'WEIGHTED', FALSE, NOW(), NOW()),
    ('gcfg-x1-bio', 'cls-x-ipa-1', 'subj-bio', 'term-2025-1', 'WEIGHTED', FALSE, NOW(), NOW()),
    ('gcfg-x1-ind', 'cls-x-ipa-1', 'subj-ind', 'term-2025-1', 'WEIGHTED', FALSE, NOW(), NOW()),
    ('gcfg-x1-ing', 'cls-x-ipa-1', 'subj-ing', 'term-2025-1', 'WEIGHTED', FALSE, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Grade Config Components Weights
INSERT INTO grade_config_components (id, grade_config_id, component_id, weight)
VALUES
    ('gcc-1', 'gcfg-x1-mtk', 'gc-tgs', 0.20),
    ('gcc-2', 'gcfg-x1-mtk', 'gc-uh1', 0.20),
    ('gcc-3', 'gcfg-x1-mtk', 'gc-uh2', 0.20),
    ('gcc-4', 'gcfg-x1-mtk', 'gc-pts', 0.20),
    ('gcc-5', 'gcfg-x1-mtk', 'gc-pas', 0.20)
ON CONFLICT (id) DO UPDATE SET weight = EXCLUDED.weight;

-- ------------------------------------------------------------------------------
-- 13. Student Grades & Final Grades
-- ------------------------------------------------------------------------------
INSERT INTO grades (id, enrollment_id, subject_id, component_id, grade_value, created_at, updated_at)
VALUES 
    -- std-001 (Rizky)
    ('grd-101', 'enr-001', 'subj-001', 'gc-tgs', 88.00, NOW(), NOW()),
    ('grd-102', 'enr-001', 'subj-001', 'gc-uh1', 85.00, NOW(), NOW()),
    ('grd-103', 'enr-001', 'subj-001', 'gc-uh2', 90.00, NOW(), NOW()),
    ('grd-104', 'enr-001', 'subj-001', 'gc-pts', 86.00, NOW(), NOW()),
    ('grd-105', 'enr-001', 'subj-001', 'gc-pas', 92.00, NOW(), NOW()),

    ('grd-106', 'enr-001', 'subj-002', 'gc-tgs', 90.00, NOW(), NOW()),
    ('grd-107', 'enr-001', 'subj-002', 'gc-uh1', 88.00, NOW(), NOW()),
    ('grd-108', 'enr-001', 'subj-002', 'gc-pts', 91.00, NOW(), NOW()),

    -- std-002 (Siti)
    ('grd-109', 'enr-002', 'subj-001', 'gc-tgs', 95.00, NOW(), NOW()),
    ('grd-110', 'enr-002', 'subj-001', 'gc-uh1', 94.00, NOW(), NOW()),
    ('grd-111', 'enr-002', 'subj-001', 'gc-uh2', 96.00, NOW(), NOW()),
    ('grd-112', 'enr-002', 'subj-001', 'gc-pts', 92.00, NOW(), NOW()),
    ('grd-113', 'enr-002', 'subj-001', 'gc-pas', 95.00, NOW(), NOW()),

    -- std-003 (Dimas)
    ('grd-114', 'enr-003', 'subj-001', 'gc-tgs', 78.00, NOW(), NOW()),
    ('grd-115', 'enr-003', 'subj-001', 'gc-uh1', 80.00, NOW(), NOW()),
    ('grd-116', 'enr-003', 'subj-001', 'gc-pts', 76.00, NOW(), NOW()),

    -- std-004 (Anisa)
    ('grd-117', 'enr-004', 'subj-001', 'gc-tgs', 84.00, NOW(), NOW()),
    ('grd-118', 'enr-004', 'subj-001', 'gc-uh1', 82.00, NOW(), NOW()),
    ('grd-119', 'enr-004', 'subj-001', 'gc-pts', 85.00, NOW(), NOW()),

    -- std-005 (Kevin)
    ('grd-120', 'enr-005', 'subj-001', 'gc-tgs', 80.00, NOW(), NOW()),
    ('grd-121', 'enr-005', 'subj-001', 'gc-uh1', 75.00, NOW(), NOW()),
    ('grd-122', 'enr-005', 'subj-001', 'gc-pts', 79.00, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET grade_value = EXCLUDED.grade_value;

-- Final Calculated Grades
INSERT INTO grade_finals (id, enrollment_id, subject_id, final_grade, finalized, calculated_at, calculation_note)
VALUES
    ('gf-001', 'enr-001', 'subj-001', 88.20, TRUE, NOW(), 'Predikat A (Sangat Baik)'),
    ('gf-002', 'enr-001', 'subj-002', 89.60, TRUE, NOW(), 'Predikat A (Sangat Baik)'),
    ('gf-003', 'enr-001', 'subj-kim', 85.00, TRUE, NOW(), 'Predikat B+ (Baik)'),
    ('gf-004', 'enr-001', 'subj-bio', 87.50, TRUE, NOW(), 'Predikat A (Sangat Baik)'),
    ('gf-005', 'enr-001', 'subj-ind', 91.00, TRUE, NOW(), 'Predikat A (Sangat Baik)'),
    ('gf-006', 'enr-001', 'subj-ing', 88.00, TRUE, NOW(), 'Predikat A (Sangat Baik)'),

    ('gf-007', 'enr-002', 'subj-001', 94.40, TRUE, NOW(), 'Predikat A+ (Istimewa)'),
    ('gf-008', 'enr-002', 'subj-002', 92.00, TRUE, NOW(), 'Predikat A (Sangat Baik)'),
    ('gf-009', 'enr-002', 'subj-kim', 90.50, TRUE, NOW(), 'Predikat A (Sangat Baik)'),

    ('gf-010', 'enr-003', 'subj-001', 78.00, TRUE, NOW(), 'Predikat B (Cukup)'),
    ('gf-011', 'enr-004', 'subj-001', 83.67, TRUE, NOW(), 'Predikat B+ (Baik)'),
    ('gf-012', 'enr-005', 'subj-001', 78.00, TRUE, NOW(), 'Predikat B (Cukup)'),
    ('gf-013', 'enr-006', 'subj-001', 86.50, TRUE, NOW(), 'Predikat A (Sangat Baik)'),
    ('gf-014', 'enr-007', 'subj-001', 82.00, TRUE, NOW(), 'Predikat B+ (Baik)'),
    ('gf-015', 'enr-008', 'subj-001', 90.00, TRUE, NOW(), 'Predikat A (Sangat Baik)')
ON CONFLICT (id) DO UPDATE SET final_grade = EXCLUDED.final_grade, calculation_note = EXCLUDED.calculation_note;

-- ------------------------------------------------------------------------------
-- 14. Attendance Records
-- ------------------------------------------------------------------------------
INSERT INTO daily_attendance (id, enrollment_id, date, status, notes, created_at, updated_at)
VALUES
    ('att-d-001', 'enr-001', '2025-08-25', 'H', 'Hadir tepat waktu', NOW(), NOW()),
    ('att-d-002', 'enr-001', '2025-08-26', 'H', 'Hadir tepat waktu', NOW(), NOW()),
    ('att-d-003', 'enr-001', '2025-08-27', 'H', 'Hadir tepat waktu', NOW(), NOW()),
    ('att-d-004', 'enr-001', '2025-08-28', 'S', 'Sakit flu dengan surat dokter', NOW(), NOW()),
    ('att-d-005', 'enr-001', '2025-08-29', 'H', 'Hadir tepat waktu', NOW(), NOW()),

    ('att-d-006', 'enr-002', '2025-08-25', 'H', 'Hadir tepat waktu', NOW(), NOW()),
    ('att-d-007', 'enr-002', '2025-08-26', 'H', 'Hadir tepat waktu', NOW(), NOW()),
    ('att-d-008', 'enr-002', '2025-08-27', 'H', 'Hadir tepat waktu', NOW(), NOW()),
    ('att-d-009', 'enr-002', '2025-08-28', 'H', 'Hadir tepat waktu', NOW(), NOW()),
    ('att-d-010', 'enr-002', '2025-08-29', 'H', 'Hadir tepat waktu', NOW(), NOW()),

    ('att-d-011', 'enr-003', '2025-08-25', 'H', 'Hadir', NOW(), NOW()),
    ('att-d-012', 'enr-003', '2025-08-26', 'I', 'Izin acara keluarga', NOW(), NOW()),
    ('att-d-013', 'enr-003', '2025-08-27', 'H', 'Hadir', NOW(), NOW()),
    ('att-d-014', 'enr-003', '2025-08-28', 'H', 'Hadir', NOW(), NOW()),
    ('att-d-015', 'enr-003', '2025-08-29', 'H', 'Hadir', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, notes = EXCLUDED.notes;

-- ------------------------------------------------------------------------------
-- 15. Behavior & BK Notes (Prestasi & Catatan Siswa)
-- ------------------------------------------------------------------------------
INSERT INTO behavior_notes (id, student_id, date, note_type, points, description, created_by, created_at, updated_at)
VALUES 
    ('bn-001', 'std-001', '2025-08-18 10:00:00', '+', 50, 'Juara 1 Olimpiade Sains Matematika Tingkat Kota Jakarta Selatan', 'usr-tch-001', NOW(), NOW()),
    ('bn-002', 'std-002', '2025-08-17 11:30:00', '+', 30, 'Petugas Pengibar Bendera Pusaka Upacara HUT RI ke-80', 'usr-tch-005', NOW(), NOW()),
    ('bn-003', 'std-003', '2025-08-20 07:15:00', '-', -5, 'Terlambat masuk sekolah lebih dari 15 menit tanpa surat keterangan', 'usr-tch-001', NOW(), NOW()),
    ('bn-004', 'std-004', '2025-08-22 09:30:00', '+', 25, 'Juara 2 Lomba Pidato Bahasa Inggris (English Speech Contest) Tingkat Provinsi', 'usr-tch-006', NOW(), NOW()),
    ('bn-005', 'std-005', '2025-08-26 13:00:00', '+', 40, 'Juara 1 Kejuaraan Bulutangkis Antar Pelajar Se-Jabodetabek', 'usr-tch-009', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET points = EXCLUDED.points, description = EXCLUDED.description;

-- ------------------------------------------------------------------------------
-- 16. School Announcements
-- ------------------------------------------------------------------------------
INSERT INTO announcements (id, title, content, audience, target_class_id, priority, is_pinned, published_at, expires_at, created_by, created_at, updated_at)
VALUES
    ('ann-001', 'Jadwal Pelaksanaan Penilaian Tengah Semester (PTS) Ganjil 2025/2026', 'Diberitahukan kepada seluruh siswa dan wali murid bahwa pelaksanaan PTS Ganjil akan dimulai tanggal 22 September 2025. Harap mempersiapkan diri dengan baik dan melunasi administrasi akademik.', 'ALL', NULL, 'HIGH', TRUE, NOW(), '2025-09-30 23:59:59', 'usr-adm-001', NOW(), NOW()),
    ('ann-002', 'Peringatan Bulan Bahasa dan Sastra Indonesia 2025', 'OSIS SMA Negeri 1 Nusantara akan menyelenggarakan berbagai lomba literasi, cipta puisi, dan drama musikal. Pendaftaran dibuka melalui wali kelas masing-masing.', 'ALL', NULL, 'NORMAL', FALSE, NOW(), '2025-10-28 23:59:59', 'usr-tch-005', NOW(), NOW()),
    ('ann-003', 'Sosialisasi Seleksi Nasional Masuk Perguruan Tinggi Negeri (SNBP & SNBT)', 'Khusus seluruh siswa kelas XII MIPA dan XII IPS beserta orang tua/wali murid, sosialisasi jalur masuk PTN akan dilaksanakan hari Sabtu pukul 09.00 WIB di Aula Utama.', 'CLASS', 'cls-xii-ipa-1', 'HIGH', TRUE, NOW(), '2025-11-15 23:59:59', 'usr-adm-002', NOW(), NOW()),
    ('ann-004', 'Jadwal Praktikum Terpadu Laboratorium Sains', 'Kegiatan praktikum Biologi dan Kimia semester ganjil dimulai pekan depan sesuai jadwal modul laboratorium.', 'CLASS', 'cls-x-ipa-1', 'NORMAL', FALSE, NOW(), '2025-10-15 23:59:59', 'usr-tch-003', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    title = EXCLUDED.title, 
    content = EXCLUDED.content, 
    is_pinned = EXCLUDED.is_pinned;

-- ------------------------------------------------------------------------------
-- 17. Academic Calendar Events
-- ------------------------------------------------------------------------------
INSERT INTO calendar_events (id, title, description, event_type, start_date, end_date, start_time, end_time, audience, target_class_id, location, created_by, created_at, updated_at)
VALUES
    ('cal-001', 'Penilaian Tengah Semester (PTS) Ganjil', 'Ujian tertulis dan praktik tengah semester', 'EXAM', '2025-09-22 00:00:00', '2025-09-27 23:59:59', '2025-09-22 07:30:00', '2025-09-27 12:00:00', 'ALL', NULL, 'Ruang Kelas Masing-masing', 'usr-adm-001', NOW(), NOW()),
    ('cal-002', 'Peringatan Hari Guru Nasional & Pentas Seni', 'Upacara bendera dan penampilan apresiasi siswa', 'CEREMONY', '2025-11-25 00:00:00', '2025-11-25 23:59:59', '2025-11-25 07:00:00', '2025-11-25 14:00:00', 'ALL', NULL, 'Lapangan Utama Sekolah', 'usr-adm-001', NOW(), NOW()),
    ('cal-003', 'Pekan Olahraga & Seni (PORSENI) Antar Kelas', 'Kompetisi futsal, basket, voli, dan tari tradisional', 'ACTIVITY', '2025-12-11 00:00:00', '2025-12-16 23:59:59', '2025-12-11 08:00:00', '2025-12-16 15:00:00', 'ALL', NULL, 'Kompleks Olahraga SMAN 1', 'usr-adm-001', NOW(), NOW()),
    ('cal-004', 'Pembagian Rapor Hasil Belajar Semester Ganjil', 'Pengambilan buku laporan pendidikan oleh orang tua/wali murid', 'ACADEMIC', '2025-12-19 00:00:00', '2025-12-19 23:59:59', '2025-12-19 08:00:00', '2025-12-19 12:00:00', 'ALL', NULL, 'Ruang Kelas Wali Kelas', 'usr-adm-001', NOW(), NOW()),
    ('cal-005', 'Libur Akhir Semester Ganjil', 'Libur semester ganjil tahun ajaran 2025/2026', 'HOLIDAY', '2025-12-22 00:00:00', '2026-01-03 23:59:59', '2025-12-22 00:00:00', '2026-01-03 23:59:59', 'ALL', NULL, '-', 'usr-adm-001', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET 
    title = EXCLUDED.title, 
    start_date = EXCLUDED.start_date, 
    end_date = EXCLUDED.end_date;

-- ------------------------------------------------------------------------------
-- 18. Student Mutations (Mutasi Siswa)
-- ------------------------------------------------------------------------------
INSERT INTO mutations (id, type, entity, entity_id, current_snapshot, requested_changes, status, reason, requested_by, reviewed_by, requested_at, reviewed_at, note)
VALUES 
    ('mut-001', 'STUDENT_DATA', 'student', 'std-001', '{"full_name": "Muhammad Rizky", "phone": "081311223301"}'::jsonb, '{"full_name": "Muhammad Rizky Pratama", "phone": "081311223301"}'::jsonb, 'APPROVED', 'Penyesuaian penulisan nama sesuai Akta Kelahiran dan Ijazah SMP', 'usr-adm-001', 'usr-sa-001', NOW() - INTERVAL '5 days', NOW() - INTERVAL '4 days', 'Verifikasi berkas akta kelahiran valid'),
    ('mut-002', 'CLASS_CHANGE', 'student', 'std-009', '{"class_id": "cls-x-ips-1"}'::jsonb, '{"class_id": "cls-x-ipa-1"}'::jsonb, 'PENDING', 'Permohonan pindah peminatan jurusan berdasarkan hasil psikotes lanjutan', 'usr-adm-001', NULL, NOW() - INTERVAL '1 day', NULL, 'Menunggu persetujuan Waka Kurikulum dan ketersediaan kuota kelas')
ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, reason = EXCLUDED.reason;

-- ------------------------------------------------------------------------------
-- 19. Archives & Document Storage Metadata
-- ------------------------------------------------------------------------------
INSERT INTO archives (id, title, category, scope, ref_term_id, ref_class_id, ref_student_id, file_path, mime_type, size_bytes, uploaded_by, uploaded_at)
VALUES
    ('arc-001', 'Kurikulum Operasional Satuan Pendidikan (KOSP) 2025/2026', 'CURRICULUM', 'GLOBAL', 'term-2025-1', NULL, NULL, '/archives/kosp_2025_2026.pdf', 'application/pdf', 4194304, 'usr-adm-001', NOW()),
    ('arc-002', 'Kalender Akademik Semester Ganjil 2025/2026', 'ACADEMIC', 'GLOBAL', 'term-2025-1', NULL, NULL, '/archives/kalender_akademik_2025.pdf', 'application/pdf', 1048576, 'usr-adm-001', NOW()),
    ('arc-003', 'Surat Keputusan Pembagian Tugas Mengajar Guru Ganjil', 'LEGAL', 'GLOBAL', 'term-2025-1', NULL, NULL, '/archives/sk_pbm_ganjil_2025.pdf', 'application/pdf', 2097152, 'usr-adm-001', NOW())
ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title;

-- ------------------------------------------------------------------------------
-- 20. Refresh All Analytics Materialized Views
-- ------------------------------------------------------------------------------
SELECT refresh_analytics_mvs();
