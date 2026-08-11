package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type configurationServiceMock struct {
	listResp  []dto.ConfigurationItem
	listErr   error
	getResp   *dto.ConfigurationItem
	updateErr error
	bulkErr   error
}

func (m *configurationServiceMock) List(ctx context.Context) ([]dto.ConfigurationItem, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listResp, nil
}

func (m *configurationServiceMock) Get(ctx context.Context, key string) (*dto.ConfigurationItem, error) {
	return m.getResp, nil
}

func (m *configurationServiceMock) Update(ctx context.Context, key, value string, actor *models.JWTClaims) (*dto.ConfigurationItem, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	return &dto.ConfigurationItem{Key: key, Value: value, Type: "STRING"}, nil
}

func (m *configurationServiceMock) BulkUpdate(ctx context.Context, req dto.BulkUpdateConfigurationRequest, actor *models.JWTClaims) ([]dto.ConfigurationItem, error) {
	if m.bulkErr != nil {
		return nil, m.bulkErr
	}
	return []dto.ConfigurationItem{}, nil
}

func TestConfigurationHandlerUpdateKeyMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewConfigurationHandler(&configurationServiceMock{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body, _ := json.Marshal(dto.UpdateConfigurationRequest{Key: "enable_reports_ui", Value: "true"})
	req, _ := http.NewRequest(http.MethodPut, "/configuration/other_key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "key", Value: "other_key"}}
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.Update(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConfigurationHandlerBulkInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewConfigurationHandler(&configurationServiceMock{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodPut, "/configuration/bulk", bytes.NewReader([]byte(`invalid`)))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.BulkUpdate(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func setupConfigurationRouter(h *ConfigurationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Test-Role"); role != "" {
			c.Set(middleware.ContextUserKey, &models.JWTClaims{
				UserID: "admin-1",
				Role:   models.UserRole(role),
			})
		}
		c.Next()
	})

	api := r.Group("/api/v1/configuration")
	api.Use(middleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)))
	{
		api.GET("", h.List)
		api.GET("/:key", h.Get)
		api.PUT("/:key", h.Update)
		api.POST("/bulk", h.BulkUpdate)
	}
	return r
}

func TestConfigurationHandlerListSuccess(t *testing.T) {
	mockSvc := &configurationServiceMock{
		listResp: []dto.ConfigurationItem{{Key: "site_name", Value: "SMA School"}},
	}
	h := NewConfigurationHandler(mockSvc)
	router := setupConfigurationRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configuration", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestConfigurationHandlerGetSuccess(t *testing.T) {
	mockSvc := &configurationServiceMock{
		getResp: &dto.ConfigurationItem{Key: "site_name", Value: "SMA School"},
	}
	h := NewConfigurationHandler(mockSvc)
	router := setupConfigurationRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configuration/site_name", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestConfigurationHandlerUpdateSuccess(t *testing.T) {
	mockSvc := &configurationServiceMock{}
	h := NewConfigurationHandler(mockSvc)
	router := setupConfigurationRouter(h)

	body, _ := json.Marshal(dto.UpdateConfigurationRequest{Key: "site_name", Value: "New SMA School"})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/configuration/site_name", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestConfigurationHandlerBulkUpdateSuccess(t *testing.T) {
	mockSvc := &configurationServiceMock{}
	h := NewConfigurationHandler(mockSvc)
	router := setupConfigurationRouter(h)

	body, _ := json.Marshal(dto.BulkUpdateConfigurationRequest{
		Items: []dto.UpdateConfigurationRequest{{Key: "k1", Value: "v1"}},
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/configuration/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleSuperAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestConfigurationHandlerUnauthorized(t *testing.T) {
	mockSvc := &configurationServiceMock{}
	h := NewConfigurationHandler(mockSvc)
	router := setupConfigurationRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configuration", nil)
	// No X-Test-Role

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestConfigurationHandlerForbidden(t *testing.T) {
	mockSvc := &configurationServiceMock{}
	h := NewConfigurationHandler(mockSvc)
	router := setupConfigurationRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configuration", nil)
	req.Header.Set("X-Test-Role", string(models.RoleTeacher)) // Not authorized

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestConfigurationHandlerInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &configurationServiceMock{listErr: appErrors.ErrInternal}
	h := NewConfigurationHandler(mockSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/configuration", nil)
	c.Request = req

	h.List(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

