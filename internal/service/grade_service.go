package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type gradeRepo interface {
	List(ctx context.Context, filter models.GradeFilter) ([]models.Grade, error)
	Upsert(ctx context.Context, grade *models.Grade) error
	BulkUpsert(ctx context.Context, grades []models.Grade) error
	FetchByEnrollments(ctx context.Context, enrollmentIDs []string, subjectID string) (map[string][]models.Grade, error)
}

type gradeFinalRepo interface {
	Upsert(ctx context.Context, finals []models.GradeFinal) error
	SetFinalized(ctx context.Context, enrollmentIDs []string, subjectID string, finalized bool) error
	FetchByEnrollments(ctx context.Context, enrollmentIDs []string, subjectID string) (map[string]models.GradeFinal, error)
	ReportCard(ctx context.Context, studentID, termID string) ([]models.GradeReportSubject, error)
	ClassReportRows(ctx context.Context, classID, subjectID, termID string) ([]models.GradeFinalReportRow, error)
	ClassDistribution(ctx context.Context, classID, subjectID, termID string) (*models.ClassGradeDistribution, error)
}

type enrollmentReader interface {
	FindByID(ctx context.Context, id string) (*models.Enrollment, error)
	ListByClassAndTerm(ctx context.Context, classID, termID string) ([]models.Enrollment, error)
}

type gradeConfigReader interface {
	FindByScope(ctx context.Context, classID, subjectID, termID string) (*models.GradeConfig, error)
}

type gradeComponentFetcher interface {
	FindByCode(ctx context.Context, code string) (*models.GradeComponent, error)
	FindByID(ctx context.Context, id string) (*models.GradeComponent, error)
}

// UpsertGradeRequest represents a single grade entry payload.
type UpsertGradeRequest struct {
	EnrollmentID  string  `json:"enrollment_id" validate:"required"`
	SubjectID     string  `json:"subject_id" validate:"required"`
	ComponentID   string  `json:"component_id"`
	ComponentCode string  `json:"component_code"`
	GradeValue    float64 `json:"grade_value" validate:"required"`
}

// BulkGradeItem represents grade info within bulk payload.
type BulkGradeItem struct {
	EnrollmentID  string  `json:"enrollment_id" validate:"required"`
	ComponentID   string  `json:"component_id"`
	ComponentCode string  `json:"component_code"`
	GradeValue    float64 `json:"grade_value" validate:"required"`
}

// BulkGradesRequest handles atomic or partial grade uploads.
type BulkGradesRequest struct {
	ClassID   string          `json:"class_id" validate:"required"`
	SubjectID string          `json:"subject_id" validate:"required"`
	TermID    string          `json:"term_id" validate:"required"`
	Mode      string          `json:"mode" validate:"omitempty,oneof=atomic partialOnError"`
	Items     []BulkGradeItem `json:"items" validate:"required,dive"`
}

// BulkGradesResult summarises partial outcomes.
type BulkGradesResult struct {
	SuccessCount int                `json:"success_count"`
	Failures     []BulkGradeFailure `json:"failures,omitempty"`
}

// BulkGradeFailure captures failed grade entries.
type BulkGradeFailure struct {
	EnrollmentID string `json:"enrollment_id"`
	Component    string `json:"component"`
	Reason       string `json:"reason"`
}

// FinalizeGradesRequest finalizes grades for a scope.
type FinalizeGradesRequest struct {
	ClassID   string `json:"class_id" validate:"required"`
	SubjectID string `json:"subject_id" validate:"required"`
	TermID    string `json:"term_id" validate:"required"`
}

// GradeService orchestrates grade entry and calculation flows.
type GradeService struct {
	grades       gradeRepo
	finals       gradeFinalRepo
	enrollments  enrollmentReader
	configs      gradeConfigReader
	components   gradeComponentFetcher
	validator    *validator.Validate
	logger       *zap.Logger
	roundingMode func(float64) float64
}

// NewGradeService constructs GradeService.
func NewGradeService(grades gradeRepo, finals gradeFinalRepo, enrollments enrollmentReader, configs gradeConfigReader, components gradeComponentFetcher, validate *validator.Validate, logger *zap.Logger) *GradeService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &GradeService{
		grades:       grades,
		finals:       finals,
		enrollments:  enrollments,
		configs:      configs,
		components:   components,
		validator:    validate,
		logger:       logger,
		roundingMode: func(v float64) float64 { return math.RoundToEven(v*100) / 100 },
	}
}

