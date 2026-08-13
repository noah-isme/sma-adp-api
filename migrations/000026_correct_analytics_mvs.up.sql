-- Migration 000026: Correct the Phase 5 analytics materialized views.
--
-- Migration 000021 predates the current schema and joins raw attendance,
-- grades, and behaviour rows in the same SELECT. That multiplies metrics
-- (and references the removed behavior_notes.category column). These views
-- aggregate each source independently before joining the results.

DROP FUNCTION IF EXISTS refresh_analytics_mvs();
DROP MATERIALIZED VIEW IF EXISTS mv_subject_statistics;
DROP MATERIALIZED VIEW IF EXISTS mv_student_performance;
DROP MATERIALIZED VIEW IF EXISTS mv_class_statistics;

CREATE MATERIALIZED VIEW mv_class_statistics AS
WITH attendance AS (
    SELECT
        enrollment_id,
        COUNT(*) AS total_days,
        COUNT(*) FILTER (WHERE status = 'H') AS present_days,
        COUNT(*) FILTER (WHERE status = 'A') AS absent_days,
        COUNT(*) FILTER (WHERE status = 'S') AS sick_days,
        COUNT(*) FILTER (WHERE status = 'I') AS permission_days
    FROM daily_attendance
    GROUP BY enrollment_id
), grades AS (
    SELECT
        enrollment_id,
        COUNT(*) AS subjects_enrolled,
        AVG(final_grade) AS average_grade,
        MIN(final_grade) AS min_grade,
        MAX(final_grade) AS max_grade,
        COUNT(*) FILTER (WHERE final_grade >= 75) AS subjects_passed,
        COUNT(*) FILTER (WHERE final_grade < 75) AS subjects_failed
    FROM grade_finals
    GROUP BY enrollment_id
), behavior AS (
    SELECT
        e.term_id,
        bn.student_id,
        COALESCE(SUM(bn.points) FILTER (WHERE bn.points > 0), 0) AS total_positive_points,
        COALESCE(SUM(ABS(bn.points)) FILTER (WHERE bn.points < 0), 0) AS total_negative_points,
        COALESCE(SUM(bn.points), 0) AS behavior_balance
    FROM behavior_notes bn
    JOIN enrollments e ON e.student_id = bn.student_id
    JOIN terms t ON t.id = e.term_id
                 AND bn.date >= t.start_date
                 AND bn.date <= t.end_date
    GROUP BY e.term_id, bn.student_id
), subject_counts AS (
    SELECT class_id, COUNT(DISTINCT subject_id) AS total_subjects
    FROM class_subjects
    GROUP BY class_id
), enrolled AS (
    SELECT
        e.id,
        e.student_id,
        e.class_id,
        e.term_id,
        COALESCE(a.total_days, 0) AS total_days,
        COALESCE(a.present_days, 0) AS present_days,
        COALESCE(a.absent_days, 0) AS absent_days,
        COALESCE(a.sick_days, 0) AS sick_days,
        COALESCE(a.permission_days, 0) AS permission_days,
        COALESCE(g.average_grade, 0) AS average_grade,
        COALESCE(g.min_grade, 0) AS min_grade,
        COALESCE(g.max_grade, 0) AS max_grade,
        COALESCE(g.subjects_passed, 0) AS subjects_passed,
        COALESCE(g.subjects_failed, 0) AS subjects_failed,
        COALESCE(b.total_positive_points, 0) AS total_positive_points,
        COALESCE(b.total_negative_points, 0) AS total_negative_points,
        COALESCE(b.behavior_balance, 0) AS behavior_balance
    FROM enrollments e
    LEFT JOIN attendance a ON a.enrollment_id = e.id
    LEFT JOIN grades g ON g.enrollment_id = e.id
    LEFT JOIN behavior b ON b.term_id = e.term_id AND b.student_id = e.student_id
    WHERE e.status = 'ACTIVE'
)
SELECT
    c.id AS class_id,
    c.name AS class_name,
    c.grade,
    c.track,
    t.id AS term_id,
    t.name AS term_name,
    t.academic_year,
    COUNT(e.student_id)::INT AS total_students,
    COALESCE(sc.total_subjects, 0)::INT AS total_subjects,
    COALESCE(ROUND(AVG(CASE WHEN e.total_days = 0 THEN 0 ELSE 100.0 * e.present_days / e.total_days END)::NUMERIC, 2), 0) AS avg_attendance_rate,
    COALESCE(SUM(e.present_days), 0)::INT AS total_present,
    COALESCE(SUM(e.absent_days), 0)::INT AS total_absent,
    COALESCE(SUM(e.sick_days), 0)::INT AS total_sick,
    COALESCE(SUM(e.permission_days), 0)::INT AS total_permission,
    COALESCE(ROUND(AVG(NULLIF(e.average_grade, 0))::NUMERIC, 2), 0) AS avg_grade,
    COALESCE(SUM(CASE WHEN e.average_grade >= 75 THEN 1 ELSE 0 END), 0)::INT AS students_passed,
    COALESCE(SUM(CASE WHEN e.average_grade > 0 AND e.average_grade < 75 THEN 1 ELSE 0 END), 0)::INT AS students_failed,
    COALESCE(MIN(NULLIF(e.min_grade, 0)), 0) AS min_grade,
    COALESCE(MAX(NULLIF(e.max_grade, 0)), 0) AS max_grade,
    COALESCE(SUM(e.total_positive_points), 0)::INT AS total_positive_points,
    COALESCE(SUM(e.total_negative_points), 0)::INT AS total_negative_points,
    COALESCE(SUM(e.behavior_balance), 0)::INT AS behavior_balance,
    NOW() AS refreshed_at
