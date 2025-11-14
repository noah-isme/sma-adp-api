package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type attendanceSummaryRepoStub struct {
	aggregate *repository.AttendanceAliasAggregate
	err       error
}

func (s attendanceSummaryRepoStub) Aggregate(ctx context.Context, filter repository.AttendanceAliasFilter) (*repository.AttendanceAliasAggregate, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.aggregate, nil
}

type assignmentAccessStub struct {
	list []models.TeacherAssignmentDetail
}

func (s assignmentAccessStub) ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error) {
	return s.list, nil
}

func (assignmentAccessStub) HasClassAccess(ctx context.Context, teacherID, classID, termID string) (bool, error) {
	return true, nil
}

type enrollmentReaderStub struct{}

func (enrollmentReaderStub) ListByClassAndTerm(ctx context.Context, classID, termID string) ([]models.Enrollment, error) {
	return nil, nil
}

func (enrollmentReaderStub) FindActiveByStudentAndTerm(ctx context.Context, studentID, termID string) ([]models.Enrollment, error) {
	return []models.Enrollment{{StudentID: studentID, ClassID: "class-1", TermID: termID}}, nil
}

type attendanceTermLookupStub struct{}

func (attendanceTermLookupStub) FindByID(ctx context.Context, id string) (*models.Term, error) {
	return &models.Term{ID: id}, nil
}

func TestAttendanceAliasServiceSummary(t *testing.T) {
	attendanceSvc := &AttendanceService{}
	aggregate := &repository.AttendanceAliasAggregate{
		TotalDays: 10,
		Present:   18,
		Sick:      1,
		Excused:   2,
		Absent:    3,
		Students: []repository.AttendanceAliasStudentRow{
			{StudentID: "stu-1", StudentName: "Alice", ClassID: "class-1", Present: 9, Sick: 1, Excused: 0, Absent: 0, Rate: 90},
			{StudentID: "stu-2", StudentName: "Bob", ClassID: "class-1", Present: 9, Sick: 0, Excused: 2, Absent: 3, Rate: 60},
		},
	}
	service := NewAttendanceAliasService(
		attendanceSvc,
		nil,
		attendanceSummaryRepoStub{aggregate: aggregate},
		assignmentAccessStub{list: []models.TeacherAssignmentDetail{
			{TeacherAssignment: models.TeacherAssignment{ClassID: "class-1", TermID: "term-1"}},
		}},
		enrollmentReaderStub{},
		attendanceTermLookupStub{},
		nil,
	)

	resp, cacheHit, err := service.Summary(context.Background(), dto.AttendanceSummaryRequest{TermID: "term-1", ClassID: "class-1"}, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})
	require.NoError(t, err)
	assert.False(t, cacheHit)
	require.Len(t, resp.PerStudent, 2)
	assert.InDelta(t, 75, resp.Summary.AttendanceRate, 0.1)
	assert.Equal(t, 3, resp.Summary.Absent)
}

func TestAttendanceAliasServiceSummaryTeacherForbidden(t *testing.T) {
	service := NewAttendanceAliasService(
		&AttendanceService{},
		nil,
		attendanceSummaryRepoStub{},
		assignmentAccessStub{},
		enrollmentReaderStub{},
		attendanceTermLookupStub{},
		nil,
	)
	_, _, err := service.Summary(context.Background(), dto.AttendanceSummaryRequest{TermID: "term-1", ClassID: "class-1"}, &models.JWTClaims{UserID: "teacher-1", Role: models.RoleTeacher})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrForbidden.Code, appErrors.FromError(err).Code)
}
