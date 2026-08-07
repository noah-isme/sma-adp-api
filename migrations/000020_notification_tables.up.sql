-- Migration 000020: Notification tables for push, email, and SMS queues
-- Creates tables for notification delivery management

-- Notification queue table (generic queue for all notification types)
CREATE TABLE IF NOT EXISTS notification_queue (
    id VARCHAR(36) PRIMARY KEY,
    notification_type VARCHAR(20) NOT NULL, -- 'PUSH', 'EMAIL', 'SMS'
    recipient_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject VARCHAR(500),
    body TEXT NOT NULL,
    template_id VARCHAR(100), -- Optional template reference
    template_data JSONB, -- Data for template rendering
    priority VARCHAR(20) NOT NULL DEFAULT 'NORMAL', -- 'LOW', 'NORMAL', 'HIGH', 'URGENT'
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'SENT', 'FAILED', 'RETRYING', 'CANCELLED'
    scheduled_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    last_error TEXT,
    provider_response JSONB, -- Store provider-specific response (FCM, SendGrid, Twilio, etc.)
    metadata JSONB, -- Additional context (e.g., { "announcementId": "ann_001" })
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notification_queue_recipient ON notification_queue(recipient_id);
CREATE INDEX IF NOT EXISTS idx_notification_queue_status ON notification_queue(status);
CREATE INDEX IF NOT EXISTS idx_notification_queue_type ON notification_queue(notification_type);
CREATE INDEX IF NOT EXISTS idx_notification_queue_scheduled ON notification_queue(scheduled_at);
CREATE INDEX IF NOT EXISTS idx_notification_queue_priority ON notification_queue(priority);
CREATE INDEX IF NOT EXISTS idx_notification_queue_processing ON notification_queue(status, scheduled_at) WHERE status IN ('PENDING', 'RETRYING');

-- Push notification specific table (device token management extended)
CREATE TABLE IF NOT EXISTS push_notifications (
    id VARCHAR(36) PRIMARY KEY,
    queue_id VARCHAR(36) NOT NULL REFERENCES notification_queue(id) ON DELETE CASCADE,
    device_token_id VARCHAR(36) NOT NULL REFERENCES device_tokens(id) ON DELETE CASCADE,
    platform VARCHAR(20) NOT NULL, -- 'ios', 'android', 'web'
    fcm_message_id VARCHAR(200), -- FCM/APNs message ID for tracking
    apns_collapse_id VARCHAR(64), -- APNs collapse ID
    payload JSONB NOT NULL, -- Full FCM/APNs payload
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP,
    delivered_at TIMESTAMP,
    opened_at TIMESTAMP,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'SENT', 'DELIVERED', 'OPENED', 'FAILED'
    error_code VARCHAR(50),
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_push_notifications_queue ON push_notifications(queue_id);
CREATE INDEX IF NOT EXISTS idx_push_notifications_device ON push_notifications(device_token_id);
CREATE INDEX IF NOT EXISTS idx_push_notifications_status ON push_notifications(status);

-- Email notification specific table
CREATE TABLE IF NOT EXISTS email_notifications (
    id VARCHAR(36) PRIMARY KEY,
    queue_id VARCHAR(36) NOT NULL REFERENCES notification_queue(id) ON DELETE CASCADE,
    to_email VARCHAR(255) NOT NULL,
    cc_emails TEXT[], -- Array of CC emails
    bcc_emails TEXT[], -- Array of BCC emails
    from_email VARCHAR(255) NOT NULL DEFAULT 'noreply@sma-adp.id',
    from_name VARCHAR(255) NOT NULL DEFAULT 'SMA ADP System',
    reply_to VARCHAR(255),
    message_id VARCHAR(200), -- SendGrid/Mailgun message ID
    subject VARCHAR(500) NOT NULL,
    html_body TEXT,
    text_body TEXT,
    headers JSONB, -- Custom headers
    tracking_enabled BOOLEAN NOT NULL DEFAULT true,
    opened_at TIMESTAMP,
    clicked_at TIMESTAMP,
    bounced_at TIMESTAMP,
    complained_at TIMESTAMP,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'SENT', 'DELIVERED', 'OPENED', 'CLICKED', 'BOUNCED', 'COMPLAINED', 'FAILED'
    error_code VARCHAR(50),
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP,
    delivered_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_email_notifications_queue ON email_notifications(queue_id);
CREATE INDEX IF NOT EXISTS idx_email_notifications_to ON email_notifications(to_email);
CREATE INDEX IF NOT EXISTS idx_email_notifications_status ON email_notifications(status);
CREATE INDEX IF NOT EXISTS idx_email_notifications_message ON email_notifications(message_id);

-- SMS notification specific table
CREATE TABLE IF NOT EXISTS sms_notifications (
    id VARCHAR(36) PRIMARY KEY,
    queue_id VARCHAR(36) NOT NULL REFERENCES notification_queue(id) ON DELETE CASCADE,
    to_phone VARCHAR(50) NOT NULL,
    from_phone VARCHAR(50),
    provider VARCHAR(50) NOT NULL, -- 'twilio', 'vonage', 'plivo', 'local'
    provider_message_id VARCHAR(200),
    body TEXT NOT NULL,
    segments INT NOT NULL DEFAULT 1,
    cost DECIMAL(10, 4), -- Cost in IDR or USD
    currency VARCHAR(3) DEFAULT 'IDR',
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'SENT', 'DELIVERED', 'FAILED', 'UNDELIVERED'
    error_code VARCHAR(50),
    error_message TEXT,
    sent_at TIMESTAMP,
    delivered_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sms_notifications_queue ON sms_notifications(queue_id);
CREATE INDEX IF NOT EXISTS idx_sms_notifications_to ON sms_notifications(to_phone);
CREATE INDEX IF NOT EXISTS idx_sms_notifications_status ON sms_notifications(status);
CREATE INDEX IF NOT EXISTS idx_sms_notifications_provider ON sms_notifications(provider_message_id);

-- Notification preferences per user (extending portal_preferences)
CREATE TABLE IF NOT EXISTS notification_preferences (
    user_id VARCHAR(255) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- Global toggles
    email_enabled BOOLEAN NOT NULL DEFAULT true,
    push_enabled BOOLEAN NOT NULL DEFAULT true,
    sms_enabled BOOLEAN NOT NULL DEFAULT false,
    -- Quiet hours
    quiet_hours_start TIME, -- e.g., '22:00:00'
    quiet_hours_end TIME,   -- e.g., '07:00:00'
    timezone VARCHAR(50) NOT NULL DEFAULT 'Asia/Jakarta',
    -- Category preferences
    grade_notifications BOOLEAN NOT NULL DEFAULT true,
    attendance_notifications BOOLEAN NOT NULL DEFAULT true,
    behavior_notifications BOOLEAN NOT NULL DEFAULT true,
    announcement_notifications BOOLEAN NOT NULL DEFAULT true,
    schedule_notifications BOOLEAN NOT NULL DEFAULT true,
    system_notifications BOOLEAN NOT NULL DEFAULT true,
    -- Frequency
    digest_frequency VARCHAR(20) NOT NULL DEFAULT 'REALTIME', -- 'REALTIME', 'HOURLY', 'DAILY', 'WEEKLY', 'NEVER'
    -- Per-channel category overrides (JSON for flexibility)
    email_categories JSONB, -- e.g., {"grade": true, "attendance": false}
    push_categories JSONB,
    sms_categories JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Notification templates
CREATE TABLE IF NOT EXISTS notification_templates (
    id VARCHAR(36) PRIMARY KEY,
    code VARCHAR(100) UNIQUE NOT NULL, -- e.g., 'GRADE_PUBLISHED', 'ATTENDANCE_ABSENT', 'ANNOUNCEMENT_NEW'
    name VARCHAR(200) NOT NULL,
    description TEXT,
    channel VARCHAR(20) NOT NULL, -- 'PUSH', 'EMAIL', 'SMS', 'ALL'
    subject_template VARCHAR(500), -- For email/push
    body_template TEXT NOT NULL, -- Template with {{placeholders}}
    html_template TEXT, -- HTML version for email
    variables JSONB NOT NULL DEFAULT '[]', -- List of expected variables: ["studentName", "subjectName", "grade"]
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_system BOOLEAN NOT NULL DEFAULT true, -- System templates can't be deleted
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notification_templates_code ON notification_templates(code);
CREATE INDEX IF NOT EXISTS idx_notification_templates_channel ON notification_templates(channel);

-- Insert default system templates
INSERT INTO notification_templates (id, code, name, description, channel, subject_template, body_template, html_template, variables, is_system) VALUES
('tmpl_001', 'GRADE_PUBLISHED', 'Grade Published', 'Notify parent/student when new grade is published', 'ALL',
 'New Grade: {{subjectName}}',
 'Hi {{recipientName}}, a new grade has been published for {{studentName}} in {{subjectName}}: {{gradeValue}} ({{letterGrade}}).',
 '<p>Hi {{recipientName}},</p><p>A new grade has been published for <strong>{{studentName}}</strong> in <strong>{{subjectName}}</strong>: <strong>{{gradeValue}}</strong> ({{letterGrade}}).</p>',
 '["recipientName", "studentName", "subjectName", "gradeValue", "letterGrade"]', true),
('tmpl_002', 'ATTENDANCE_ABSENT', 'Student Absent', 'Notify parent when student is marked absent', 'ALL',
 'Attendance Alert: {{studentName}} Absent',
 'Hi {{recipientName}}, {{studentName}} was marked absent on {{date}} ({{status}}).',
 '<p>Hi {{recipientName}},</p><p><strong>{{studentName}}</strong> was marked absent on <strong>{{date}}</strong> ({{status}}).</p>',
 '["recipientName", "studentName", "date", "status"]', true),
('tmpl_003', 'ANNOUNCEMENT_NEW', 'New Announcement', 'Notify about new announcement', 'ALL',
 'New Announcement: {{title}}',
 'Hi {{recipientName}}, a new announcement has been posted: {{title}}. {{content}}',
 '<p>Hi {{recipientName}},</p><p>A new announcement has been posted: <strong>{{title}}</strong>.</p><p>{{content}}</p>',
 '["recipientName", "title", "content"]', true),
('tmpl_004', 'BEHAVIOR_NOTE', 'Behavior Note Added', 'Notify parent about behavior note', 'ALL',
 'Behavior Note: {{category}} for {{studentName}}',
 'Hi {{recipientName}}, a {{category}} behavior note was added for {{studentName}}: {{title}} - {{description}} ({{points}} points).',
 '<p>Hi {{recipientName}},</p><p>A <strong>{{category}}</strong> behavior note was added for <strong>{{studentName}}</strong>: {{title}} - {{description}} ({{points}} points).</p>',
 '["recipientName", "studentName", "category", "title", "description", "points"]', true),
('tmpl_005', 'SCHEDULE_CHANGE', 'Schedule Changed', 'Notify about schedule changes', 'ALL',
 'Schedule Update: {{subjectName}}',
 'Hi {{recipientName}}, the schedule for {{subjectName}} has been changed. New time: {{newTime}}, Room: {{room}}.',
 '<p>Hi {{recipientName}},</p><p>The schedule for <strong>{{subjectName}}</strong> has been changed.</p><p>New time: <strong>{{newTime}}</strong>, Room: <strong>{{room}}</strong>.</p>',
 '["recipientName", "subjectName", "newTime", "room"]', true)
ON CONFLICT (code) DO NOTHING;