package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// subjectAttendanceRepoStub records the filter it was handed so tests can assert
// on normalisation instead of reaching for a database.
type subjectAttendanceRepoStub struct {
	listFilter    models.SubjectAttendanceFilter
	listResp      []models.SubjectAttendanceRecord
	listTotal     int
	listErr       error
	findResp      *models.SubjectAttendanceRecord
	findErr       error
	deletedID     string
	deleteErr     error
	summaryFilter models.SubjectAttendanceFilter
	summaryResp   *models.SubjectAttendanceSummary
	summaryErr    error
}

func (s *subjectAttendanceRepoStub) List(ctx context.Context, filter models.SubjectAttendanceFilter) ([]models.SubjectAttendanceRecord, int, error) {
	s.listFilter = filter
	return s.listResp, s.listTotal, s.listErr
}

func (s *subjectAttendanceRepoStub) Upsert(ctx context.Context, record *models.SubjectAttendance) (*models.SubjectAttendance, error) {
	return record, nil
}

func (s *subjectAttendanceRepoStub) BulkInsert(ctx context.Context, records []models.SubjectAttendance, atomic bool) ([]models.SubjectAttendance, error) {
	return nil, nil
}

func (s *subjectAttendanceRepoStub) SessionReport(ctx context.Context, scheduleID string, date time.Time) ([]models.SubjectAttendanceReportRow, error) {
	return nil, nil
}

func (s *subjectAttendanceRepoStub) FindByID(ctx context.Context, id string) (*models.SubjectAttendanceRecord, error) {
	return s.findResp, s.findErr
}

func (s *subjectAttendanceRepoStub) Delete(ctx context.Context, id string) error {
	s.deletedID = id
	return s.deleteErr
}

func (s *subjectAttendanceRepoStub) Summary(ctx context.Context, filter models.SubjectAttendanceFilter) (*models.SubjectAttendanceSummary, error) {
	s.summaryFilter = filter
	return s.summaryResp, s.summaryErr
}

func newLessonAttendanceService(repo *subjectAttendanceRepoStub) *AttendanceService {
	return NewAttendanceService(nil, repo, nil, nil)
}

func TestListSubjectPassesLessonFilters(t *testing.T) {
	repo := &subjectAttendanceRepoStub{listTotal: 7}
	svc := newLessonAttendanceService(repo)

	from := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 5, 31, 0, 0, 0, 0, time.UTC)
	status := "a"

	_, pagination, err := svc.ListSubject(context.Background(), SubjectAttendanceListRequest{
		ClassID:   "class-1",
		SubjectID: "subject-2",
		TermID:    "term-3",
		StudentID: "student-4",
		DateFrom:  &from,
		DateTo:    &to,
		Status:    &status,
		Page:      2,
		PageSize:  25,
	})
	require.NoError(t, err)

	assert.Equal(t, "class-1", repo.listFilter.ClassID)
	assert.Equal(t, "subject-2", repo.listFilter.SubjectID)
	assert.Equal(t, "term-3", repo.listFilter.TermID)
	assert.Equal(t, "student-4", repo.listFilter.StudentID)
	require.NotNil(t, repo.listFilter.Status)
	// Lower-case input from the UI must be normalised to the stored form.
	assert.Equal(t, models.AttendanceStatusAbsent, *repo.listFilter.Status)
	assert.Equal(t, 2, pagination.Page)
	assert.Equal(t, 25, pagination.PageSize)
	assert.Equal(t, 7, pagination.TotalCount)
}

func TestListSubjectAppliesPaginationDefaults(t *testing.T) {
	repo := &subjectAttendanceRepoStub{}
	svc := newLessonAttendanceService(repo)

	_, pagination, err := svc.ListSubject(context.Background(), SubjectAttendanceListRequest{})
	require.NoError(t, err)

	assert.Equal(t, 1, repo.listFilter.Page)
	assert.Equal(t, 50, repo.listFilter.PageSize)
	assert.Equal(t, 1, pagination.Page)
	assert.Equal(t, 50, pagination.PageSize)
}

func TestListSubjectClampsPageSize(t *testing.T) {
	repo := &subjectAttendanceRepoStub{}
	svc := newLessonAttendanceService(repo)

	_, pagination, err := svc.ListSubject(context.Background(), SubjectAttendanceListRequest{PageSize: 100000})
	require.NoError(t, err)

	assert.Equal(t, 200, repo.listFilter.PageSize)
	assert.Equal(t, 200, pagination.PageSize)
}

