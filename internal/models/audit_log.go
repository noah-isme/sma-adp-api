package models

import (
	"time"

	"github.com/jmoiron/sqlx/types"
)

// AuditLogRecord defines the structured model for persisted audit logs.
type AuditLogRecord struct {
	ID          string         `db:"id" json:"id"`
	UserID      *string        `db:"user_id" json:"user_id,omitempty"`
	Action      string         `db:"action" json:"action"`
	Resource    string         `db:"resource" json:"resource"`
	ResourceID  *string        `db:"resource_id" json:"resource_id,omitempty"`
	Timestamp   time.Time      `db:"timestamp" json:"timestamp"`
	DetailsJSON types.JSONText `db:"details_json" json:"details_json,omitempty"`
	CreatedAt   time.Time      `db:"created_at" json:"created_at"`
}
