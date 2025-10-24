package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type dailyAttendanceRepository interface {
	List(ctx context.Context, filter models.DailyAttendanceFilter) ([]models.DailyAttendanceRecord, int, error)
	Upsert(ctx context.Context, record *models.DailyAttendance) (*models.DailyAttendance, error)
	BulkInsert(ctx context.Context, records []models.DailyAttendance, atomic bool) ([]models.DailyAttendance, error)
	ClassReport(ctx context.Context, classID string, date time.Time) ([]models.DailyAttendanceReportRow, error)
	StudentHistory(ctx context.Context, studentID string, from, to *time.Time) ([]models.DailyAttendanceHistoryRow, error)
	StudentSummary(ctx context.Context, studentID string, termID string) (*models.DailyAttendanceSummary, error)
}

type subjectAttendanceRepository interface {
	List(ctx context.Context, filter models.SubjectAttendanceFilter) ([]models.SubjectAttendanceRecord, int, error)
	Upsert(ctx context.Context, record *models.SubjectAttendance) (*models.SubjectAttendance, error)
	BulkInsert(ctx context.Context, records []models.SubjectAttendance, atomic bool) ([]models.SubjectAttendance, error)
	SessionReport(ctx context.Context, scheduleID string, date time.Time) ([]models.SubjectAttendanceReportRow, error)
}

// AttendanceService coordinates attendance workflows.
type AttendanceService struct {
	dailyRepo   dailyAttendanceRepository
	subjectRepo subjectAttendanceRepository
	validator   *validator.Validate
	logger      *zap.Logger
}

// NewAttendanceService constructs the attendance service.
func NewAttendanceService(daily dailyAttendanceRepository, subject subjectAttendanceRepository, validate *validator.Validate, logger *zap.Logger) *AttendanceService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	svc := &AttendanceService{dailyRepo: daily, subjectRepo: subject, validator: validate, logger: logger}
	svc.validator.RegisterValidation("attendance_status", func(fl validator.FieldLevel) bool {
		status := models.AttendanceStatus(strings.ToUpper(fl.Field().String()))
		return status.Valid()
	})
	svc.validator.RegisterValidation("bulk_mode", func(fl validator.FieldLevel) bool {
		mode := models.BulkOperationMode(strings.ToLower(fl.Field().String()))
		return mode == models.BulkModeAtomic || mode == models.BulkModePartialOnError
	})
	return svc
}

// DailyAttendanceListRequest is used for listing daily attendance.
type DailyAttendanceListRequest struct {
	ClassID   string     `json:"class_id"`
	TermID    string     `json:"term_id"`
	Status    *string    `json:"status" validate:"omitempty,attendance_status"`
	DateFrom  *time.Time `json:"date_from"`
	DateTo    *time.Time `json:"date_to"`
	StudentID string     `json:"student_id"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
	SortBy    string     `json:"sort_by"`
	SortOrder string     `json:"sort_order"`
}

// MarkDailyAttendanceRequest describes payload for marking single daily attendance.
type MarkDailyAttendanceRequest struct {
	EnrollmentID string  `json:"enrollment_id" validate:"required"`
	Date         string  `json:"date" validate:"required"`
	Status       string  `json:"status" validate:"required,attendance_status"`
	Notes        *string `json:"notes"`
}

// BulkDailyAttendanceItem holds entries for bulk operations.
type BulkDailyAttendanceItem struct {
	EnrollmentID string  `json:"enrollment_id" validate:"required"`
	Status       string  `json:"status" validate:"required,attendance_status"`
	Notes        *string `json:"notes"`
}

// BulkMarkDailyAttendanceRequest describes the bulk mark payload.
type BulkMarkDailyAttendanceRequest struct {
	Date  string                    `json:"date" validate:"required"`
	Items []BulkDailyAttendanceItem `json:"items" validate:"required,min=1,dive"`
	Mode  string                    `json:"mode" validate:"required,bulk_mode"`
}

// BulkAttendanceResult summarises bulk execution.
type BulkAttendanceResult struct {
	Processed int                             `json:"processed"`
	Success   int                             `json:"success"`
	Conflicts []models.AttendanceBulkConflict `json:"conflicts,omitempty"`
}

// SubjectAttendanceListRequest describes filters for subject attendance listing.
type SubjectAttendanceListRequest struct {
	ScheduleID string     `json:"schedule_id"`
	Date       *time.Time `json:"date"`
	Status     *string    `json:"status" validate:"omitempty,attendance_status"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	SortBy     string     `json:"sort_by"`
	SortOrder  string     `json:"sort_order"`
}

// MarkSubjectAttendanceRequest describes a single subject attendance payload.
type MarkSubjectAttendanceRequest struct {
	EnrollmentID string  `json:"enrollment_id" validate:"required"`
	ScheduleID   string  `json:"schedule_id" validate:"required"`
	Date         string  `json:"date" validate:"required"`
	Status       string  `json:"status" validate:"required,attendance_status"`
	Notes        *string `json:"notes"`
}

