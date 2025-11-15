package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
)

type schedulePreferenceServiceMock struct {
	upsertCalled bool
}

func (m *schedulePreferenceServiceMock) Get(ctx context.Context, teacherID string) (*models.TeacherPreference, error) {
	return &models.TeacherPreference{TeacherID: teacherID}, nil
}

func (m *schedulePreferenceServiceMock) Upsert(ctx context.Context, teacherID string, req service.UpsertTeacherPreferenceRequest) (*models.TeacherPreference, error) {
	m.upsertCalled = true
	return &models.TeacherPreference{TeacherID: teacherID}, nil
}

func TestSchedulePreferenceAliasHandlerRequiresTeacherID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSchedulePreferenceHandler(&schedulePreferenceServiceMock{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/schedules/preferences", nil)
	c.Request = req

	handler.Get(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSchedulePreferenceAliasHandlerGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSchedulePreferenceHandler(&schedulePreferenceServiceMock{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/schedules/preferences?teacher_id=teacher-1", nil)
	c.Request = req

	handler.Get(c)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestSchedulePreferenceAliasHandlerUpsert(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &schedulePreferenceServiceMock{}
	handler := NewSchedulePreferenceHandler(mockSvc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := []byte(`{"preferredDays":[1,2]}`)
	req, _ := http.NewRequest(http.MethodPost, "/schedules/preferences?teacher_id=teacher-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.Upsert(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, mockSvc.upsertCalled)
}
