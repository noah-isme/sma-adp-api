package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type mockAnalyticsRepo struct {
	attendance      []models.AnalyticsAttendanceSummary
	grades          []models.AnalyticsGradeSummary
	behavior        []models.AnalyticsBehaviorSummary
	attendanceCalls int
	gradesCalls     int
	behaviorCalls   int
	attendanceErr   error
	gradesErr       error
	behaviorErr     error
}

func (m *mockAnalyticsRepo) AttendanceSummary(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error) {
	m.attendanceCalls++
	if m.attendanceErr != nil {
		return nil, m.attendanceErr
	}
	return m.attendance, nil
}

func (m *mockAnalyticsRepo) GradeSummary(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error) {
	m.gradesCalls++
	if m.gradesErr != nil {
		return nil, m.gradesErr
	}
	return m.grades, nil
}

func (m *mockAnalyticsRepo) BehaviorSummary(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error) {
	m.behaviorCalls++
	if m.behaviorErr != nil {
		return nil, m.behaviorErr
	}
	return m.behavior, nil
}

type stubCacheRepo struct {
	store map[string][]byte
}

func (s *stubCacheRepo) Get(_ context.Context, key string, dest interface{}) error {
	if s.store == nil {
		return appErrors.ErrCacheMiss
	}
	payload, ok := s.store[key]
	if !ok {
		return appErrors.ErrCacheMiss
	}
	return json.Unmarshal(payload, dest)
}

func (s *stubCacheRepo) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	if s.store == nil {
		s.store = make(map[string][]byte)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.store[key] = payload
	return nil
}

func (s *stubCacheRepo) DeleteByPattern(_ context.Context, _ string) error {
	return nil
}

func TestAnalyticsServiceAttendanceCaching(t *testing.T) {
	repo := &mockAnalyticsRepo{attendance: []models.AnalyticsAttendanceSummary{{TermID: "term-1", ClassID: "class-1", PresentCount: 10, AbsentCount: 2, Percentage: 83.3}}}
	cacheRepo := &stubCacheRepo{}
	cacheSvc := NewCacheService(cacheRepo, nil, time.Minute, zap.NewNop(), true)
	svc := NewAnalyticsService(repo, cacheSvc, nil, zap.NewNop())

	filter := models.AnalyticsAttendanceFilter{TermID: "term-1", ClassID: "class-1"}
	ctx := context.Background()

	result, cacheHit, err := svc.Attendance(ctx, filter)
	require.NoError(t, err)
	assert.False(t, cacheHit)
	assert.Equal(t, 1, repo.attendanceCalls)
	assert.Equal(t, repo.attendance, result)

	resultCached, cacheHit2, err := svc.Attendance(ctx, filter)
	require.NoError(t, err)
	assert.True(t, cacheHit2)
	assert.Equal(t, 1, repo.attendanceCalls)
	assert.Equal(t, result, resultCached)
}

func TestAnalyticsServiceAttendanceErrorPassthrough(t *testing.T) {
	repo := &mockAnalyticsRepo{attendanceErr: assert.AnError}
	cacheSvc := NewCacheService(nil, nil, time.Minute, zap.NewNop(), false)
	svc := NewAnalyticsService(repo, cacheSvc, nil, zap.NewNop())

	_, _, err := svc.Attendance(context.Background(), models.AnalyticsAttendanceFilter{})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}