// List returns grade entries.
func (s *GradeService) List(ctx context.Context, filter models.GradeFilter) ([]models.Grade, error) {
	grades, err := s.grades.List(ctx, filter)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list grades")
	}
	return grades, nil
}

// Upsert handles single grade entry.
func (s *GradeService) Upsert(ctx context.Context, req UpsertGradeRequest) (*models.Grade, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid grade payload")
	}
	if req.ComponentID == "" && req.ComponentCode == "" {
		return nil, appErrors.Clone(appErrors.ErrValidation, "component identifier required")
	}
	enrollment, err := s.enrollments.FindByID(ctx, req.EnrollmentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "enrollment not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load enrollment")
	}
	config, err := s.configs.FindByScope(ctx, enrollment.ClassID, req.SubjectID, enrollment.TermID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, "grade config missing")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade config")
	}
	if config.Finalized {
		return nil, appErrors.Clone(appErrors.ErrFinalized, "grade config finalized")
	}
	componentID, err := s.resolveComponent(ctx, config, req.ComponentID, req.ComponentCode)
	if err != nil {
		return nil, err
	}
	finals, err := s.finals.FetchByEnrollments(ctx, []string{req.EnrollmentID}, req.SubjectID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to inspect final grade")
	}
	if final, ok := finals[req.EnrollmentID]; ok && final.Finalized {
		return nil, appErrors.Clone(appErrors.ErrFinalized, "final grade already finalized")
	}
	grade := &models.Grade{EnrollmentID: req.EnrollmentID, SubjectID: req.SubjectID, ComponentID: componentID, GradeValue: req.GradeValue}
	if err := s.grades.Upsert(ctx, grade); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to upsert grade")
	}
	if err := s.recalculate(ctx, config, []models.Enrollment{*enrollment}); err != nil {
		return nil, err
	}
	grades, err := s.grades.List(ctx, models.GradeFilter{EnrollmentID: req.EnrollmentID, SubjectID: req.SubjectID, ComponentID: componentID})
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade")
	}
	if len(grades) == 0 {
		return grade, nil
	}
	return &grades[0], nil
}

// BulkUpsert handles bulk grade submissions.
func (s *GradeService) BulkUpsert(ctx context.Context, req BulkGradesRequest) (*BulkGradesResult, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid bulk payload")
	}
	config, err := s.configs.FindByScope(ctx, req.ClassID, req.SubjectID, req.TermID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, "grade config missing")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade config")
	}
	if config.Finalized {
		return nil, appErrors.Clone(appErrors.ErrFinalized, "grade config finalized")
	}
	enrollmentMap := make(map[string]*models.Enrollment)
	for _, item := range req.Items {
		if _, ok := enrollmentMap[item.EnrollmentID]; ok {
			continue
		}
		enrollment, err := s.enrollments.FindByID(ctx, item.EnrollmentID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, appErrors.Clone(appErrors.ErrNotFound, fmt.Sprintf("enrollment %s not found", item.EnrollmentID))
			}
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load enrollment")
		}
		if enrollment.ClassID != req.ClassID || enrollment.TermID != req.TermID {
			return nil, appErrors.Clone(appErrors.ErrValidation, fmt.Sprintf("enrollment %s not in scope", item.EnrollmentID))
		}
		enrollmentMap[item.EnrollmentID] = enrollment
	}
	finals, err := s.finals.FetchByEnrollments(ctx, keys(enrollmentMap), req.SubjectID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check finals")
	}
	for id, final := range finals {
		if final.Finalized {
			return nil, appErrors.Clone(appErrors.ErrFinalized, fmt.Sprintf("final grade finalized for enrollment %s", id))
		}
	}
	items := req.Items
	atomic := req.Mode == "" || req.Mode == "atomic"
	result := &BulkGradesResult{}
	var gradesToUpsert []models.Grade
	var recalculationEnrollments []models.Enrollment
	for _, item := range items {
		componentID, err := s.resolveComponent(ctx, config, item.ComponentID, item.ComponentCode)
		if err != nil {
			if atomic {
				return nil, err
			}
			result.Failures = append(result.Failures, BulkGradeFailure{EnrollmentID: item.EnrollmentID, Component: componentLabel(item), Reason: err.Error()})
			continue
		}
		enrollment := enrollmentMap[item.EnrollmentID]
		if enrollment == nil {
			if atomic {
				return nil, appErrors.Clone(appErrors.ErrValidation, fmt.Sprintf("enrollment %s missing", item.EnrollmentID))
			}
			result.Failures = append(result.Failures, BulkGradeFailure{EnrollmentID: item.EnrollmentID, Component: componentLabel(item), Reason: "enrollment missing"})
			continue
		}
		grade := models.Grade{EnrollmentID: item.EnrollmentID, SubjectID: req.SubjectID, ComponentID: componentID, GradeValue: item.GradeValue}
		if atomic {
			gradesToUpsert = append(gradesToUpsert, grade)
		} else {
			if err := s.grades.Upsert(ctx, &grade); err != nil {
				result.Failures = append(result.Failures, BulkGradeFailure{EnrollmentID: item.EnrollmentID, Component: componentLabel(item), Reason: err.Error()})
				continue
			}
			result.SuccessCount++
			recalculationEnrollments = append(recalculationEnrollments, *enrollment)
		}
	}
	if atomic {
		if err := s.grades.BulkUpsert(ctx, gradesToUpsert); err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to bulk upsert grades")
		}
		for _, enrollment := range enrollmentMap {
			recalculationEnrollments = append(recalculationEnrollments, *enrollment)
		}
		result.SuccessCount = len(gradesToUpsert)
	}
	if err := s.recalculate(ctx, config, dedupeEnrollments(recalculationEnrollments)); err != nil {
		return nil, err
	}
	return result, nil
}

