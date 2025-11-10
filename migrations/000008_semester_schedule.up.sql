CREATE TABLE IF NOT EXISTS semester_schedules (
    id VARCHAR(36) PRIMARY KEY,
    term_id VARCHAR(36) NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    class_id VARCHAR(36) NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    meta JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(term_id, class_id, version)
);

CREATE INDEX IF NOT EXISTS idx_sem_sched_term_class ON semester_schedules(term_id, class_id);

CREATE TABLE IF NOT EXISTS semester_schedule_slots (
    id VARCHAR(36) PRIMARY KEY,
    semester_schedule_id VARCHAR(36) NOT NULL REFERENCES semester_schedules(id) ON DELETE CASCADE,
    day_of_week SMALLINT NOT NULL,
    time_slot SMALLINT NOT NULL,
    subject_id VARCHAR(36) NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    teacher_id VARCHAR(36) NOT NULL REFERENCES teachers(id) ON DELETE RESTRICT,
    room VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(semester_schedule_id, day_of_week, time_slot)
);

CREATE INDEX IF NOT EXISTS idx_sem_sched_slots_teacher ON semester_schedule_slots(teacher_id, day_of_week, time_slot);
