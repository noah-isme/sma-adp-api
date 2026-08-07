-- Migration 000021: Advanced materialized views for Phase 5 Analytics
-- Creates materialized views for class statistics, student performance, and subject statistics

-- Class statistics materialized view
-- Aggregates class-level metrics per term
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_class_statistics AS
SELECT
    c.id AS class_id,
    c.name AS class_name,
    c.grade,
    c.track,
    t.id AS term_id,
    t.name AS term_name,
    t.academic_year,
    COUNT(DISTINCT e.student_id) AS total_students,
    COUNT(DISTINCT cs.subject_id) AS total_subjects,
    -- Attendance stats
    ROUND(
        CASE
            WHEN COUNT(DISTINCT da.id) = 0 THEN 0
            ELSE (SUM(CASE WHEN da.status = 'H' THEN 1 ELSE 0 END)::DECIMAL / COUNT(DISTINCT da.id)) * 100
        END, 2
    ) AS avg_attendance_rate,
    COUNT(CASE WHEN da.status = 'H' THEN 1 END) AS total_present,
    COUNT(CASE WHEN da.status = 'A' THEN 1 END) AS total_absent,
    COUNT(CASE WHEN da.status = 'S' THEN 1 END) AS total_sick,
    COUNT(CASE WHEN da.status = 'I' THEN 1 END) AS total_permission,
    -- Grade stats
    ROUND(AVG(gf.final_grade)::NUMERIC, 2) AS avg_grade,
    COUNT(CASE WHEN gf.final_grade >= 75 THEN 1 END) AS students_passed,
    COUNT(CASE WHEN gf.final_grade < 75 THEN 1 END) AS students_failed,
    MIN(gf.final_grade) AS min_grade,
    MAX(gf.final_grade) AS max_grade,
    -- Behavior stats
    COALESCE(SUM(CASE WHEN bn.points > 0 THEN bn.points ELSE 0 END), 0) AS total_positive_points,
    COALESCE(SUM(CASE WHEN bn.points < 0 THEN ABS(bn.points) ELSE 0 END), 0) AS total_negative_points,
    COALESCE(SUM(bn.points), 0) AS behavior_balance,
    NOW() AS refreshed_at
FROM classes c
CROSS JOIN terms t
LEFT JOIN enrollments e ON e.class_id = c.id AND e.term_id = t.id AND e.status = 'ACTIVE'
LEFT JOIN class_subjects cs ON cs.class_id = c.id
LEFT JOIN daily_attendance da ON da.enrollment_id = e.id
LEFT JOIN grade_finals gf ON gf.enrollment_id = e.id
LEFT JOIN behavior_notes bn ON bn.student_id = e.student_id AND bn.date >= t.start_date AND bn.date <= t.end_date
WHERE t.is_active = true
GROUP BY c.id, c.name, c.grade, c.track, t.id, t.name, t.academic_year;

CREATE UNIQUE INDEX IF NOT EXISTS mv_class_statistics_idx ON mv_class_statistics(class_id, term_id);

