package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/pkg/export"
	"github.com/noah-isme/sma-adp-api/pkg/storage"
)

type analyticsRepository interface {
	AttendanceSummary(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error)
	GradeSummary(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error)
	BehaviorSummary(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error)
}

type fileStorage interface {
	Save(filename string, data []byte) (string, error)
	Open(filename string) (*os.File, error)
	Delete(filename string) error
	CleanupOlderThan(ttl time.Duration) ([]string, error)
}

// ExportConfig tunes export behaviour.
type ExportConfig struct {
	APIPrefix string
	ResultTTL time.Duration
}

// ExportResult captures successful generation metadata.
type ExportResult struct {
	RelativePath string
	Token        string
	URL          string
	Format       models.ReportFormat
	ExpiresAt    time.Time
}

// ExportService builds report datasets and persists rendered files.
type ExportService struct {
	analytics analyticsRepository
	storage   fileStorage
	csv       csvRenderer
	pdf       pdfRenderer
	signer    *storage.SignedURLSigner
	logger    *zap.Logger
	cfg       ExportConfig
}

type csvRenderer interface {
	Render(data export.Dataset) ([]byte, error)
}

type pdfRenderer interface {
	Render(data export.Dataset, title string) ([]byte, error)
}

// NewExportService constructs an ExportService.
func NewExportService(analytics analyticsRepository, storage fileStorage, signer *storage.SignedURLSigner, cfg ExportConfig, logger *zap.Logger, csv csvRenderer, pdf pdfRenderer) *ExportService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.ResultTTL <= 0 {
		cfg.ResultTTL = 24 * time.Hour
	}
	if csv == nil {
		csv = export.NewCSVExporter()
	}
	if pdf == nil {
		pdf = export.NewPDFExporter()
	}
	return &ExportService{
		analytics: analytics,
		storage:   storage,
		csv:       csv,
		pdf:       pdf,
		signer:    signer,
		logger:    logger,
		cfg:       cfg,
	}
}

// Generate builds dataset according to job definition and stores the rendered export.
func (s *ExportService) Generate(ctx context.Context, job *models.ReportJob) (*ExportResult, error) {
	if job == nil {
		return nil, fmt.Errorf("job nil")
	}
	dataset, title, err := s.buildDataset(ctx, job)
	if err != nil {
		return nil, err
	}

	var payload []byte
	switch job.Params.Format {
	case models.ReportFormatCSV:
		payload, err = s.csv.Render(dataset)
	case models.ReportFormatPDF:
		payload, err = s.pdf.Render(dataset, title)
	default:
		err = fmt.Errorf("unsupported format %s", job.Params.Format)
	}
	if err != nil {
		return nil, err
	}

	filename := s.buildFilename(job)
	relPath, err := s.storage.Save(filename, payload)
	if err != nil {
		return nil, err
	}

	token, expiresAt, err := s.signer.Generate(job.ID, relPath)
	if err != nil {
		return nil, err
	}
	signedURL := strings.TrimRight(s.cfg.APIPrefix, "/")
	if signedURL == "" {
		signedURL = "/api/v1"
	}
	signedURL = fmt.Sprintf("%s/export/%s", signedURL, token)

	return &ExportResult{
		RelativePath: relPath,
		Token:        token,
		URL:          signedURL,
		Format:       job.Params.Format,
		ExpiresAt:    expiresAt,
	}, nil
}

// ParseToken validates download token metadata.
func (s *ExportService) ParseToken(token string, allowExpired bool) (jobID, relPath string, expiresAt time.Time, err error) {
	return s.signer.Parse(token, allowExpired)
}

// Open returns a handle to the stored file.
func (s *ExportService) Open(relPath string) (*os.File, error) {
	return s.storage.Open(relPath)
}

// Delete removes a stored export file.
func (s *ExportService) Delete(relPath string) error {
	return s.storage.Delete(relPath)
}

// Cleanup removes files older than ttl (defaults to configured ResultTTL when ttl <= 0).
func (s *ExportService) Cleanup(ttl time.Duration) ([]string, error) {
	if ttl <= 0 {
		ttl = s.cfg.ResultTTL
	}
	return s.storage.CleanupOlderThan(ttl)
}

