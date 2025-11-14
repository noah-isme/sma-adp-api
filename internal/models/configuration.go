package models

import "time"

// ConfigurationType defines supported types for configuration values.
type ConfigurationType string

const (
	ConfigurationTypeString  ConfigurationType = "STRING"
	ConfigurationTypeBoolean ConfigurationType = "BOOLEAN"
)

// Configuration represents a persisted configuration entry.
type Configuration struct {
	Key         string            `db:"key" json:"key"`
	Value       string            `db:"value" json:"value"`
	Type        ConfigurationType `db:"type" json:"type"`
	Description *string           `db:"description" json:"description,omitempty"`
	UpdatedBy   *string           `db:"updated_by" json:"updated_by,omitempty"`
	UpdatedAt   time.Time         `db:"updated_at" json:"updated_at"`
}
