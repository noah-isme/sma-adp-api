package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type portalAnnouncementsRepoStub struct {
	page      int
	limit     int
	active    bool
	studentID string
	termID    string
	items     []models.Announcement
	total     int
}

func (s *portalAnnouncementsRepoStub) ListByStudentAndTerm(context.Context, string, string) ([]models.Announcement, error) {
	return s.items, nil
}

func (s *portalAnnouncementsRepoStub) FindByID(context.Context, string) (*models.Announcement, error) {
	return nil, nil
}

func (s *portalAnnouncementsRepoStub) ListByStudentAndTermPage(_ context.Context, studentID, termID string, page, limit int, activeOnly bool) ([]models.Announcement, int, error) {
	s.studentID = studentID
	s.termID = termID
	s.page = page
	s.limit = limit
	s.active = activeOnly
	return s.items, s.total, nil
}

func TestPortalAnnouncementsServiceUsesRequestedPaginationAndTotal(t *testing.T) {
	now := time.Date(2026, time.August, 12, 8, 0, 0, 0, time.UTC)
	repo := &portalAnnouncementsRepoStub{
		items: []models.Announcement{{
			ID:          "announcement-1",
			Title:       "Announcement",
			Content:     "Content",
			Audience:    models.AnnouncementAudienceAll,
			Priority:    models.AnnouncementPriorityHigh,
			PublishedAt: now,
		}},
		total: 7,
	}
	students := &mockStudentReader{students: map[string]*models.StudentDetail{
		"student-1": {Student: models.Student{ID: "student-1"}},
	}}
	service := NewPortalAnnouncementsService(repo, nil, students, nil, nil)

	result, err := service.GetAnnouncements(context.Background(), models.PortalAnnouncementsRequest{
		StudentID:  "student-1",
		TermID:     "term-1",
		Page:       2,
		Limit:      3,
		ActiveOnly: false,
	})

	require.NoError(t, err)
	require.NotNil(t, result.Pagination)
	assert.Equal(t, "student-1", repo.studentID)
	assert.Equal(t, "term-1", repo.termID)
	assert.Equal(t, 2, repo.page)
	assert.Equal(t, 3, repo.limit)
	assert.False(t, repo.active)
	assert.Equal(t, 2, result.Pagination.Page)
	assert.Equal(t, 3, result.Pagination.PageSize)
	assert.Equal(t, 7, result.Pagination.TotalCount)
	assert.Equal(t, string(models.AnnouncementAudienceAll), result.Announcements[0].Audience)
}

func TestPortalAnnouncementsServiceRejectsInvalidPagination(t *testing.T) {
	students := &mockStudentReader{students: map[string]*models.StudentDetail{
		"student-1": {Student: models.Student{ID: "student-1"}},
	}}
	service := NewPortalAnnouncementsService(&portalAnnouncementsRepoStub{}, nil, students, nil, nil)

	_, err := service.GetAnnouncements(context.Background(), models.PortalAnnouncementsRequest{StudentID: "student-1", Page: -1, Limit: 20})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}