-- Student performance materialized view
-- Comprehensive student-level metrics per class per term
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_student_performance AS
SELECT
    s.id AS student_id,
    s.nis,
    s.full_name,
    s.gender,
    s.birth_date,
    e.class_id,
    c.name AS class_name,
    c.grade,
    e.term_id,
    t.name AS term_name,
    t.academic_year,
    -- Enrollment info
    e.status AS enrollment_status,
    e.joined_at,
    e.left_at,
    -- Attendance metrics
    COUNT(DISTINCT da.date) AS total_attendance_days,
    COUNT(CASE WHEN da.status = 'H' THEN 1 END) AS days_present,
    COUNT(CASE WHEN da.status = 'A' THEN 1 END) AS days_absent,
    COUNT(CASE WHEN da.status = 'S' THEN 1 END) AS days_sick,
    COUNT(CASE WHEN da.status = 'I' THEN 1 END) AS days_permission,
    ROUND(
        CASE
            WHEN COUNT(DISTINCT da.date) = 0 THEN 0
            ELSE (100.0 * COUNT(CASE WHEN da.status = 'H' THEN 1 END) / NULLIF(COUNT(DISTINCT da.date), 0))
        END, 2
    ) AS attendance_percentage,
    -- Grade metrics
    COUNT(DISTINCT gf.subject_id) AS subjects_enrolled,
    ROUND(AVG(gf.final_grade)::NUMERIC, 2) AS gpa,
    MIN(gf.final_grade) AS lowest_grade,
    MAX(gf.final_grade) AS highest_grade,
    COUNT(CASE WHEN gf.final_grade >= 75 THEN 1 END) AS subjects_passed,
    COUNT(CASE WHEN gf.final_grade < 75 THEN 1 END) AS subjects_failed,
    -- Subject breakdown
    JSON_AGG(
        JSON_BUILD_OBJECT(
            'subjectId', gf.subject_id,
            'subjectName', sub.name,
            'subjectCode', sub.code,
            'finalGrade', gf.final_grade,
            'letterGrade', sub.code -- Will be computed in application layer
        ) ORDER BY sub.name
    ) FILTER (WHERE gf.subject_id IS NOT NULL) AS subject_grades,
    -- Behavior metrics
    COALESCE(SUM(bn.points), 0) AS behavior_points,
    COUNT(CASE WHEN bn.category = 'POSITIVE' THEN 1 END) AS positive_notes,
    COUNT(CASE WHEN bn.category = 'NEGATIVE' THEN 1 END) AS negative_notes,
    COUNT(CASE WHEN bn.category = 'NEUTRAL' THEN 1 END) AS neutral_notes,
    -- Homeroom teacher
    ht.id AS homeroom_teacher_id,
    u.full_name AS homeroom_teacher_name,
    NOW() AS refreshed_at
FROM students s
JOIN enrollments e ON e.student_id = s.id AND e.status = 'ACTIVE'
JOIN classes c ON c.id = e.class_id
JOIN terms t ON t.id = e.term_id
LEFT JOIN daily_attendance da ON da.enrollment_id = e.id
LEFT JOIN grade_finals gf ON gf.enrollment_id = e.id
LEFT JOIN subjects sub ON sub.id = gf.subject_id
LEFT JOIN behavior_notes bn ON bn.student_id = s.id AND bn.date >= t.start_date AND bn.date <= t.end_date
LEFT JOIN teachers ht ON ht.id = c.homeroom_teacher_id
LEFT JOIN users u ON u.id = ht.user_id
WHERE t.is_active = true AND s.active = true
GROUP BY s.id, s.nis, s.full_name, s.gender, s.birth_date, e.class_id, c.name, c.grade, e.term_id, t.name, t.academic_year, e.status, e.joined_at, e.left_at, ht.id, u.full_name;

CREATE UNIQUE INDEX IF NOT EXISTS mv_student_performance_idx ON mv_student_performance(student_id, class_id, term_id);

