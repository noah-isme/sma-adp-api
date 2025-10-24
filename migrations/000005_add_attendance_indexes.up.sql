CREATE INDEX IF NOT EXISTS idx_daily_attendance_class_date ON daily_attendance(enrollment_id, date);
CREATE INDEX IF NOT EXISTS idx_subject_attendance_schedule_date ON subject_attendance(schedule_id, date);
CREATE INDEX IF NOT EXISTS idx_announcements_active ON announcements(published_at, expires_at) WHERE expires_at IS NULL OR expires_at > NOW();
CREATE INDEX IF NOT EXISTS idx_behavior_notes_student_date ON behavior_notes(student_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_calendar_events_date_range ON calendar_events(start_date, end_date);
