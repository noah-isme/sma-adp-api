package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type fakeAnalytics struct {
	attendance    []models.AnalyticsAttendanceSummary
	grades        []models.AnalyticsGradeSummary
	behavior      []models.AnalyticsBehaviorSummary
	attendanceErr error
	gradesErr     error
	behaviorErr   error
	attendanceHit bool
	gradesHit     bool
	behaviorHit   bool
}

func (f *fakeAnalytics) Attendance(context.Context, models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, bool, error) {
	return f.attendance, f.attendanceHit, f.attendanceErr
}

func (f *fakeAnalytics) Grades(context.Context, models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, bool, error) {
	return f.grades, f.gradesHit, f.gradesErr
}

func (f *fakeAnalytics) Behavior(context.Context, models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, bool, error) {
	return f.behavior, f.behaviorHit, f.behaviorErr
}

type fakeAnalyticsRepo struct {
	attendance []models.AnalyticsAttendanceSummary
	grades     []models.AnalyticsGradeSummary
	behavior   []models.AnalyticsBehaviorSummary
}

func (f *fakeAnalyticsRepo) AttendanceSummary(context.Context, models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error) {
	return f.attendance, nil
}

func (f *fakeAnalyticsRepo) GradeSummary(context.Context, models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error) {
	return f.grades, nil
}

func (f *fakeAnalyticsRepo) BehaviorSummary(context.Context, models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error) {
	return f.behavior, nil
}

type fakeCalendar struct {
	events []models.CalendarEvent
	err    error
}

func (f *fakeCalendar) List(context.Context, CalendarListRequest) ([]models.CalendarEvent, *models.Pagination, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.events, nil, nil
}

type fakeAnnouncements struct {
	total int
	err   error
}

func (f *fakeAnnouncements) List(context.Context, AnnouncementListRequest) ([]models.Announcement, *models.Pagination, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return nil, &models.Pagination{TotalCount: f.total}, nil
}

type fakeAssignments struct {
	assignments []models.TeacherAssignmentDetail
	err         error
}

func (f *fakeAssignments) ListByTeacher(context.Context, string) ([]models.TeacherAssignmentDetail, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.assignments, nil
}

type fakeSchedules struct {
	schedules []models.Schedule
	err       error
}

func (f *fakeSchedules) ListByTeacher(context.Context, string) ([]models.Schedule, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.schedules, nil
}

func TestDashboardServiceAdmin_ComposesAndCaches(t *testing.T) {
	cacheRepo := &stubCacheRepo{}
	cacheSvc := NewCacheService(cacheRepo, nil, time.Minute, zap.NewNop(), true)

	now := time.Date(2024, 11, 10, 7, 0, 0, 0, time.UTC)
	svc := NewDashboardService(DashboardServiceParams{
		Analytics: &fakeAnalytics{
			attendance: []models.AnalyticsAttendanceSummary{
				{ClassID: "class-a", PresentCount: 90, AbsentCount: 10, Percentage: 90},
				{ClassID: "class-b", PresentCount: 40, AbsentCount: 10, Percentage: 80},
			},
			grades: []models.AnalyticsGradeSummary{
				{ClassID: "class-a", AverageScore: 88, Rank: []models.AnalyticsGradeRank{{StudentID: "stu-1", Score: 92}}},
				{ClassID: "class-b", AverageScore: 72},
			},
			behavior: []models.AnalyticsBehaviorSummary{
				{StudentID: "stu-1", TotalPositive: 8, TotalNegative: 1, Balance: 7},
				{StudentID: "stu-2", TotalPositive: 2, TotalNegative: 5, Balance: -3},
				{StudentID: "stu-3", TotalPositive: 5, TotalNegative: 4, Balance: 1},
			},
		},
		Calendar: &fakeCalendar{events: []models.CalendarEvent{
			{ID: "evt-1", Title: "PTS", StartDate: now},
		}},
		Announcements: &fakeAnnouncements{total: 4},
		Cache:         cacheSvc,
		Logger:        zap.NewNop(),
	})
	svc.now = func() time.Time { return now }

	ctx := context.Background()
	result, cacheHit, err := svc.Admin(ctx, "term-1")
	require.NoError(t, err)
	assert.False(t, cacheHit)
	assert.InDelta(t, 86.666, result.Attendance.OverallRate, 0.001)
	assert.Equal(t, 2, len(result.Grades.AverageByClass))
	assert.Equal(t, 5, len(result.Grades.Distribution))
	assert.Equal(t, 3, len(result.Behavior.TopPositive))
	assert.Equal(t, 3, len(result.Behavior.TopNegative))
	assert.Equal(t, 4, result.Ops.OpenAnnouncements)
	assert.Len(t, result.Ops.UpcomingEvents, 1)

	resultCached, cacheHit2, err := svc.Admin(ctx, "term-1")
	require.NoError(t, err)
	assert.True(t, cacheHit2)
	assert.Equal(t, result, resultCached)
}

