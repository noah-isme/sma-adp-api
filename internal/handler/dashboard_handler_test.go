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

func TestDashboardHandlerAdminRequiresTerm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDashboardHandler(&fakeDashboardSrv{})

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
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/dashboard?termId=term-1", nil)

	handler.Admin(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	var envelope responseEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &envelope)
	assert.Equal(t, true, envelope.Meta["cache_hit"])
	assert.Equal(t, "term-1", envelope.Data["termId"])
}

func TestDashboardHandlerTeacherInvalidDate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDashboardHandler(&fakeDashboardSrv{})

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
		teacherResp: &dto.TeacherDashboardResponse{TeacherID: "teacher-1"},
		teacherHit:  false,
	}
	handler := NewDashboardHandler(service)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/dashboard/academics?termId=term-1", nil)
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "teacher-1"})

	handler.Teacher(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "teacher-1", service.lastTeacher.teacherID)
	assert.Equal(t, "term-1", service.lastTeacher.termID)
	assert.False(t, service.lastTeacher.date.IsZero())
}

type responseEnvelope struct {
	Data map[string]interface{} `json:"data"`
	Meta map[string]interface{} `json:"meta"`
}
