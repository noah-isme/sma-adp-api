package models

import "time"

// PasswordResetToken is the persisted state for an admin password reset.
// The raw token is never stored; TokenHash contains its SHA-256 digest.
type PasswordResetToken struct {
	ID        string     `db:"id" json:"-"`
	UserID    string     `db:"user_id" json:"-"`
	TokenHash string     `db:"token_hash" json:"-"`
	ExpiresAt time.Time  `db:"expires_at" json:"-"`
	UsedAt    *time.Time `db:"used_at" json:"-"`
	CreatedAt time.Time  `db:"created_at" json:"-"`
}
