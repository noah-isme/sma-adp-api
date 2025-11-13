package dto

import "github.com/noah-isme/sma-adp-api/internal/models"

// ReportRequest captures POST /reports/generate payload.
type ReportRequest struct {
	Type    models.ReportType   `json:"type"`
	TermID  string              `json:"termId"`
	ClassID *string             `json:"classId,omitempty"`
	Format  models.ReportFormat `json:"format"`
}

// ReportJobResponse is returned after enqueueing a report.
type ReportJobResponse struct {
	ID       string              `json:"id"`
	Status   models.ReportStatus `json:"status"`
	Progress int                 `json:"progress"`
}

// ReportStatusResponse exposes job progress metadata.
type ReportStatusResponse struct {
	ID        string              `json:"id"`
	Status    models.ReportStatus `json:"status"`
	Progress  int                 `json:"progress"`
	ResultURL *string             `json:"resultUrl,omitempty"`
	Error     *string             `json:"error,omitempty"`
}
