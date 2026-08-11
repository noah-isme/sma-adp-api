package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	internalmiddleware "github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type scheduleGeneratorMock struct {
	captured dto.GenerateScheduleRequest
	genErr   error
}

func (m *scheduleGeneratorMock) Generate(ctx context.Context, req dto.GenerateScheduleRequest) (*dto.GenerateScheduleResponse, error) {
	m.captured = req
	if m.genErr != nil {
		return nil, m.genErr
	}
	return &dto.GenerateScheduleResponse{ProposalID: "proposal-1"}, nil
}

func (m *scheduleGeneratorMock) Save(ctx context.Context, req dto.SaveScheduleRequest) (string, error) {
	return "", nil
}

func (m *scheduleGeneratorMock) List(ctx context.Context, query dto.SemesterScheduleQuery) ([]models.SemesterSchedule, error) {
	return nil, nil
}

func (m *scheduleGeneratorMock) GetSlots(ctx context.Context, id string) ([]models.SemesterScheduleSlot, error) {
	return nil, nil
}

func (m *scheduleGeneratorMock) Delete(ctx context.Context, id string) error {
	return nil
}

func TestScheduleGeneratorAliasSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &scheduleGeneratorMock{}
	handler := &ScheduleGeneratorHandler{service: mockSvc}
	payload := validGeneratorPayload()
	req, _ := http.NewRequest(http.MethodPost, "/schedules/generator", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.GenerateAlias(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "2025", mockSvc.captured.TermID)
	require.Equal(t, "10A", mockSvc.captured.ClassID)
}

func TestScheduleGeneratorAliasMultiClassSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &scheduleGeneratorMock{}
	handler := &ScheduleGeneratorHandler{service: mockSvc}
	payload := []byte(`{"termId":"2025","classIds":["10A","10B"],"timeSlotsPerDay":4,"days":[1,2],"subjectLoads":[{"subjectId":"math","teacherId":"t1","weeklyCount":4}]}`)
	req, _ := http.NewRequest(http.MethodPost, "/schedules/generator", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.GenerateAlias(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "2025", mockSvc.captured.TermID)
	require.Equal(t, []string{"10A", "10B"}, mockSvc.captured.ClassIDs)
}

func TestScheduleGeneratorAliasValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &ScheduleGeneratorHandler{service: &scheduleGeneratorMock{}}
	req, _ := http.NewRequest(http.MethodPost, "/schedules/generator", bytes.NewReader([]byte(`{"termId":`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.GenerateAlias(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleGeneratorAliasUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &ScheduleGeneratorHandler{service: &scheduleGeneratorMock{}}
	router := gin.New()
	router.POST("/schedules/generator", internalmiddleware.RBAC(string(models.RoleAdmin)), handler.GenerateAlias)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/schedules/generator", bytes.NewReader(validGeneratorPayload()))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestScheduleGeneratorAliasForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &ScheduleGeneratorHandler{service: &scheduleGeneratorMock{}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(internalmiddleware.ContextUserKey, &models.JWTClaims{UserID: "teacher-1", Role: models.RoleTeacher})
		c.Next()
	})
	router.POST("/schedules/generator", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), handler.GenerateAlias)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/schedules/generator", bytes.NewReader(validGeneratorPayload()))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestScheduleGeneratorSaveSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &scheduleGeneratorMock{}
	handler := &ScheduleGeneratorHandler{service: mockSvc}

	payload := []byte(`{"proposalId":"prop-1","classId":"10A","termId":"2025"}`)
	req, _ := http.NewRequest(http.MethodPost, "/schedule/save", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Save(c)
	require.Equal(t, http.StatusCreated, w.Code)
}

func TestScheduleGeneratorListSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &scheduleGeneratorMock{}
	handler := &ScheduleGeneratorHandler{service: mockSvc}

	req, _ := http.NewRequest(http.MethodGet, "/semester-schedule?termId=2025&classId=10A", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.List(c)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestScheduleGeneratorSlotsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &scheduleGeneratorMock{}
	handler := &ScheduleGeneratorHandler{service: mockSvc}

	req, _ := http.NewRequest(http.MethodGet, "/semester-schedule/sched-1/slots", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "sched-1"}}
	c.Request = req

	handler.Slots(c)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestScheduleGeneratorDeleteSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &scheduleGeneratorMock{}
	handler := &ScheduleGeneratorHandler{service: mockSvc}
	router := gin.New()
	router.DELETE("/semester-schedule/:id", handler.Delete)

	req, _ := http.NewRequest(http.MethodDelete, "/semester-schedule/sched-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestScheduleGeneratorInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &scheduleGeneratorMock{genErr: appErrors.ErrInternal}
	handler := &ScheduleGeneratorHandler{service: mockSvc}

	req, _ := http.NewRequest(http.MethodPost, "/schedules/generator", bytes.NewReader(validGeneratorPayload()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.GenerateAlias(c)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func validGeneratorPayload() []byte {
	return []byte(`{"termId":"2025","classId":"10A","timeSlotsPerDay":4,"days":[1,2],"subjectLoads":[{"subjectId":"math","teacherId":"t1","weeklyCount":4}]}`)
}

