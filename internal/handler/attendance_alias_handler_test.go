package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type attendanceAliasServiceMock struct {
	summaryResp *dto.AttendanceSummaryResponse
}

func (m *attendanceAliasServiceMock) ListDaily(ctx context.Context, req dto.AttendanceDailyRequest, claims *models.JWTClaims) ([]models.DailyAttendanceRecord, *models.Pagination, error) {
	return nil, nil, nil
}

func (m *attendanceAliasServiceMock) Summary(ctx context.Context, req dto.AttendanceSummaryRequest, claims *models.JWTClaims) (*dto.AttendanceSummaryResponse, bool, error) {
	if req.TermID == "" {
		return nil, false, appErrors.Clone(appErrors.ErrValidation, "termId is required")
	}
	return m.summaryResp, false, nil
}

func TestAttendanceAliasHandlerSummaryValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAttendanceAliasHandler(&attendanceAliasServiceMock{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/attendance", nil)
	c.Request = req
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.Summary(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAttendanceAliasHandlerDailyInvalidDate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAttendanceAliasHandler(&attendanceAliasServiceMock{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/attendance/daily?termId=term-1&dateFrom=bad-date", nil)
	c.Request = req
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.Daily(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