FROM classes c
CROSS JOIN terms t
LEFT JOIN enrolled e ON e.class_id = c.id AND e.term_id = t.id
LEFT JOIN subject_counts sc ON sc.class_id = c.id
GROUP BY c.id, c.name, c.grade, c.track, t.id, t.name, t.academic_year, sc.total_subjects;

CREATE UNIQUE INDEX mv_class_statistics_idx ON mv_class_statistics(class_id, term_id);

CREATE MATERIALIZED VIEW mv_student_performance AS
WITH attendance AS (
    SELECT
        enrollment_id,
        COUNT(*) AS total_attendance_days,
        COUNT(*) FILTER (WHERE status = 'H') AS days_present,
        COUNT(*) FILTER (WHERE status = 'A') AS days_absent,
        COUNT(*) FILTER (WHERE status = 'S') AS days_sick,
        COUNT(*) FILTER (WHERE status = 'I') AS days_permission
    FROM daily_attendance
    GROUP BY enrollment_id
), grades AS (
    SELECT
        enrollment_id,
        COUNT(*) AS subjects_enrolled,
        AVG(final_grade) AS gpa,
        MIN(final_grade) AS lowest_grade,
        MAX(final_grade) AS highest_grade,
        COUNT(*) FILTER (WHERE final_grade >= 75) AS subjects_passed,
        COUNT(*) FILTER (WHERE final_grade < 75) AS subjects_failed
    FROM grade_finals
    GROUP BY enrollment_id
), subject_grades AS (
    SELECT
        gf.enrollment_id,
        JSON_AGG(JSON_BUILD_OBJECT(
            'subject_id', gf.subject_id,
            'subject_name', s.name,
            'subject_code', s.code,
            'final_grade', gf.final_grade
        ) ORDER BY s.name, gf.subject_id) AS subject_grades
    FROM grade_finals gf
    JOIN subjects s ON s.id = gf.subject_id
    GROUP BY gf.enrollment_id
), behavior AS (
    SELECT
        e.term_id,
        bn.student_id,
        COALESCE(SUM(bn.points), 0) AS behavior_points,
        COUNT(*) FILTER (WHERE bn.note_type = '+') AS positive_notes,
        COUNT(*) FILTER (WHERE bn.note_type = '-') AS negative_notes,
        COUNT(*) FILTER (WHERE bn.note_type = '0') AS neutral_notes
    FROM behavior_notes bn
    JOIN enrollments e ON e.student_id = bn.student_id
    JOIN terms t ON t.id = e.term_id
                 AND bn.date >= t.start_date
                 AND bn.date <= t.end_date
    GROUP BY e.term_id, bn.student_id
)
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
    e.status AS enrollment_status,
    e.joined_at,
    e.left_at,
    COALESCE(a.total_attendance_days, 0)::INT AS total_attendance_days,
    COALESCE(a.days_present, 0)::INT AS days_present,
    COALESCE(a.days_absent, 0)::INT AS days_absent,
    COALESCE(a.days_sick, 0)::INT AS days_sick,
    COALESCE(a.days_permission, 0)::INT AS days_permission,
    CASE WHEN COALESCE(a.total_attendance_days, 0) = 0 THEN 0
         ELSE ROUND((100.0 * a.days_present / a.total_attendance_days)::NUMERIC, 2) END AS attendance_percentage,
    COALESCE(g.subjects_enrolled, 0)::INT AS subjects_enrolled,
    COALESCE(g.gpa, 0) AS gpa,
    COALESCE(g.lowest_grade, 0) AS lowest_grade,
    COALESCE(g.highest_grade, 0) AS highest_grade,
    COALESCE(g.subjects_passed, 0)::INT AS subjects_passed,
    COALESCE(g.subjects_failed, 0)::INT AS subjects_failed,
    COALESCE(sg.subject_grades, '[]'::JSON) AS subject_grades,
    COALESCE(b.behavior_points, 0)::INT AS behavior_points,
    COALESCE(b.positive_notes, 0)::INT AS positive_notes,
    COALESCE(b.negative_notes, 0)::INT AS negative_notes,
    COALESCE(b.neutral_notes, 0)::INT AS neutral_notes,
    NOW() AS refreshed_at
