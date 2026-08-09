package archives

import (
	"time"

	"github.com/google/uuid"
)

// DocumentResponse is the DTO representation for returning document details over API.
type DocumentResponse struct {
	ID                uuid.UUID              `json:"id"`
	Filename          string                 `json:"filename"`
	OriginalFilename  string                 `json:"originalFilename"`
	MimeType          string                 `json:"mimeType"`
	SizeBytes         int64                  `json:"sizeBytes"`
	Checksum          string                 `json:"checksum"`
	StorageTier       StorageTier            `json:"storageTier"`
	Category          DocumentCategory       `json:"category"`
	Tags              []string               `json:"tags"`
	Metadata          map[string]interface{} `json:"metadata"`
	OCRStatus         OCRStatus              `json:"ocrStatus"`
	OCRText           string                 `json:"ocrText,omitempty"`
	RetentionPolicyID *uuid.UUID             `json:"retentionPolicyId,omitempty"`
	RetainUntil       time.Time              `json:"retainUntil"`
	LegalHold         bool                   `json:"legalHold"`
	LegalHoldReason   string                 `json:"legalHoldReason,omitempty"`
	UploadedBy        uuid.UUID              `json:"uploadedBy"`
	UploadedAt        time.Time              `json:"uploadedAt"`
	DownloadURL       string                 `json:"downloadUrl,omitempty"`
}

// SearchResultItem represents one document item in search results with highlight snippets.
type SearchResultItem struct {
	DocumentResponse
	Snippet string `json:"snippet,omitempty"`
}

// UpdateRetentionRequest holds data for modifying retention date or legal hold status.
type UpdateRetentionRequest struct {
	Action      string     `json:"action"` // EXTEND, REDUCE, LEGAL_HOLD, RELEASE_HOLD
	RetainUntil *time.Time `json:"retainUntil,omitempty"`
	Reason      string     `json:"reason,omitempty"`
}

// BulkActionRequest holds parameters for executing bulk operations.
type BulkActionRequest struct {
	Action     string            `json:"action"` // DOWNLOAD, DELETE, CHANGE_CATEGORY, APPLY_RETENTION
	IDs        []uuid.UUID       `json:"ids"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

// BulkActionResponse reports results of bulk processing.
type BulkActionResponse struct {
	ProcessedCount int         `json:"processedCount"`
	SkippedCount   int         `json:"skippedCount"`
	FailedCount    int         `json:"failedCount"`
	Errors         []string    `json:"errors,omitempty"`
	SkippedIDs     []uuid.UUID `json:"skippedIds,omitempty"`
}

// GDPRRequest holds parameters for processing GDPR/PDPA data subject requests.
type GDPRRequest struct {
	Type           string            `json:"type"` // ACCESS, RECTIFICATION, ERASURE, PORTABILITY
	SubjectID      string            `json:"subjectId"`
	DocumentID     *uuid.UUID        `json:"documentId,omitempty"`
	RequesterEmail string            `json:"requesterEmail"`
	Corrections    map[string]string `json:"corrections,omitempty"`
}

// GDPRResponse reports outcomes of GDPR request processing.
type GDPRResponse struct {
	RequestID string      `json:"requestId"`
	Type      string      `json:"type"`
	Status    string      `json:"status"`
	ExportURL string      `json:"exportUrl,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// RetentionPolicyResponse returns details of a retention policy.
type RetentionPolicyResponse struct {
	ID                uuid.UUID        `json:"id"`
	Name              string           `json:"name"`
	Category          DocumentCategory `json:"category"`
	RetentionYears    int              `json:"retentionYears"`
	AutoDelete        bool             `json:"autoDelete"`
	LegalHoldOverride bool             `json:"legalHoldOverride"`
	Description       string           `json:"description"`
}

// CreateRetentionPolicyRequest payload for creating a new retention policy.
type CreateRetentionPolicyRequest struct {
	Name              string           `json:"name"`
	Category          DocumentCategory `json:"category"`
	RetentionYears    int              `json:"retentionYears"`
	AutoDelete        bool             `json:"autoDelete"`
	LegalHoldOverride bool             `json:"legalHoldOverride"`
	Description       string           `json:"description"`
}

// AuditLogFilter specifies query parameters for querying audit logs.
type AuditLogFilter struct {
	DocumentID *uuid.UUID `json:"documentId,omitempty"`
	Action     string     `json:"action,omitempty"`
	UserID     *uuid.UUID `json:"userId,omitempty"`
	DateFrom   *time.Time `json:"dateFrom,omitempty"`
	DateTo     *time.Time `json:"dateTo,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
}

// AuditLogResponse wraps paginated audit log queries.
type AuditLogResponse struct {
	Data  []*AuditLog `json:"data"`
	Total int64       `json:"total"`
}

