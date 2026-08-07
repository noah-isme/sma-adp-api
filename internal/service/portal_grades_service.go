package service

import (
	"context"
	"database/sql"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// PortalGradesService provides grades data for parent/student portal.
type PortalGradesService struct {
	enrollmentRepo enrollmentReader
	gradeFinalRepo gradeFinalRepo
	studentRepo    studentReader
	validator      *validator.Validate
	logger         *zap.Logger
}

// NewPortalGradesService constructs the portal grades service.
func NewPortalGradesService(
	enrollmentRepo enrollmentReader,
	gradeFinalRepo gradeFinalRepo,
	studentRepo studentReader,
	validate *validator.Validate,
	logger *zap.Logger,
) *PortalGradesService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PortalGradesService{
		enrollmentRepo: enrollmentRepo,
		gradeFinalRepo: gradeFinalRepo,
		studentRepo:    studentRepo,
		validator:      validate,
		logger:         logger,
	}
}

// GetGrades returns grades for a student in a term.
// For parents: requires permission check on parent_students link.
// For students: uses their own student_id.
func (s *PortalGradesService) GetGrades(ctx context.Context, req models.PortalGradesRequest) (*models.PortalGradesResponse, error) {
	// Get student detail
	_, err := s.studentRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch student")
	}

	// Get enrollments for student in term
	enrollments, err := s.enrollmentRepo.ListByStudentAndTerm(ctx, req.StudentID, req.TermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch enrollments")
	}

	if len(enrollments) == 0 {
		// Return empty response
		return &models.PortalGradesResponse{
			TermID: req.TermID,
			Grades: []*models.PortalGrade{},
			Summary: &models.GradesSummary{
				GPA:            0,
				TotalSubjects:  0,
				PassedSubjects: 0,
				FailedSubjects: 0,
			},
		}, nil
	}

	// Collect enrollment IDs
	enrollmentIDs := make([]string, len(enrollments))
	for i, e := range enrollments {
		enrollmentIDs[i] = e.ID
	}

	// If subjectID specified, fetch only that subject
	var finalGrades map[string]models.GradeFinal
	if req.SubjectID != "" {
		finalGrades, err = s.gradeFinalRepo.FetchByEnrollments(ctx, enrollmentIDs, req.SubjectID)
		if err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch final grades")
		}
	} else {
		// Fetch all final grades for these enrollments
		allFinals, err := s.gradeFinalRepo.ListByStudentAndTerm(ctx, req.StudentID, req.TermID)
		if err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch final grades")
		}
		finalGrades = make(map[string]models.GradeFinal, len(allFinals))
		for _, fg := range allFinals {
			finalGrades[fg.EnrollmentID+"|"+fg.SubjectID] = fg
		}
	}

	// Build portal grades
	portalGrades := make([]*models.PortalGrade, 0, len(enrollments))
	var totalGrade float64
	passed := 0
	failed := 0

	for _, enrollment := range enrollments {
		key := enrollment.ID
		if req.SubjectID != "" {
			key = enrollment.ID + "|" + req.SubjectID
		}

		finalGrade, ok := finalGrades[key]
		if !ok {
			continue // Skip subjects without final grades
		}

		// Determine letter grade and pass/fail
		letterGrade := s.letterGrade(finalGrade.FinalGrade)
		isPassed := finalGrade.FinalGrade >= 70 // Assuming 70 is passing threshold

		if isPassed {
			passed++
		} else {
			failed++
		}
		totalGrade += finalGrade.FinalGrade
		subjectID := ""
		if enrollment.SubjectID != nil {
			subjectID = *enrollment.SubjectID
		}

		subjectName := ""
		if enrollment.SubjectName != nil {
			subjectName = *enrollment.SubjectName
		}
		subjectCode := ""
		if enrollment.SubjectCode != nil {
			subjectCode = *enrollment.SubjectCode
		}

		portalGrades = append(portalGrades, &models.PortalGrade{
			StudentID:    req.StudentID,
			EnrollmentID: enrollment.ID,
			SubjectID:    subjectID,
			SubjectName:  subjectName,
			SubjectCode:  subjectCode,
			ClassName:    enrollment.ClassName,
			ComponentGrades: map[string]float64{
				"final": finalGrade.FinalGrade,
			},
			FinalGrade:  finalGrade.FinalGrade,
			LetterGrade: letterGrade,
			IsPassed:    isPassed,
			TeacherName: enrollment.TeacherName,
		})
	}

	gpa := 0.0
	if len(portalGrades) > 0 {
		gpa = totalGrade / float64(len(portalGrades))
	}

	return &models.PortalGradesResponse{
		TermID: req.TermID,
		Grades: portalGrades,
		Summary: &models.GradesSummary{
			GPA:              gpa,
			TotalSubjects:    len(portalGrades),
			PassedSubjects:   passed,
			FailedSubjects:   failed,
		},
	}, nil
}

