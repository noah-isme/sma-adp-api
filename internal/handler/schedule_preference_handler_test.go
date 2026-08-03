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
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type schedulePreferenceServiceMock struct {
	upsertCalled bool
	listCalled   bool
	listFilter   models.TeacherPreferenceFilter
	listResp     []models.TeacherPreference
	resp         *models.TeacherPreference
	err          error
}

func (m *schedulePreferenceServiceMock) Get(ctx context.Context, teacherID string) (*models.TeacherPreference, error) {
	return m.resp, m.err
}

func (m *schedulePreferenceServiceMock) Upsert(ctx context.Context, teacherID string, req service.UpsertTeacherPreferenceRequest) (*models.TeacherPreference, error) {
	m.upsertCalled = true
	return m.resp, m.err
}

func (m *schedulePreferenceServiceMock) ListAll(ctx context.Context, filter models.TeacherPreferenceFilter) ([]models.TeacherPreference, *models.Pagination, error) {
	m.listCalled = true
	m.listFilter = filter
	if m.err != nil {
		return nil, nil, m.err
	}
	return m.listResp, &models.Pagination{Page: filter.Page, PageSize: filter.PageSize, TotalCount: len(m.listResp)}, nil
}

// Omitting teacher_id now means "list every teacher's preferences" rather than
// erroring, which is what the schedule generator needs to seed constraints.
func TestSchedulePreferenceAliasHandlerListsWhenTeacherIDOmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &schedulePreferenceServiceMock{
		listResp: []models.TeacherPreference{{TeacherID: "t1"}, {TeacherID: "t2"}},
	}
	handler := NewSchedulePreferenceHandler(mockSvc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/schedules/preferences", nil)
	c.Request = req

	handler.Get(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, mockSvc.listCalled)
}

func TestSchedulePreferenceAliasHandlerListHonoursPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &schedulePreferenceServiceMock{}
	handler := NewSchedulePreferenceHandler(mockSvc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/schedules/preferences?page=3&limit=25&sortBy=max_load_per_day&sortOrder=DESC", nil)
	c.Request = req

	handler.Get(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 3, mockSvc.listFilter.Page)
	require.Equal(t, 25, mockSvc.listFilter.PageSize)
	require.Equal(t, "max_load_per_day", mockSvc.listFilter.SortBy)
	require.Equal(t, "desc", mockSvc.listFilter.SortOrder)
}

func TestSchedulePreferenceAliasHandlerInvalidTeacherID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSchedulePreferenceHandler(&schedulePreferenceServiceMock{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/schedules/preferences?teacher_id=@@@", nil)
	c.Request = req

	handler.Get(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSchedulePreferenceAliasHandlerGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSchedulePreferenceHandler(&schedulePreferenceServiceMock{resp: &models.TeacherPreference{TeacherID: "1"}})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/schedules/preferences?teacher_id=1", nil)
	c.Request = req

	handler.Get(c)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestSchedulePreferenceAliasHandlerUpsert(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &schedulePreferenceServiceMock{resp: &models.TeacherPreference{TeacherID: "1"}}
	handler := NewSchedulePreferenceHandler(mockSvc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := []byte(`{"preferredDays":[1,2]}`)
	req, _ := http.NewRequest(http.MethodPost, "/schedules/preferences?teacher_id=1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.Upsert(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, mockSvc.upsertCalled)
}

func TestSchedulePreferenceAliasHandlerGetNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &schedulePreferenceServiceMock{err: appErrors.ErrNotFound}
	handler := NewSchedulePreferenceHandler(mockSvc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/schedules/preferences?teacher_id=123", nil)
	c.Request = req

	handler.Get(c)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSchedulePreferenceAliasHandlerUpsertNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &schedulePreferenceServiceMock{err: appErrors.ErrNotFound}
	handler := NewSchedulePreferenceHandler(mockSvc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := []byte(`{"preferredDays":[1]}`)
	req, _ := http.NewRequest(http.MethodPost, "/schedules/preferences?teacher_id=123", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.Upsert(c)

	require.Equal(t, http.StatusNotFound, w.Code)
}
