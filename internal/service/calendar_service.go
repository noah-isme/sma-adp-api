package service

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type calendarRepository interface {
	List(ctx context.Context, filter models.CalendarFilter) ([]models.CalendarEvent, int, error)
	GetByID(ctx context.Context, id string) (*models.CalendarEvent, error)
	Create(ctx context.Context, event *models.CalendarEvent) error
	Update(ctx context.Context, event *models.CalendarEvent) error
	Delete(ctx context.Context, id string) error
}

// CalendarService manages calendar events.
type CalendarService struct {
	repo      calendarRepository
	validator *validator.Validate
	logger    *zap.Logger
}

// NewCalendarService constructs the service.
func NewCalendarService(repo calendarRepository, validate *validator.Validate, logger *zap.Logger) *CalendarService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	svc := &CalendarService{repo: repo, validator: validate, logger: logger}
	svc.validator.RegisterValidation("audience", func(fl validator.FieldLevel) bool {
		switch models.AnnouncementAudience(strings.ToUpper(fl.Field().String())) {
		case models.AnnouncementAudienceAll, models.AnnouncementAudienceGuru, models.AnnouncementAudienceSiswa, models.AnnouncementAudienceClass:
			return true
		default:
			return false
		}
	})
	return svc
}

// CalendarListRequest describes filters for listing events.
type CalendarListRequest struct {
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	Audience  []string   `json:"audience"`
	ClassIDs  []string   `json:"class_ids"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}

// CreateCalendarEventRequest describes create payload.
type CreateCalendarEventRequest struct {
	Title         string     `json:"title" validate:"required"`
	Description   string     `json:"description" validate:"required"`
	EventType     string     `json:"event_type" validate:"required"`
	StartDate     time.Time  `json:"start_date" validate:"required"`
	EndDate       time.Time  `json:"end_date" validate:"required"`
	StartTime     *time.Time `json:"start_time"`
	EndTime       *time.Time `json:"end_time"`
	Audience      string     `json:"audience" validate:"required,audience"`
	TargetClassID *string    `json:"target_class_id"`
	Location      *string    `json:"location"`
	CreatedBy     string     `json:"created_by" validate:"required"`
}

// UpdateCalendarEventRequest describes update payload.
type UpdateCalendarEventRequest struct {
	Title         string     `json:"title" validate:"required"`
	Description   string     `json:"description" validate:"required"`
	EventType     string     `json:"event_type" validate:"required"`
	StartDate     time.Time  `json:"start_date" validate:"required"`
	EndDate       time.Time  `json:"end_date" validate:"required"`
	StartTime     *time.Time `json:"start_time"`
	EndTime       *time.Time `json:"end_time"`
	Audience      string     `json:"audience" validate:"required,audience"`
	TargetClassID *string    `json:"target_class_id"`
	Location      *string    `json:"location"`
}

// List returns calendar events.
func (s *CalendarService) List(ctx context.Context, req CalendarListRequest) ([]models.CalendarEvent, *models.Pagination, error) {
	filter := models.CalendarFilter{
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		ClassIDs:  req.ClassIDs,
		Page:      req.Page,
		PageSize:  req.PageSize,
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 50
	}
	if len(req.Audience) > 0 {
		filter.Audience = make([]models.AnnouncementAudience, len(req.Audience))
		for i, a := range req.Audience {
			filter.Audience[i] = models.AnnouncementAudience(strings.ToUpper(a))
		}
	}
	events, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list calendar events")
	}
	pagination := &models.Pagination{Page: filter.Page, PageSize: filter.PageSize, TotalCount: total}
	return events, pagination, nil
}

// Get returns a calendar event by id.
func (s *CalendarService) Get(ctx context.Context, id string) (*models.CalendarEvent, error) {
	event, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "event not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to get event")
	}
	return event, nil
}

// Create registers a new event.
func (s *CalendarService) Create(ctx context.Context, req CreateCalendarEventRequest) (*models.CalendarEvent, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid payload")
	}
	if req.EndDate.Before(req.StartDate) {
		return nil, appErrors.Clone(appErrors.ErrValidation, "end_date must be on or after start_date")
	}
	if err := s.ensureAudienceTarget(req.Audience, req.TargetClassID); err != nil {
		return nil, err
	}
	event := &models.CalendarEvent{
		Title:         req.Title,
		Description:   req.Description,
		EventType:     req.EventType,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
		Audience:      models.AnnouncementAudience(strings.ToUpper(req.Audience)),
		TargetClassID: req.TargetClassID,
		Location:      req.Location,
		CreatedBy:     req.CreatedBy,
	}
	if err := s.repo.Create(ctx, event); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create event")
	}
	return event, nil
}

// Update modifies an event.
func (s *CalendarService) Update(ctx context.Context, id string, req UpdateCalendarEventRequest) (*models.CalendarEvent, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid payload")
	}
	if req.EndDate.Before(req.StartDate) {
		return nil, appErrors.Clone(appErrors.ErrValidation, "end_date must be on or after start_date")
	}
	if err := s.ensureAudienceTarget(req.Audience, req.TargetClassID); err != nil {
		return nil, err
	}
	event, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "event not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load event")
	}
	event.Title = req.Title
	event.Description = req.Description
	event.EventType = req.EventType
	event.StartDate = req.StartDate
	event.EndDate = req.EndDate
	event.StartTime = req.StartTime
	event.EndTime = req.EndTime
	event.Audience = models.AnnouncementAudience(strings.ToUpper(req.Audience))
	event.TargetClassID = req.TargetClassID
	event.Location = req.Location
	if err := s.repo.Update(ctx, event); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update event")
	}
	return event, nil
}

// Delete removes a calendar event.
func (s *CalendarService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete event")
	}
	return nil
}

func (s *CalendarService) ensureAudienceTarget(audience string, target *string) error {
	if strings.ToUpper(audience) == string(models.AnnouncementAudienceClass) && (target == nil || *target == "") {
		return appErrors.Clone(appErrors.ErrValidation, "target_class_id required for CLASS audience")
	}
	return nil
}