// BulkSubjectAttendanceItem for bulk operations.
type BulkSubjectAttendanceItem struct {
	EnrollmentID string  `json:"enrollment_id" validate:"required"`
	Status       string  `json:"status" validate:"required,attendance_status"`
	Notes        *string `json:"notes"`
}

// BulkMarkSubjectAttendanceRequest describes a bulk subject attendance request.
type BulkMarkSubjectAttendanceRequest struct {
	ScheduleID string                      `json:"schedule_id" validate:"required"`
	Date       string                      `json:"date" validate:"required"`
	Mode       string                      `json:"mode" validate:"required,bulk_mode"`
	Items      []BulkSubjectAttendanceItem `json:"items" validate:"required,min=1,dive"`
}

// ListDaily returns paginated daily attendance.
func (s *AttendanceService) ListDaily(ctx context.Context, req DailyAttendanceListRequest) ([]models.DailyAttendanceRecord, *models.Pagination, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid filter")
	}
	var status *models.AttendanceStatus
	if req.Status != nil {
		st := models.AttendanceStatus(strings.ToUpper(*req.Status))
		status = &st
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.PageSize
	if size <= 0 {
		size = 50
	}
	filter := models.DailyAttendanceFilter{
		ClassID:   req.ClassID,
		TermID:    req.TermID,
		Status:    status,
		DateFrom:  req.DateFrom,
		DateTo:    req.DateTo,
		StudentID: req.StudentID,
		Page:      page,
		PageSize:  size,
		SortBy:    req.SortBy,
		SortOrder: req.SortOrder,
	}
	rows, total, err := s.dailyRepo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list daily attendance")
	}
	pagination := &models.Pagination{Page: page, PageSize: size, TotalCount: total}
	return rows, pagination, nil
}

// MarkDaily marks a single student's attendance for a day.
func (s *AttendanceService) MarkDaily(ctx context.Context, req MarkDailyAttendanceRequest) (*models.DailyAttendance, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid payload")
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "invalid date format, expected YYYY-MM-DD")
	}
	status := models.AttendanceStatus(strings.ToUpper(req.Status))
	record := &models.DailyAttendance{EnrollmentID: req.EnrollmentID, Date: date, Status: status, Notes: req.Notes}
	stored, err := s.dailyRepo.Upsert(ctx, record)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to mark attendance")
	}
	return stored, nil
}

// BulkMarkDaily records daily attendance for multiple students.
func (s *AttendanceService) BulkMarkDaily(ctx context.Context, req BulkMarkDailyAttendanceRequest) (*BulkAttendanceResult, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid payload")
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "invalid date format, expected YYYY-MM-DD")
	}
	mode := models.BulkOperationMode(strings.ToLower(req.Mode))
	seen := map[string]struct{}{}
	records := make([]models.DailyAttendance, len(req.Items))
	for i, item := range req.Items {
		key := fmt.Sprintf("%s|%s", item.EnrollmentID, date.Format("2006-01-02"))
		if _, ok := seen[key]; ok {
			return nil, appErrors.Clone(appErrors.ErrConflict, "duplicate enrollment in payload")
		}
		seen[key] = struct{}{}
		status := models.AttendanceStatus(strings.ToUpper(item.Status))
		records[i] = models.DailyAttendance{EnrollmentID: item.EnrollmentID, Date: date, Status: status, Notes: item.Notes}
	}
	conflicts, err := s.dailyRepo.BulkInsert(ctx, records, mode == models.BulkModeAtomic)
	if err != nil {
		if mode == models.BulkModeAtomic {
			return nil, appErrors.Wrap(err, appErrors.ErrConflict.Code, appErrors.ErrConflict.Status, err.Error())
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "bulk mark failed")
	}
	result := &BulkAttendanceResult{Processed: len(records), Success: len(records) - len(conflicts)}
	if len(conflicts) > 0 {
		result.Conflicts = make([]models.AttendanceBulkConflict, len(conflicts))
		for i, conflict := range conflicts {
			result.Conflicts[i] = models.AttendanceBulkConflict{
				EnrollmentID: conflict.EnrollmentID,
				Date:         conflict.Date,
				Reason:       "duplicate record",
			}
		}
	}
	return result, nil
}

// ClassReport returns daily attendance for a class by date.
func (s *AttendanceService) ClassReport(ctx context.Context, classID string, date time.Time) ([]models.DailyAttendanceReportRow, error) {
	rows, err := s.dailyRepo.ClassReport(ctx, classID, date)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class report")
	}
	return rows, nil
}

// StudentReport returns history and summary.
type StudentAttendanceReport struct {
	History []models.DailyAttendanceHistoryRow `json:"history"`
	Summary *models.DailyAttendanceSummary     `json:"summary"`
}

// StudentAttendanceReport returns a student's daily attendance history and summary.
func (s *AttendanceService) StudentAttendanceReport(ctx context.Context, studentID string, from, to *time.Time, termID string) (*StudentAttendanceReport, error) {
	history, err := s.dailyRepo.StudentHistory(ctx, studentID, from, to)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch attendance history")
	}
	summary, err := s.dailyRepo.StudentSummary(ctx, studentID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to summarise attendance")
	}
	return &StudentAttendanceReport{History: history, Summary: summary}, nil
}

