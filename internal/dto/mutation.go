package dto

import (
	"encoding/json"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// CreateMutationRequest payload for requesting structured data changes.
type CreateMutationRequest struct {
	Type             models.MutationType `json:"type"`
	Entity           string              `json:"entity"`
	EntityID         string              `json:"entityId"`
	Reason           string              `json:"reason"`
	RequestedChanges json.RawMessage     `json:"requestedChanges"`
}

// ReviewMutationRequest captures reviewer decision and optional note.
type ReviewMutationRequest struct {
	Status models.MutationStatus `json:"status"`
	Note   string                `json:"note"`
}

// MutationQuery mirrors supported listing filters.
type MutationQuery struct {
	Status []models.MutationStatus
	Entity string
	Type   models.MutationType
}
