package dto

import "github.com/noah-isme/sma-adp-api/internal/models"

// CreateArchiveRequest contains metadata submitted alongside a file upload.
type CreateArchiveRequest struct {
	Title        string              `form:"title" json:"title"`
	Category     string              `form:"category" json:"category"`
	Scope        models.ArchiveScope `form:"scope" json:"scope"`
	RefTermID    *string             `form:"refTermId" json:"refTermId"`
	RefClassID   *string             `form:"refClassId" json:"refClassId"`
	RefStudentID *string             `form:"refStudentId" json:"refStudentId"`
}

// ArchiveFilter DTO used for handlers to capture query parameters.
type ArchiveFilter struct {
	Scope    models.ArchiveScope
	Category string
	TermID   string
	ClassID  string
}

// ArchiveDownloadResponse enriches metadata with a signed download URL.
type ArchiveDownloadResponse struct {
	models.ArchiveItem
	DownloadURL string `json:"downloadUrl"`
}
