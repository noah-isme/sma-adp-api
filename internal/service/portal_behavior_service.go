package service

import (
	"context"
	"database/sql"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// PortalBehaviorService provides behavior data for parent/student portal.
type PortalBehaviorService struct {
	behaviorRepo   behaviorReader
	enrollmentRepo enrollmentReader
	studentRepo    studentReader
	validator      *validator.Validate
	logger         *zap.Logger
}

// NewPortalBehaviorService constructs the portal behavior service.
func NewPortalBehaviorService(
	behaviorRepo behaviorReader,
	enrollmentRepo enrollmentReader,
	studentRepo studentReader,
	validate *validator.Validate,
	logger *zap.Logger,
) *PortalBehaviorService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PortalBehaviorService{
		behaviorRepo:   behaviorRepo,
		enrollmentRepo: enrollmentRepo,
		studentRepo:    studentRepo,
		validator:      validate,
		logger:         logger,
	}
}

// GetBehaviorNotes returns behavior notes for a student in a term.
func (s *PortalBehaviorService) GetBehaviorNotes(ctx context.Context, req models.PortalBehaviorRequest) (*models.PortalBehaviorResponse, error) {
	// Get student detail
	_, err := s.studentRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch student")
	}

	// Get behavior notes
	notes, err := s.behaviorRepo.ListByStudentAndTerm(ctx, req.StudentID, req.TermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch behavior notes")
	}

	// Get behavior summary
	summary, err := s.behaviorRepo.Summary(ctx, req.StudentID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch behavior summary")
	}

	// Build response
	items := make([]*models.PortalBehaviorNote, len(notes))
	for i, n := range notes {
		items[i] = &models.PortalBehaviorNote{
			ID:           n.ID,
			Date:         n.NoteDate.Format("2006-01-02"),
			Category:     string(n.NoteType),
			Points:       n.Points,
			Description:  n.Description,
			ReporterName: &n.CreatedBy,
		}
	}

	return &models.PortalBehaviorResponse{
		StudentID: req.StudentID,
		TermID:    req.TermID,
		Notes:     items,
		Summary: models.PortalBehaviorSummary{
			TotalPoints:   summary.TotalPoints,
			PositiveNotes: summary.PositiveCount,
			NegativeNotes: summary.NegativeCount,
			NeutralNotes:  summary.NeutralCount,
			TotalNotes:    summary.PositiveCount + summary.NegativeCount + summary.NeutralCount,
		},
	}, nil
}

// GetBehaviorSummary returns behavior summary for a student.
func (s *PortalBehaviorService) GetBehaviorSummary(ctx context.Context, studentID string) (*models.PortalBehaviorSummary, error) {
	summary, err := s.behaviorRepo.Summary(ctx, studentID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch behavior summary")
	}

	return &models.PortalBehaviorSummary{
		TotalPoints:   summary.TotalPoints,
		PositiveNotes: summary.PositiveCount,
		NegativeNotes: summary.NegativeCount,
		NeutralNotes:  summary.NeutralCount,
		TotalNotes:    summary.PositiveCount + summary.NegativeCount + summary.NeutralCount,
	}, nil
}
