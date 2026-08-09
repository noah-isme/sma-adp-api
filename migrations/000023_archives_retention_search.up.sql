-- Migration: 000023_archives_retention_search.up.sql
-- Description: Create tables for Archives module retention policies, document metadata, and audit log.

CREATE TABLE IF NOT EXISTS retention_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    category VARCHAR(50) NOT NULL,
    retention_years INT NOT NULL,
    auto_delete BOOLEAN DEFAULT TRUE,
    legal_hold_override BOOLEAN DEFAULT FALSE,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_retention_policies_category ON retention_policies (category);

CREATE TABLE IF NOT EXISTS archive_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    checksum VARCHAR(64) NOT NULL,
    storage_path VARCHAR(500) NOT NULL,
    storage_tier VARCHAR(10) DEFAULT 'HOT',
    category VARCHAR(50) NOT NULL,
    tags TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    ocr_text TEXT,
    ocr_status VARCHAR(20) DEFAULT 'PENDING',
    retention_policy_id UUID REFERENCES retention_policies(id) ON DELETE SET NULL,
    retain_until DATE NOT NULL,
    legal_hold BOOLEAN DEFAULT FALSE,
    legal_hold_reason TEXT,
    uploaded_by UUID NOT NULL,
    uploaded_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_archive_documents_category ON archive_documents (category);
CREATE INDEX IF NOT EXISTS idx_archive_documents_tags ON archive_documents USING GIN (tags);
CREATE INDEX IF NOT EXISTS idx_archive_documents_retain_until ON archive_documents (retain_until);
CREATE INDEX IF NOT EXISTS idx_archive_documents_legal_hold ON archive_documents (legal_hold);
CREATE INDEX IF NOT EXISTS idx_archive_documents_storage_tier ON archive_documents (storage_tier);
CREATE INDEX IF NOT EXISTS idx_archive_documents_uploaded_by ON archive_documents (uploaded_by);
CREATE INDEX IF NOT EXISTS idx_archive_documents_uploaded_at ON archive_documents (uploaded_at);
CREATE INDEX IF NOT EXISTS idx_archive_documents_deleted_at ON archive_documents (deleted_at) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS archive_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID REFERENCES archive_documents(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    user_id UUID NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    details JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_archive_audit_log_document_id ON archive_audit_log (document_id);
CREATE INDEX IF NOT EXISTS idx_archive_audit_log_action ON archive_audit_log (action);
CREATE INDEX IF NOT EXISTS idx_archive_audit_log_user_id ON archive_audit_log (user_id);
CREATE INDEX IF NOT EXISTS idx_archive_audit_log_created_at ON archive_audit_log (created_at);

-- Seed Default Retention Policies for 8 Categories
INSERT INTO retention_policies (id, name, category, retention_years, auto_delete, legal_hold_override, description)
VALUES 
  ('11111111-1111-1111-1111-111111111101', 'Student Record Policy', 'STUDENT_RECORD', 7, TRUE, FALSE, 'PDPA/FERPA compliant student academic records retention'),
  ('11111111-1111-1111-1111-111111111102', 'Grade Report Policy', 'GRADE_REPORT', 7, TRUE, FALSE, 'Academic performance and grade transcript retention'),
  ('11111111-1111-1111-1111-111111111103', 'Attendance Record Policy', 'ATTENDANCE_RECORD', 5, TRUE, FALSE, 'Daily attendance log and absence record retention'),
  ('11111111-1111-1111-1111-111111111104', 'Behavior Note Policy', 'BEHAVIOR_NOTE', 3, TRUE, TRUE, 'Disciplinary notes and counseling session logs'),
  ('11111111-1111-1111-1111-111111111105', 'Medical Record Policy', 'MEDICAL_RECORD', 10, FALSE, FALSE, 'HIPAA/PDPA health forms and immunization history'),
  ('11111111-1111-1111-1111-111111111106', 'Financial Document Policy', 'FINANCIAL_DOCUMENT', 10, TRUE, FALSE, 'Tax invoice, tuition payment, and audit trail retention'),
  ('11111111-1111-1111-1111-111111111107', 'Legal Document Policy', 'LEGAL_DOCUMENT', 99, FALSE, FALSE, 'Permanent legal contracts, waivers, and school charter docs'),
  ('11111111-1111-1111-1111-111111111108', 'Correspondence Policy', 'CORRESPONDENCE', 3, TRUE, TRUE, 'Official parent communication and general administrative notices')
ON CONFLICT (name) DO NOTHING;
