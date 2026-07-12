-- Reverse of 000001: drop all base tables in reverse FK dependency order.

DROP TABLE IF EXISTS subject_attendance;
DROP TABLE IF EXISTS daily_attendance;
DROP TABLE IF EXISTS calendar_events;
DROP TABLE IF EXISTS behavior_notes;
DROP TABLE IF EXISTS announcements;
DROP TABLE IF EXISTS grade_finals;
DROP TABLE IF EXISTS grades;
DROP TABLE IF EXISTS grade_config_components;
DROP TABLE IF EXISTS grade_configs;
DROP TABLE IF EXISTS grade_components;
DROP TABLE IF EXISTS enrollments;
DROP TABLE IF EXISTS students;
DROP TABLE IF EXISTS schedules;
DROP TABLE IF EXISTS class_subjects;
DROP TABLE IF EXISTS classes;
DROP TABLE IF EXISTS subjects;
DROP TABLE IF EXISTS terms;
DROP TABLE IF EXISTS users;