FROM students s
JOIN enrollments e ON e.student_id = s.id AND e.status = 'ACTIVE'
JOIN classes c ON c.id = e.class_id
JOIN terms t ON t.id = e.term_id
LEFT JOIN attendance a ON a.enrollment_id = e.id
LEFT JOIN grades g ON g.enrollment_id = e.id
LEFT JOIN subject_grades sg ON sg.enrollment_id = e.id
LEFT JOIN behavior b ON b.term_id = e.term_id AND b.student_id = e.student_id
WHERE s.active = true;

CREATE UNIQUE INDEX mv_student_performance_idx ON mv_student_performance(student_id, class_id, term_id);

CREATE MATERIALIZED VIEW mv_subject_statistics AS
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
    cs.teacher_id,
    u.full_name AS teacher_name,
    COUNT(gf.final_grade)::INT AS total_students,
    COALESCE(ROUND(AVG(gf.final_grade)::NUMERIC, 2), 0) AS avg_grade,
    COALESCE(STDDEV_POP(gf.final_grade), 0) AS grade_stddev,
    COALESCE(MIN(gf.final_grade), 0) AS min_grade,
    COALESCE(MAX(gf.final_grade), 0) AS max_grade,
    COALESCE(PERCENTILE_CONT(0.25) WITHIN GROUP (ORDER BY gf.final_grade), 0) AS percentile_25,
    COALESCE(PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY gf.final_grade), 0) AS median_grade,
    COALESCE(PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY gf.final_grade), 0) AS percentile_75,
    COUNT(*) FILTER (WHERE gf.final_grade >= 75)::INT AS students_passed,
    COUNT(*) FILTER (WHERE gf.final_grade < 75)::INT AS students_failed,
    COALESCE(100.0 * COUNT(*) FILTER (WHERE gf.final_grade >= 75) / NULLIF(COUNT(gf.final_grade), 0), 0) AS pass_rate,
    COUNT(*) FILTER (WHERE gf.final_grade >= 90)::INT AS grade_a_count,
    COUNT(*) FILTER (WHERE gf.final_grade >= 80 AND gf.final_grade < 90)::INT AS grade_b_count,
    COUNT(*) FILTER (WHERE gf.final_grade >= 70 AND gf.final_grade < 80)::INT AS grade_c_count,
    COUNT(*) FILTER (WHERE gf.final_grade >= 60 AND gf.final_grade < 70)::INT AS grade_d_count,
    COUNT(*) FILTER (WHERE gf.final_grade < 60)::INT AS grade_e_count,
    '[]'::JSON AS component_averages,
    NOW() AS refreshed_at
FROM subjects sub
JOIN class_subjects cs ON cs.subject_id = sub.id
JOIN classes c ON c.id = cs.class_id
CROSS JOIN terms t
LEFT JOIN enrollments e ON e.class_id = cs.class_id AND e.term_id = t.id AND e.status = 'ACTIVE'
LEFT JOIN grade_finals gf ON gf.enrollment_id = e.id AND gf.subject_id = sub.id
LEFT JOIN teachers tch ON tch.id = cs.teacher_id
LEFT JOIN users u ON u.id = tch.user_id
GROUP BY sub.id, sub.code, sub.name, sub.subject_group, cs.class_id, c.name,
         c.grade, c.track, t.id, t.name, t.academic_year, cs.teacher_id, u.full_name;

CREATE UNIQUE INDEX mv_subject_statistics_idx ON mv_subject_statistics(subject_id, class_id, term_id);

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

COMMENT ON FUNCTION refresh_analytics_mvs() IS
'Refreshes all analytics materialized views without multiplying attendance, grade, or behavior rows.';
