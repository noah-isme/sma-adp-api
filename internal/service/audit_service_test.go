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

type auditRepoStub struct {
	listFilter models.AuditLogFilter
	listResp   []models.AuditLogEntry
	listTotal  int
	listErr    error
	findResp   *models.AuditLogEntry
	findErr    error
	facetsResp *models.AuditLogFacets
	facetsErr  error
}

func (s *auditRepoStub) List(ctx context.Context, filter models.AuditLogFilter) ([]models.AuditLogEntry, int, error) {
	s.listFilter = filter
	return s.listResp, s.listTotal, s.listErr
}

func (s *auditRepoStub) FindByID(ctx context.Context, id string) (*models.AuditLogEntry, error) {
	return s.findResp, s.findErr
}

func (s *auditRepoStub) Facets(ctx context.Context) (*models.AuditLogFacets, error) {
	return s.facetsResp, s.facetsErr
}

func TestAuditServiceListAppliesPaginationDefaults(t *testing.T) {
	repo := &auditRepoStub{listTotal: 3}
	svc := NewAuditService(repo, nil)

	_, pagination, err := svc.List(context.Background(), AuditLogListRequest{})
	require.NoError(t, err)
	require.NotNil(t, pagination)

	assert.Equal(t, 1, repo.listFilter.Page)
	assert.Equal(t, defaultAuditPageSize, repo.listFilter.PageSize)
	assert.Equal(t, 1, pagination.Page)
	assert.Equal(t, defaultAuditPageSize, pagination.PageSize)
	assert.Equal(t, 3, pagination.TotalCount)
}

// A caller asking for a huge page must not be able to pull the whole table.
func TestAuditServiceListClampsPageSize(t *testing.T) {
	repo := &auditRepoStub{}
	svc := NewAuditService(repo, nil)

	_, pagination, err := svc.List(context.Background(), AuditLogListRequest{PageSize: 100000})
	require.NoError(t, err)

	assert.Equal(t, maxAuditPageSize, repo.listFilter.PageSize)
	assert.Equal(t, maxAuditPageSize, pagination.PageSize)
}

func TestAuditServiceListTrimsFilterInput(t *testing.T) {
	repo := &auditRepoStub{}
	svc := NewAuditService(repo, nil)

	_, _, err := svc.List(context.Background(), AuditLogListRequest{
		UserID:   "  user-1  ",
		Action:   "  LOGIN ",
		Resource: " users ",
		Search:   "  admin ",
	})
	require.NoError(t, err)

	assert.Equal(t, "user-1", repo.listFilter.UserID)
	assert.Equal(t, "LOGIN", repo.listFilter.Action)
	assert.Equal(t, "users", repo.listFilter.Resource)
	assert.Equal(t, "admin", repo.listFilter.Search)
}

func TestAuditServiceListRejectsInvertedDateRange(t *testing.T) {
	svc := NewAuditService(&auditRepoStub{}, nil)
	from := time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)
	to := from.Add(-24 * time.Hour)

	_, _, err := svc.List(context.Background(), AuditLogListRequest{DateFrom: &from, DateTo: &to})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}

// An empty result must serialise as [] rather than null so the client's list
// rendering does not need a null guard.
func TestAuditServiceListReturnsEmptySliceNotNil(t *testing.T) {
	svc := NewAuditService(&auditRepoStub{listResp: nil}, nil)

	entries, _, err := svc.List(context.Background(), AuditLogListRequest{})
	require.NoError(t, err)
	require.NotNil(t, entries)
	assert.Len(t, entries, 0)
}

func TestAuditServiceListWrapsRepositoryError(t *testing.T) {
	svc := NewAuditService(&auditRepoStub{listErr: sql.ErrConnDone}, nil)

	_, _, err := svc.List(context.Background(), AuditLogListRequest{})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrInternal.Code, appErrors.FromError(err).Code)
}

func TestAuditServiceGetRequiresID(t *testing.T) {
	svc := NewAuditService(&auditRepoStub{}, nil)

	_, err := svc.Get(context.Background(), "   ")
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}

func TestAuditServiceGetMapsNoRowsToNotFound(t *testing.T) {
	svc := NewAuditService(&auditRepoStub{findErr: sql.ErrNoRows}, nil)

	_, err := svc.Get(context.Background(), "log-1")
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrNotFound.Code, appErrors.FromError(err).Code)
}

func TestAuditServiceGetSuccess(t *testing.T) {
	want := &models.AuditLogEntry{ID: "log-1", Action: "LOGIN"}
	svc := NewAuditService(&auditRepoStub{findResp: want}, nil)

	got, err := svc.Get(context.Background(), "log-1")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAuditServiceFacetsNormalisesNilSlices(t *testing.T) {
	svc := NewAuditService(&auditRepoStub{facetsResp: &models.AuditLogFacets{}}, nil)

	facets, err := svc.Facets(context.Background())
	require.NoError(t, err)
	require.NotNil(t, facets.Actions)
	require.NotNil(t, facets.Resources)
	assert.Len(t, facets.Actions, 0)
	assert.Len(t, facets.Resources, 0)
}

func TestAuditServiceRejectsMissingRepository(t *testing.T) {
	svc := NewAuditService(nil, nil)

	_, _, listErr := svc.List(context.Background(), AuditLogListRequest{})
	require.Error(t, listErr)

	_, getErr := svc.Get(context.Background(), "log-1")
	require.Error(t, getErr)

	_, facetsErr := svc.Facets(context.Background())
	require.Error(t, facetsErr)
}
