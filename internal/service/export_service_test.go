package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/pkg/export"
	"github.com/noah-isme/sma-adp-api/pkg/storage"
)

type analyticsStub struct{}

func (analyticsStub) AttendanceSummary(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error) {
	return []models.AnalyticsAttendanceSummary{
		{TermID: filter.TermID, ClassID: "class-1", PresentCount: 20, AbsentCount: 5, Percentage: 80.0, UpdatedAt: ptrTime(time.Now())},
	}, nil
}

func (analyticsStub) GradeSummary(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error) {
	return []models.AnalyticsGradeSummary{
		{TermID: filter.TermID, ClassID: "class-1", SubjectID: "math", AverageScore: 85.5, MedianScore: 84.0, UpdatedAt: ptrTime(time.Now())},
	}, nil
}

func (analyticsStub) BehaviorSummary(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error) {
	return []models.AnalyticsBehaviorSummary{
		{TermID: filter.TermID, StudentID: "student-1", TotalPositive: 5, TotalNegative: 1, Balance: 4, UpdatedAt: ptrTime(time.Now())},
	}, nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func newExportServiceForTest(t *testing.T) (*ExportService, *storage.LocalStorage) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewLocalStorage(dir)
	require.NoError(t, err)
	signer := storage.NewSignedURLSigner("secret", time.Hour)
	cfg := ExportConfig{APIPrefix: "/api/v1", ResultTTL: time.Hour}
	svc := NewExportService(analyticsStub{}, store, signer, cfg, zap.NewNop(), export.NewCSVExporter(), export.NewPDFExporter())
	return svc, store
}

func TestExportServiceGenerateCSV(t *testing.T) {
	svc, store := newExportServiceForTest(t)
	job := &models.ReportJob{
		ID:        "job-1",
		Type:      models.ReportTypeAttendance,
		Params:    models.ReportJobParams{TermID: "term-1", Format: models.ReportFormatCSV},
		CreatedBy: "admin",
	}
	result, err := svc.Generate(context.Background(), job)
	require.NoError(t, err)
	require.NotEmpty(t, result.RelativePath)
	require.Contains(t, result.URL, "/export/")

	path := store.Path(result.RelativePath)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}

func TestExportServiceGeneratePDF(t *testing.T) {
	svc, store := newExportServiceForTest(t)
	job := &models.ReportJob{
		ID:        "job-2",
		Type:      models.ReportTypeSummary,
		Params:    models.ReportJobParams{TermID: "term-1", Format: models.ReportFormatPDF},
		CreatedBy: "admin",
	}
	result, err := svc.Generate(context.Background(), job)
	require.NoError(t, err)
	require.Equal(t, models.ReportFormatPDF, result.Format)

	path := filepath.Clean(store.Path(result.RelativePath))
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}
