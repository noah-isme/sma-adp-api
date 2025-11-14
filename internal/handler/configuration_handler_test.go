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
)

type configurationServiceMock struct {
	listResp  []dto.ConfigurationItem
	getResp   *dto.ConfigurationItem
	updateErr error
	bulkErr   error
}

func (m *configurationServiceMock) List(ctx context.Context) ([]dto.ConfigurationItem, error) {
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
