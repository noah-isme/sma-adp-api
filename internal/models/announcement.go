package models

import "time"

// AnnouncementAudience defines who can see an announcement.
type AnnouncementAudience string

const (
	AnnouncementAudienceAll   AnnouncementAudience = "ALL"
	AnnouncementAudienceGuru  AnnouncementAudience = "GURU"
	AnnouncementAudienceSiswa AnnouncementAudience = "SISWA"
	AnnouncementAudienceClass AnnouncementAudience = "CLASS"
)

// AnnouncementPriority defines ordering for announcements.
type AnnouncementPriority string

const (
	AnnouncementPriorityLow    AnnouncementPriority = "LOW"
	AnnouncementPriorityNormal AnnouncementPriority = "NORMAL"
	AnnouncementPriorityHigh   AnnouncementPriority = "HIGH"
)

// Announcement represents a persisted announcement row.
type Announcement struct {
	ID            string               `db:"id" json:"id"`
	Title         string               `db:"title" json:"title"`
	Content       string               `db:"content" json:"content"`
	Audience      AnnouncementAudience `db:"audience" json:"audience"`
	TargetClassID *string              `db:"target_class_id" json:"target_class_id,omitempty"`
	Priority      AnnouncementPriority `db:"priority" json:"priority"`
	IsPinned      bool                 `db:"is_pinned" json:"is_pinned"`
	PublishedAt   time.Time            `db:"published_at" json:"published_at"`
	ExpiresAt     *time.Time           `db:"expires_at" json:"expires_at,omitempty"`
	CreatedBy     string               `db:"created_by" json:"created_by"`
	CreatedAt     time.Time            `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time            `db:"updated_at" json:"updated_at"`
}

// AnnouncementFilter allows listing announcements.
type AnnouncementFilter struct {
	AudienceRoles []UserRole
	ClassIDs      []string
	IncludePinned bool
	Page          int
	PageSize      int
}