// AttendancePercentage returns attendance percentage for a student term.
func (s *AttendanceService) AttendancePercentage(ctx context.Context, studentID, termID string) (*models.DailyAttendanceSummary, error) {
	summary, err := s.dailyRepo.StudentSummary(ctx, studentID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to calculate percentage")
	}
	return summary, nil
}

// ListSubject returns subject attendance list.
func (s *AttendanceService) ListSubject(ctx context.Context, req SubjectAttendanceListRequest) ([]models.SubjectAttendanceRecord, *models.Pagination, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid filter")
	}
	var status *models.AttendanceStatus
	if req.Status != nil {
		st := models.AttendanceStatus(strings.ToUpper(*req.Status))
		status = &st
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.PageSize
	if size <= 0 {
		size = 50
	}
	filter := models.SubjectAttendanceFilter{
		ScheduleID: req.ScheduleID,
		Date:       req.Date,
		Status:     status,
		Page:       page,
		PageSize:   size,
		SortBy:     req.SortBy,
		SortOrder:  req.SortOrder,
	}
	rows, total, err := s.subjectRepo.List(ctx, filter)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list subject attendance")
	}
	pagination := &models.Pagination{Page: page, PageSize: size, TotalCount: total}
	return rows, pagination, nil
}

// MarkSubject marks attendance for a specific session.
func (s *AttendanceService) MarkSubject(ctx context.Context, req MarkSubjectAttendanceRequest) (*models.SubjectAttendance, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid payload")
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "invalid date format, expected YYYY-MM-DD")
	}
	record := &models.SubjectAttendance{
		EnrollmentID: req.EnrollmentID,
		ScheduleID:   req.ScheduleID,
		Date:         date,
		Status:       models.AttendanceStatus(strings.ToUpper(req.Status)),
		Notes:        req.Notes,
	}
	stored, err := s.subjectRepo.Upsert(ctx, record)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to mark subject attendance")
	}
	return stored, nil
}

// BulkMarkSubject handles bulk session attendance writes.
func (s *AttendanceService) BulkMarkSubject(ctx context.Context, req BulkMarkSubjectAttendanceRequest) (*BulkAttendanceResult, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid payload")
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "invalid date format, expected YYYY-MM-DD")
	}
	mode := models.BulkOperationMode(strings.ToLower(req.Mode))
	seen := map[string]struct{}{}
	records := make([]models.SubjectAttendance, len(req.Items))
	for i, item := range req.Items {
		key := fmt.Sprintf("%s|%s|%s", item.EnrollmentID, req.ScheduleID, date.Format("2006-01-02"))
		if _, ok := seen[key]; ok {
			return nil, appErrors.Clone(appErrors.ErrConflict, "duplicate enrollment in payload")
		}
		seen[key] = struct{}{}
		records[i] = models.SubjectAttendance{
			EnrollmentID: item.EnrollmentID,
			ScheduleID:   req.ScheduleID,
			Date:         date,
			Status:       models.AttendanceStatus(strings.ToUpper(item.Status)),
			Notes:        item.Notes,
		}
	}
	conflicts, err := s.subjectRepo.BulkInsert(ctx, records, mode == models.BulkModeAtomic)
	if err != nil {
		if mode == models.BulkModeAtomic {
			return nil, appErrors.Wrap(err, appErrors.ErrConflict.Code, appErrors.ErrConflict.Status, err.Error())
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "bulk mark failed")
	}
	result := &BulkAttendanceResult{Processed: len(records), Success: len(records) - len(conflicts)}
	if len(conflicts) > 0 {
		result.Conflicts = make([]models.AttendanceBulkConflict, len(conflicts))
		for i, conflict := range conflicts {
			scheduleID := conflict.ScheduleID
			result.Conflicts[i] = models.AttendanceBulkConflict{
				EnrollmentID: conflict.EnrollmentID,
				ScheduleID:   &scheduleID,
				Date:         conflict.Date,
				Reason:       "duplicate record",
			}
		}
	}
	return result, nil
}

// SubjectSessionReport returns session report rows.
func (s *AttendanceService) SubjectSessionReport(ctx context.Context, scheduleID string, date time.Time) ([]models.SubjectAttendanceReportRow, error) {
	rows, err := s.subjectRepo.SessionReport(ctx, scheduleID, date)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load session report")
	}
	return rows, nil
}

// ValidateStudentAccess ensures student data exists (placeholder for RBAC hooks).
func (s *AttendanceService) ValidateStudentAccess(_ context.Context, studentID string) error {
	if studentID == "" {
		return appErrors.Clone(appErrors.ErrValidation, "student id required")
	}
	return nil
}

// CheckEnrollmentExists ensures repository operations return expected rows.
func (s *AttendanceService) CheckEnrollmentExists(err error) error {
	if err == nil {
		return nil
	}
	if err == sql.ErrNoRows {
		return appErrors.Clone(appErrors.ErrNotFound, "enrollment not found")
	}
	return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "unexpected repository error")
}
