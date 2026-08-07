package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// PortalCalendarService provides calendar data for parent/student portal.
type PortalCalendarService struct {
	calendarRepo calendarReader
	enrollmentRepo enrollmentReader
	studentRepo studentReader
	validator *validator.Validate
	logger *zap.Logger
}

// NewPortalCalendarService constructs the portal calendar service.
func NewPortalCalendarService(
	calendarRepo calendarReader,
	enrollmentRepo enrollmentReader,
	studentRepo studentReader,
	validate *validator.Validate,
	logger *zap.Logger,
) *PortalCalendarService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PortalCalendarService{
		calendarRepo: calendarRepo,
		enrollmentRepo: enrollmentRepo,
		studentRepo: studentRepo,
		validator: validate,
		logger: logger,
	}
}

// GetCalendarEvents returns calendar events for a student in a term.
func (s *PortalCalendarService) GetCalendarEvents(ctx context.Context, req models.PortalCalendarRequest) (*models.PortalCalendarResponse, error) {
	// Get student detail
	_, err := s.studentRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch student")
	}

	// Get calendar events
	events, err := s.calendarRepo.ListByStudentAndTerm(ctx, req.StudentID, req.TermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch calendar events")
	}

	// Build response
	items := make([]*models.PortalCalendarEvent, len(events))
	for i, e := range events {
		desc := e.Description
		items[i] = &models.PortalCalendarEvent{
			ID:          e.ID,
			Title:       e.Title,
			Description: &desc,
			EventType:   e.EventType,
			StartDate:   e.StartDate.Format("2006-01-02"),
			EndDate:     e.EndDate.Format("2006-01-02"),
			StartTime:   formatTimePtr(e.StartTime),
			EndTime:     formatTimePtr(e.EndTime),
			Audience:    string(e.Audience),
			Location:    e.Location,
		}
	}

	return &models.PortalCalendarResponse{
		Events: items,
	}, nil
}

// formatTimePtr formats a time pointer to RFC3339 string
func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.Format(time.RFC3339)
	return &formatted
}

// GetUpcomingEvents returns upcoming calendar events for a student (next 7 days).
func (s *PortalCalendarService) GetUpcomingEvents(ctx context.Context, studentID string) (*models.PortalCalendarResponse, error) {
	// Get student detail
	_, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch student")
	}

	// Calculate date range: today to 7 days from now
	now := time.Now().UTC()

	// Get calendar events for the next 7 days
	// We need to filter by date range - for now, get all events and filter
	// TODO: Add a date-filtered method to the repository for efficiency
	allEvents, err := s.calendarRepo.ListByStudentAndTerm(ctx, studentID, "")
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch calendar events")
	}

	// Filter events to next 7 days
	var upcomingEvents []models.CalendarEvent
	for _, e := range allEvents {
		eventStart := e.StartDate
		if eventStart.After(now) || eventStart.Equal(now) {
			eventEnd := e.EndDate
			if eventEnd.Before(now.AddDate(0, 0, 7)) || eventEnd.Equal(now.AddDate(0, 0, 7)) {
				upcomingEvents = append(upcomingEvents, e)
			}
		}
	}

	// Build response
	items := make([]*models.PortalCalendarEvent, len(upcomingEvents))
	for i, e := range upcomingEvents {
		desc := e.Description
		items[i] = &models.PortalCalendarEvent{
			ID:          e.ID,
			Title:       e.Title,
			Description: &desc,
			EventType:   e.EventType,
			StartDate:   e.StartDate.Format("2006-01-02"),
			EndDate:     e.EndDate.Format("2006-01-02"),
			StartTime:   formatTimePtr(e.StartTime),
			EndTime:     formatTimePtr(e.EndTime),
			Audience:    string(e.Audience),
			Location:    e.Location,
		}
	}

	return &models.PortalCalendarResponse{
		Events: items,
	}, nil
}