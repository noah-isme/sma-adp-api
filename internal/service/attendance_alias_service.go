package service

import (
	"context"
	"database/sql"
	"strings"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type analyticsAttendanceProvider interface {
	Attendance(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, bool, error)
}

type attendanceSummaryRepository interface {
	Aggregate(ctx context.Context, filter repository.AttendanceAliasFilter) (*repository.AttendanceAliasAggregate, error)
}

type aliasEnrollmentReader interface {
	ListByClassAndTerm(ctx context.Context, classID, termID string) ([]models.Enrollment, error)
	FindActiveByStudentAndTerm(ctx context.Context, studentID, termID string) ([]models.Enrollment, error)
}

type teacherAssignmentAccessor interface {
	ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error)
	HasClassAccess(ctx context.Context, teacherID, classID, termID string) (bool, error)
}

type termLookup interface {
	FindByID(ctx context.Context, id string) (*models.Term, error)
}

// AttendanceAliasService exposes /attendance and /attendance/daily adapters.
type AttendanceAliasService struct {
	attendance  *AttendanceService
	analytics   analyticsAttendanceProvider
	summaries   attendanceSummaryRepository
	assignments teacherAssignmentAccessor
	enrollments aliasEnrollmentReader
	terms       termLookup
	logger      *zap.Logger
}

// NewAttendanceAliasService constructs the alias service.
func NewAttendanceAliasService(
	attendance *AttendanceService,
	analytics analyticsAttendanceProvider,
	summaries attendanceSummaryRepository,
	assignments teacherAssignmentAccessor,
	enrollments aliasEnrollmentReader,
	terms termLookup,
	logger *zap.Logger,
) *AttendanceAliasService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AttendanceAliasService{
		attendance:  attendance,
		analytics:   analytics,
		summaries:   summaries,
		assignments: assignments,
		enrollments: enrollments,
		terms:       terms,
		logger:      logger,
	}
}

// ListDaily proxies to AttendanceService.ListDaily with additional RBAC assurances.
func (s *AttendanceAliasService) ListDaily(ctx context.Context, req dto.AttendanceDailyRequest, claims *models.JWTClaims) ([]models.DailyAttendanceRecord, *models.Pagination, error) {
	if claims == nil {
		return nil, nil, appErrors.ErrUnauthorized
	}
	if req.TermID == "" {
		return nil, nil, appErrors.Clone(appErrors.ErrValidation, "termId is required")
	}
	if err := s.ensureTerm(ctx, req.TermID); err != nil {
		return nil, nil, err
	}

	if claims.Role == models.RoleTeacher {
		if req.ClassID == "" {
			return nil, nil, appErrors.Clone(appErrors.ErrValidation, "classId is required for teachers")
		}
		if err := s.assertClassAccess(ctx, claims.UserID, req.ClassID, req.TermID); err != nil {
			return nil, nil, err
		}
	}

	filter := DailyAttendanceListRequest{
		TermID:    req.TermID,
		ClassID:   req.ClassID,
		StudentID: req.StudentID,
		DateFrom:  req.DateFrom,
		DateTo:    req.DateTo,
		Page:      req.Page,
		PageSize:  req.PageSize,
		SortBy:    req.SortBy,
		SortOrder: req.SortOrder,
	}
	if req.Status != nil {
		status := strings.ToUpper(*req.Status)
		filter.Status = &status
	}
	return s.attendance.ListDaily(ctx, filter)
}

