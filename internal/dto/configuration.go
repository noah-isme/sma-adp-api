package dto

// ConfigurationItem represents a configuration entry exposed via API.
type ConfigurationItem struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// UpdateConfigurationRequest describes payload for updating a single configuration.
type UpdateConfigurationRequest struct {
	Key   string `json:"key" validate:"required"`
	Value string `json:"value" validate:"required"`
}

// BulkUpdateConfigurationRequest holds multiple update requests.
type BulkUpdateConfigurationRequest struct {
	Items []UpdateConfigurationRequest `json:"items" validate:"required,min=1,dive"`
}
