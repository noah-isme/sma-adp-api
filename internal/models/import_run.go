package models

import "time"

// ImportRun records the idempotency and audit state for a CSV import request.
type ImportRun struct {
	ID             string     `db:"id" json:"id"`
	ImportType     string     `db:"import_type" json:"importType"`
	IdempotencyKey string     `db:"idempotency_key" json:"idempotencyKey"`
	RequestHash    string     `db:"request_hash" json:"requestHash"`
	UserID         *string    `db:"user_id" json:"userId,omitempty"`
	Status         string     `db:"status" json:"status"`
	CreatedCount   int        `db:"created_count" json:"createdCount"`
	FailedCount    int        `db:"failed_count" json:"failedCount"`
	Result         []byte     `db:"result" json:"result,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"createdAt"`
	CompletedAt    *time.Time `db:"completed_at" json:"completedAt,omitempty"`
}
