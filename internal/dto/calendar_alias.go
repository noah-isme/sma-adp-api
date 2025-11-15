package dto

import "time"

// CalendarAliasRequest captures query parameters for the /calendar alias.
type CalendarAliasRequest struct {
	TermID    string
	ClassID   string
	StartDate *time.Time
	EndDate   *time.Time
}

// CalendarAliasEvent represents the API response item for /calendar.
type CalendarAliasEvent struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Type        string  `json:"type"`
	StartDate   string  `json:"startDate"`
	EndDate     string  `json:"endDate"`
	Description *string `json:"description,omitempty"`
	Audience    string  `json:"audience"`
	ClassID     *string `json:"classId,omitempty"`
}

// CalendarAliasRange describes the summarised date range returned to FE.
type CalendarAliasRange struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// CalendarAliasResponse wraps the curated payload for frontend consumption.
type CalendarAliasResponse struct {
	TermID  *string              `json:"term_id,omitempty"`
	ClassID *string              `json:"class_id,omitempty"`
	Range   CalendarAliasRange   `json:"range"`
	Events  []CalendarAliasEvent `json:"events"`
}
