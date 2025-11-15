package service

import (
	"context"
	"database/sql"
	"time"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type calendarEventProvider interface {
	List(ctx context.Context, req CalendarListRequest) ([]models.CalendarEvent, *models.Pagination, error)
}

type aliasTermReader interface {
	FindByID(ctx context.Context, id string) (*models.Term, error)
	FindActive(ctx context.Context) (*models.Term, error)
}

type teacherAssignmentLister interface {
	ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error)
}

type calendarAliasClassReader interface {
	FindByID(ctx context.Context, id string) (*models.Class, error)
}

// CalendarAliasService exposes a thin adapter above CalendarService.
type CalendarAliasService struct {
	calendar    calendarEventProvider
	terms       aliasTermReader
	assignments teacherAssignmentLister
	classes     calendarAliasClassReader
	logger      *zap.Logger
}

// NewCalendarAliasService constructs the alias service.
func NewCalendarAliasService(calendar calendarEventProvider, terms aliasTermReader, assignments teacherAssignmentLister, classes calendarAliasClassReader, logger *zap.Logger) *CalendarAliasService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CalendarAliasService{
		calendar:    calendar,
		terms:       terms,
		assignments: assignments,
		classes:     classes,
		logger:      logger,
	}
}

// List returns calendar events constrained by date range or term.
func (s *CalendarAliasService) List(ctx context.Context, req dto.CalendarAliasRequest, claims *models.JWTClaims) (*dto.CalendarAliasResponse, error) {
	if claims == nil {
		return nil, appErrors.ErrUnauthorized
	}

	rangeMeta, err := s.resolveRange(ctx, req.TermID, req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	if req.ClassID != "" {
		if err := s.ensureClass(ctx, req.ClassID); err != nil {
			return nil, err
		}
	}

	classIDsFilter, err := s.resolveClassFilter(ctx, claims, req.ClassID, req.TermID)
	if err != nil {
		return nil, err
	}
	if claims.Role == models.RoleTeacher && len(classIDsFilter) == 0 && req.ClassID == "" {
		// Teacher without any class for the selected term should not see class-restricted events.
		// They may still see global events, so we proceed without class filter.
	}

	listReq := CalendarListRequest{
		StartDate: &rangeMeta.Start,
		EndDate:   &rangeMeta.End,
		Page:      1,
		PageSize:  500,
	}
	if req.ClassID != "" {
		listReq.ClassIDs = []string{req.ClassID}
	} else if len(classIDsFilter) > 0 {
		listReq.ClassIDs = classIDsFilter
	}

	events, _, err := s.calendar.List(ctx, listReq)
	if err != nil {
		return nil, err
	}

	allowedClasses := map[string]struct{}{}
	if claims.Role == models.RoleTeacher {
		for _, classID := range classIDsFilter {
			allowedClasses[classID] = struct{}{}
		}
		if req.ClassID != "" {
			allowedClasses[req.ClassID] = struct{}{}
		}
	}

	response := &dto.CalendarAliasResponse{
		Range: dto.CalendarAliasRange{
			StartDate: rangeMeta.Start.Format("2006-01-02"),
			EndDate:   rangeMeta.End.Format("2006-01-02"),
		},
		Events: make([]dto.CalendarAliasEvent, 0, len(events)),
	}
	if rangeMeta.TermID != nil {
		response.TermID = rangeMeta.TermID
	}
	if req.ClassID != "" {
		response.ClassID = &req.ClassID
	}

	for _, event := range events {
		if claims.Role == models.RoleTeacher && event.TargetClassID != nil {
			if _, ok := allowedClasses[*event.TargetClassID]; !ok {
				continue
			}
		}
		response.Events = append(response.Events, dto.CalendarAliasEvent{
			ID:          event.ID,
			Title:       event.Title,
			Type:        event.EventType,
			StartDate:   event.StartDate.Format("2006-01-02"),
			EndDate:     event.EndDate.Format("2006-01-02"),
			Description: nullableString(event.Description),
			Audience:    string(event.Audience),
			ClassID:     event.TargetClassID,
		})
	}
	return response, nil
}

type calendarRange struct {
	Start  time.Time
	End    time.Time
	TermID *string
}

func (s *CalendarAliasService) resolveRange(ctx context.Context, termID string, start, end *time.Time) (*calendarRange, error) {
	var startDate, endDate *time.Time
	var resolvedTerm *models.Term
	if start != nil {
		startCopy := start.UTC()
		startDate = &startCopy
	}
	if end != nil {
		endCopy := end.UTC()
		endDate = &endCopy
	}

	if termID != "" {
		term, err := s.terms.FindByID(ctx, termID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, appErrors.Clone(appErrors.ErrNotFound, "term not found")
			}
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load term")
		}
		if startDate == nil {
			startCopy := term.StartDate
			startDate = &startCopy
		}
		if endDate == nil {
			endCopy := term.EndDate
			endDate = &endCopy
		}
		resolvedTerm = term
	}

	if startDate == nil && endDate == nil {
		active, err := s.terms.FindActive(ctx)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, appErrors.Clone(appErrors.ErrValidation, "provide termId or start/end date range")
			}
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to resolve active term")
		}
		startCopy := active.StartDate
		endCopy := active.EndDate
		startDate = &startCopy
		endDate = &endCopy
		resolvedTerm = active
	}

	if startDate == nil || endDate == nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "incomplete date range")
	}

	if startDate.After(*endDate) {
		return nil, appErrors.Clone(appErrors.ErrValidation, "start_date cannot be after end_date")
	}

	result := &calendarRange{
		Start: startDate.UTC(),
		End:   endDate.UTC(),
	}
	if resolvedTerm != nil {
		result.TermID = &resolvedTerm.ID
	}

	return result, nil
}

func (s *CalendarAliasService) resolveClassFilter(ctx context.Context, claims *models.JWTClaims, classID, termID string) ([]string, error) {
	if claims.Role != models.RoleTeacher {
		return nil, nil
	}

	assignments, err := s.assignments.ListByTeacher(ctx, claims.UserID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to resolve teacher assignments")
	}
	classSet := map[string]struct{}{}
	for _, assignment := range assignments {
		if termID != "" && assignment.TermID != termID {
			continue
		}
		classSet[assignment.ClassID] = struct{}{}
	}
	if classID != "" {
		if _, ok := classSet[classID]; !ok {
			return nil, appErrors.ErrForbidden
		}
		return []string{classID}, nil
	}

	list := make([]string, 0, len(classSet))
	for id := range classSet {
		list = append(list, id)
	}
	return list, nil
}

func (s *CalendarAliasService) ensureClass(ctx context.Context, classID string) error {
	if s.classes == nil {
		return nil
	}
	if _, err := s.classes.FindByID(ctx, classID); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "class not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class")
	}
	return nil
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
