package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	internalmiddleware "github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

func TestAliasRoutesIntegration(t *testing.T) {
	router := buildAliasRouter()

	t.Run("calendar success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/calendar?term_id=2024_1", nil)
		req.Header.Set("X-Test-Role", string(models.RoleAdmin))
		resp := performRequest(router, req)
		require.Equal(t, http.StatusOK, resp.Code)
		require.Contains(t, resp.Body.String(), `"events"`)
	})

	t.Run("calendar unauthorized", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/calendar?term_id=2024_1", nil)
		resp := performRequest(router, req)
		require.Equal(t, http.StatusUnauthorized, resp.Code)
	})

	t.Run("schedules generator forbidden", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/schedules/generator", bytes.NewBufferString(defaultGeneratorPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Role", string(models.RoleTeacher))
		resp := performRequest(router, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("schedules generator success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/schedules/generator", bytes.NewBufferString(defaultGeneratorPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Role", string(models.RoleAdmin))
		resp := performRequest(router, req)
		require.Equal(t, http.StatusOK, resp.Code)
		require.Contains(t, resp.Body.String(), `"mode":"preview"`)
	})

	t.Run("schedule preferences get success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/schedules/preferences?teacher_id=123", nil)
		req.Header.Set("X-Test-Role", string(models.RoleAdmin))
		resp := performRequest(router, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("schedule preferences get forbidden", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/schedules/preferences?teacher_id=123", nil)
		req.Header.Set("X-Test-Role", string(models.RoleTeacher))
		resp := performRequest(router, req)
		require.Equal(t, http.StatusForbidden, resp.Code)
	})

	t.Run("schedule preferences post success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/schedules/preferences?teacher_id=teacher-1", bytes.NewBufferString(`{"max_load_per_day":4}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Role", string(models.RoleSuperAdmin))
		resp := performRequest(router, req)
		require.Equal(t, http.StatusOK, resp.Code)
	})
}

func buildAliasRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Test-Role"); role != "" {
			c.Set(internalmiddleware.ContextUserKey, &models.JWTClaims{
				UserID: "test-user",
				Role:   models.UserRole(role),
			})
		}
		c.Next()
	})

	calendarHandler := NewCalendarAliasHandler(&calendarAliasServiceIntegrationMock{}, zap.NewNop())
	schedulerHandler := &ScheduleGeneratorHandler{service: &scheduleGeneratorIntegrationMock{}}
	preferenceHandler := NewSchedulePreferenceHandler(&schedulePreferenceIntegrationMock{})

	secured := router.Group("")
	secured.GET("/calendar", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarHandler.List)
	secured.POST("/schedules/generator", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), schedulerHandler.GenerateAlias)
	secured.GET("/schedules/preferences", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), preferenceHandler.Get)
	secured.POST("/schedules/preferences", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), preferenceHandler.Upsert)

	return router
}

func performRequest(router *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

type calendarAliasServiceIntegrationMock struct{}

func (calendarAliasServiceIntegrationMock) List(ctx context.Context, req dto.CalendarAliasRequest, claims *models.JWTClaims) (*dto.CalendarAliasResponse, error) {
	return &dto.CalendarAliasResponse{
		Range: dto.CalendarAliasRange{
			StartDate: "2024-01-01",
			EndDate:   "2024-01-31",
		},
		Events: []dto.CalendarAliasEvent{
			{ID: "evt-1", Title: "Exam", Type: "EXAM", StartDate: "2024-01-10", EndDate: "2024-01-10", Audience: "ALL"},
		},
	}, nil
}

type scheduleGeneratorIntegrationMock struct{}

func (scheduleGeneratorIntegrationMock) Generate(ctx context.Context, req dto.GenerateScheduleRequest) (*dto.GenerateScheduleResponse, error) {
	return &dto.GenerateScheduleResponse{ProposalID: "proposal-1"}, nil
}

func (scheduleGeneratorIntegrationMock) Save(ctx context.Context, req dto.SaveScheduleRequest) (string, error) {
	return "", nil
}

func (scheduleGeneratorIntegrationMock) List(ctx context.Context, query dto.SemesterScheduleQuery) ([]models.SemesterSchedule, error) {
	return nil, nil
}

func (scheduleGeneratorIntegrationMock) GetSlots(ctx context.Context, id string) ([]models.SemesterScheduleSlot, error) {
	return nil, nil
}

func (scheduleGeneratorIntegrationMock) Delete(ctx context.Context, id string) error {
	return nil
}

type schedulePreferenceIntegrationMock struct{}

func (schedulePreferenceIntegrationMock) Get(ctx context.Context, teacherID string) (*models.TeacherPreference, error) {
	if teacherID == "missing" {
		return nil, appErrors.ErrNotFound
	}
	return &models.TeacherPreference{TeacherID: teacherID}, nil
}

func (schedulePreferenceIntegrationMock) Upsert(ctx context.Context, teacherID string, req service.UpsertTeacherPreferenceRequest) (*models.TeacherPreference, error) {
	return &models.TeacherPreference{TeacherID: teacherID, MaxLoadPerDay: req.MaxLoadPerDay}, nil
}

const defaultGeneratorPayload = `{"termId":"2024","classId":"10A","timeSlotsPerDay":4,"days":[1,2],"subjectLoads":[{"subjectId":"math","teacherId":"t1","weeklyCount":4}]}`
