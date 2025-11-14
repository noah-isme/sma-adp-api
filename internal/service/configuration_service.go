package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type configurationRepository interface {
	ListByKeys(ctx context.Context, keys []string) ([]models.Configuration, error)
	Get(ctx context.Context, key string) (*models.Configuration, error)
	Upsert(ctx context.Context, cfg *models.Configuration) error
	BulkUpsert(ctx context.Context, cfgs []models.Configuration) error
}

type configurationTermReader interface {
	FindByID(ctx context.Context, id string) (*models.Term, error)
}

type configurationAuditLogger interface {
	CreateAuditLog(ctx context.Context, log *models.AuditLog) error
}

type allowedConfiguration struct {
	Key          string
	Type         models.ConfigurationType
	Description  string
	RequiresTerm bool
}

var allowedConfigurationKeys = []string{
	"active_term_id",
	"default_dashboard_term_id",
	"default_calendar_term_id",
	"enable_reports_ui",
	"enable_archives_ui",
	"school_display_name",
}

var allowedConfigurations = map[string]allowedConfiguration{
	"active_term_id": {
		Key:          "active_term_id",
		Type:         models.ConfigurationTypeString,
		Description:  "Term ID currently active across the system",
		RequiresTerm: true,
	},
	"default_dashboard_term_id": {
		Key:          "default_dashboard_term_id",
		Type:         models.ConfigurationTypeString,
		Description:  "Default term used for dashboard views",
		RequiresTerm: true,
	},
	"default_calendar_term_id": {
		Key:          "default_calendar_term_id",
		Type:         models.ConfigurationTypeString,
		Description:  "Default term used for calendar views",
		RequiresTerm: true,
	},
	"enable_reports_ui": {
		Key:         "enable_reports_ui",
		Type:        models.ConfigurationTypeBoolean,
		Description: "Toggle to show/hide reports menu in UI",
	},
	"enable_archives_ui": {
		Key:         "enable_archives_ui",
		Type:        models.ConfigurationTypeBoolean,
		Description: "Toggle to show/hide archives menu in UI",
	},
	"school_display_name": {
		Key:         "school_display_name",
		Type:        models.ConfigurationTypeString,
		Description: "Display name for the school shown in headers",
	},
}

var builtinConfigurationDefaults = map[string]string{
	"enable_reports_ui":  "false",
	"enable_archives_ui": "false",
}

// ConfigurationServiceConfig tunes runtime behaviour.
type ConfigurationServiceConfig struct {
	Defaults map[string]string
}

// ConfigurationService orchestrates CRUD workflow for configuration entries.
type ConfigurationService struct {
	repo      configurationRepository
	terms     configurationTermReader
	audit     configurationAuditLogger
	validator *validator.Validate
	logger    *zap.Logger
	defaults  map[string]string
}

// NewConfigurationService constructs a ConfigurationService.
func NewConfigurationService(repo configurationRepository, terms configurationTermReader, audit configurationAuditLogger, validate *validator.Validate, logger *zap.Logger, cfg ConfigurationServiceConfig) *ConfigurationService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	defaults := make(map[string]string, len(builtinConfigurationDefaults))
	for key, value := range builtinConfigurationDefaults {
		defaults[key] = value
	}
	for key, value := range cfg.Defaults {
		if value == "" {
			continue
		}
		defaults[key] = value
	}
	return &ConfigurationService{
		repo:      repo,
		terms:     terms,
		audit:     audit,
		validator: validate,
		logger:    logger,
		defaults:  defaults,
	}
}

// List returns configuration items scoped to allowed keys.
func (s *ConfigurationService) List(ctx context.Context) ([]dto.ConfigurationItem, error) {
	keys := allowedKeys()
	rows, err := s.repo.ListByKeys(ctx, keys)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list configurations")
	}
	existing := make(map[string]models.Configuration, len(rows))
	for _, row := range rows {
		existing[row.Key] = row
	}

	items := make([]dto.ConfigurationItem, 0, len(keys))
	for _, key := range keys {
		meta := allowedConfigurations[key]
		item := dto.ConfigurationItem{
			Key:         key,
			Type:        string(meta.Type),
			Description: meta.Description,
		}
		if row, ok := existing[key]; ok {
			item.Value = row.Value
			if row.Description != nil && *row.Description != "" {
				item.Description = *row.Description
			}
		} else if def, ok := s.defaultValue(key); ok {
			item.Value = def
		}
		items = append(items, item)
	}
	return items, nil
}

