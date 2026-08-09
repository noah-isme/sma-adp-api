package archives

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Common Domain Errors
var (
	ErrDocumentNotFound    = errors.New("archive document not found")
	ErrPolicyNotFound      = errors.New("retention policy not found")
	ErrLegalHoldActive     = errors.New("cannot perform action: legal hold is active on document")
	ErrRetentionNotExpired = errors.New("retention policy has not expired yet")
	ErrInvalidToken        = errors.New("invalid or tampered signed URL token")
	ErrTokenExpired        = errors.New("signed URL token has expired")
	ErrIPMismatch          = errors.New("client IP address does not match signed URL restriction")
)

// DocumentCategory represents document classification types.
type DocumentCategory string

const (
	CategoryStudentRecord    DocumentCategory = "STUDENT_RECORD"
	CategoryGradeReport      DocumentCategory = "GRADE_REPORT"
	CategoryAttendanceRecord DocumentCategory = "ATTENDANCE_RECORD"
	CategoryBehaviorNote     DocumentCategory = "BEHAVIOR_NOTE"
	CategoryMedicalRecord    DocumentCategory = "MEDICAL_RECORD"
	CategoryFinancialDoc     DocumentCategory = "FINANCIAL_DOCUMENT"
	CategoryLegalDoc         DocumentCategory = "LEGAL_DOCUMENT"
	CategoryCorrespondence   DocumentCategory = "CORRESPONDENCE"
	CategoryOther            DocumentCategory = "OTHER"
)

// StorageTier represents physical storage tier classification.
type StorageTier string

const (
	StorageTierHot  StorageTier = "HOT"
	StorageTierWarm StorageTier = "WARM"
	StorageTierCold StorageTier = "COLD"
)

// OCRStatus represents OCR text extraction lifecycle status.
type OCRStatus string

const (
	OCRStatusPending    OCRStatus = "PENDING"
	OCRStatusProcessing OCRStatus = "PROCESSING"
	OCRStatusCompleted  OCRStatus = "COMPLETED"
	OCRStatusFailed     OCRStatus = "FAILED"
)

// Audit Actions
const (
	AuditActionUpload                       = "UPLOAD"
	AuditActionDownload                     = "DOWNLOAD"
	AuditActionSearch                       = "SEARCH"
	AuditActionRetentionChange              = "RETENTION_CHANGE"
	AuditActionLegalHold                    = "LEGAL_HOLD"
	AuditActionDelete                       = "DELETE"
	AuditActionGDPRRequest                  = "GDPR_REQUEST"
	AuditActionTierMigration                = "TIER_MIGRATION"
	AuditActionSkippedLegalHold             = "SKIPPED_LEGAL_HOLD"
	AuditActionRetentionExpired             = "RETENTION_EXPIRED"
	AuditActionRetentionExpiredManualReview = "RETENTION_EXPIRED_MANUAL_REVIEW"
)

// JSONMap is a custom type for handling JSONB fields in PostgreSQL with sqlx.
type JSONMap map[string]interface{}

// Value implements driver.Valuer interface for database storage.
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	return json.Marshal(m)
}

// Scan implements sql.Scanner interface for database retrieval.
func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(JSONMap)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("type assertion to []byte failed: %T", value)
	}
	if len(bytes) == 0 {
		*m = make(JSONMap)
		return nil
	}
	if err := json.Unmarshal(bytes, m); err != nil {
		return err
	}
	if *m == nil {
		*m = make(JSONMap)
	}
	return nil
}

// ArchiveDocument represents an archived document entity.
type ArchiveDocument struct {
	ID                uuid.UUID        `db:"id" json:"id"`
	Filename          string           `db:"filename" json:"filename"`
	OriginalFilename  string           `db:"original_filename" json:"originalFilename"`
	MimeType          string           `db:"mime_type" json:"mimeType"`
	SizeBytes         int64            `db:"size_bytes" json:"sizeBytes"`
	Checksum          string           `db:"checksum" json:"checksum"`
	StoragePath       string           `db:"storage_path" json:"storagePath"`
	StorageTier       StorageTier      `db:"storage_tier" json:"storageTier"`
	Category          DocumentCategory `db:"category" json:"category"`
	Tags              pq.StringArray   `db:"tags" json:"tags"`
	Metadata          JSONMap          `db:"metadata" json:"metadata"`
	OCRText           string           `db:"ocr_text,omitempty" json:"ocrText,omitempty"`
	OCRStatus         OCRStatus        `db:"ocr_status" json:"ocrStatus"`
	RetentionPolicyID *uuid.UUID       `db:"retention_policy_id" json:"retentionPolicyId,omitempty"`
	RetainUntil       time.Time        `db:"retain_until" json:"retainUntil"`
	LegalHold         bool             `db:"legal_hold" json:"legalHold"`
	LegalHoldReason   string           `db:"legal_hold_reason,omitempty" json:"legalHoldReason,omitempty"`
	UploadedBy        uuid.UUID        `db:"uploaded_by" json:"uploadedBy"`
	UploadedAt        time.Time        `db:"uploaded_at" json:"uploadedAt"`
	UpdatedAt         time.Time        `db:"updated_at" json:"updatedAt"`
	DeletedAt         *time.Time       `db:"deleted_at,omitempty" json:"deletedAt,omitempty"`
}

