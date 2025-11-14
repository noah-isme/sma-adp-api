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
