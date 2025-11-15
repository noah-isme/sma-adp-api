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
	"github.com/noah-isme/sma-adp-api/internal/models"
)

type scheduleGeneratorMock struct {
	captured dto.GenerateScheduleRequest
}

func (m *scheduleGeneratorMock) Generate(ctx context.Context, req dto.GenerateScheduleRequest) (*dto.GenerateScheduleResponse, error) {
	m.captured = req
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
	payload := []byte(`{"termId":"2025","classId":"10A","timeSlotsPerDay":4,"days":[1,2],"subjectLoads":[{"subjectId":"math","teacherId":"t1","weeklyCount":4}]}`)
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