func (s *ExportService) buildFilename(job *models.ReportJob) string {
	timestamp := time.Now().UTC().Format("20060102_150405")
	termPart := sanitizeFilename(job.Params.TermID)
	name := fmt.Sprintf("%s_%s_%s.%s", strings.ToLower(string(job.Type)), termPart, timestamp, job.Params.Format)
	return name
}

func sanitizeFilename(raw string) string {
	if raw == "" {
		return "na"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "-", "\\", "-", ":", "-", "..", ".", "__", "_")
	result := replacer.Replace(raw)
	if len(result) > 100 {
		return result[:100]
	}
	return result
}

func (s *ExportService) buildDataset(ctx context.Context, job *models.ReportJob) (export.Dataset, string, error) {
	switch job.Type {
	case models.ReportTypeAttendance:
		return s.buildAttendanceDataset(ctx, job.Params)
	case models.ReportTypeGrades:
		return s.buildGradeDataset(ctx, job.Params)
	case models.ReportTypeBehavior:
		return s.buildBehaviorDataset(ctx, job.Params)
	case models.ReportTypeSummary:
		return s.buildSummaryDataset(ctx, job.Params)
	default:
		return export.Dataset{}, "", fmt.Errorf("unsupported report type %s", job.Type)
	}
}

func (s *ExportService) buildAttendanceDataset(ctx context.Context, params models.ReportJobParams) (export.Dataset, string, error) {
	filter := models.AnalyticsAttendanceFilter{
		TermID:  params.TermID,
		ClassID: deref(params.ClassID),
	}
	rows, err := s.analytics.AttendanceSummary(ctx, filter)
	if err != nil {
		return export.Dataset{}, "", err
	}
	dataRows := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		dataRows = append(dataRows, map[string]string{
			"Term ID":        row.TermID,
			"Class ID":       row.ClassID,
			"Present":        fmt.Sprintf("%d", row.PresentCount),
			"Absent":         fmt.Sprintf("%d", row.AbsentCount),
			"Attendance (%)": fmt.Sprintf("%.2f", row.Percentage),
			"Updated At":     formatReportTime(row.UpdatedAt),
		})
	}
	dataset := export.Dataset{
		Headers: []string{"Term ID", "Class ID", "Present", "Absent", "Attendance (%)", "Updated At"},
		Rows:    dataRows,
	}
	title := fmt.Sprintf("Attendance Report %s", params.TermID)
	return dataset, title, nil
}

func (s *ExportService) buildGradeDataset(ctx context.Context, params models.ReportJobParams) (export.Dataset, string, error) {
	filter := models.AnalyticsGradeFilter{
		TermID:  params.TermID,
		ClassID: deref(params.ClassID),
	}
	summaries, err := s.analytics.GradeSummary(ctx, filter)
	if err != nil {
		return export.Dataset{}, "", err
	}
	dataRows := make([]map[string]string, 0, len(summaries))
	for _, row := range summaries {
		dataRows = append(dataRows, map[string]string{
			"Term ID":       row.TermID,
			"Class ID":      row.ClassID,
			"Subject ID":    row.SubjectID,
			"Average Score": fmt.Sprintf("%.2f", row.AverageScore),
			"Median Score":  fmt.Sprintf("%.2f", row.MedianScore),
			"Updated At":    formatReportTime(row.UpdatedAt),
		})
	}
	dataset := export.Dataset{
		Headers: []string{"Term ID", "Class ID", "Subject ID", "Average Score", "Median Score", "Updated At"},
		Rows:    dataRows,
	}
	title := fmt.Sprintf("Grade Report %s", params.TermID)
	return dataset, title, nil
}