// Get retrieves a single configuration.
func (s *ConfigurationService) Get(ctx context.Context, key string) (*dto.ConfigurationItem, error) {
	meta, err := s.requireAllowedKey(key)
	if err != nil {
		return nil, err
	}
	cfg, err := s.repo.Get(ctx, key)
	if err != nil {
		if err == sql.ErrNoRows {
			if def, ok := s.defaultValue(key); ok {
				return &dto.ConfigurationItem{
					Key:         key,
					Value:       def,
					Type:        string(meta.Type),
					Description: meta.Description,
				}, nil
			}
			return nil, appErrors.Clone(appErrors.ErrNotFound, "configuration not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to get configuration")
	}
	description := meta.Description
	if cfg.Description != nil && *cfg.Description != "" {
		description = *cfg.Description
	}
	return &dto.ConfigurationItem{
		Key:         cfg.Key,
		Value:       cfg.Value,
		Type:        string(cfg.Type),
		Description: description,
	}, nil
}

// Update upserts a configuration entry.
func (s *ConfigurationService) Update(ctx context.Context, key string, value string, actor *models.JWTClaims) (*dto.ConfigurationItem, error) {
	meta, err := s.requireAllowedKey(key)
	if err != nil {
		return nil, err
	}
	value, err = s.validateValue(ctx, meta, value)
	if err != nil {
		return nil, err
	}

	prev, err := s.repo.Get(ctx, key)
	if err != nil && err != sql.ErrNoRows {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch configuration")
	}
	if prev != nil && prev.Type != meta.Type {
		return nil, appErrors.Clone(appErrors.ErrValidation, "configuration type mismatch")
	}

	cfg := &models.Configuration{
		Key:         key,
		Value:       value,
		Type:        meta.Type,
		Description: strPtr(meta.Description),
		UpdatedBy:   userIDPtr(actor),
	}
	if err := s.repo.Upsert(ctx, cfg); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update configuration")
	}

	s.emitAudit(ctx, actor, key, prevValue(prev), value)

	return &dto.ConfigurationItem{
		Key:         key,
		Value:       value,
		Type:        string(meta.Type),
		Description: meta.Description,
	}, nil
}

// BulkUpdate applies multiple updates transactionally.
func (s *ConfigurationService) BulkUpdate(ctx context.Context, req dto.BulkUpdateConfigurationRequest, actor *models.JWTClaims) ([]dto.ConfigurationItem, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid bulk payload")
	}
	if actor == nil {
		return nil, appErrors.ErrUnauthorized
	}

	keys := make([]string, 0, len(req.Items))
	for _, item := range req.Items {
		keys = append(keys, item.Key)
	}
	existing, err := s.repo.ListByKeys(ctx, keys)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load existing configurations")
	}
	existingMap := make(map[string]models.Configuration, len(existing))
	for _, cfg := range existing {
		existingMap[cfg.Key] = cfg
	}

	toUpsert := make([]models.Configuration, 0, len(req.Items))
	for _, item := range req.Items {
		meta, err := s.requireAllowedKey(item.Key)
		if err != nil {
			return nil, err
		}
		normalizedValue, err := s.validateValue(ctx, meta, item.Value)
		if err != nil {
			return nil, err
		}
		if prev, ok := existingMap[item.Key]; ok && prev.Type != meta.Type {
			return nil, appErrors.Clone(appErrors.ErrValidation, fmt.Sprintf("configuration type mismatch for %s", item.Key))
		}
		toUpsert = append(toUpsert, models.Configuration{
			Key:         item.Key,
			Value:       normalizedValue,
			Type:        meta.Type,
			Description: strPtr(meta.Description),
			UpdatedBy:   userIDPtr(actor),
		})
	}

	if err := s.repo.BulkUpsert(ctx, toUpsert); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to bulk update configurations")
	}

	result := make([]dto.ConfigurationItem, 0, len(toUpsert))
	for _, cfg := range toUpsert {
		result = append(result, dto.ConfigurationItem{
			Key:         cfg.Key,
			Value:       cfg.Value,
			Type:        string(cfg.Type),
			Description: allowedConfigurations[cfg.Key].Description,
		})
		prev := existingMap[cfg.Key]
		s.emitAudit(ctx, actor, cfg.Key, prevValue(&prev), cfg.Value)
	}
	return result, nil
}

// GetActiveTermID returns the configured active term with fallback.
func (s *ConfigurationService) GetActiveTermID(ctx context.Context) (string, error) {
	return s.getTermValue(ctx, "active_term_id")
}

