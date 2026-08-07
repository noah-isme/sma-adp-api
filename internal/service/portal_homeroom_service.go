package service

import (
	"context"
	"database/sql"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// portalTermReader interface for resolving active term
type portalTermReader interface {
	FindActive(ctx context.Context) (*models.Term, error)
	FindByID(ctx context.Context, id string) (*models.Term, error)
}

// PortalHomeroomService provides homeroom data for parent/student portal.
type PortalHomeroomService struct {
	homeroomRepo   homeroomStore
	enrollmentRepo enrollmentReader
	studentRepo    studentReader
	termRepo       portalTermReader
	validator      *validator.Validate
	logger         *zap.Logger
}

// NewPortalHomeroomService constructs the portal homeroom service.
func NewPortalHomeroomService(
	homeroomRepo homeroomStore,
	enrollmentRepo enrollmentReader,
	studentRepo studentReader,
	termRepo portalTermReader,
	validate *validator.Validate,
	logger *zap.Logger,
) *PortalHomeroomService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PortalHomeroomService{
		homeroomRepo:   homeroomRepo,
		enrollmentRepo: enrollmentRepo,
		studentRepo:    studentRepo,
		termRepo:       termRepo,
		validator:      validate,
		logger:         logger,
	}
}

// PortalHomeroomRequest represents a request for homeroom data.
type PortalHomeroomRequest struct {
	StudentID string
	TermID    string
}

// PortalHomeroomResponse represents homeroom data for a student.
type PortalHomeroomResponse struct {
	StudentID      string             `json:"studentId"`
	StudentName    string             `json:"studentName"`
	TermID         string             `json:"termId"`
	TermName       string             `json:"termName"`
	ClassID        string             `json:"classId"`
	ClassName      string             `json:"className"`
	HomeroomTeacher *HomeroomTeacher  `json:"homeroomTeacher,omitempty"`
}

// HomeroomTeacher represents homeroom teacher information.
type HomeroomTeacher struct {
	ID   string  `json:"id"`
	Name *string `json:"name,omitempty"`
}

// GetHomeroom returns homeroom information for a student in a term.
// For parents: requires permission check on parent_students link.
// For students: uses their own student_id.
func (s *PortalHomeroomService) GetHomeroom(ctx context.Context, req PortalHomeroomRequest) (*PortalHomeroomResponse, error) {
	// Get student detail
	student, err := s.studentRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch student")
	}

	// Resolve term ID - if empty, use active term
	termID := req.TermID
	if termID == "" {
		activeTerm, err := s.termRepo.FindActive(ctx)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, appErrors.Clone(appErrors.ErrNotFound, "active term not found")
			}
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch active term")
		}
		termID = activeTerm.ID
	}

	// Get enrollment for student in term to find class
	enrollments, err := s.enrollmentRepo.ListByStudentAndTerm(ctx, req.StudentID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch enrollments")
	}

	if len(enrollments) == 0 {
		return nil, appErrors.Clone(appErrors.ErrNotFound, "no enrollment found for student in term")
	}

	// Use the first enrollment's class (student typically has one primary class)
	enrollment := enrollments[0]
	classID := enrollment.ClassID
	termName := enrollment.TermName
	className := enrollment.ClassName

	// Get homeroom info
	homeroom, err := s.homeroomRepo.Get(ctx, classID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch homeroom")
	}

	resp := &PortalHomeroomResponse{
		StudentID:   req.StudentID,
		StudentName: student.FullName,
		TermID:      termID,
		TermName:    termName,
		ClassID:     classID,
		ClassName:   className,
	}

	if homeroom != nil && homeroom.HomeroomTeacherID != nil {
		teacherName := homeroom.HomeroomTeacherName
		resp.HomeroomTeacher = &HomeroomTeacher{
			ID:   *homeroom.HomeroomTeacherID,
			Name: teacherName,
		}
	}

	return resp, nil
}

// GetHomeroomByClass returns homeroom information for a specific class and term.
// This is useful for teachers/admins viewing a specific class.
func (s *PortalHomeroomService) GetHomeroomByClass(ctx context.Context, classID, termID string) (*PortalHomeroomResponse, error) {
	// Get homeroom info
	homeroom, err := s.homeroomRepo.Get(ctx, classID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch homeroom")
	}

	// Get class info
	// TODO: Use class reader interface to get class details

	resp := &PortalHomeroomResponse{
		ClassID:  classID,
		TermID:   termID,
	}

	if homeroom != nil {
		resp.ClassName = homeroom.ClassName
		resp.TermName = homeroom.TermName
		if homeroom.HomeroomTeacherID != nil {
			teacherName := homeroom.HomeroomTeacherName
			resp.HomeroomTeacher = &HomeroomTeacher{
				ID:   *homeroom.HomeroomTeacherID,
				Name: teacherName,
			}
		}
	}

	return resp, nil
}