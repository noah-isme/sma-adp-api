package models

import "time"

// RefreshToken represents a persisted refresh token session.
type RefreshToken struct {
	ID        string     `db:"id" json:"id"`
	UserID    string     `db:"user_id" json:"user_id"`
	Token     string     `db:"token" json:"token"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	Revoked   bool       `db:"revoked" json:"revoked"`
	RevokedAt *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	IPAddress string     `db:"ip_address" json:"ip_address"`
	UserAgent string     `db:"user_agent" json:"user_agent"`
}