-- Subject statistics materialized view
-- Subject-level analytics per class per term
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_subject_statistics AS
SELECT
    sub.id AS subject_id,
    sub.code AS subject_code,
    sub.name AS subject_name,
    sub.subject_group,
    cs.class_id,
    c.name AS class_name,
    c.grade,
    c.track,
    t.id AS term_id,
    t.name AS term_name,
    t.academic_year,
    -- Teacher info
    tch.id AS teacher_id,
    u.full_name AS teacher_name,
    -- Student count
    COUNT(DISTINCT gf.enrollment_id) AS total_students,
    -- Grade distribution
    ROUND(AVG(gf.final_grade)::NUMERIC, 2) AS avg_grade,
    STDDEV_POP(gf.final_grade) AS grade_stddev,
    MIN(gf.final_grade) AS min_grade,
    MAX(gf.final_grade) AS max_grade,
    -- Percentiles
    PERCENTILE_CONT(0.25) WITHIN GROUP (ORDER BY gf.final_grade) AS percentile_25,
    PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY gf.final_grade) AS median_grade,
    PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY gf.final_grade) AS percentile_75,
    -- Pass/Fail
    COUNT(CASE WHEN gf.final_grade >= 75 THEN 1 END) AS students_passed,
    COUNT(CASE WHEN gf.final_grade < 75 THEN 1 END) AS students_failed,
    ROUND(
        CASE
            WHEN COUNT(gf.final_grade) = 0 THEN 0
            ELSE (100.0 * COUNT(CASE WHEN gf.final_grade >= 75 THEN 1 END) / COUNT(gf.final_grade))
        END, 2
    ) AS pass_rate,
    -- Grade distribution buckets
    COUNT(CASE WHEN gf.final_grade >= 90 THEN 1 END) AS grade_a_count,   -- 90-100
    COUNT(CASE WHEN gf.final_grade >= 80 AND gf.final_grade < 90 THEN 1 END) AS grade_b_count,  -- 80-89
    COUNT(CASE WHEN gf.final_grade >= 70 AND gf.final_grade < 80 THEN 1 END) AS grade_c_count,  -- 70-79
    COUNT(CASE WHEN gf.final_grade >= 60 AND gf.final_grade < 70 THEN 1 END) AS grade_d_count,  -- 60-69
    COUNT(CASE WHEN gf.final_grade < 60 THEN 1 END) AS grade_e_count,   -- <60
    -- Component breakdown (if available)
    JSON_AGG(
        JSON_BUILD_OBJECT(
            'componentId', gc.component_id,
            'componentCode', gc.code,
            'componentName', gc.name,
            'weight', gcc.weight,
            'avgScore', ROUND(AVG(gr.grade_value)::NUMERIC, 2)
        ) ORDER BY gc.code
    ) FILTER (WHERE gc.component_id IS NOT NULL) AS component_averages,
    NOW() AS refreshed_at
FROM subjects sub
JOIN class_subjects cs ON cs.subject_id = sub.id
JOIN classes c ON c.id = cs.class_id
JOIN terms t ON t.id = cs.class_id -- terms joined via enrollments
JOIN enrollments e ON e.class_id = c.id AND e.term_id = t.id AND e.status = 'ACTIVE'
LEFT JOIN grade_finals gf ON gf.enrollment_id = e.id AND gf.subject_id = sub.id
LEFT JOIN teachers tch ON tch.id = cs.teacher_id
LEFT JOIN users u ON u.id = tch.user_id
LEFT JOIN grades gr ON gr.enrollment_id = e.id AND gr.subject_id = sub.id
LEFT JOIN grade_configs gcfg ON gcfg.class_id = c.id AND gcfg.subject_id = sub.id AND gcfg.term_id = t.id
LEFT JOIN grade_config_components gcc ON gcc.grade_config_id = gcfg.id
LEFT JOIN grade_components gc ON gc.id = gcc.component_id
WHERE t.is_active = true
GROUP BY sub.id, sub.code, sub.name, sub.subject_group, cs.class_id, c.name, c.grade, c.track, t.id, t.name, t.academic_year, tch.id, u.full_name;

CREATE UNIQUE INDEX IF NOT EXISTS mv_subject_statistics_idx ON mv_subject_statistics(subject_id, class_id, term_id);

-- Function to refresh all analytics materialized views
CREATE OR REPLACE FUNCTION refresh_analytics_mvs()
RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY mv_class_statistics;
    REFRESH MATERIALIZED VIEW CONCURRENTLY mv_student_performance;
    REFRESH MATERIALIZED VIEW CONCURRENTLY mv_subject_statistics;
    REFRESH MATERIALIZED VIEW CONCURRENTLY attendance_summary_mv;
    REFRESH MATERIALIZED VIEW CONCURRENTLY grade_summary_mv;
    REFRESH MATERIALIZED VIEW CONCURRENTLY behavior_summary_mv;
END;
$$;

-- Comment on refresh strategy
COMMENT ON FUNCTION refresh_analytics_mvs() IS 'Refreshes all analytics materialized views. Run via pg_cron hourly or on-demand after grade/attendance/behavior updates. Use CONCURRENTLY to avoid blocking reads.';