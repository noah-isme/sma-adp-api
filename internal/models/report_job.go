package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// ReportType enumerates supported asynchronous report categories.
type ReportType string

const (
	ReportTypeAttendance ReportType = "attendance"
	ReportTypeGrades     ReportType = "grades"
	ReportTypeBehavior   ReportType = "behavior"
	ReportTypeSummary    ReportType = "summary"
)

// ReportFormat enumerates supported export formats.
type ReportFormat string

const (
	ReportFormatCSV ReportFormat = "csv"
	ReportFormatPDF ReportFormat = "pdf"
)

// ReportStatus captures background job lifecycle states.
type ReportStatus string

const (
	ReportStatusQueued     ReportStatus = "QUEUED"
	ReportStatusProcessing ReportStatus = "PROCESSING"
	ReportStatusFinished   ReportStatus = "FINISHED"
	ReportStatusFailed     ReportStatus = "FAILED"
)

// ReportJob persisted background job metadata.
type ReportJob struct {
	ID           string          `db:"id" json:"id"`
	Type         ReportType      `db:"type" json:"type"`
	Params       ReportJobParams `db:"params" json:"params"`
	Status       ReportStatus    `db:"status" json:"status"`
	Progress     int             `db:"progress" json:"progress"`
	ResultURL    *string         `db:"result_url" json:"result_url,omitempty"`
	CreatedBy    string          `db:"created_by" json:"created_by"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	FinishedAt   *time.Time      `db:"finished_at" json:"finished_at,omitempty"`
	ErrorMessage *string         `db:"error_message" json:"error_message,omitempty"`
}

// ReportJobParams stores request-scoped options persisted as JSONB.
type ReportJobParams struct {
	TermID  string            `json:"termId"`
	ClassID *string           `json:"classId,omitempty"`
	Format  ReportFormat      `json:"format"`
	Extras  map[string]string `json:"extras,omitempty"`
}

// Value marshals params to JSON for persistence.
func (p ReportJobParams) Value() (driver.Value, error) {
	if p.Extras == nil {
		p.Extras = map[string]string{}
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal report job params: %w", err)
	}
	return data, nil
}

// Scan unmarshals JSON payloads into the params struct.
func (p *ReportJobParams) Scan(value interface{}) error {
	if value == nil {
		*p = ReportJobParams{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported type %T for ReportJobParams", value)
	}
	if len(data) == 0 {
		*p = ReportJobParams{}
		return nil
	}
	if err := json.Unmarshal(data, p); err != nil {
		return fmt.Errorf("unmarshal report job params: %w", err)
	}
	return nil
}
