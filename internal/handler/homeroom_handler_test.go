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

type homeroomServiceMock struct {
	listResp   []dto.HomeroomItem
	listErr    error
	getResp    *dto.HomeroomItem
	getErr     error
	setResp    *dto.HomeroomItem
	setErr     error
	lastFilter dto.HomeroomFilter
	listCalled bool
	getCalled  bool
	setCalled  bool
}

func (m *homeroomServiceMock) List(ctx context.Context, filter dto.HomeroomFilter, claims *models.JWTClaims) ([]dto.HomeroomItem, error) {
	m.listCalled = true
	m.lastFilter = filter
	return m.listResp, m.listErr
}

func (m *homeroomServiceMock) Get(ctx context.Context, classID, termID string, claims *models.JWTClaims) (*dto.HomeroomItem, error) {
	m.getCalled = true
	return m.getResp, m.getErr
}

func (m *homeroomServiceMock) Set(ctx context.Context, req dto.SetHomeroomRequest, actor *models.JWTClaims) (*dto.HomeroomItem, error) {
	m.setCalled = true
	return m.setResp, m.setErr
}

func TestHomeroomHandlerList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &homeroomServiceMock{
		listResp: []dto.HomeroomItem{{ClassID: "class-1"}},
	}
	handler := NewHomeroomHandler(mockSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/homerooms?termId=term-1", nil)
	c.Request = req
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.List(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, mockSvc.listCalled)
	assert.Equal(t, "term-1", mockSvc.lastFilter.TermID)
}

func TestHomeroomHandlerSetInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHomeroomHandler(&homeroomServiceMock{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodPost, "/homerooms", bytes.NewBufferString(`{"classId":"abc"`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.Set(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHomeroomHandlerSetServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &homeroomServiceMock{
		setErr: appErrors.ErrForbidden,
	}
	handler := NewHomeroomHandler(mockSvc)

	payload, _ := json.Marshal(dto.SetHomeroomRequest{ClassID: "class-1", TermID: "term-1", TeacherID: "teacher-1"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodPost, "/homerooms", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.Set(c)
	require.Equal(t, http.StatusForbidden, w.Code)
	assert.True(t, mockSvc.setCalled)
}
