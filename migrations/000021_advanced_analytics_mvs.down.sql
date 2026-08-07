-- Migration 000021: Advanced materialized views - Down migration

DROP FUNCTION IF EXISTS refresh_analytics_mvs();
DROP MATERIALIZED VIEW IF EXISTS mv_subject_statistics;
DROP MATERIALIZED VIEW IF EXISTS mv_student_performance;
DROP MATERIALIZED VIEW IF EXISTS mv_class_statistics;