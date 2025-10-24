CREATE MATERIALIZED VIEW IF NOT EXISTS attendance_summary_mv AS
SELECT
    e.term_id,
    e.class_id,
    SUM(CASE WHEN da.status = 'H' THEN 1 ELSE 0 END) AS present_count,
    SUM(CASE WHEN da.status = 'A' THEN 1 ELSE 0 END) AS absent_count,
    CASE WHEN COUNT(*) = 0 THEN 0 ELSE (SUM(CASE WHEN da.status = 'H' THEN 1 ELSE 0 END)::DECIMAL / COUNT(*)) * 100 END AS percentage,
    MAX(da.updated_at) AS updated_at
FROM daily_attendances da
JOIN enrollments e ON e.id = da.enrollment_id
GROUP BY e.term_id, e.class_id;

CREATE UNIQUE INDEX IF NOT EXISTS attendance_summary_mv_idx ON attendance_summary_mv(term_id, class_id);

CREATE MATERIALIZED VIEW IF NOT EXISTS grade_summary_mv AS
WITH student_scores AS (
    SELECT
        e.term_id,
        e.class_id,
        g.subject_id,
        e.student_id,
        AVG(g.grade_value) AS avg_score
    FROM grades g
    JOIN enrollments e ON e.id = g.enrollment_id
    GROUP BY e.term_id, e.class_id, g.subject_id, e.student_id
), ranked AS (
    SELECT
        term_id,
        class_id,
        subject_id,
        student_id,
        avg_score,
        DENSE_RANK() OVER (PARTITION BY term_id, class_id, subject_id ORDER BY avg_score DESC) AS rank
    FROM student_scores
)
SELECT
    term_id,
    class_id,
    subject_id,
    AVG(avg_score) AS avg_score,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY avg_score) AS median_score,
    COALESCE(JSON_AGG(JSON_BUILD_OBJECT('student_id', student_id, 'score', avg_score, 'rank', rank) ORDER BY rank), '[]'::JSON) AS rank_json,
    NOW() AS updated_at
FROM ranked
GROUP BY term_id, class_id, subject_id;

CREATE UNIQUE INDEX IF NOT EXISTS grade_summary_mv_idx ON grade_summary_mv(term_id, class_id, subject_id);

CREATE MATERIALIZED VIEW IF NOT EXISTS behavior_summary_mv AS
SELECT
    e.term_id,
    bn.student_id,
    SUM(CASE WHEN bn.points > 0 THEN bn.points ELSE 0 END) AS total_positive,
    SUM(CASE WHEN bn.points < 0 THEN ABS(bn.points) ELSE 0 END) AS total_negative,
    SUM(bn.points) AS balance,
    MAX(bn.updated_at) AS updated_at
FROM behavior_notes bn
JOIN enrollments e ON e.student_id = bn.student_id
GROUP BY e.term_id, bn.student_id;

CREATE UNIQUE INDEX IF NOT EXISTS behavior_summary_mv_idx ON behavior_summary_mv(term_id, student_id);
