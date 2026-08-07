-- Migration 000019: Portal tables for Parent/Student access
-- Creates tables for parent-student relationships, portal preferences, and device tokens

-- Parent-Student relationship table
CREATE TABLE IF NOT EXISTS parent_students (
    id VARCHAR(36) PRIMARY KEY,
    parent_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    student_id VARCHAR(255) NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    relationship VARCHAR(50) NOT NULL DEFAULT 'PARENT',
    can_view_grades BOOLEAN NOT NULL DEFAULT true,
    can_view_attendance BOOLEAN NOT NULL DEFAULT true,
    can_view_behavior BOOLEAN NOT NULL DEFAULT true,
    can_view_announcements BOOLEAN NOT NULL DEFAULT true,
    can_receive_notifications BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(parent_id, student_id)
);

CREATE INDEX IF NOT EXISTS idx_parent_students_parent ON parent_students(parent_id);
CREATE INDEX IF NOT EXISTS idx_parent_students_student ON parent_students(student_id);

-- Portal preferences per user
CREATE TABLE IF NOT EXISTS portal_preferences (
    user_id VARCHAR(255) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    language VARCHAR(10) NOT NULL DEFAULT 'id',
    timezone VARCHAR(50) NOT NULL DEFAULT 'Asia/Jakarta',
    email_notifications BOOLEAN NOT NULL DEFAULT true,
    push_notifications BOOLEAN NOT NULL DEFAULT true,
    sms_notifications BOOLEAN NOT NULL DEFAULT false,
    grade_alerts BOOLEAN NOT NULL DEFAULT true,
    attendance_alerts BOOLEAN NOT NULL DEFAULT true,
    behavior_alerts BOOLEAN NOT NULL DEFAULT true,
    announcement_alerts BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Device tokens for push notifications
CREATE TABLE IF NOT EXISTS device_tokens (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(500) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    device_id VARCHAR(255),
    app_version VARCHAR(50),
    last_used_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, token)
);

CREATE INDEX IF NOT EXISTS idx_device_tokens_user ON device_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_device_tokens_platform ON device_tokens(platform);