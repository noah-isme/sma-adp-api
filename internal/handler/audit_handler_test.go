package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type auditServiceMock struct {
	listReq     service.AuditLogListRequest
	listCalled  bool
	listResp    []models.AuditLogEntry
	listErr     error
	getID       string
	getResp     *models.AuditLogEntry
	getErr      error
	facetsResp  *models.AuditLogFacets
	facetsErr   error
	facetsCalls int
}

func (m *auditServiceMock) List(ctx context.Context, req service.AuditLogListRequest) ([]models.AuditLogEntry, *models.Pagination, error) {
	m.listCalled = true
	m.listReq = req
	if m.listErr != nil {
		return nil, nil, m.listErr
	}
	return m.listResp, &models.Pagination{Page: req.Page, PageSize: req.PageSize, TotalCount: len(m.listResp)}, nil
}

func (m *auditServiceMock) Get(ctx context.Context, id string) (*models.AuditLogEntry, error) {
	m.getID = id
	return m.getResp, m.getErr
}

func (m *auditServiceMock) Facets(ctx context.Context) (*models.AuditLogFacets, error) {
	m.facetsCalls++
	return m.facetsResp, m.facetsErr
}

func newAuditRequest(t *testing.T, target string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodGet, target, nil)
	require.NoError(t, err)
	c.Request = req
	return c, w
}

func TestAuditHandlerListPassesFilters(t *testing.T) {
	mockSvc := &auditServiceMock{
		listResp: []models.AuditLogEntry{{ID: "log-1", Action: "LOGIN"}},
	}
	handler := NewAuditHandler(mockSvc)

	c, w := newAuditRequest(t, "/audit-logs?userId=user-1&action=login&resource=users&resourceId=user-9&search=admin&page=2&limit=10&sortBy=action&sortOrder=ASC")
	handler.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, mockSvc.listCalled)
	assert.Equal(t, "user-1", mockSvc.listReq.UserID)
	assert.Equal(t, "login", mockSvc.listReq.Action)
	assert.Equal(t, "users", mockSvc.listReq.Resource)
	assert.Equal(t, "user-9", mockSvc.listReq.ResourceID)
	assert.Equal(t, "admin", mockSvc.listReq.Search)
	assert.Equal(t, 2, mockSvc.listReq.Page)
	assert.Equal(t, 10, mockSvc.listReq.PageSize)
	assert.Equal(t, "action", mockSvc.listReq.SortBy)
	assert.Equal(t, "asc", mockSvc.listReq.SortOrder)
}

// A bare dateTo must cover the whole day, otherwise dateFrom=dateTo returns
// nothing because both resolve to midnight.
func TestAuditHandlerListWidensBareDateToEndOfDay(t *testing.T) {
	mockSvc := &auditServiceMock{}
	handler := NewAuditHandler(mockSvc)

	c, w := newAuditRequest(t, "/audit-logs?dateFrom=2024-03-01&dateTo=2024-03-01")
	handler.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mockSvc.listReq.DateFrom)
	require.NotNil(t, mockSvc.listReq.DateTo)
	assert.Equal(t, time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), *mockSvc.listReq.DateFrom)
	assert.True(t, mockSvc.listReq.DateTo.After(*mockSvc.listReq.DateFrom))
	assert.Equal(t, 2024, mockSvc.listReq.DateTo.Year())
	assert.Equal(t, time.March, mockSvc.listReq.DateTo.Month())
	assert.Equal(t, 1, mockSvc.listReq.DateTo.Day())
	assert.Equal(t, 23, mockSvc.listReq.DateTo.Hour())
}

func TestAuditHandlerListAcceptsRFC3339(t *testing.T) {
	mockSvc := &auditServiceMock{}
	handler := NewAuditHandler(mockSvc)

	c, w := newAuditRequest(t, "/audit-logs?dateFrom=2024-03-01T08:30:00Z")
	handler.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mockSvc.listReq.DateFrom)
	assert.Equal(t, 8, mockSvc.listReq.DateFrom.Hour())
	assert.Equal(t, 30, mockSvc.listReq.DateFrom.Minute())
}

func TestAuditHandlerListRejectsInvalidDate(t *testing.T) {
	handler := NewAuditHandler(&auditServiceMock{})

	c, w := newAuditRequest(t, "/audit-logs?dateFrom=not-a-date")
	handler.List(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuditHandlerListPropagatesServiceError(t *testing.T) {
	handler := NewAuditHandler(&auditServiceMock{listErr: appErrors.ErrForbidden})

	c, w := newAuditRequest(t, "/audit-logs")
	handler.List(c)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuditHandlerGetNotFound(t *testing.T) {
	handler := NewAuditHandler(&auditServiceMock{getErr: appErrors.ErrNotFound})

	c, w := newAuditRequest(t, "/audit-logs/missing")
	c.Params = gin.Params{{Key: "id", Value: "missing"}}
	handler.Get(c)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuditHandlerGetSuccess(t *testing.T) {
	mockSvc := &auditServiceMock{
		getResp: &models.AuditLogEntry{ID: "log-7", Action: "USER_UPDATE"},
	}
	handler := NewAuditHandler(mockSvc)

	c, w := newAuditRequest(t, "/audit-logs/log-7")
	c.Params = gin.Params{{Key: "id", Value: "log-7"}}
	handler.Get(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "log-7", mockSvc.getID)

	var envelope struct {
		Data models.AuditLogEntry `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	assert.Equal(t, "USER_UPDATE", envelope.Data.Action)
}

func TestAuditHandlerFacets(t *testing.T) {
	mockSvc := &auditServiceMock{
		facetsResp: &models.AuditLogFacets{
			Actions:   []models.AuditLogFacetCount{{Value: "LOGIN", Count: 12}},
			Resources: []models.AuditLogFacetCount{{Value: "users", Count: 5}},
		},
	}
	handler := NewAuditHandler(mockSvc)

	c, w := newAuditRequest(t, "/audit-logs/facets")
	handler.Facets(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, mockSvc.facetsCalls)

	var envelope struct {
		Data models.AuditLogFacets `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Actions, 1)
	assert.Equal(t, "LOGIN", envelope.Data.Actions[0].Value)
	assert.Equal(t, 12, envelope.Data.Actions[0].Count)
}

func TestAuditHandlerRejectsUnconfiguredService(t *testing.T) {
	handler := NewAuditHandler(nil)

	c, w := newAuditRequest(t, "/audit-logs")
	handler.List(c)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}
