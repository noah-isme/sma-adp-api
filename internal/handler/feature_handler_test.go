package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/pkg/config"
)

func performFeatureRequest(t *testing.T, cfg *config.Config) (*httptest.ResponseRecorder, config.FeatureResponse) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewFeatureHandler(cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodGet, "/features", nil)
	require.NoError(t, err)
	c.Request = req

	handler.List(c)
	require.Equal(t, http.StatusOK, w.Code)

	var envelope struct {
		Data config.FeatureResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	return w, envelope.Data
}

func TestFeatureHandlerReportsDisabledModules(t *testing.T) {
	cfg := &config.Config{Env: config.EnvDevelopment, APIPrefix: "/api/v1"}

	_, payload := performFeatureRequest(t, cfg)

	assert.Equal(t, "/api/v1", payload.APIPrefix)
	assert.Equal(t, config.EnvDevelopment, payload.Env)
	assert.False(t, payload.Features.Reports)
	assert.False(t, payload.Features.Scheduler)
	assert.False(t, payload.Features.AttendanceAlias)
	assert.True(t, payload.Features.Audit)
}

func TestFeatureHandlerReportsEnabledModules(t *testing.T) {
	cfg := &config.Config{Env: config.EnvProduction, APIPrefix: "/api/v1"}
	cfg.Reports.Enabled = true
	cfg.Scheduler.Enabled = true
	cfg.Archives.Enabled = true
	cfg.Aliases.AttendanceEnabled = true

	_, payload := performFeatureRequest(t, cfg)

	assert.True(t, payload.Features.Reports)
	assert.True(t, payload.Features.Scheduler)
	assert.True(t, payload.Features.Archives)
	assert.True(t, payload.Features.Documents)
	assert.True(t, payload.Features.AttendanceAlias)
	assert.True(t, payload.Features.LessonAttendance)
}

// The payload must be a plain flag map: no secrets, connection strings, or term
// ids leak through, since this route is served unauthenticated.
func TestFeatureHandlerLeaksNoConfigurationValues(t *testing.T) {
	cfg := &config.Config{Env: config.EnvProduction, APIPrefix: "/api/v1"}
	cfg.JWT.Secret = "super-secret-token"
	cfg.Database.Password = "db-password"
	cfg.Reports.SignedURLSecret = "reports-secret"
	cfg.Archives.SignedURLSecret = "archives-secret"
	cfg.Configuration.ActiveTermID = "term-secret"

	w, _ := performFeatureRequest(t, cfg)
	body := w.Body.String()

	assert.NotContains(t, body, "super-secret-token")
	assert.NotContains(t, body, "db-password")
	assert.NotContains(t, body, "reports-secret")
	assert.NotContains(t, body, "archives-secret")
	assert.NotContains(t, body, "term-secret")
}
