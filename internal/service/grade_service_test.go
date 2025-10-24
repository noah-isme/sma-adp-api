package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

type mockGradeRepo struct {
	storedGrades map[string]models.Grade
}

func (m *mockGradeRepo) List(ctx context.Context, filter models.GradeFilter) ([]models.Grade, error) {
	var result []models.Grade
	for _, g := range m.storedGrades {
		if filter.EnrollmentID != "" && filter.EnrollmentID != g.EnrollmentID {
			continue
		}
		if filter.SubjectID != "" && filter.SubjectID != g.SubjectID {
			continue
		}
		if filter.ComponentID != "" && filter.ComponentID != g.ComponentID {
			continue
		}
		result = append(result, g)
	}
	return result, nil
}

func (m *mockGradeRepo) Upsert(ctx context.Context, grade *models.Grade) error {
	if m.storedGrades == nil {
		m.storedGrades = make(map[string]models.Grade)
	}
	key := grade.EnrollmentID + grade.ComponentID
	m.storedGrades[key] = *grade
	return nil
}

func (m *mockGradeRepo) BulkUpsert(ctx context.Context, grades []models.Grade) error {
	for i := range grades {
		_ = m.Upsert(ctx, &grades[i])
	}
	return nil
}

func (m *mockGradeRepo) FetchByEnrollments(ctx context.Context, enrollmentIDs []string, subjectID string) (map[string][]models.Grade, error) {
	result := make(map[string][]models.Grade)
	for _, grade := range m.storedGrades {
		if grade.SubjectID != subjectID {
			continue
		}
		result[grade.EnrollmentID] = append(result[grade.EnrollmentID], grade)
	}
	return result, nil
}

type mockGradeFinalRepo struct {
	finals      map[string]models.GradeFinal
	finalizedID []string
}

func (m *mockGradeFinalRepo) Upsert(ctx context.Context, finals []models.GradeFinal) error {
	if m.finals == nil {
		m.finals = make(map[string]models.GradeFinal)
	}
	for _, final := range finals {
		m.finals[final.EnrollmentID] = final
	}
	return nil
}

func (m *mockGradeFinalRepo) SetFinalized(ctx context.Context, enrollmentIDs []string, subjectID string, finalized bool) error {
	m.finalizedID = append(m.finalizedID, enrollmentIDs...)
	for _, id := range enrollmentIDs {
		if final, ok := m.finals[id]; ok {
			final.Finalized = finalized
			m.finals[id] = final
		}
	}
	return nil
}

func (m *mockGradeFinalRepo) FetchByEnrollments(ctx context.Context, enrollmentIDs []string, subjectID string) (map[string]models.GradeFinal, error) {
	finals := make(map[string]models.GradeFinal)
	for _, id := range enrollmentIDs {
		if final, ok := m.finals[id]; ok {
			finals[id] = final
		}
	}
	return finals, nil
}

func (m *mockGradeFinalRepo) ReportCard(ctx context.Context, studentID, termID string) ([]models.GradeReportSubject, error) {
	return []models.GradeReportSubject{{SubjectID: "sub", SubjectName: "Subject", FinalGrade: ptrFloat(80)}}, nil
}

func (m *mockGradeFinalRepo) ClassReportRows(ctx context.Context, classID, subjectID, termID string) ([]models.GradeFinalReportRow, error) {
	return []models.GradeFinalReportRow{{StudentID: "stu", StudentName: "Student", FinalGrade: ptrFloat(90)}}, nil
}

func (m *mockGradeFinalRepo) ClassDistribution(ctx context.Context, classID, subjectID, termID string) (*models.ClassGradeDistribution, error) {
	return &models.ClassGradeDistribution{SubjectID: subjectID, TermID: termID, Min: ptrFloat(70), Max: ptrFloat(95), Average: ptrFloat(85)}, nil
}

type mockEnrollmentReader struct {
	enrollments map[string]*models.Enrollment
}

