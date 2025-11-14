package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type configurationRepoStub struct {
	items map[string]models.Configuration
	err   error
}

func (s *configurationRepoStub) ListByKeys(ctx context.Context, keys []string) ([]models.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := []models.Configuration{}
	for _, key := range keys {
		if cfg, ok := s.items[key]; ok {
			result = append(result, cfg)
		}
	}
	return result, nil
}

func (s *configurationRepoStub) Get(ctx context.Context, key string) (*models.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}
	if cfg, ok := s.items[key]; ok {
		return &cfg, nil
	}
	return nil, sql.ErrNoRows
}

func (s *configurationRepoStub) Upsert(ctx context.Context, cfg *models.Configuration) error {
	if s.err != nil {
		return s.err
	}
	if s.items == nil {
		s.items = make(map[string]models.Configuration)
	}
	s.items[cfg.Key] = *cfg
	return nil
}

func (s *configurationRepoStub) BulkUpsert(ctx context.Context, cfgs []models.Configuration) error {
	if s.err != nil {
		return s.err
	}
	if s.items == nil {
		s.items = make(map[string]models.Configuration)
	}
	for _, cfg := range cfgs {
		s.items[cfg.Key] = cfg
	}
	return nil
}

type configurationTermRepoStub struct {
	err error
}

func (t configurationTermRepoStub) FindByID(ctx context.Context, id string) (*models.Term, error) {
	if t.err != nil {
		return nil, t.err
	}
	return &models.Term{ID: id}, nil
}

type auditLoggerStub struct {
	logs []*models.AuditLog
}

func (a *auditLoggerStub) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	a.logs = append(a.logs, log)
	return nil
}

func TestConfigurationServiceUpdateBoolean(t *testing.T) {
	repo := &configurationRepoStub{}
	service := NewConfigurationService(repo, configurationTermRepoStub{}, &auditLoggerStub{}, validator.New(), nil, ConfigurationServiceConfig{})
	item, err := service.Update(context.Background(), "enable_reports_ui", "true", &models.JWTClaims{UserID: "admin"})
	require.NoError(t, err)
	assert.Equal(t, "true", item.Value)
	assert.Equal(t, "BOOLEAN", item.Type)
}

func TestConfigurationServiceUpdateInvalidKey(t *testing.T) {
	service := NewConfigurationService(&configurationRepoStub{}, configurationTermRepoStub{}, &auditLoggerStub{}, validator.New(), nil, ConfigurationServiceConfig{})
	_, err := service.Update(context.Background(), "unknown_key", "abc", &models.JWTClaims{UserID: "admin"})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}

func TestConfigurationServiceUpdateValidatesTerm(t *testing.T) {
	termErr := sql.ErrNoRows
	service := NewConfigurationService(&configurationRepoStub{}, configurationTermRepoStub{err: termErr}, &auditLoggerStub{}, validator.New(), nil, ConfigurationServiceConfig{})
	_, err := service.Update(context.Background(), "active_term_id", "term-x", &models.JWTClaims{UserID: "admin"})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrNotFound.Code, appErrors.FromError(err).Code)
}

func TestConfigurationServiceBulkUpdateRollbackOnValidation(t *testing.T) {
	repo := &configurationRepoStub{}
	service := NewConfigurationService(repo, configurationTermRepoStub{}, &auditLoggerStub{}, validator.New(), nil, ConfigurationServiceConfig{})
	req := dto.BulkUpdateConfigurationRequest{
		Items: []dto.UpdateConfigurationRequest{
			{Key: "enable_reports_ui", Value: "true"},
			{Key: "unknown", Value: "value"},
		},
	}
	_, err := service.BulkUpdate(context.Background(), req, &models.JWTClaims{UserID: "admin"})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
	assert.Len(t, repo.items, 0)
}

func TestConfigurationServiceListFiltersKeys(t *testing.T) {
	repo := &configurationRepoStub{
		items: map[string]models.Configuration{
			"enable_reports_ui": {Key: "enable_reports_ui", Value: "true", Type: models.ConfigurationTypeBoolean},
			"other_key":         {Key: "other_key", Value: "secret", Type: models.ConfigurationTypeString},
		},
	}
	service := NewConfigurationService(repo, configurationTermRepoStub{}, &auditLoggerStub{}, validator.New(), nil, ConfigurationServiceConfig{})
	items, err := service.List(context.Background())
	require.NoError(t, err)
	require.Len(t, items, len(allowedConfigurationKeys))
	found := false
	for _, item := range items {
		if item.Key == "other_key" {
			t.Fatalf("unexpected key returned: %s", item.Key)
		}
		if item.Key == "enable_reports_ui" {
			found = true
			assert.Equal(t, "true", item.Value)
		}
	}
	assert.True(t, found, "expected enable_reports_ui to be present")
}

func TestConfigurationServiceUpdateHandlesRepoError(t *testing.T) {
	repo := &configurationRepoStub{err: errors.New("db down")}
	service := NewConfigurationService(repo, configurationTermRepoStub{}, &auditLoggerStub{}, validator.New(), nil, ConfigurationServiceConfig{})
	_, err := service.Update(context.Background(), "school_display_name", "SMA ADP", &models.JWTClaims{UserID: "admin"})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrInternal.Code, appErrors.FromError(err).Code)
}

func TestConfigurationServiceGetUsesDefaults(t *testing.T) {
	service := NewConfigurationService(
		&configurationRepoStub{},
		configurationTermRepoStub{},
		&auditLoggerStub{},
		validator.New(),
		nil,
		ConfigurationServiceConfig{
			Defaults: map[string]string{"school_display_name": "SMA ADP"},
		},
	)

	item, err := service.Get(context.Background(), "school_display_name")
	require.NoError(t, err)
	assert.Equal(t, "SMA ADP", item.Value)
}

func TestConfigurationServiceGetActiveTermIDFallback(t *testing.T) {
	service := NewConfigurationService(
		&configurationRepoStub{},
		configurationTermRepoStub{},
		&auditLoggerStub{},
		validator.New(),
		nil,
		ConfigurationServiceConfig{Defaults: map[string]string{"active_term_id": "term-default"}},
	)
	value, err := service.GetActiveTermID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "term-default", value)
}