func TestListSubjectRejectsInvertedDateRange(t *testing.T) {
	svc := newLessonAttendanceService(&subjectAttendanceRepoStub{})
	from := time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)
	to := from.Add(-24 * time.Hour)

	_, _, err := svc.ListSubject(context.Background(), SubjectAttendanceListRequest{DateFrom: &from, DateTo: &to})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}

func TestListSubjectRejectsInvalidStatus(t *testing.T) {
	svc := newLessonAttendanceService(&subjectAttendanceRepoStub{})
	status := "Z"

	_, _, err := svc.ListSubject(context.Background(), SubjectAttendanceListRequest{Status: &status})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}

// An empty status string is "no filter", not an invalid status.
func TestListSubjectTreatsBlankStatusAsUnset(t *testing.T) {
	repo := &subjectAttendanceRepoStub{}
	svc := newLessonAttendanceService(repo)
	status := ""

	_, _, err := svc.ListSubject(context.Background(), SubjectAttendanceListRequest{Status: &status})
	require.NoError(t, err)
	assert.Nil(t, repo.listFilter.Status)
}

func TestGetSubjectRequiresID(t *testing.T) {
	svc := newLessonAttendanceService(&subjectAttendanceRepoStub{})

	_, err := svc.GetSubject(context.Background(), "  ")
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}

func TestGetSubjectMapsNoRowsToNotFound(t *testing.T) {
	svc := newLessonAttendanceService(&subjectAttendanceRepoStub{findErr: sql.ErrNoRows})

	_, err := svc.GetSubject(context.Background(), "sa-1")
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrNotFound.Code, appErrors.FromError(err).Code)
}

func TestGetSubjectSuccess(t *testing.T) {
	want := &models.SubjectAttendanceRecord{
		SubjectAttendance: models.SubjectAttendance{ID: "sa-1", Status: models.AttendanceStatusPresent},
	}
	svc := newLessonAttendanceService(&subjectAttendanceRepoStub{findResp: want})

	got, err := svc.GetSubject(context.Background(), "sa-1")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestDeleteSubjectRequiresID(t *testing.T) {
	svc := newLessonAttendanceService(&subjectAttendanceRepoStub{})

	err := svc.DeleteSubject(context.Background(), "")
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}

// Deleting a row that is already gone must be a 404, not a silent success.
func TestDeleteSubjectMapsNoRowsToNotFound(t *testing.T) {
	svc := newLessonAttendanceService(&subjectAttendanceRepoStub{deleteErr: sql.ErrNoRows})

	err := svc.DeleteSubject(context.Background(), "missing")
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrNotFound.Code, appErrors.FromError(err).Code)
}

func TestDeleteSubjectSuccess(t *testing.T) {
	repo := &subjectAttendanceRepoStub{}
	svc := newLessonAttendanceService(repo)

	require.NoError(t, svc.DeleteSubject(context.Background(), "sa-9"))
	assert.Equal(t, "sa-9", repo.deletedID)
}

func TestSubjectSummaryReusesListFilters(t *testing.T) {
	repo := &subjectAttendanceRepoStub{
		summaryResp: &models.SubjectAttendanceSummary{Present: 9, Absent: 1, Total: 10, Percent: 90},
	}
	svc := newLessonAttendanceService(repo)

	summary, err := svc.SubjectSummary(context.Background(), SubjectAttendanceListRequest{
		ClassID:   "class-1",
		SubjectID: "subject-1",
	})
	require.NoError(t, err)

	assert.Equal(t, "class-1", repo.summaryFilter.ClassID)
	assert.Equal(t, "subject-1", repo.summaryFilter.SubjectID)
	assert.Equal(t, 10, summary.Total)
	assert.InDelta(t, 90.0, summary.Percent, 0.001)
}

func TestSubjectSummaryRejectsInvertedDateRange(t *testing.T) {
	svc := newLessonAttendanceService(&subjectAttendanceRepoStub{})
	from := time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)
	to := from.Add(-time.Hour)

	_, err := svc.SubjectSummary(context.Background(), SubjectAttendanceListRequest{DateFrom: &from, DateTo: &to})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}

func TestSubjectSummaryWrapsRepositoryError(t *testing.T) {
	svc := newLessonAttendanceService(&subjectAttendanceRepoStub{summaryErr: sql.ErrConnDone})

	_, err := svc.SubjectSummary(context.Background(), SubjectAttendanceListRequest{})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrInternal.Code, appErrors.FromError(err).Code)
}
