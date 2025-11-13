package models

import "time"

// MutationType enumerates supported mutation categories.
type MutationType string

const (
	MutationTypeStudentData     MutationType = "STUDENT_DATA"
	MutationTypeGradeCorrection MutationType = "GRADE_CORRECTION"
	MutationTypeAttendanceFix   MutationType = "ATTENDANCE_CORRECTION"
	MutationTypeClassChange     MutationType = "CLASS_CHANGE"
	MutationTypeOther           MutationType = "OTHER"
)

// MutationStatus captures workflow states for change requests.
type MutationStatus string

const (
	MutationStatusPending  MutationStatus = "PENDING"
	MutationStatusApproved MutationStatus = "APPROVED"
	MutationStatusRejected MutationStatus = "REJECTED"
)

// Mutation stores structured data change requests awaiting review.
type Mutation struct {
	ID               string         `db:"id" json:"id"`
	Type             MutationType   `db:"type" json:"type"`
	Entity           string         `db:"entity" json:"entity"`
	EntityID         string         `db:"entity_id" json:"entityId"`
	CurrentSnapshot  []byte         `db:"current_snapshot" json:"currentSnapshot"`
	RequestedChanges []byte         `db:"requested_changes" json:"requestedChanges"`
	Status           MutationStatus `db:"status" json:"status"`
	Reason           string         `db:"reason" json:"reason"`
	RequestedBy      string         `db:"requested_by" json:"requestedBy"`
	ReviewedBy       *string        `db:"reviewed_by" json:"reviewedBy,omitempty"`
	RequestedAt      time.Time      `db:"requested_at" json:"requestedAt"`
	ReviewedAt       *time.Time     `db:"reviewed_at" json:"reviewedAt,omitempty"`
	Note             *string        `db:"note" json:"note,omitempty"`
}

// MutationFilter constrains listing queries.
type MutationFilter struct {
	Status      []MutationStatus
	Entity      string
	Type        MutationType
	EntityID    string
	RequestedBy string
	ReviewerID  string
	Limit       int
	Offset      int
}
