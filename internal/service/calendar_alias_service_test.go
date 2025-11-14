package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type calendarProviderStub struct {
	events []models.CalendarEvent
}

func (s calendarProviderStub) List(ctx context.Context, req CalendarListRequest) ([]models.CalendarEvent, *models.Pagination, error) {
	return s.events, nil, nil
}

type termReaderStub struct {
	term   *models.Term
	active *models.Term
	err    error
}

func (s termReaderStub) FindByID(ctx context.Context, id string) (*models.Term, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.term != nil {
		return s.term, nil
	}
	return &models.Term{ID: id, StartDate: time.Now().Add(-24 * time.Hour), EndDate: time.Now().Add(24 * time.Hour)}, nil
}

func (s termReaderStub) FindActive(ctx context.Context) (*models.Term, error) {
	if s.active != nil {
		return s.active, nil
	}
	return &models.Term{ID: "term-active", StartDate: time.Now().Add(-24 * time.Hour), EndDate: time.Now().Add(24 * time.Hour)}, nil
}

type assignmentListerStub struct {
	items []models.TeacherAssignmentDetail
}

func (s assignmentListerStub) ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error) {
	return s.items, nil
}

func TestCalendarAliasServiceListTeacherFiltered(t *testing.T) {
	now := time.Now()
	events := []models.CalendarEvent{
		{ID: "event-1", Title: "All Hands", EventType: "EVENT", StartDate: now, EndDate: now},
		{ID: "event-2", Title: "Class Specific", EventType: "EVENT", StartDate: now, EndDate: now, TargetClassID: testStringPtr("class-1")},
		{ID: "event-3", Title: "Other Class", EventType: "EVENT", StartDate: now, EndDate: now, TargetClassID: testStringPtr("class-2")},
	}
	service := NewCalendarAliasService(
		calendarProviderStub{events: events},
		termReaderStub{term: &models.Term{ID: "term-1", StartDate: now.Add(-time.Hour), EndDate: now.Add(time.Hour)}},
		assignmentListerStub{items: []models.TeacherAssignmentDetail{
			{TeacherAssignment: models.TeacherAssignment{ClassID: "class-1", TermID: "term-1"}},
		}},
		nil,
	)

	result, err := service.List(context.Background(), dto.CalendarAliasRequest{TermID: "term-1"}, &models.JWTClaims{UserID: "teacher-1", Role: models.RoleTeacher})
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "event-1", result[0].ID)
	assert.Equal(t, "event-2", result[1].ID)
}

func TestCalendarAliasServiceListForbiddenClass(t *testing.T) {
	service := NewCalendarAliasService(
		calendarProviderStub{},
		termReaderStub{},
		assignmentListerStub{},
		nil,
	)
	_, err := service.List(context.Background(), dto.CalendarAliasRequest{TermID: "term-1", ClassID: "class-x"}, &models.JWTClaims{UserID: "teacher-1", Role: models.RoleTeacher})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrForbidden.Code, appErrors.FromError(err).Code)
}

func testStringPtr(val string) *string {
	return &val
}
