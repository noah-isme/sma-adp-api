package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
)

func setupSettingsTestRouter(h *ConfigurationHandler) *gin.Engine {
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

	api := r.Group("/api/v1/settings")
	api.Use(middleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)))
	{
		api.GET("", h.List)
		api.GET("/:key", h.Get)
		api.PUT("/:key", h.Update)
		api.POST("/bulk", h.BulkUpdate)
	}
	return r
}

func TestSettingsHandlerListSuccess(t *testing.T) {
	mockSvc := &configurationServiceMock{
		listResp: []dto.ConfigurationItem{{Key: "school_name", Value: "SMA 1"}},
	}
	h := NewConfigurationHandler(mockSvc)
	router := setupSettingsTestRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "school_name")
}

func TestSettingsHandlerUnauthorized(t *testing.T) {
	mockSvc := &configurationServiceMock{}
	h := NewConfigurationHandler(mockSvc)
	router := setupSettingsTestRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/settings", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSettingsHandlerForbidden(t *testing.T) {
	mockSvc := &configurationServiceMock{}
	h := NewConfigurationHandler(mockSvc)
	router := setupSettingsTestRouter(h)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	req.Header.Set("X-Test-Role", string(models.RoleStudent))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandlerValidationError(t *testing.T) {
	mockSvc := &configurationServiceMock{}
	h := NewConfigurationHandler(mockSvc)
	router := setupSettingsTestRouter(h)

	body, _ := json.Marshal(dto.UpdateConfigurationRequest{Key: "key_a", Value: "val_a"})
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/settings/key_b", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
