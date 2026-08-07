-- Migration 000019: Portal tables for Parent/Student access - Rollback
DROP TABLE IF EXISTS device_tokens;
DROP TABLE IF EXISTS portal_preferences;
DROP TABLE IF EXISTS parent_students;