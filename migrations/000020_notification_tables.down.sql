-- Migration 000020: Notification tables - Down migration

DROP TABLE IF EXISTS notification_templates;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS sms_notifications;
DROP TABLE IF EXISTS email_notifications;
DROP TABLE IF EXISTS push_notifications;
DROP TABLE IF EXISTS notification_queue;