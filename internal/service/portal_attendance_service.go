package service

import (
	"context"
	"database/sql"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// PortalAttendanceService provides attendance data for parent/student portal.
type PortalAttendanceService struct {
	dailyAttendanceRepo dailyAttendanceReader
	enrollmentRepo      enrollmentReader
	studentRepo         studentReader
	validator           *validator.Validate
	logger              *zap.Logger
}

// NewPortalAttendanceService constructs the portal attendance service.
func NewPortalAttendanceService(
	dailyAttendanceRepo dailyAttendanceReader,
	enrollmentRepo enrollmentReader,
	studentRepo studentReader,
	validate *validator.Validate,
	logger *zap.Logger,
) *PortalAttendanceService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PortalAttendanceService{
		dailyAttendanceRepo: dailyAttendanceRepo,
		enrollmentRepo:      enrollmentRepo,
		studentRepo:         studentRepo,
		validator:           validate,
		logger:              logger,
	}
}

// GetAttendance returns attendance for a student in a term.
func (s *PortalAttendanceService) GetAttendance(ctx context.Context, req models.PortalAttendanceRequest) (*models.PortalAttendanceResponse, error) {
	// Get student detail
	_, err := s.studentRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch student")
	}

	// Get enrollments for student in term (to get class info)
	_, err = s.enrollmentRepo.ListByStudentAndTerm(ctx, req.StudentID, req.TermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch enrollments")
	}

	// Get daily attendance records
	dailyRecords, err := s.dailyAttendanceRepo.ListByStudentAndTerm(ctx, req.StudentID, req.TermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch daily attendance")
	}

	// Get attendance summary
	summary, err := s.dailyAttendanceRepo.StudentSummary(ctx, req.StudentID, req.TermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch attendance summary")
	}

	// Build daily attendance response
	daily := make([]*models.PortalDailyAttendance, len(dailyRecords))
	for i, r := range dailyRecords {
		daily[i] = &models.PortalDailyAttendance{
			ID:     r.ID,
			Date:   r.Date.Format("2006-01-02"),
			Status: string(r.Status),
			Notes:  r.Notes,
		}
	}

	// Build subject attendance response (placeholder - would need subject attendance repo)
	subject := []*models.PortalSubjectAttendance{}

	return &models.PortalAttendanceResponse{
		StudentID: req.StudentID,
		TermID:    req.TermID,
		Daily:     daily,
		Subject:   subject,
		Summary: models.PortalAttendanceSummary{
			TotalDays:  summary.Total,
			Present:    summary.Present,
			Sick:       summary.Sick,
			Permission: summary.Excused,
			Absent:     summary.Absent,
			Percentage: summary.Percent,
		},
	}, nil
}

// GetAttendanceStats returns attendance statistics for a student in a term.
func (s *PortalAttendanceService) GetAttendanceStats(ctx context.Context, studentID, termID string) (*models.PortalAttendanceSummary, error) {
	summary, err := s.dailyAttendanceRepo.StudentSummary(ctx, studentID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch attendance summary")
	}

	return &models.PortalAttendanceSummary{
		TotalDays:  summary.Total,
		Present:    summary.Present,
		Sick:       summary.Sick,
		Permission: summary.Excused,
		Absent:     summary.Absent,
		Percentage: summary.Percent,
	}, nil
}