// letterGrade converts numeric grade to letter grade
func (s *PortalGradesService) letterGrade(grade float64) string {
	switch {
	case grade >= 85:
		return "A"
	case grade >= 75:
		return "B+"
	case grade >= 70:
		return "B"
	case grade >= 65:
		return "C+"
	case grade >= 60:
		return "C"
	case grade >= 55:
		return "D"
	default:
		return "E"
	}
}

// GetReportCard returns full report card for a student in a term.
func (s *PortalGradesService) GetReportCard(ctx context.Context, studentID, termID string) (*models.PortalReportCardResponse, error) {
	// Get student detail
	_, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "student not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch student")
	}

	// Get enrollments for student in term
	enrollments, err := s.enrollmentRepo.ListByStudentAndTerm(ctx, studentID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch enrollments")
	}

	if len(enrollments) == 0 {
		// Return empty response
		return &models.PortalReportCardResponse{
			StudentID: studentID,
			TermID:    termID,
			Subjects:  []*models.PortalReportCardSubject{},
			Summary: &models.GradesSummary{
				GPA:            0,
				TotalSubjects:  0,
				PassedSubjects: 0,
				FailedSubjects: 0,
			},
		}, nil
	}

	// Collect enrollment IDs
	enrollmentIDs := make([]string, len(enrollments))
	for i, e := range enrollments {
		enrollmentIDs[i] = e.ID
	}

	// Fetch all final grades for these enrollments
	allFinals, err := s.gradeFinalRepo.ListByStudentAndTerm(ctx, studentID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch final grades")
	}
	finalGrades := make(map[string]models.GradeFinal, len(allFinals))
	for _, fg := range allFinals {
		finalGrades[fg.EnrollmentID+"|"+fg.SubjectID] = fg
	}

	// Build portal report card subjects
	portalSubjects := make([]*models.PortalReportCardSubject, 0, len(enrollments))
	var totalGrade float64
	passed := 0
	failed := 0

	for _, enrollment := range enrollments {
		subjectID := ""
		if enrollment.SubjectID != nil {
			subjectID = *enrollment.SubjectID
		}
		key := enrollment.ID + "|" + subjectID
		finalGrade, ok := finalGrades[key]
		if !ok {
			continue // Skip subjects without final grades
		}

		// Determine letter grade and pass/fail
		letterGrade := s.letterGrade(finalGrade.FinalGrade)
		isPassed := finalGrade.FinalGrade >= 70 // Assuming 70 is passing threshold

		if isPassed {
			passed++
		} else {
			failed++
		}
		totalGrade += finalGrade.FinalGrade

		subjectName := ""
		if enrollment.SubjectName != nil {
			subjectName = *enrollment.SubjectName
		}
		subjectCode := ""
		if enrollment.SubjectCode != nil {
			subjectCode = *enrollment.SubjectCode
		}

		portalSubjects = append(portalSubjects, &models.PortalReportCardSubject{
			SubjectID:   subjectID,
			SubjectName: subjectName,
			SubjectCode: subjectCode,
			FinalGrade:  finalGrade.FinalGrade,
			LetterGrade: letterGrade,
			IsPassed:    isPassed,
			TeacherName: enrollment.TeacherName,
		})
	}

	gpa := 0.0
	if len(portalSubjects) > 0 {
		gpa = totalGrade / float64(len(portalSubjects))
	}

	return &models.PortalReportCardResponse{
		StudentID: studentID,
		TermID:    termID,
		Subjects:  portalSubjects,
		Summary: &models.GradesSummary{
			GPA:              gpa,
			TotalSubjects:    len(portalSubjects),
			PassedSubjects:   passed,
			FailedSubjects:   failed,
		},
	}, nil
}