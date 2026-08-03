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
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type attendanceServiceMock struct {
	listReq       service.SubjectAttendanceListRequest
	listCalled    bool
	listResp      []models.SubjectAttendanceRecord
	listErr       error
	getID         string
	getResp       *models.SubjectAttendanceRecord
	getErr        error
	deletedID     string
	deleteErr     error
	summaryReq    service.SubjectAttendanceListRequest
	summaryResp   *models.SubjectAttendanceSummary
	summaryErr    error
	summaryCalled bool
}

func (m *attendanceServiceMock) MarkDaily(ctx context.Context, req service.MarkDailyAttendanceRequest) (*models.DailyAttendance, error) {
	return &models.DailyAttendance{EnrollmentID: req.EnrollmentID}, nil
}

func (m *attendanceServiceMock) BulkMarkDaily(ctx context.Context, req service.BulkMarkDailyAttendanceRequest) (*service.BulkAttendanceResult, error) {
	return &service.BulkAttendanceResult{}, nil
}

func (m *attendanceServiceMock) MarkSubject(ctx context.Context, req service.MarkSubjectAttendanceRequest) (*models.SubjectAttendance, error) {
	return &models.SubjectAttendance{EnrollmentID: req.EnrollmentID, ScheduleID: req.ScheduleID}, nil
}

func (m *attendanceServiceMock) BulkMarkSubject(ctx context.Context, req service.BulkMarkSubjectAttendanceRequest) (*service.BulkAttendanceResult, error) {
	return &service.BulkAttendanceResult{}, nil
}

func (m *attendanceServiceMock) ListSubject(ctx context.Context, req service.SubjectAttendanceListRequest) ([]models.SubjectAttendanceRecord, *models.Pagination, error) {
	m.listCalled = true
	m.listReq = req
	if m.listErr != nil {
		return nil, nil, m.listErr
	}
	return m.listResp, &models.Pagination{Page: req.Page, PageSize: req.PageSize, TotalCount: len(m.listResp)}, nil
}

func (m *attendanceServiceMock) GetSubject(ctx context.Context, id string) (*models.SubjectAttendanceRecord, error) {
	m.getID = id
	return m.getResp, m.getErr
}

func (m *attendanceServiceMock) DeleteSubject(ctx context.Context, id string) error {
	m.deletedID = id
	return m.deleteErr
}

func (m *attendanceServiceMock) SubjectSummary(ctx context.Context, req service.SubjectAttendanceListRequest) (*models.SubjectAttendanceSummary, error) {
	m.summaryCalled = true
	m.summaryReq = req
	return m.summaryResp, m.summaryErr
}

func newAttendanceContext(t *testing.T, method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(method, target, nil)
	require.NoError(t, err)
	c.Request = req
	return c, w
}

// The lesson attendance screen sends camelCase; the service layer speaks
// snake_case. Both spellings must resolve to the same filter.
func TestAttendanceHandlerListSubjectAcceptsCamelCase(t *testing.T) {
	mockSvc := &attendanceServiceMock{}
	handler := NewAttendanceHandler(mockSvc)

	c, w := newAttendanceContext(t, http.MethodGet,
		"/attendance/subject?classId=class-1&subjectId=subject-2&termId=term-3&studentId=student-4&dateFrom=2024-05-01&dateTo=2024-05-31&status=a&page=2&limit=10&sortBy=student_name&sortOrder=ASC")
	handler.ListSubject(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, mockSvc.listCalled)
	assert.Equal(t, "class-1", mockSvc.listReq.ClassID)
	assert.Equal(t, "subject-2", mockSvc.listReq.SubjectID)
	assert.Equal(t, "term-3", mockSvc.listReq.TermID)
	assert.Equal(t, "student-4", mockSvc.listReq.StudentID)
	require.NotNil(t, mockSvc.listReq.Status)
	assert.Equal(t, "a", *mockSvc.listReq.Status)
	assert.Equal(t, 2, mockSvc.listReq.Page)
	assert.Equal(t, 10, mockSvc.listReq.PageSize)
	assert.Equal(t, "student_name", mockSvc.listReq.SortBy)
	assert.Equal(t, "asc", mockSvc.listReq.SortOrder)
	require.NotNil(t, mockSvc.listReq.DateFrom)
	require.NotNil(t, mockSvc.listReq.DateTo)
}

