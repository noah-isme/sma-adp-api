package models

import "time"

// AuditAction constants represent actions to be logged.
const (
	AuditActionLogin          = "LOGIN"
	AuditActionLogout         = "LOGOUT"
	AuditActionUserCreate     = "USER_CREATE"
	AuditActionUserUpdate     = "USER_UPDATE"
	AuditActionUserDelete     = "USER_DELETE"
	AuditActionPasswordChange = "PASSWORD_CHANGE"
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
