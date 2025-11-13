package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// ReportRepository persists report job metadata.
type ReportRepository struct {
	db *sqlx.DB
}

// NewReportRepository constructs the repository.
func NewReportRepository(db *sqlx.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

// Create inserts a new report job row with generated defaults.
func (r *ReportRepository) Create(ctx context.Context, job *models.ReportJob) error {
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if job.Status == "" {
		job.Status = models.ReportStatusQueued
	}
	const query = `INSERT INTO report_jobs (id, type, params, status, progress, result_url, created_by, created_at, finished_at, error_message)
VALUES (:id, :type, :params, :status, :progress, :result_url, :created_by, :created_at, :finished_at, :error_message)`
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}
	if _, err := r.db.NamedExecContext(ctx, query, job); err != nil {
		return fmt.Errorf("create report job: %w", err)
	}
	return nil
}

// GetByID returns a job row by its identifier.
func (r *ReportRepository) GetByID(ctx context.Context, id string) (*models.ReportJob, error) {
	const query = `SELECT id, type, params, status, progress, result_url, created_by, created_at, finished_at, error_message
FROM report_jobs WHERE id = $1`
	var job models.ReportJob
	if err := r.db.GetContext(ctx, &job, query, id); err != nil {
		return nil, fmt.Errorf("get report job: %w", err)
	}
	return &job, nil
}

// UpdateReportJobParams defines the mutable fields.
type UpdateReportJobParams struct {
	Status       *models.ReportStatus
	Progress     *int
	ResultURL    *string
	ErrorMessage *string
	FinishedAt   *time.Time
}

// Update persists the provided changes for a job row.
func (r *ReportRepository) Update(ctx context.Context, id string, params UpdateReportJobParams) error {
	set := make([]string, 0, 5)
	args := make([]interface{}, 0, 6)
	argPos := 1

	if params.Status != nil {
		set = append(set, fmt.Sprintf("status = $%d", argPos))
		args = append(args, *params.Status)
		argPos++
	}
	if params.Progress != nil {
		set = append(set, fmt.Sprintf("progress = $%d", argPos))
		args = append(args, *params.Progress)
		argPos++
	}
	if params.ResultURL != nil {
		set = append(set, fmt.Sprintf("result_url = $%d", argPos))
		args = append(args, *params.ResultURL)
		argPos++
	}
	if params.ErrorMessage != nil {
		set = append(set, fmt.Sprintf("error_message = $%d", argPos))
		args = append(args, *params.ErrorMessage)
		argPos++
	}
	if params.FinishedAt != nil {
		set = append(set, fmt.Sprintf("finished_at = $%d", argPos))
		args = append(args, *params.FinishedAt)
		argPos++
	}

	if len(set) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE report_jobs SET %s WHERE id = $%d", strings.Join(set, ", "), argPos)
	args = append(args, id)

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update report job: %w", err)
	}
	return nil
}

// ListQueued fetches queued jobs (used for cold start recovery).
func (r *ReportRepository) ListQueued(ctx context.Context, limit int) ([]models.ReportJob, error) {
	if limit <= 0 {
		limit = 20
	}
	const query = `SELECT id, type, params, status, progress, result_url, created_by, created_at, finished_at, error_message
FROM report_jobs WHERE status = 'QUEUED' ORDER BY created_at ASC LIMIT $1`
	var jobs []models.ReportJob
	if err := r.db.SelectContext(ctx, &jobs, query, limit); err != nil {
		return nil, fmt.Errorf("list queued report jobs: %w", err)
	}
	return jobs, nil
}

// ListFinishedBefore retrieves completed jobs prior to cutoff for cleanup.
func (r *ReportRepository) ListFinishedBefore(ctx context.Context, cutoff time.Time, limit int) ([]models.ReportJob, error) {
	if limit <= 0 {
		limit = 50
	}
	const query = `SELECT id, type, params, status, progress, result_url, created_by, created_at, finished_at, error_message
FROM report_jobs WHERE status = 'FINISHED' AND finished_at IS NOT NULL AND finished_at < $1 ORDER BY finished_at ASC LIMIT $2`
	var jobs []models.ReportJob
	if err := r.db.SelectContext(ctx, &jobs, query, cutoff, limit); err != nil {
		return nil, fmt.Errorf("list finished report jobs: %w", err)
	}
	return jobs, nil
}