// Recalculate recomputes final grades for class/subject/term scope.
func (s *GradeService) Recalculate(ctx context.Context, filter models.FinalGradeFilter) error {
	if filter.ClassID == "" || filter.SubjectID == "" || filter.TermID == "" {
		return appErrors.Clone(appErrors.ErrValidation, "class, subject and term required")
	}
	config, err := s.configs.FindByScope(ctx, filter.ClassID, filter.SubjectID, filter.TermID)
	if err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrPreconditionFailed, "grade config missing")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade config")
	}
	if config.Finalized {
		return appErrors.Clone(appErrors.ErrFinalized, "grade config finalized")
	}
	enrollments, err := s.enrollments.ListByClassAndTerm(ctx, filter.ClassID, filter.TermID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list enrollments")
	}
	return s.recalculate(ctx, config, enrollments)
}

// Finalize locks final grades for scope.
func (s *GradeService) Finalize(ctx context.Context, req FinalizeGradesRequest) error {
	if err := s.validator.Struct(req); err != nil {
		return appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid finalize payload")
	}
	config, err := s.configs.FindByScope(ctx, req.ClassID, req.SubjectID, req.TermID)
	if err != nil {
		if err == sql.ErrNoRows {
			return appErrors.Clone(appErrors.ErrPreconditionFailed, "grade config missing")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load grade config")
	}
	enrollments, err := s.enrollments.ListByClassAndTerm(ctx, req.ClassID, req.TermID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list enrollments")
	}
	if err := s.recalculate(ctx, config, enrollments); err != nil {
		return err
	}
	if err := s.finals.SetFinalized(ctx, extractIDs(enrollments), req.SubjectID, true); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to finalize finals")
	}
	return nil
}

// ReportCard returns student report card.
func (s *GradeService) ReportCard(ctx context.Context, studentID, termID string) (*models.StudentReportCard, error) {
	subjects, err := s.finals.ReportCard(ctx, studentID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load report card")
	}
	return &models.StudentReportCard{StudentID: studentID, TermID: termID, Subjects: subjects}, nil
}

// ClassReport returns aggregated class grade report.
func (s *GradeService) ClassReport(ctx context.Context, classID, subjectID, termID string) (*models.ClassGradeReport, error) {
	rows, err := s.finals.ClassReportRows(ctx, classID, subjectID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class grades")
	}
	distribution, err := s.finals.ClassDistribution(ctx, classID, subjectID, termID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to aggregate class grades")
	}
	return &models.ClassGradeReport{ClassID: classID, SubjectID: subjectID, TermID: termID, Students: rows, Distribution: distribution}, nil
}