func TestAttendanceHandlerListSubjectAcceptsSnakeCase(t *testing.T) {
	mockSvc := &attendanceServiceMock{}
	handler := NewAttendanceHandler(mockSvc)

	c, w := newAttendanceContext(t, http.MethodGet,
		"/attendance/subject?class_id=class-9&subject_id=subject-9&date_from=2024-01-01")
	handler.ListSubject(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "class-9", mockSvc.listReq.ClassID)
	assert.Equal(t, "subject-9", mockSvc.listReq.SubjectID)
	require.NotNil(t, mockSvc.listReq.DateFrom)
}

func TestAttendanceHandlerListSubjectByScheduleID(t *testing.T) {
	mockSvc := &attendanceServiceMock{
		listResp: []models.SubjectAttendanceRecord{
			{SubjectAttendance: models.SubjectAttendance{ID: "sa-1", ScheduleID: "sched-1"}},
		},
	}
	handler := NewAttendanceHandler(mockSvc)

	c, w := newAttendanceContext(t, http.MethodGet, "/attendance/subject?scheduleId=sched-1&date=2024-05-02")
	handler.ListSubject(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "sched-1", mockSvc.listReq.ScheduleID)
	require.NotNil(t, mockSvc.listReq.Date)
	assert.Equal(t, time.Date(2024, 5, 2, 0, 0, 0, 0, time.UTC), *mockSvc.listReq.Date)
}

func TestAttendanceHandlerListSubjectRejectsBadDate(t *testing.T) {
	handler := NewAttendanceHandler(&attendanceServiceMock{})

	c, w := newAttendanceContext(t, http.MethodGet, "/attendance/subject?dateFrom=05-2024")
	handler.ListSubject(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAttendanceHandlerListSubjectPropagatesError(t *testing.T) {
	handler := NewAttendanceHandler(&attendanceServiceMock{listErr: appErrors.ErrForbidden})

	c, w := newAttendanceContext(t, http.MethodGet, "/attendance/subject")
	handler.ListSubject(c)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestAttendanceHandlerSubjectSummary(t *testing.T) {
	mockSvc := &attendanceServiceMock{
		summaryResp: &models.SubjectAttendanceSummary{Present: 8, Absent: 2, Total: 10, Percent: 80},
	}
	handler := NewAttendanceHandler(mockSvc)

	c, w := newAttendanceContext(t, http.MethodGet, "/attendance/subject/summary?classId=class-1&subjectId=subject-1")
	handler.SubjectSummary(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, mockSvc.summaryCalled)
	assert.Equal(t, "class-1", mockSvc.summaryReq.ClassID)

	var envelope struct {
		Data models.SubjectAttendanceSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	assert.Equal(t, 10, envelope.Data.Total)
	assert.InDelta(t, 80.0, envelope.Data.Percent, 0.001)
}

func TestAttendanceHandlerGetSubject(t *testing.T) {
	mockSvc := &attendanceServiceMock{
		getResp: &models.SubjectAttendanceRecord{
			SubjectAttendance: models.SubjectAttendance{ID: "sa-1", Status: models.AttendanceStatusPresent},
		},
	}
	handler := NewAttendanceHandler(mockSvc)

	c, w := newAttendanceContext(t, http.MethodGet, "/attendance/subject/sa-1")
	c.Params = gin.Params{{Key: "id", Value: "sa-1"}}
	handler.GetSubject(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "sa-1", mockSvc.getID)
}

func TestAttendanceHandlerGetSubjectNotFound(t *testing.T) {
	handler := NewAttendanceHandler(&attendanceServiceMock{getErr: appErrors.ErrNotFound})

	c, w := newAttendanceContext(t, http.MethodGet, "/attendance/subject/missing")
	c.Params = gin.Params{{Key: "id", Value: "missing"}}
	handler.GetSubject(c)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// Delete answers 204 with an empty body, and gin only flushes a bare status when
// the request runs through the engine, so these two go through a real router.
func newAttendanceDeleteRouter(handler *AttendanceHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/attendance/subject/:id", handler.DeleteSubject)
	return router
}

func TestAttendanceHandlerDeleteSubject(t *testing.T) {
	mockSvc := &attendanceServiceMock{}
	router := newAttendanceDeleteRouter(NewAttendanceHandler(mockSvc))

	req, err := http.NewRequest(http.MethodDelete, "/attendance/subject/sa-1", nil)
	require.NoError(t, err)
	w := performRequest(router, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
	assert.Equal(t, "sa-1", mockSvc.deletedID)
}

func TestAttendanceHandlerDeleteSubjectNotFound(t *testing.T) {
	router := newAttendanceDeleteRouter(NewAttendanceHandler(&attendanceServiceMock{deleteErr: appErrors.ErrNotFound}))

	req, err := http.NewRequest(http.MethodDelete, "/attendance/subject/missing", nil)
	require.NoError(t, err)
	w := performRequest(router, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}
