package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	internalmiddleware "github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
)

type mockAnalyticsRepo struct {
	attendanceSummaries []models.AnalyticsAttendanceSummary
	attendanceErr       error
	gradeSummaries      []models.AnalyticsGradeSummary
	gradeErr           error
	behaviorSummaries   []models.AnalyticsBehaviorSummary
	behaviorErr        error
}

func (m *mockAnalyticsRepo) AttendanceSummary(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error) {
	if m.attendanceErr != nil {
		return nil, m.attendanceErr
	}
	return m.attendanceSummaries, nil
}

func (m *mockAnalyticsRepo) GradeSummary(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error) {
	if m.gradeErr != nil {
		return nil, m.gradeErr
	}
	return m.gradeSummaries, nil
}

func (m *mockAnalyticsRepo) BehaviorSummary(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error) {
	if m.behaviorErr != nil {
		return nil, m.behaviorErr
	}
	return m.behaviorSummaries, nil
}

func setupAnalyticsRouter(h *AnalyticsHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Test-Role"); role != "" {
			c.Set(internalmiddleware.ContextUserKey, &models.JWTClaims{
				UserID: "user-1",
				Role:   models.UserRole(role),
			})
		}
		c.Next()
	})

	api := r.Group("/api/v1/analytics")
	api.Use(internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleTeacher)))
	{
		api.GET("/attendance", h.Attendance)
		api.GET("/grades", h.Grades)
		api.GET("/behavior", h.Behavior)
		api.GET("/system", h.System)
	}
	return r
}

func TestAnalyticsHandlerAttendanceSuccess(t *testing.T) {
	repo := &mockAnalyticsRepo{
		attendanceSummaries: []models.AnalyticsAttendanceSummary{
			{ClassID: "class-1", PresentCount: 20},
		},
	}
	analyticsSvc := service.NewAnalyticsService(repo, nil, nil, zap.NewNop())
	h := NewAnalyticsHandler(analyticsSvc)

	router := setupAnalyticsRouter(h)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/attendance?term_id=term-1&class_id=class-1", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var envelope responseEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &envelope)
	require.NoError(t, err)
	assert.NotNil(t, envelope.Data)
}

func TestAnalyticsHandlerGradesSuccess(t *testing.T) {
	repo := &mockAnalyticsRepo{
		gradeSummaries: []models.AnalyticsGradeSummary{
			{ClassID: "class-1", SubjectID: "math", AverageScore: 85.5},
		},
	}
	analyticsSvc := service.NewAnalyticsService(repo, nil, nil, zap.NewNop())
	h := NewAnalyticsHandler(analyticsSvc)

	router := setupAnalyticsRouter(h)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/grades?term_id=term-1&class_id=class-1", nil)
	req.Header.Set("X-Test-Role", string(models.RoleTeacher))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnalyticsHandlerBehaviorSuccess(t *testing.T) {
	repo := &mockAnalyticsRepo{
		behaviorSummaries: []models.AnalyticsBehaviorSummary{
			{StudentID: "std-1", TotalPositive: 3, TotalNegative: 0},
		},
	}
	analyticsSvc := service.NewAnalyticsService(repo, nil, nil, zap.NewNop())
	h := NewAnalyticsHandler(analyticsSvc)

	router := setupAnalyticsRouter(h)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/behavior?term_id=term-1&student_id=std-1", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnalyticsHandlerSystemSuccess(t *testing.T) {
	analyticsSvc := service.NewAnalyticsService(&mockAnalyticsRepo{}, nil, nil, zap.NewNop())
	h := NewAnalyticsHandler(analyticsSvc)

	router := setupAnalyticsRouter(h)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/system", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnalyticsHandlerUnauthorized(t *testing.T) {
	analyticsSvc := service.NewAnalyticsService(&mockAnalyticsRepo{}, nil, nil, zap.NewNop())
	h := NewAnalyticsHandler(analyticsSvc)

	router := setupAnalyticsRouter(h)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/attendance?term_id=term-1", nil)
	// No X-Test-Role header set

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAnalyticsHandlerForbidden(t *testing.T) {
	analyticsSvc := service.NewAnalyticsService(&mockAnalyticsRepo{}, nil, nil, zap.NewNop())
	h := NewAnalyticsHandler(analyticsSvc)

	router := setupAnalyticsRouter(h)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/attendance?term_id=term-1", nil)
	req.Header.Set("X-Test-Role", string(models.RoleStudent)) // RoleStudent is not allowed

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAnalyticsHandlerAttendanceInvalidDate(t *testing.T) {
	analyticsSvc := service.NewAnalyticsService(&mockAnalyticsRepo{}, nil, nil, zap.NewNop())
	h := NewAnalyticsHandler(analyticsSvc)

	router := setupAnalyticsRouter(h)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/attendance?date_from=invalid-date", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsHandlerBehaviorInvalidDate(t *testing.T) {
	analyticsSvc := service.NewAnalyticsService(&mockAnalyticsRepo{}, nil, nil, zap.NewNop())
	h := NewAnalyticsHandler(analyticsSvc)

	router := setupAnalyticsRouter(h)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/behavior?date_to=invalid-date", nil)
	req.Header.Set("X-Test-Role", string(models.RoleAdmin))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsHandlerNilService(t *testing.T) {
	h := NewAnalyticsHandler(nil)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/analytics/attendance", nil)
	c.Request = req

	h.Attendance(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = req
	h.Grades(c2)
	assert.Equal(t, http.StatusInternalServerError, w2.Code)

	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = req
	h.Behavior(c3)
	assert.Equal(t, http.StatusInternalServerError, w3.Code)

	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request = req
	h.System(c4)
	assert.Equal(t, http.StatusInternalServerError, w4.Code)
}
