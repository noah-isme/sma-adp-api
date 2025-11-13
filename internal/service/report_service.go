package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/jobs"
)

type classAccessChecker interface {
	HasClassAccess(ctx context.Context, teacherID, classID, termID string) (bool, error)
}

type reportJobStore interface {
	Create(ctx context.Context, job *models.ReportJob) error
	GetByID(ctx context.Context, id string) (*models.ReportJob, error)
	Update(ctx context.Context, id string, params repository.UpdateReportJobParams) error
	ListQueued(ctx context.Context, limit int) ([]models.ReportJob, error)
	ListFinishedBefore(ctx context.Context, cutoff time.Time, limit int) ([]models.ReportJob, error)
}

type jobDispatcher interface {
	Enqueue(job jobs.Job) error
}

type exportGenerator interface {
	Generate(ctx context.Context, job *models.ReportJob) (*ExportResult, error)
}

// ReportService orchestrates report job lifecycle management.
type ReportService struct {
	repo        reportJobStore
	assignments classAccessChecker
	queue       jobDispatcher
	exporter    *ExportService
	logger      *zap.Logger
	cfg         ReportServiceConfig
}

// ReportServiceConfig governs queue recovery and cleanup.
type ReportServiceConfig struct {
	ResultTTL       time.Duration
	CleanupInterval time.Duration
	MaxRetries      int
}

// ReportDownload aggregates resolved download data.
type ReportDownload struct {
	File      *os.File
	Filename  string
	Format    models.ReportFormat
	ExpiresAt time.Time
}

// NewReportService constructs the report service.
func NewReportService(repo reportJobStore, assignments classAccessChecker, queue jobDispatcher, exporter *ExportService, logger *zap.Logger, cfg ReportServiceConfig) *ReportService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.ResultTTL <= 0 {
		cfg.ResultTTL = 24 * time.Hour
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	return &ReportService{
		repo:        repo,
		assignments: assignments,
		queue:       queue,
		exporter:    exporter,
		logger:      logger,
		cfg:         cfg,
	}
}

// CreateJob validates request, persists job, and enqueues processing.
func (s *ReportService) CreateJob(ctx context.Context, req dto.ReportRequest, actorID string, role models.UserRole) (*dto.ReportJobResponse, error) {
	if err := s.validateRequest(ctx, req, actorID, role); err != nil {
		return nil, err
	}
	job := &models.ReportJob{
		Type:      req.Type,
		Params:    models.ReportJobParams{TermID: req.TermID, ClassID: req.ClassID, Format: req.Format},
		Status:    models.ReportStatusQueued,
		Progress:  0,
		CreatedBy: actorID,
	}
	if err := s.repo.Create(ctx, job); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create report job")
	}
	if err := s.queue.Enqueue(jobs.Job{ID: job.ID, Type: string(job.Type)}); err != nil {
		status := models.ReportStatusFailed
		msg := "failed to enqueue job"
		now := time.Now().UTC()
		progress := 100
		_ = s.repo.Update(ctx, job.ID, repository.UpdateReportJobParams{
			Status:       &status,
			Progress:     &progress,
			ErrorMessage: &msg,
			FinishedAt:   &now,
		})
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to enqueue report job")
	}
	return &dto.ReportJobResponse{ID: job.ID, Status: job.Status, Progress: job.Progress}, nil
}

// GetStatus exposes job metadata to clients, enforcing ownership for teachers.
func (s *ReportService) GetStatus(ctx context.Context, id string, actorID string, role models.UserRole) (*dto.ReportStatusResponse, error) {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.ErrNotFound
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load report job")
	}
	if role == models.RoleTeacher && job.CreatedBy != actorID {
		return nil, appErrors.ErrForbidden
	}
	resp := &dto.ReportStatusResponse{
		ID:       job.ID,
		Status:   job.Status,
		Progress: job.Progress,
	}
	if job.ResultURL != nil {
		resp.ResultURL = job.ResultURL
	}
	if job.ErrorMessage != nil && *job.ErrorMessage != "" {
		resp.Error = job.ErrorMessage
	}
	return resp, nil
}

// ResolveDownload validates token and opens the stored export file.
func (s *ReportService) ResolveDownload(ctx context.Context, token string) (*ReportDownload, error) {
	jobID, relPath, expiresAt, err := s.exporter.ParseToken(token, false)
	if err != nil {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "invalid or expired download token")
	}
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.ErrNotFound
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load report job")
	}
	if job.ResultURL == nil || !strings.HasSuffix(*job.ResultURL, token) {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "token mismatch")
	}
	if job.Status != models.ReportStatusFinished {
		return nil, appErrors.Clone(appErrors.ErrForbidden, "report not ready")
	}
	file, err := s.exporter.Open(relPath)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to open export file")
	}
	filename := filepath.Base(relPath)
	return &ReportDownload{
		File:      file,
		Filename:  filename,
		Format:    job.Params.Format,
		ExpiresAt: expiresAt,
	}, nil
}

// RecoverPendingJobs replays queued jobs (e.g. after process restart).
func (s *ReportService) RecoverPendingJobs(ctx context.Context) {
	pending, err := s.repo.ListQueued(ctx, 50)
	if err != nil {
		s.logger.Sugar().Warnw("failed to recover queued report jobs", "error", err)
		return
	}
	for _, job := range pending {
		if err := s.queue.Enqueue(jobs.Job{ID: job.ID, Type: string(job.Type)}); err != nil {
			s.logger.Sugar().Warnw("failed to requeue pending job", "job_id", job.ID, "error", err)
		}
	}
}

