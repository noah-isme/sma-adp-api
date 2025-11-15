package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type calendarAliasServiceMock struct {
	captured dto.CalendarAliasRequest
}

func (m *calendarAliasServiceMock) List(ctx context.Context, req dto.CalendarAliasRequest, claims *models.JWTClaims) ([]dto.CalendarAliasEvent, error) {
	if claims == nil {
		return nil, appErrors.ErrUnauthorized
	}
	m.captured = req
	return []dto.CalendarAliasEvent{
		{ID: "event-1", Title: "Test", Type: "EXAM", StartDate: "2025-01-01", EndDate: "2025-01-01", Audience: "ALL"},
	}, nil
}

func TestCalendarAliasHandlerRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewCalendarAliasHandler(&calendarAliasServiceMock{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/calendar", nil)
	c.Request = req

	handler.List(c)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCalendarAliasHandlerParsesSnakeCaseParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &calendarAliasServiceMock{}
	handler := NewCalendarAliasHandler(mockSvc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/calendar?term_id=2025&class_id=10a&start_date=2025-01-10&end_date=2025-01-11", nil)
	c.Request = req
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "2025", mockSvc.captured.TermID)
	require.Equal(t, "10a", mockSvc.captured.ClassID)
	require.NotNil(t, mockSvc.captured.StartDate)
	require.NotNil(t, mockSvc.captured.EndDate)
	require.Equal(t, time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC), mockSvc.captured.StartDate.UTC())
	require.Equal(t, time.Date(2025, 1, 11, 0, 0, 0, 0, time.UTC), mockSvc.captured.EndDate.UTC())
}

func TestCalendarAliasHandlerRejectsInvalidDate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewCalendarAliasHandler(&calendarAliasServiceMock{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/calendar?start_date=bad", nil)
	c.Request = req
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.List(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