func (m *mockEnrollmentReader) FindByID(ctx context.Context, id string) (*models.Enrollment, error) {
	if e, ok := m.enrollments[id]; ok {
		return e, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockEnrollmentReader) ListByClassAndTerm(ctx context.Context, classID, termID string) ([]models.Enrollment, error) {
	var list []models.Enrollment
	for _, e := range m.enrollments {
		if e.ClassID == classID && e.TermID == termID {
			list = append(list, *e)
		}
	}
	return list, nil
}

type mockConfigReader struct {
	config *models.GradeConfig
}

func (m *mockConfigReader) FindByScope(ctx context.Context, classID, subjectID, termID string) (*models.GradeConfig, error) {
	if m.config != nil && m.config.ClassID == classID && m.config.SubjectID == subjectID && m.config.TermID == termID {
		return m.config, nil
	}
	return nil, sql.ErrNoRows
}

type mockComponentFetcher struct {
	components map[string]*models.GradeComponent
}

func (m *mockComponentFetcher) FindByCode(ctx context.Context, code string) (*models.GradeComponent, error) {
	if c, ok := m.components[code]; ok {
		return c, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockComponentFetcher) FindByID(ctx context.Context, id string) (*models.GradeComponent, error) {
	for _, c := range m.components {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, sql.ErrNoRows
}

func ptrFloat(v float64) *float64 {
	return &v
}

func TestGradeServiceUpsert(t *testing.T) {
	gradeRepo := &mockGradeRepo{}
	finalRepo := &mockGradeFinalRepo{}
	enrollments := &mockEnrollmentReader{enrollments: map[string]*models.Enrollment{"en1": {ID: "en1", StudentID: "stu1", ClassID: "class", TermID: "term", Status: models.EnrollmentStatusActive}}}
	config := &models.GradeConfig{ID: "cfg", ClassID: "class", SubjectID: "sub", TermID: "term", CalculationScheme: models.GradeSchemeWeighted, Components: []models.GradeConfigComponent{{ComponentID: "comp1", Weight: 100, ComponentCode: "CODE"}}}
	configReader := &mockConfigReader{config: config}
	componentFetcher := &mockComponentFetcher{components: map[string]*models.GradeComponent{"CODE": {ID: "comp1", Code: "CODE", Name: "Test"}}}
	svc := NewGradeService(gradeRepo, finalRepo, enrollments, configReader, componentFetcher, validator.New(), zap.NewNop())

	grade, err := svc.Upsert(context.Background(), UpsertGradeRequest{EnrollmentID: "en1", SubjectID: "sub", ComponentCode: "code", GradeValue: 90})
	require.NoError(t, err)
	assert.Equal(t, "en1", grade.EnrollmentID)
	assert.Len(t, finalRepo.finals, 1)
	final := finalRepo.finals["en1"]
	assert.Equal(t, 90.0, final.FinalGrade)
}

func TestGradeServiceBulkUpsertAtomic(t *testing.T) {
	gradeRepo := &mockGradeRepo{}
	finalRepo := &mockGradeFinalRepo{}
	enrollments := &mockEnrollmentReader{enrollments: map[string]*models.Enrollment{"en1": {ID: "en1", StudentID: "stu1", ClassID: "class", TermID: "term", Status: models.EnrollmentStatusActive}}}
	config := &models.GradeConfig{ID: "cfg", ClassID: "class", SubjectID: "sub", TermID: "term", CalculationScheme: models.GradeSchemeAverage, Components: []models.GradeConfigComponent{{ComponentID: "comp1", ComponentCode: "CODE"}}}
	configReader := &mockConfigReader{config: config}
	componentFetcher := &mockComponentFetcher{components: map[string]*models.GradeComponent{"CODE": {ID: "comp1", Code: "CODE", Name: "Test"}}}
	svc := NewGradeService(gradeRepo, finalRepo, enrollments, configReader, componentFetcher, validator.New(), zap.NewNop())

	result, err := svc.BulkUpsert(context.Background(), BulkGradesRequest{ClassID: "class", SubjectID: "sub", TermID: "term", Mode: "atomic", Items: []BulkGradeItem{{EnrollmentID: "en1", ComponentCode: "code", GradeValue: 80}}})
	require.NoError(t, err)
	assert.Equal(t, 1, result.SuccessCount)
	assert.Len(t, finalRepo.finals, 1)
}

func TestGradeServiceFinalize(t *testing.T) {
	gradeRepo := &mockGradeRepo{}
	finalRepo := &mockGradeFinalRepo{}
	enrollments := &mockEnrollmentReader{enrollments: map[string]*models.Enrollment{"en1": {ID: "en1", StudentID: "stu1", ClassID: "class", TermID: "term", Status: models.EnrollmentStatusActive}}}
	config := &models.GradeConfig{ID: "cfg", ClassID: "class", SubjectID: "sub", TermID: "term", CalculationScheme: models.GradeSchemeAverage, Components: []models.GradeConfigComponent{{ComponentID: "comp1", ComponentCode: "CODE"}}}
	configReader := &mockConfigReader{config: config}
	componentFetcher := &mockComponentFetcher{components: map[string]*models.GradeComponent{"CODE": {ID: "comp1", Code: "CODE", Name: "Test"}}}
	svc := NewGradeService(gradeRepo, finalRepo, enrollments, configReader, componentFetcher, validator.New(), zap.NewNop())

	// pre-populate grade
	gradeRepo.Upsert(context.Background(), &models.Grade{EnrollmentID: "en1", SubjectID: "sub", ComponentID: "comp1", GradeValue: 85})
	err := svc.Finalize(context.Background(), FinalizeGradesRequest{ClassID: "class", SubjectID: "sub", TermID: "term"})
	require.NoError(t, err)
	assert.Contains(t, finalRepo.finalizedID, "en1")
}

func TestGradeServiceReport(t *testing.T) {
	gradeRepo := &mockGradeRepo{}
	finalRepo := &mockGradeFinalRepo{finals: make(map[string]models.GradeFinal)}
	enrollments := &mockEnrollmentReader{enrollments: map[string]*models.Enrollment{}}
	configReader := &mockConfigReader{config: &models.GradeConfig{}}
	componentFetcher := &mockComponentFetcher{components: map[string]*models.GradeComponent{}}
	svc := NewGradeService(gradeRepo, finalRepo, enrollments, configReader, componentFetcher, validator.New(), zap.NewNop())

	report, err := svc.ReportCard(context.Background(), "student", "term")
	require.NoError(t, err)
	assert.Len(t, report.Subjects, 1)

	classReport, err := svc.ClassReport(context.Background(), "class", "sub", "term")
	require.NoError(t, err)
	assert.NotNil(t, classReport.Distribution)
}