func (s *GradeService) resolveComponent(ctx context.Context, config *models.GradeConfig, componentID, componentCode string) (string, error) {
	if componentID != "" {
		for _, comp := range config.Components {
			if comp.ComponentID == componentID {
				return componentID, nil
			}
		}
		return "", appErrors.Clone(appErrors.ErrValidation, "component not part of configuration")
	}
	componentCode = strings.TrimSpace(strings.ToUpper(componentCode))
	for _, comp := range config.Components {
		if strings.ToUpper(comp.ComponentCode) == componentCode {
			return comp.ComponentID, nil
		}
	}
	component, err := s.components.FindByCode(ctx, componentCode)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", appErrors.Clone(appErrors.ErrValidation, "unknown component code")
		}
		return "", appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to resolve component")
	}
	for _, comp := range config.Components {
		if comp.ComponentID == component.ID {
			return comp.ComponentID, nil
		}
	}
	return "", appErrors.Clone(appErrors.ErrValidation, "component not part of configuration")
}

func (s *GradeService) recalculate(ctx context.Context, config *models.GradeConfig, enrollments []models.Enrollment) error {
	if len(enrollments) == 0 {
		return nil
	}
	enrollmentIDs := extractIDs(enrollments)
	grades, err := s.grades.FetchByEnrollments(ctx, enrollmentIDs, config.SubjectID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch grades")
	}
	existingFinals, err := s.finals.FetchByEnrollments(ctx, enrollmentIDs, config.SubjectID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to fetch finals")
	}
	finals := make([]models.GradeFinal, 0, len(enrollments))
	for _, enrollment := range enrollments {
		final, ok := existingFinals[enrollment.ID]
		if ok && final.Finalized {
			continue
		}
		calculated, note := s.calculateFinal(config, grades[enrollment.ID])
		finals = append(finals, models.GradeFinal{
			ID:              final.ID,
			EnrollmentID:    enrollment.ID,
			SubjectID:       config.SubjectID,
			FinalGrade:      calculated,
			Finalized:       false,
			CalculatedAt:    time.Now().UTC(),
			CalculationNote: note,
		})
	}
	if len(finals) == 0 {
		return nil
	}
	if err := s.finals.Upsert(ctx, finals); err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to upsert final grades")
	}
	return nil
}

func (s *GradeService) calculateFinal(config *models.GradeConfig, grades []models.Grade) (float64, string) {
	if len(grades) == 0 {
		return 0, "no grades recorded"
	}
	switch config.CalculationScheme {
	case models.GradeSchemeWeighted:
		weightMap := make(map[string]float64, len(config.Components))
		for _, comp := range config.Components {
			weightMap[comp.ComponentID] = comp.Weight
		}
		totalWeight := 0.0
		sum := 0.0
		for _, grade := range grades {
			weight, ok := weightMap[grade.ComponentID]
			if !ok {
				continue
			}
			totalWeight += weight
			sum += grade.GradeValue * weight
		}
		if totalWeight == 0 {
			return 0, "weights missing"
		}
		return s.roundingMode(sum / 100), "weighted"
	case models.GradeSchemeAverage:
		sum := 0.0
		for _, grade := range grades {
			sum += grade.GradeValue
		}
		avg := sum / float64(len(grades))
		return s.roundingMode(avg), "average"
	default:
		return 0, "scheme unsupported"
	}
}

func componentLabel(item BulkGradeItem) string {
	if item.ComponentCode != "" {
		return item.ComponentCode
	}
	return item.ComponentID
}

func extractIDs(enrollments []models.Enrollment) []string {
	ids := make([]string, 0, len(enrollments))
	for _, enrollment := range enrollments {
		ids = append(ids, enrollment.ID)
	}
	return ids
}

func keys(m map[string]*models.Enrollment) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}

func dedupeEnrollments(enrollments []models.Enrollment) []models.Enrollment {
	seen := make(map[string]bool, len(enrollments))
	unique := make([]models.Enrollment, 0, len(enrollments))
	for _, enrollment := range enrollments {
		if !seen[enrollment.ID] {
			unique = append(unique, enrollment)
			seen[enrollment.ID] = true
		}
	}
	return unique
}