// Summary aggregates attendance data for the alias endpoint.
func (s *AttendanceAliasService) Summary(ctx context.Context, req dto.AttendanceSummaryRequest, claims *models.JWTClaims) (*dto.AttendanceSummaryResponse, bool, error) {
	if claims == nil {
		return nil, false, appErrors.ErrUnauthorized
	}
	if req.TermID == "" {
		return nil, false, appErrors.Clone(appErrors.ErrValidation, "termId is required")
	}
	if err := s.ensureTerm(ctx, req.TermID); err != nil {
		return nil, false, err
	}

	var classFilterIDs []string
	if claims.Role == models.RoleTeacher {
		classSet, err := s.teacherClasses(ctx, claims.UserID, req.TermID)
		if err != nil {
			return nil, false, err
		}
		if req.ClassID != "" {
			if _, ok := classSet[req.ClassID]; !ok {
				return nil, false, appErrors.ErrForbidden
			}
		} else {
			for id := range classSet {
				classFilterIDs = append(classFilterIDs, id)
			}
			if len(classFilterIDs) == 0 {
				// Teacher without classes should receive empty payload.
				return s.emptySummaryResponse(req), false, nil
			}
		}
	}

	if req.StudentID != "" && claims.Role == models.RoleTeacher {
		if err := s.ensureTeacherCanSeeStudent(ctx, claims.UserID, req.StudentID, req.TermID); err != nil {
			return nil, false, err
		}
	}

	filter := repository.AttendanceAliasFilter{
		TermID:    req.TermID,
		ClassID:   req.ClassID,
		ClassIDs:  classFilterIDs,
		StudentID: req.StudentID,
		DateFrom:  req.FromDate,
		DateTo:    req.ToDate,
	}
	aggregate, err := s.summaries.Aggregate(ctx, filter)
	if err != nil {
		return nil, false, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to summarise attendance")
	}

	response := dto.AttendanceSummaryResponse{
		Scope: dto.AttendanceSummaryScope{
			TermID: req.TermID,
		},
	}
	if req.ClassID != "" {
		response.Scope.ClassID = &req.ClassID
	}
	if req.StudentID != "" {
		response.Scope.StudentID = &req.StudentID
	}

	totalRecords := aggregate.Present + aggregate.Sick + aggregate.Excused + aggregate.Absent
	var rate float64
	if totalRecords > 0 {
		rate = float64(aggregate.Present) / float64(totalRecords) * 100
	}
	response.Summary = dto.AttendanceSummaryStats{
		TotalDays:      aggregate.TotalDays,
		Present:        aggregate.Present,
		Sick:           aggregate.Sick,
		Excused:        aggregate.Excused,
		Absent:         aggregate.Absent,
		AttendanceRate: rate,
	}

	perStudent := make([]dto.AttendanceSummaryStudent, 0, len(aggregate.Students))
	for _, row := range aggregate.Students {
		perStudent = append(perStudent, dto.AttendanceSummaryStudent{
			StudentID:      row.StudentID,
			StudentName:    row.StudentName,
			ClassID:        row.ClassID,
			Present:        row.Present,
			Sick:           row.Sick,
			Excused:        row.Excused,
			Absent:         row.Absent,
			AttendanceRate: row.Rate,
		})
	}
	response.PerStudent = perStudent

	cacheHit := false
	if s.analytics != nil {
		_, cacheHit, _ = s.analytics.Attendance(ctx, models.AnalyticsAttendanceFilter{
			TermID:  req.TermID,
			ClassID: req.ClassID,
		})
	}

	return &response, cacheHit, nil
}

func (s *AttendanceAliasService) ensureTerm(ctx context.Context, termID string) error {
	if _, err := s.terms.FindByID(ctx, termID); err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrNotFound, "term not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load term")
	}
	return nil
}

func (s *AttendanceAliasService) teacherClasses(ctx context.Context, teacherID, termID string) (map[string]struct{}, error) {
	assignments, err := s.assignments.ListByTeacher(ctx, teacherID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to resolve assignments")
	}
	result := make(map[string]struct{})
	for _, assignment := range assignments {
		if assignment.TermID != termID {
			continue
		}
		result[assignment.ClassID] = struct{}{}
	}
	return result, nil
}

func (s *AttendanceAliasService) assertClassAccess(ctx context.Context, teacherID, classID, termID string) error {
	has, err := s.assignments.HasClassAccess(ctx, teacherID, classID, termID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify class access")
	}
	if !has {
		return appErrors.ErrForbidden
	}
	return nil
}

func (s *AttendanceAliasService) ensureTeacherCanSeeStudent(ctx context.Context, teacherID, studentID, termID string) error {
	enrollments, err := s.enrollments.FindActiveByStudentAndTerm(ctx, studentID, termID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to resolve student enrollment")
	}
	for _, enrollment := range enrollments {
		if err := s.assertClassAccess(ctx, teacherID, enrollment.ClassID, termID); err == nil {
			return nil
		}
	}
	return appErrors.ErrForbidden
}

func (s *AttendanceAliasService) emptySummaryResponse(req dto.AttendanceSummaryRequest) *dto.AttendanceSummaryResponse {
	response := &dto.AttendanceSummaryResponse{
		Scope: dto.AttendanceSummaryScope{
			TermID: req.TermID,
		},
		Summary: dto.AttendanceSummaryStats{},
	}
	if req.ClassID != "" {
		response.Scope.ClassID = &req.ClassID
	}
	if req.StudentID != "" {
		response.Scope.StudentID = &req.StudentID
	}
	return response
}
