package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/service"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type portalAccessReaderForHandler struct {
	links map[string]*models.ParentStudentLink
}

func (s *portalAccessReaderForHandler) FindParentStudentLinkByParentAndStudent(_ context.Context, parentID, studentID string) (*models.ParentStudentLink, error) {
	link, ok := s.links[parentID+":"+studentID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return link, nil
}

type portalAnnouncementsRepoForHandler struct {
	page      int
	limit     int
	active    bool
	studentID string
	termID    string
}

func (s *portalAnnouncementsRepoForHandler) ListByStudentAndTerm(context.Context, string, string) ([]models.Announcement, error) {
	return nil, nil
}

func (s *portalAnnouncementsRepoForHandler) FindByID(context.Context, string) (*models.Announcement, error) {
	return nil, sql.ErrNoRows
}

func (s *portalAnnouncementsRepoForHandler) ListByStudentAndTermPage(_ context.Context, studentID, termID string, page, limit int, activeOnly bool) ([]models.Announcement, int, error) {
	s.studentID = studentID
	s.termID = termID
	s.page = page
	s.limit = limit
	s.active = activeOnly
	return []models.Announcement{{
		ID:          "announcement-1",
		Title:       "Announcement",
		Content:     "Content",
		Audience:    models.AnnouncementAudienceAll,
		Priority:    models.AnnouncementPriorityNormal,
		PublishedAt: time.Date(2026, time.August, 12, 8, 0, 0, 0, time.UTC),
	}}, 4, nil
}

type portalStudentReaderForHandler struct{}

func (*portalStudentReaderForHandler) FindByID(context.Context, string) (*models.StudentDetail, error) {
	return &models.StudentDetail{}, nil
}

func portalDataTestContext(path string, claims *models.JWTClaims) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	c.Set(middleware.PortalContextUserKey, claims)
	return c, recorder
}

func TestPortalDataHandlerParentCannotAccessUnlinkedStudent(t *testing.T) {
	h := &PortalDataHandler{accessReader: &portalAccessReaderForHandler{links: map[string]*models.ParentStudentLink{
		"parent-1:student-1": {
			ParentID:             "parent-1",
			StudentID:            "student-1",
			CanViewAnnouncements: true,
		},
	}}}
	c, recorder := portalDataTestContext("/portal/announcements?studentId=student-2", &models.JWTClaims{UserID: "parent-1", Role: models.RoleOrtu})

	h.GetAnnouncements(c)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	var body struct {
		Error *appErrors.Error `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.NotNil(t, body.Error)
	assert.Equal(t, appErrors.ErrForbidden.Code, body.Error.Code)
}

func TestPortalDataHandlerStudentCannotOverrideOwnScope(t *testing.T) {
	h := &PortalDataHandler{}
	c, _ := portalDataTestContext("/portal/grades?studentId=student-2", &models.JWTClaims{
		UserID:    "student-user",
		Role:      models.RoleStudent,
		StudentID: "student-1",
	})

	studentID, err := h.getStudentIDForRequest(c, &models.JWTClaims{
		UserID:    "student-user",
		Role:      models.RoleStudent,
		StudentID: "student-1",
	})

	require.Error(t, err)
	assert.Empty(t, studentID)
	assert.Equal(t, appErrors.ErrForbidden.Code, appErrors.FromError(err).Code)
}

func TestPortalDataHandlerRejectsNonPortalRole(t *testing.T) {
	h := &PortalDataHandler{}
	c, _ := portalDataTestContext("/portal/grades?studentId=student-1", &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	_, err := h.getStudentIDForRequest(c, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})

	require.Error(t, err)
	assert.Equal(t, appErrors.ErrForbidden.Code, appErrors.FromError(err).Code)
}

func TestPortalDataHandlerParsesAnnouncementPaginationAndKeepsEnvelope(t *testing.T) {
	repo := &portalAnnouncementsRepoForHandler{}
	announcements := service.NewPortalAnnouncementsService(repo, nil, &portalStudentReaderForHandler{}, nil, nil)
	h := &PortalDataHandler{announcementsService: announcements}
	c, recorder := portalDataTestContext("/portal/announcements?page=2&limit=3&active=false&termId=term-1", &models.JWTClaims{
		UserID:    "student-user",
		Role:      models.RoleStudent,
		StudentID: "student-1",
	})

	h.GetAnnouncements(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 2, repo.page)
	assert.Equal(t, 3, repo.limit)
	assert.False(t, repo.active)
	assert.Equal(t, "student-1", repo.studentID)
	assert.Equal(t, "term-1", repo.termID)

	var envelope struct {
		Data struct {
			Announcements []models.PortalAnnouncement `json:"announcements"`
			Pagination    models.PaginationMeta       `json:"pagination"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Announcements, 1)
	assert.Equal(t, 2, envelope.Data.Pagination.Page)
	assert.Equal(t, 3, envelope.Data.Pagination.PageSize)
	assert.Equal(t, 4, envelope.Data.Pagination.TotalCount)
}

func TestPortalDataHandlerRejectsInvalidAnnouncementPagination(t *testing.T) {
	h := &PortalDataHandler{}
	c, recorder := portalDataTestContext("/portal/announcements?page=0", &models.JWTClaims{
		UserID:    "student-user",
		Role:      models.RoleStudent,
		StudentID: "student-1",
	})

	h.GetAnnouncements(c)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	var body struct {
		Error *appErrors.Error `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.NotNil(t, body.Error)
	assert.Equal(t, appErrors.ErrValidation.Code, body.Error.Code)
}