// RetentionPolicy represents a retention policy rule.
type RetentionPolicy struct {
	ID                uuid.UUID        `db:"id" json:"id"`
	Name              string           `db:"name" json:"name"`
	Category          DocumentCategory `db:"category" json:"category"`
	RetentionYears    int              `db:"retention_years" json:"retentionYears"`
	AutoDelete        bool             `db:"auto_delete" json:"autoDelete"`
	LegalHoldOverride bool             `db:"legal_hold_override" json:"legalHoldOverride"`
	Description       string           `db:"description" json:"description"`
	CreatedAt         time.Time        `db:"created_at" json:"createdAt"`
	UpdatedAt         time.Time        `db:"updated_at" json:"updatedAt"`
}

// AuditLog represents an audit log entry for archive activities.
type AuditLog struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	DocumentID *uuid.UUID `db:"document_id" json:"documentId,omitempty"`
	Action     string     `db:"action" json:"action"`
	UserID     uuid.UUID  `db:"user_id" json:"userId"`
	IPAddress  string     `db:"ip_address" json:"ipAddress"`
	UserAgent  string     `db:"user_agent" json:"userAgent"`
	Details    JSONMap    `db:"details" json:"details"`
	CreatedAt  time.Time  `db:"created_at" json:"createdAt"`
}

// ArchiveFilter contains query parameters for filtering document listings.
type ArchiveFilter struct {
	Query          string           `json:"query,omitempty"`
	Category       DocumentCategory `json:"category,omitempty"`
	Tags           []string         `json:"tags,omitempty"`
	LegalHoldOnly  bool             `json:"legalHoldOnly,omitempty"`
	OCRCompleted   bool             `json:"ocrCompleted,omitempty"`
	StorageTier    StorageTier      `json:"storageTier,omitempty"`
	DateFrom       *time.Time       `json:"dateFrom,omitempty"`
	DateTo         *time.Time       `json:"dateTo,omitempty"`
	IncludeDeleted bool             `json:"includeDeleted,omitempty"`
	Limit          int              `json:"limit,omitempty"`
	Offset         int              `json:"offset,omitempty"`
}

// SearchRequest holds parameters for full-text searching archives.
type SearchRequest struct {
	Query         string           `json:"q"`
	Category      DocumentCategory `json:"category"`
	Tags          []string         `json:"tags"`
	StorageTier   StorageTier      `json:"storageTier,omitempty"`
	LegalHoldOnly bool             `json:"legalHoldOnly,omitempty"`
	DateFrom      *time.Time       `json:"dateFrom,omitempty"`
	DateTo        *time.Time       `json:"dateTo,omitempty"`
	Page          int              `json:"page"`
	Limit         int              `json:"limit"`
}

// SearchHit represents a single document match in search results with highlights.
type SearchHit struct {
	ID               uuid.UUID        `json:"id"`
	Filename         string           `json:"filename"`
	OriginalFilename string           `json:"original_filename"`
	MimeType         string           `json:"mime_type"`
	SizeBytes        int64            `json:"size_bytes"`
	Checksum         string           `json:"checksum"`
	StorageTier      StorageTier      `json:"storage_tier"`
	Category         DocumentCategory `json:"category"`
	Tags             []string         `json:"tags"`
	Metadata         JSONMap          `json:"metadata"`
	OCRStatus        OCRStatus        `json:"ocr_status"`
	OCRText          string           `json:"ocr_text,omitempty"`
	Snippet          string           `json:"snippet"`
	RetainUntil      time.Time        `json:"retain_until"`
	LegalHold        bool             `json:"legal_hold"`
	UploadedBy       uuid.UUID        `json:"uploaded_by"`
	UploadedAt       time.Time        `json:"uploaded_at"`
}

// SearchResult wraps paginated search output.
type SearchResult struct {
	Data        []SearchHit `json:"data"`
	Total       int64       `json:"total"`
	Page        int         `json:"page"`
	Limit       int         `json:"limit"`
	QueryTimeMs int64       `json:"query_time_ms"`
}
