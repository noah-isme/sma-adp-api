package models

import "time"

// ArchiveScope constrains document visibility.
type ArchiveScope string

const (
	ArchiveScopeGlobal  ArchiveScope = "GLOBAL"
	ArchiveScopeTerm    ArchiveScope = "TERM"
	ArchiveScopeClass   ArchiveScope = "CLASS"
	ArchiveScopeStudent ArchiveScope = "STUDENT"
)

// ArchiveItem represents one archived document metadata row.
type ArchiveItem struct {
	ID           string       `db:"id" json:"id"`
	Title        string       `db:"title" json:"title"`
	Category     string       `db:"category" json:"category"`
	Scope        ArchiveScope `db:"scope" json:"scope"`
	RefTermID    *string      `db:"ref_term_id" json:"refTermId,omitempty"`
	RefClassID   *string      `db:"ref_class_id" json:"refClassId,omitempty"`
	RefStudentID *string      `db:"ref_student_id" json:"refStudentId,omitempty"`
	FilePath     string       `db:"file_path" json:"filePath"`
	MimeType     string       `db:"mime_type" json:"mimeType"`
	SizeBytes    int64        `db:"size_bytes" json:"sizeBytes"`
	UploadedBy   string       `db:"uploaded_by" json:"uploadedBy"`
	UploadedAt   time.Time    `db:"uploaded_at" json:"uploadedAt"`
	DeletedAt    *time.Time   `db:"deleted_at" json:"deletedAt,omitempty"`
}

// ArchiveFilter narrows listing queries by metadata fields.
type ArchiveFilter struct {
	Scope          ArchiveScope
	Category       string
	TermID         string
	ClassID        string
	IncludeDeleted bool
	Limit          int
	Offset         int
}