func TestDashboardServiceTeacher_ComposesSummary(t *testing.T) {
	cacheSvc := NewCacheService(nil, nil, time.Minute, zap.NewNop(), false)
	assignments := &fakeAssignments{
		assignments: []models.TeacherAssignmentDetail{
			{TeacherAssignment: models.TeacherAssignment{ClassID: "class-a", TermID: "term-1"}},
			{TeacherAssignment: models.TeacherAssignment{ClassID: "class-b", TermID: "term-1"}},
			{TeacherAssignment: models.TeacherAssignment{ClassID: "class-c", TermID: "term-2"}},
		},
	}
	schedules := &fakeSchedules{
		schedules: []models.Schedule{
			{ClassID: "class-a", SubjectID: "math", TermID: "term-1", DayOfWeek: "MONDAY", TimeSlot: "1", Room: "Lab"},
			{ClassID: "class-b", SubjectID: "bio", TermID: "term-1", DayOfWeek: "TUESDAY", TimeSlot: "2", Room: "201"},
		},
	}
	analytics := &fakeAnalytics{
		attendance: []models.AnalyticsAttendanceSummary{
			{ClassID: "class-a", Percentage: 82},
			{ClassID: "class-b", Percentage: 91},
		},
		grades: []models.AnalyticsGradeSummary{
			{ClassID: "class-a", AverageScore: 75},
			{ClassID: "class-b", AverageScore: 88},
		},
	}

	svc := NewDashboardService(DashboardServiceParams{
		Analytics:   analytics,
		Assignments: assignments,
		Schedules:   schedules,
		Cache:       cacheSvc,
		Config: DashboardServiceConfig{
			LowAttendanceThreshold: 85,
			GradeOutlierThreshold:  80,
		},
		Logger: zap.NewNop(),
	})

	date := time.Date(2024, 11, 11, 0, 0, 0, 0, time.UTC) // Monday
	result, cacheHit, err := svc.Teacher(context.Background(), "teacher-1", "term-1", date)
	require.NoError(t, err)
	assert.False(t, cacheHit)
	assert.Equal(t, "teacher-1", result.TeacherID)
	require.Len(t, result.Classes, 2)
	assert.Contains(t, result.Alerts.LowAttendanceClasses, "class-a")
	assert.Contains(t, result.Alerts.GradeOutliers, "class-a")
	require.Len(t, result.Today.Schedules, 1)
	assert.Equal(t, "math", result.Today.Schedules[0].SubjectID)
	assert.Equal(t, 1, result.Today.Schedules[0].TimeSlot)
	assert.NotNil(t, result.Today.Schedules[0].Room)
	assert.Equal(t, "Lab", *result.Today.Schedules[0].Room)
}

func TestDashboardServiceAnalyticsFallback(t *testing.T) {
	cacheSvc := NewCacheService(nil, nil, time.Minute, zap.NewNop(), false)
	repo := &fakeAnalyticsRepo{
		attendance: []models.AnalyticsAttendanceSummary{{ClassID: "class-a", Percentage: 95}},
		grades:     []models.AnalyticsGradeSummary{{ClassID: "class-a", AverageScore: 90}},
		behavior:   []models.AnalyticsBehaviorSummary{{StudentID: "stu-1", TotalPositive: 5}},
	}
	svc := NewDashboardService(DashboardServiceParams{
		AnalyticsRepo: repo,
		Assignments:   &fakeAssignments{},
		Schedules:     &fakeSchedules{},
		Cache:         cacheSvc,
		Logger:        zap.NewNop(),
	})

	_, _, err := svc.Admin(context.Background(), "term-1")
	require.NoError(t, err)
}

func TestDashboardServiceTeacherValidation(t *testing.T) {
	svc := NewDashboardService(DashboardServiceParams{})
	_, _, err := svc.Teacher(context.Background(), "", "term-1", time.Now())
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}

func TestDashboardServiceAdminValidation(t *testing.T) {
	svc := NewDashboardService(DashboardServiceParams{})
	_, _, err := svc.Admin(context.Background(), "")
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
}