// GetDefaultDashboardTermID returns the default term for dashboard views.
func (s *ConfigurationService) GetDefaultDashboardTermID(ctx context.Context) (string, error) {
	return s.getTermValue(ctx, "default_dashboard_term_id")
}

// GetDefaultCalendarTermID returns the default term for calendar alias usage.
func (s *ConfigurationService) GetDefaultCalendarTermID(ctx context.Context) (string, error) {
	return s.getTermValue(ctx, "default_calendar_term_id")
}

func (s *ConfigurationService) requireAllowedKey(key string) (allowedConfiguration, error) {
	meta, ok := allowedConfigurations[key]
	if !ok {
		return allowedConfiguration{}, appErrors.Clone(appErrors.ErrValidation, "unsupported configuration key")
	}
	return meta, nil
}

func (s *ConfigurationService) validateValue(ctx context.Context, meta allowedConfiguration, value string) (string, error) {
	switch meta.Type {
	case models.ConfigurationTypeBoolean:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true":
			return "true", nil
		case "false":
			return "false", nil
		default:
			return "", appErrors.Clone(appErrors.ErrValidation, fmt.Sprintf("%s expects boolean value", meta.Key))
		}
	case models.ConfigurationTypeString:
		value = strings.TrimSpace(value)
		if meta.RequiresTerm {
			if value == "" {
				return "", appErrors.Clone(appErrors.ErrValidation, fmt.Sprintf("%s requires termId value", meta.Key))
			}
			if err := s.ensureTermExists(ctx, value); err != nil {
				return "", err
			}
		}
		return value, nil
	default:
		return "", appErrors.Clone(appErrors.ErrValidation, "unsupported configuration type")
	}
}

func (s *ConfigurationService) ensureTermExists(ctx context.Context, termID string) error {
	if s.terms == nil {
		return nil
	}
	_, err := s.terms.FindByID(ctx, termID)
	if err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "term not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify term")
	}
	return nil
}

func (s *ConfigurationService) emitAudit(ctx context.Context, actor *models.JWTClaims, key, oldValue, newValue string) {
	if s.audit == nil {
		return
	}
	oldPayload := map[string]string{"key": key, "value": oldValue}
	newPayload := map[string]string{"key": key, "value": newValue}
	oldBytes, _ := json.Marshal(oldPayload)
	newBytes, _ := json.Marshal(newPayload)
	log := &models.AuditLog{
		UserID:     userIDPtr(actor),
		Action:     models.AuditActionConfigUpdate,
		Resource:   "configuration",
		ResourceID: &key,
		OldValues:  oldBytes,
		NewValues:  newBytes,
		IPAddress:  "system",
		UserAgent:  "configuration-service",
	}
	if err := s.audit.CreateAuditLog(ctx, log); err != nil {
		s.logger.Warn("failed to record configuration audit", zap.Error(err))
	}
}

func allowedKeys() []string {
	keys := make([]string, len(allowedConfigurationKeys))
	copy(keys, allowedConfigurationKeys)
	return keys
}

func prevValue(cfg *models.Configuration) string {
	if cfg == nil {
		return ""
	}
	return cfg.Value
}

func userIDPtr(actor *models.JWTClaims) *string {
	if actor == nil || actor.UserID == "" {
		return nil
	}
	return &actor.UserID
}

func strPtr(value string) *string {
	if value == "" {
		return nil
	}
	result := value
	return &result
}

func (s *ConfigurationService) defaultValue(key string) (string, bool) {
	if s.defaults == nil {
		return "", false
	}
	value, ok := s.defaults[key]
	return value, ok
}

func (s *ConfigurationService) getValueOrDefault(ctx context.Context, key string) (string, error) {
	cfg, err := s.repo.Get(ctx, key)
	if err != nil {
		if err == sql.ErrNoRows {
			if def, ok := s.defaultValue(key); ok {
				return def, nil
			}
			return "", nil
		}
		return "", appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to get configuration")
	}
	return cfg.Value, nil
}

func (s *ConfigurationService) getTermValue(ctx context.Context, key string) (string, error) {
	value, err := s.getValueOrDefault(ctx, key)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", appErrors.Clone(appErrors.ErrNotFound, fmt.Sprintf("%s not configured", key))
	}
	meta, err := s.requireAllowedKey(key)
	if err != nil {
		return "", err
	}
	if meta.RequiresTerm {
		if err := s.ensureTermExists(ctx, value); err != nil {
			return "", err
		}
	}
	return value, nil
}
