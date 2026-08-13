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

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
)

type drilldownHandlerRepoMock struct {
	student *models.AnalyticsStudentAnalytics
}

func (m *drilldownHandlerRepoMock) AttendanceSummary(context.Context, models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error) {
	return nil, nil
}
func (m *drilldownHandlerRepoMock) GradeSummary(context.Context, models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error) {
	return nil, nil
}
func (m *drilldownHandlerRepoMock) BehaviorSummary(context.Context, models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error) {
	return nil, nil
}
func (m *drilldownHandlerRepoMock) ClassAnalytics(context.Context, string, string) (*models.AnalyticsClassAnalytics, error) {
	return &models.AnalyticsClassAnalytics{ClassID: "class-1", TermID: "term-1"}, nil
}
func (m *drilldownHandlerRepoMock) StudentAnalytics(context.Context, string, string) (*models.AnalyticsStudentAnalytics, error) {
	return m.student, nil
}
func (m *drilldownHandlerRepoMock) SubjectAnalytics(context.Context, string, string, string) (*models.AnalyticsSubjectAnalytics, error) {
	return &models.AnalyticsSubjectAnalytics{SubjectID: "subject-1", TermID: "term-1"}, nil
}
func (m *drilldownHandlerRepoMock) Leaderboard(context.Context, string, models.AnalyticsLeaderboardFilter) ([]models.AnalyticsLeaderboardEntry, error) {
	return []models.AnalyticsLeaderboardEntry{{StudentID: "student-1", Score: 90}}, nil
}

func newDrilldownHandlerTestRouter(h *AnalyticsHandler, claims *models.JWTClaims) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextUserKey, claims)
		c.Next()
	})
	r.GET("/analytics/class/:class_id", h.Class)
	r.GET("/analytics/student/:student_id", h.Student)
	r.GET("/analytics/subject/:subject_id", h.Subject)
	r.GET("/analytics/leaderboard/gpa", h.LeaderboardGPA)
	return r
}

func TestAnalyticsHandlerStudentEnforcesSelfScope(t *testing.T) {
	repo := &drilldownHandlerRepoMock{student: &models.AnalyticsStudentAnalytics{StudentID: "student-1"}}
	h := NewAnalyticsHandler(service.NewAnalyticsService(repo, nil, nil, zap.NewNop()))
	router := newDrilldownHandlerTestRouter(h, &models.JWTClaims{Role: models.RoleStudent, StudentID: "student-1"})

	req := httptest.NewRequest(http.MethodGet, "/analytics/student/student-2?term_id=term-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAnalyticsHandlerLeaderboardDefaultAndInvalidLimit(t *testing.T) {
	repo := &drilldownHandlerRepoMock{}
	h := NewAnalyticsHandler(service.NewAnalyticsService(repo, nil, nil, zap.NewNop()))
	router := newDrilldownHandlerTestRouter(h, &models.JWTClaims{Role: models.RoleAdmin})

	req := httptest.NewRequest(http.MethodGet, "/analytics/leaderboard/gpa?term_id=term-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	badReq := httptest.NewRequest(http.MethodGet, "/analytics/leaderboard/gpa?term_id=term-1&limit=101", nil)
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, badReq)
	assert.Equal(t, http.StatusBadRequest, badRec.Code)
}

func TestAnalyticsHandlerClassUsesSnakeCaseEnvelope(t *testing.T) {
	repo := &drilldownHandlerRepoMock{}
	h := NewAnalyticsHandler(service.NewAnalyticsService(repo, nil, nil, zap.NewNop()))
	router := newDrilldownHandlerTestRouter(h, &models.JWTClaims{Role: models.RoleAdmin})
	req := httptest.NewRequest(http.MethodGet, "/analytics/class/class-1?term_id=term-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	data, ok := payload["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "class-1", data["class_id"])
	_, hasCamelCase := data["classId"]
	assert.False(t, hasCamelCase)
}
