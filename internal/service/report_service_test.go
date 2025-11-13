package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
	"github.com/noah-isme/sma-adp-api/pkg/jobs"
	"github.com/google/uuid"
)

type reportRepoStub struct {
	jobs map[string]*models.ReportJob
}

func newReportRepoStub() *reportRepoStub {
	return &reportRepoStub{jobs: map[string]*models.ReportJob{}}
}

func (r *reportRepoStub) Create(ctx context.Context, job *models.ReportJob) error {
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	r.jobs[job.ID] = job
	return nil
}

func (r *reportRepoStub) GetByID(ctx context.Context, id string) (*models.ReportJob, error) {
	job, ok := r.jobs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return job, nil
}

func (r *reportRepoStub) Update(ctx context.Context, id string, params repository.UpdateReportJobParams) error {
	job, ok := r.jobs[id]
	if !ok {
		return errors.New("not found")
	}
	if params.Status != nil {
		job.Status = *params.Status
	}
	if params.Progress != nil {
		job.Progress = *params.Progress
	}
	if params.ResultURL != nil {
		job.ResultURL = params.ResultURL
	}
	if params.ErrorMessage != nil {
		job.ErrorMessage = params.ErrorMessage
	}
	if params.FinishedAt != nil {
		job.FinishedAt = params.FinishedAt
	}
	return nil
}

func (r *reportRepoStub) ListQueued(ctx context.Context, limit int) ([]models.ReportJob, error) {
	var queued []models.ReportJob
	for _, job := range r.jobs {
		if job.Status == models.ReportStatusQueued {
			queued = append(queued, *job)
		}
	}
	return queued, nil
}

func (r *reportRepoStub) ListFinishedBefore(ctx context.Context, cutoff time.Time, limit int) ([]models.ReportJob, error) {
	return nil, nil
}

type queueStub struct {
	jobs []jobs.Job
	err  error
}

func (q *queueStub) Enqueue(job jobs.Job) error {
	if q.err != nil {
		return q.err
	}
	q.jobs = append(q.jobs, job)
	return nil
}

type assignmentStub struct {
	allow bool
	err   error
}

func (a assignmentStub) HasClassAccess(ctx context.Context, teacherID, classID, termID string) (bool, error) {
	if a.err != nil {
		return false, a.err
	}
	return a.allow, nil
}

func newReportServiceForTest(t *testing.T) (*ReportService, *reportRepoStub, *queueStub, *ExportService) {
	t.Helper()
	repo := newReportRepoStub()
	queue := &queueStub{}
	exportSvc, _ := newExportServiceForTest(t)
	service := NewReportService(repo, assignmentStub{allow: true}, queue, exportSvc, zap.NewNop(), ReportServiceConfig{
		ResultTTL:      time.Hour,
		CleanupInterval: time.Hour,
		MaxRetries:     3,
	})
	return service, repo, queue, exportSvc
}

func TestReportServiceCreateJob(t *testing.T) {
	svc, repo, queue, _ := newReportServiceForTest(t)
	resp, err := svc.CreateJob(context.Background(), dto.ReportRequest{
		Type:   models.ReportTypeGrades,
		TermID: "term-1",
		Format: models.ReportFormatCSV,
	}, "admin", models.RoleAdmin)
	require.NoError(t, err)
	require.NotEmpty(t, resp.ID)
	require.Len(t, queue.jobs, 1)
	assert.Equal(t, models.ReportStatusQueued, resp.Status)
	assert.Contains(t, repo.jobs, resp.ID)
}

func TestReportServiceCreateJobTeacherValidation(t *testing.T) {
	svc, _, _, _ := newReportServiceForTest(t)
	_, err := svc.CreateJob(context.Background(), dto.ReportRequest{
		Type:   models.ReportTypeGrades,
		TermID: "term-1",
		Format: models.ReportFormatCSV,
	}, "teacher-1", models.RoleTeacher)
	require.Error(t, err)
}

func TestReportServiceGetStatus(t *testing.T) {
	svc, repo, _, _ := newReportServiceForTest(t)
	job := &models.ReportJob{
		ID:        "job-1",
		Type:      models.ReportTypeAttendance,
		Params:    models.ReportJobParams{TermID: "term-1", Format: models.ReportFormatCSV},
		Status:    models.ReportStatusFinished,
		Progress:  100,
		CreatedBy: "admin",
	}
	repo.jobs[job.ID] = job
	resp, err := svc.GetStatus(context.Background(), job.ID, "admin", models.RoleAdmin)
	require.NoError(t, err)
	assert.Equal(t, job.Status, resp.Status)
	assert.Equal(t, job.Progress, resp.Progress)
}

func TestReportServiceResolveDownload(t *testing.T) {
	svc, repo, _, exportSvc := newReportServiceForTest(t)
	job := &models.ReportJob{
		ID:        "job-download",
		Type:      models.ReportTypeAttendance,
		Params:    models.ReportJobParams{TermID: "term-1", Format: models.ReportFormatCSV},
		Status:    models.ReportStatusFinished,
		Progress:  100,
		CreatedBy: "admin",
	}
	repo.jobs[job.ID] = job
	result, err := exportSvc.Generate(context.Background(), job)
	require.NoError(t, err)
	job.ResultURL = &result.URL
	now := time.Now()
	job.FinishedAt = &now

	download, err := svc.ResolveDownload(context.Background(), result.Token)
	require.NoError(t, err)
	assert.Equal(t, filepath.Base(result.RelativePath), download.Filename)
	download.File.Close()
}

type exportStub struct {
	result *ExportResult
	err    error
}

func (e exportStub) Generate(ctx context.Context, job *models.ReportJob) (*ExportResult, error) {
	if e.err != nil {
		return nil, e.err
	}
	return e.result, nil
}

func TestReportWorkerHandleSuccess(t *testing.T) {
	repo := &reportRepoStub{
		jobs: map[string]*models.ReportJob{
			"job-1": {
				ID:        "job-1",
				Type:      models.ReportTypeGrades,
				Params:    models.ReportJobParams{TermID: "term-1", Format: models.ReportFormatCSV},
				Status:    models.ReportStatusQueued,
				CreatedBy: "admin",
			},
		},
	}
	exporter := exportStub{result: &ExportResult{URL: "/api/v1/export/token"}}
	worker := NewReportWorker(repo, exporter, 3, zap.NewNop())

	err := worker.Handle(context.Background(), jobs.Job{ID: "job-1"})
	require.NoError(t, err)
	require.Equal(t, models.ReportStatusFinished, repo.jobs["job-1"].Status)
	require.Equal(t, 100, repo.jobs["job-1"].Progress)
}

func TestReportWorkerHandleFailureRetries(t *testing.T) {
	repo := &reportRepoStub{
		jobs: map[string]*models.ReportJob{
			"job-1": {
				ID:        "job-1",
				Type:      models.ReportTypeGrades,
				Params:    models.ReportJobParams{TermID: "term-1", Format: models.ReportFormatCSV},
				Status:    models.ReportStatusQueued,
				CreatedBy: "admin",
			},
		},
	}
	exporter := exportStub{err: errors.New("boom")}
	worker := NewReportWorker(repo, exporter, 2, zap.NewNop())

	err := worker.Handle(context.Background(), jobs.Job{ID: "job-1", Attempt: 2})
	require.Error(t, err)
	require.Equal(t, models.ReportStatusFailed, repo.jobs["job-1"].Status)
}
