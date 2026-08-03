package models

import (
	"time"

	"github.com/jmoiron/sqlx/types"
)

// AuditAction constants represent actions to be logged.
const (
	AuditActionLogin          = "LOGIN"
	AuditActionLogout         = "LOGOUT"
	AuditActionUserCreate     = "USER_CREATE"
	AuditActionUserUpdate     = "USER_UPDATE"
	AuditActionUserDelete     = "USER_DELETE"
	AuditActionPasswordChange = "PASSWORD_CHANGE"
	AuditActionMutationCreate = "MUTATION_REQUEST"
	AuditActionMutationReview = "MUTATION_REVIEW"
	AuditActionArchiveUpload  = "ARCHIVE_UPLOAD"
	AuditActionArchiveDelete  = "ARCHIVE_DELETE"
	AuditActionHomeroomUpdate = "HOMEROOM_UPDATE"
	AuditActionConfigUpdate   = "CONFIGURATION_UPDATE"
	AuditActionCSVImport      = "CSV_IMPORT"
)

// AuditLog represents an audit trail record.
type AuditLog struct {
	ID         string    `db:"id" json:"id"`
	UserID     *string   `db:"user_id" json:"user_id,omitempty"`
	Action     string    `db:"action" json:"action"`
	Resource   string    `db:"resource" json:"resource"`
	ResourceID *string   `db:"resource_id" json:"resource_id,omitempty"`
	OldValues  []byte    `db:"old_values" json:"old_values,omitempty"`
	NewValues  []byte    `db:"new_values" json:"new_values,omitempty"`
	IPAddress  string    `db:"ip_address" json:"ip_address"`
	UserAgent  string    `db:"user_agent" json:"user_agent"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

// AuditLogEntry is the read model for the audit viewer. It enriches a row with
// the actor's identity so the client can render "who" without a lookup per row.
//
// It deliberately does not embed AuditLog: the write model types the JSON
// columns as []byte, which encoding/json emits as base64. A viewer needs the
// stored JSON verbatim, so those columns are typed as types.JSONText here.
type AuditLogEntry struct {
	ID           string          `db:"id" json:"id"`
	UserID       *string         `db:"user_id" json:"user_id,omitempty"`
	Action       string          `db:"action" json:"action"`
	Resource     string          `db:"resource" json:"resource"`
	ResourceID   *string         `db:"resource_id" json:"resource_id,omitempty"`
	OldValues    *types.JSONText `db:"old_values" json:"old_values,omitempty"`
	NewValues    *types.JSONText `db:"new_values" json:"new_values,omitempty"`
	IPAddress    string          `db:"ip_address" json:"ip_address"`
	UserAgent    string          `db:"user_agent" json:"user_agent"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UserEmail    *string         `db:"user_email" json:"user_email,omitempty"`
	UserFullName *string         `db:"user_full_name" json:"user_full_name,omitempty"`
	UserRole     *string         `db:"user_role" json:"user_role,omitempty"`
}

// AuditLogFilter narrows audit log queries for the viewer endpoint.
type AuditLogFilter struct {
	UserID     string
	Action     string
	Resource   string
	ResourceID string
	Search     string
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

// AuditLogFacetCount reports how many entries share a facet value. It powers the
// action/resource filter dropdowns without a separate schema.
type AuditLogFacetCount struct {
	Value string `db:"value" json:"value"`
	Count int    `db:"count" json:"count"`
}

// AuditLogFacets groups the distinct filter values available to the viewer.
type AuditLogFacets struct {
	Actions   []AuditLogFacetCount `json:"actions"`
	Resources []AuditLogFacetCount `json:"resources"`
}