// StartCleanup boots a goroutine that purges expired exports periodically.
func (s *ReportService) StartCleanup(ctx context.Context) {
	if s.cfg.CleanupInterval <= 0 {
		return
	}
	ticker := time.NewTicker(s.cfg.CleanupInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanupExpired(ctx)
			}
		}
	}()
}

func (s *ReportService) cleanupExpired(ctx context.Context) {
	cutoff := time.Now().Add(-s.cfg.ResultTTL)
	for {
		jobs, err := s.repo.ListFinishedBefore(ctx, cutoff, 100)
		if err != nil {
			s.logger.Sugar().Warnw("cleanup list failed", "error", err)
			return
		}
		if len(jobs) == 0 {
			break
		}
		for _, job := range jobs {
			if job.ResultURL == nil {
				continue
			}
			token := extractToken(*job.ResultURL)
			if token == "" {
				continue
			}
			_, relPath, _, err := s.exporter.ParseToken(token, true)
			if err != nil {
				continue
			}
			if err := s.exporter.Delete(relPath); err != nil {
				s.logger.Sugar().Warnw("cleanup delete failed", "job_id", job.ID, "error", err)
			}
		}
		if len(jobs) < 100 {
			break
		}
	}
	if _, err := s.exporter.Cleanup(s.cfg.ResultTTL); err != nil {
		s.logger.Sugar().Warnw("filesystem cleanup failed", "error", err)
	}
}

func (s *ReportService) validateRequest(ctx context.Context, req dto.ReportRequest, actorID string, role models.UserRole) error {
	if req.TermID == "" {
		return appErrors.Clone(appErrors.ErrValidation, "termId is required")
	}
	if !isValidReportType(req.Type) {
		return appErrors.Clone(appErrors.ErrValidation, "unsupported report type")
	}
	if !isValidFormat(req.Format) {
		return appErrors.Clone(appErrors.ErrValidation, "unsupported report format")
	}
	if role == models.RoleTeacher {
		if req.ClassID == nil || *req.ClassID == "" {
			return appErrors.Clone(appErrors.ErrValidation, "classId is required for teacher reports")
		}
		if s.assignments == nil {
			return appErrors.Wrap(fmt.Errorf("assignment checker missing"), appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "report access validation error")
		}
		hasAccess, err := s.assignments.HasClassAccess(ctx, actorID, *req.ClassID, req.TermID)
		if err != nil {
			return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to validate class access")
		}
		if !hasAccess {
			return appErrors.ErrForbidden
		}
	}
	return nil
}

func isValidReportType(t models.ReportType) bool {
	switch t {
	case models.ReportTypeAttendance, models.ReportTypeGrades, models.ReportTypeBehavior, models.ReportTypeSummary:
		return true
	default:
		return false
	}
}

func isValidFormat(f models.ReportFormat) bool {
	return f == models.ReportFormatCSV || f == models.ReportFormatPDF
}

func extractToken(url string) string {
	if url == "" {
		return ""
	}
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

// ReportWorker bridges queue jobs to ExportService.
type ReportWorker struct {
	repo       reportJobStore
	exporter   exportGenerator
	logger     *zap.Logger
	maxRetries int
}

// NewReportWorker constructs a worker.
func NewReportWorker(repo reportJobStore, exporter exportGenerator, maxRetries int, logger *zap.Logger) *ReportWorker {
	if logger == nil {
		logger = zap.NewNop()
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &ReportWorker{
		repo:       repo,
		exporter:   exporter,
		logger:     logger,
		maxRetries: maxRetries,
	}
}

// Handle processes a queue job.
func (w *ReportWorker) Handle(ctx context.Context, job jobs.Job) error {
	record, err := w.repo.GetByID(ctx, job.ID)
	if err != nil {
		return err
	}
	processing := models.ReportStatusProcessing
	progress := 10
	if err := w.repo.Update(ctx, job.ID, repository.UpdateReportJobParams{
		Status:   &processing,
		Progress: &progress,
	}); err != nil {
		return err
	}
	result, err := w.exporter.Generate(ctx, record)
	if err != nil {
		msg := err.Error()
		if job.Attempt >= w.maxRetries {
			failed := models.ReportStatusFailed
			progress = 100
			now := time.Now().UTC()
			if updateErr := w.repo.Update(ctx, job.ID, repository.UpdateReportJobParams{
				Status:       &failed,
				Progress:     &progress,
				ErrorMessage: &msg,
				FinishedAt:   &now,
			}); updateErr != nil {
				w.logger.Sugar().Warnw("failed to mark job failed", "job_id", job.ID, "error", updateErr)
			}
		} else {
			queued := models.ReportStatusQueued
			reset := 0
			if updateErr := w.repo.Update(ctx, job.ID, repository.UpdateReportJobParams{
				Status:       &queued,
				Progress:     &reset,
				ErrorMessage: &msg,
			}); updateErr != nil {
				w.logger.Sugar().Warnw("failed to mark job queued", "job_id", job.ID, "error", updateErr)
			}
		}
		return err
	}
	finished := models.ReportStatusFinished
	progress = 100
	now := time.Now().UTC()
	url := result.URL
	clear := ""
	if err := w.repo.Update(ctx, job.ID, repository.UpdateReportJobParams{
		Status:       &finished,
		Progress:     &progress,
		ResultURL:    &url,
		ErrorMessage: &clear,
		FinishedAt:   &now,
	}); err != nil {
		w.logger.Sugar().Warnw("failed to mark job finished", "job_id", job.ID, "error", err)
		return err
	}
	return nil
}
