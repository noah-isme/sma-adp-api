package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type drilldownAnalyticsRepoMock struct {
	class        *models.AnalyticsClassAnalytics
	student      *models.AnalyticsStudentAnalytics
	subject      *models.AnalyticsSubjectAnalytics
	leaderboard  []models.AnalyticsLeaderboardEntry
	classCalls   int
	studentCalls int
	subjectCalls int
	boardCalls   int
}

func (m *drilldownAnalyticsRepoMock) AttendanceSummary(context.Context, models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error) {
	return nil, nil
}

func (m *drilldownAnalyticsRepoMock) GradeSummary(context.Context, models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error) {
	return nil, nil
}

func (m *drilldownAnalyticsRepoMock) BehaviorSummary(context.Context, models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error) {
	return nil, nil
}

func (m *drilldownAnalyticsRepoMock) ClassAnalytics(context.Context, string, string) (*models.AnalyticsClassAnalytics, error) {
	m.classCalls++
	return m.class, nil
}

func (m *drilldownAnalyticsRepoMock) StudentAnalytics(context.Context, string, string) (*models.AnalyticsStudentAnalytics, error) {
	m.studentCalls++
	return m.student, nil
}

func (m *drilldownAnalyticsRepoMock) SubjectAnalytics(context.Context, string, string, string) (*models.AnalyticsSubjectAnalytics, error) {
	m.subjectCalls++
	return m.subject, nil
}

func (m *drilldownAnalyticsRepoMock) Leaderboard(context.Context, string, models.AnalyticsLeaderboardFilter) ([]models.AnalyticsLeaderboardEntry, error) {
	m.boardCalls++
	return m.leaderboard, nil
}

type analyticsAuthorizerMock struct {
	calls  int
	denied error
}

func (m *analyticsAuthorizerMock) AuthorizeAnalytics(context.Context, *models.JWTClaims, string, string, string) error {
	m.calls++
	return m.denied
}

func TestAnalyticsServiceClassRequiresTermAndCaches(t *testing.T) {
	repo := &drilldownAnalyticsRepoMock{class: &models.AnalyticsClassAnalytics{ClassID: "class-1", TermID: "term-1"}}
	svc := NewAnalyticsService(repo, nil, nil, zap.NewNop())

	_, _, err := svc.Class(context.Background(), "class-1", "")
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
	assert.Equal(t, 0, repo.classCalls)

	result, hit, err := svc.Class(context.Background(), "class-1", "term-1")
	require.NoError(t, err)
	assert.False(t, hit)
	assert.Equal(t, "class-1", result.ClassID)
	assert.Equal(t, 1, repo.classCalls)
}

func TestAnalyticsServiceStudentAuthorizerRunsBeforeRepository(t *testing.T) {
	repo := &drilldownAnalyticsRepoMock{student: &models.AnalyticsStudentAnalytics{StudentID: "student-1"}}
	authorizer := &analyticsAuthorizerMock{denied: appErrors.ErrForbidden}
	svc := NewAnalyticsService(repo, nil, nil, zap.NewNop())
	svc.SetObjectAuthorizer(authorizer)

	_, _, err := svc.StudentForClaims(context.Background(), "student-1", "term-1", &models.JWTClaims{StudentID: "student-1"})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrForbidden.Code, appErrors.FromError(err).Code)
	assert.Equal(t, 1, authorizer.calls)
	assert.Equal(t, 0, repo.studentCalls)
}

func TestAnalyticsServiceLeaderboardValidatesLimitAndAssignsRanks(t *testing.T) {
	repo := &drilldownAnalyticsRepoMock{leaderboard: []models.AnalyticsLeaderboardEntry{
		{StudentID: "student-2", Score: 90},
		{StudentID: "student-1", Score: 90},
	}}
	svc := NewAnalyticsService(repo, nil, nil, zap.NewNop())

	_, _, err := svc.Leaderboard(context.Background(), "gpa", models.AnalyticsLeaderboardFilter{TermID: "term-1", Limit: 101})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrValidation.Code, appErrors.FromError(err).Code)
	assert.Equal(t, 0, repo.boardCalls)

	result, hit, err := svc.Leaderboard(context.Background(), "gpa", models.AnalyticsLeaderboardFilter{TermID: "term-1", Limit: 2})
	require.NoError(t, err)
	assert.False(t, hit)
	assert.Equal(t, []int{1, 2}, []int{result.Leaderboard[0].Rank, result.Leaderboard[1].Rank})
	assert.Equal(t, "gpa", result.Metric)
}