func (s *ExportService) buildBehaviorDataset(ctx context.Context, params models.ReportJobParams) (export.Dataset, string, error) {
	filter := models.AnalyticsBehaviorFilter{
		TermID:   params.TermID,
		ClassID:  deref(params.ClassID),
		DateFrom: nil,
		DateTo:   nil,
	}
	summaries, err := s.analytics.BehaviorSummary(ctx, filter)
	if err != nil {
		return export.Dataset{}, "", err
	}
	dataRows := make([]map[string]string, 0, len(summaries))
	for _, row := range summaries {
		dataRows = append(dataRows, map[string]string{
			"Term ID":         row.TermID,
			"Student ID":      row.StudentID,
			"Positive Points": fmt.Sprintf("%d", row.TotalPositive),
			"Negative Points": fmt.Sprintf("%d", row.TotalNegative),
			"Balance":         fmt.Sprintf("%d", row.Balance),
			"Updated At":      formatReportTime(row.UpdatedAt),
		})
	}
	dataset := export.Dataset{
		Headers: []string{"Term ID", "Student ID", "Positive Points", "Negative Points", "Balance", "Updated At"},
		Rows:    dataRows,
	}
	title := fmt.Sprintf("Behavior Report %s", params.TermID)
	return dataset, title, nil
}

func (s *ExportService) buildSummaryDataset(ctx context.Context, params models.ReportJobParams) (export.Dataset, string, error) {
	attendanceRows, err := s.analytics.AttendanceSummary(ctx, models.AnalyticsAttendanceFilter{
		TermID:  params.TermID,
		ClassID: deref(params.ClassID),
	})
	if err != nil {
		return export.Dataset{}, "", err
	}
	gradeRows, err := s.analytics.GradeSummary(ctx, models.AnalyticsGradeFilter{
		TermID:  params.TermID,
		ClassID: deref(params.ClassID),
	})
	if err != nil {
		return export.Dataset{}, "", err
	}
	behaviorRows, err := s.analytics.BehaviorSummary(ctx, models.AnalyticsBehaviorFilter{
		TermID:  params.TermID,
		ClassID: deref(params.ClassID),
	})
	if err != nil {
		return export.Dataset{}, "", err
	}

	avgAttendance := averageAttendance(attendanceRows)
	bestClass := bestAttendanceClass(attendanceRows)
	avgGrade := averageGrade(gradeRows)
	behaviorBalance := aggregateBehaviorBalance(behaviorRows)

	rows := []map[string]string{
		{"Metric": "Average Attendance", "Term ID": params.TermID, "Value": fmt.Sprintf("%.2f", avgAttendance), "Notes": ""},
		{"Metric": "Best Attendance Class", "Term ID": params.TermID, "Value": bestClass, "Notes": ""},
		{"Metric": "Average Grade", "Term ID": params.TermID, "Value": fmt.Sprintf("%.2f", avgGrade), "Notes": ""},
		{"Metric": "Behavior Balance", "Term ID": params.TermID, "Value": fmt.Sprintf("%d", behaviorBalance), "Notes": ""},
	}

	dataset := export.Dataset{
		Headers: []string{"Metric", "Term ID", "Value", "Notes"},
		Rows:    rows,
	}
	title := fmt.Sprintf("Summary Report %s", params.TermID)
	return dataset, title, nil
}

func deref(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func formatReportTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func averageAttendance(rows []models.AnalyticsAttendanceSummary) float64 {
	if len(rows) == 0 {
		return 0
	}
	var total float64
	for _, row := range rows {
		total += row.Percentage
	}
	return total / float64(len(rows))
}

func bestAttendanceClass(rows []models.AnalyticsAttendanceSummary) string {
	best := ""
	var bestScore float64
	for _, row := range rows {
		if row.Percentage > bestScore {
			bestScore = row.Percentage
			best = row.ClassID
		}
	}
	return best
}

func averageGrade(rows []models.AnalyticsGradeSummary) float64 {
	if len(rows) == 0 {
		return 0
	}
	var total float64
	for _, row := range rows {
		total += row.AverageScore
	}
	return total / float64(len(rows))
}

func aggregateBehaviorBalance(rows []models.AnalyticsBehaviorSummary) int {
	total := 0
	for _, row := range rows {
		total += row.Balance
	}
	return total
}
