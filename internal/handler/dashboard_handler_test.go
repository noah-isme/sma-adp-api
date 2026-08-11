package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type fakeDashboardSrv struct {
	adminResp   *dto.AdminDashboardResponse
	adminErr    error
	adminHit    bool
	teacherResp *dto.TeacherDashboardResponse
	teacherErr  error
	teacherHit  bool
	lastTeacher struct {
		teacherID string
		termID    string
		date      time.Time
	}
}

func (f *fakeDashboardSrv) Admin(context.Context, string) (*dto.AdminDashboardResponse, bool, error) {
	return f.adminResp, f.adminHit, f.adminErr
}

func (f *fakeDashboardSrv) Teacher(_ context.Context, teacherID, termID string, date time.Time) (*dto.TeacherDashboardResponse, bool, error) {
	f.lastTeacher.teacherID = teacherID
	f.lastTeacher.termID = termID
	f.lastTeacher.date = date
	return f.teacherResp, f.teacherHit, f.teacherErr
}

type fakeTeacherResolver struct {
	teacher *models.Teacher
	err     error
}

func (f *fakeTeacherResolver) FindByUserID(_ context.Context, _ string) (*models.Teacher, error) {
	return f.teacher, f.err
}

func TestDashboardHandlerAdminRequiresTerm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDashboardHandler(&fakeDashboardSrv{}, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/dashboard", nil)

	handler.Admin(c)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDashboardHandlerAdminSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDashboardHandler(&fakeDashboardSrv{
		adminResp: &dto.AdminDashboardResponse{TermID: "term-1"},
		adminHit:  true,
	}, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/dashboard?termId=term-1", nil)

	handler.Admin(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	var envelope responseEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &envelope)
	assert.Equal(t, true, envelope.Meta["cache_hit"])
	dataMap, _ := envelope.Data.(map[string]interface{})
	assert.Equal(t, "term-1", dataMap["termId"])
}

func TestDashboardHandlerTeacherInvalidDate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDashboardHandler(&fakeDashboardSrv{}, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/dashboard/academics?termId=term-1&date=99-99-9999", nil)
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "teacher-1"})

	handler.Teacher(c)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDashboardHandlerTeacherSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeDashboardSrv{
		teacherResp: &dto.TeacherDashboardResponse{TeacherID: "tch-1"},
		teacherHit:  false,
	}
	resolver := &fakeTeacherResolver{
		teacher: &models.Teacher{ID: "tch-1"},
	}
	handler := NewDashboardHandler(service, resolver)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/dashboard/academics?termId=term-1", nil)
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "usr-1", TeacherID: "tch-1", Role: models.RoleTeacher})

	handler.Teacher(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "tch-1", service.lastTeacher.teacherID)
	assert.Equal(t, "term-1", service.lastTeacher.termID)
	assert.False(t, service.lastTeacher.date.IsZero())
}

func TestDashboardHandlerTeacherUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDashboardHandler(&fakeDashboardSrv{}, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/dashboard/academics?termId=term-1", nil)
	// No claims set

	handler.Teacher(c)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDashboardHandlerForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDashboardHandler(&fakeDashboardSrv{}, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Test-Role"); role != "" {
			c.Set(middleware.ContextUserKey, &models.JWTClaims{
				UserID: "usr-1",
				Role:   models.UserRole(role),
			})
		}
		c.Next()
	})
	r.GET("/dashboard", middleware.RBAC(string(models.RoleAdmin)), handler.Admin)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard?termId=term-1", nil)
	req.Header.Set("X-Test-Role", string(models.RoleStudent))
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestDashboardHandlerAdminInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDashboardHandler(&fakeDashboardSrv{
		adminErr: appErrors.ErrInternal,
	}, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/dashboard?termId=term-1", nil)

	handler.Admin(c)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

type responseEnvelope struct {
	Data interface{}            `json:"data"`
	Meta map[string]interface{} `json:"meta"`
}

