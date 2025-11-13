package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
)

type reportServiceMock struct {
	createResp *dto.ReportJobResponse
	createErr  error
	statusResp *dto.ReportStatusResponse
	statusErr  error
	download   *service.ReportDownload
	downloadErr error
}

func (m *reportServiceMock) CreateJob(ctx context.Context, req dto.ReportRequest, actorID string, role models.UserRole) (*dto.ReportJobResponse, error) {
	return m.createResp, m.createErr
}

func (m *reportServiceMock) GetStatus(ctx context.Context, id string, actorID string, role models.UserRole) (*dto.ReportStatusResponse, error) {
	return m.statusResp, m.statusErr
}

func (m *reportServiceMock) ResolveDownload(ctx context.Context, token string) (*service.ReportDownload, error) {
	return m.download, m.downloadErr
}

func newGinContext(method, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c, w
}

func TestReportHandlerGenerateReport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &reportServiceMock{
		createResp: &dto.ReportJobResponse{ID: "job-1", Status: models.ReportStatusQueued, Progress: 0},
	}
	handler := NewReportHandler(mockSvc, nil)

	payload, _ := json.Marshal(dto.ReportRequest{Type: models.ReportTypeGrades, TermID: "term-1", Format: models.ReportFormatCSV})
	c, w := newGinContext(http.MethodPost, "/reports/generate", payload)
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.GenerateReport(c)
	require.Equal(t, http.StatusAccepted, w.Code)
}

func TestReportHandlerReportStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := &reportServiceMock{
		statusResp: &dto.ReportStatusResponse{ID: "job-1", Status: models.ReportStatusFinished, Progress: 100},
	}
	handler := NewReportHandler(mockSvc, nil)

	c, w := newGinContext(http.MethodGet, "/reports/status/job-1", nil)
	c.Params = gin.Params{{Key: "id", Value: "job-1"}}
	c.Set(middleware.ContextUserKey, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	handler.ReportStatus(c)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestReportHandlerDownloadReport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	file, err := os.CreateTemp("", "report*.csv")
	require.NoError(t, err)
	defer os.Remove(file.Name())
	_, _ = file.WriteString("data")
	_, _ = file.Seek(0, 0)

	mockSvc := &reportServiceMock{
		download: &service.ReportDownload{
			File:      file,
			Filename:  "report.csv",
			Format:    models.ReportFormatCSV,
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	handler := NewReportHandler(mockSvc, nil)

	c, w := newGinContext(http.MethodGet, "/export/token", nil)
	c.Params = gin.Params{{Key: "token", Value: "token"}}

	handler.DownloadReport(c)
	require.Equal(t, http.StatusOK, w.Code)
}
