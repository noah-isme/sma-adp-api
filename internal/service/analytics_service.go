package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// AnalyticsRepository describes the persistence layer required by AnalyticsService.
type AnalyticsRepository interface {
	AttendanceSummary(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error)
	GradeSummary(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error)
	BehaviorSummary(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error)
}

// AnalyticsService provides read-optimised access to analytics datasets with cache integration.
type AnalyticsService struct {
	repo    AnalyticsRepository
	cache   *CacheService
	metrics *MetricsService
	logger  *zap.Logger
}

// NewAnalyticsService constructs an analytics service.
func NewAnalyticsService(repo AnalyticsRepository, cache *CacheService, metrics *MetricsService, logger *zap.Logger) *AnalyticsService {
	return &AnalyticsService{repo: repo, cache: cache, metrics: metrics, logger: logger}
}

// Attendance returns aggregated attendance analytics. The boolean indicates whether data originated from cache.
func (s *AnalyticsService) Attendance(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, bool, error) {
	cacheKey := makeAnalyticsCacheKey("attendance", filter.TermID, filter.ClassID, formatTime(filter.DateFrom), formatTime(filter.DateTo))
	var cached []models.AnalyticsAttendanceSummary
	if s.cache != nil {
		if hit, err := s.cache.Get(ctx, cacheKey, &cached); err != nil {
			return nil, false, fmt.Errorf("get attendance cache: %w", err)
		} else if hit {
			return cached, true, nil
		}
	}

	start := time.Now()
	summaries, err := s.repo.AttendanceSummary(ctx, filter)
	if err != nil {
		return nil, false, err
	}
	if s.metrics != nil {
		s.metrics.ObserveDBQuery("analytics_attendance", time.Since(start))
	}
	if s.cache != nil {
		if err := s.cache.Set(ctx, cacheKey, summaries, 0); err != nil && s.logger != nil {
			s.logger.Warn("cache attendance", zap.Error(err))
		}
	}
	return summaries, false, nil
}

// Grades returns aggregated grade analytics.
func (s *AnalyticsService) Grades(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, bool, error) {
	cacheKey := makeAnalyticsCacheKey("grades", filter.TermID, filter.ClassID, filter.SubjectID)
	var cached []models.AnalyticsGradeSummary
	if s.cache != nil {
		if hit, err := s.cache.Get(ctx, cacheKey, &cached); err != nil {
			return nil, false, fmt.Errorf("get grade cache: %w", err)
		} else if hit {
			return cached, true, nil
		}
	}

	start := time.Now()
	summaries, err := s.repo.GradeSummary(ctx, filter)
	if err != nil {
		return nil, false, err
	}
	if s.metrics != nil {
		s.metrics.ObserveDBQuery("analytics_grades", time.Since(start))
	}
	if s.cache != nil {
		if err := s.cache.Set(ctx, cacheKey, summaries, 0); err != nil && s.logger != nil {
			s.logger.Warn("cache grades", zap.Error(err))
		}
	}
	return summaries, false, nil
}

// Behavior returns aggregated behaviour analytics.
func (s *AnalyticsService) Behavior(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, bool, error) {
	cacheKey := makeAnalyticsCacheKey("behavior", filter.TermID, filter.ClassID, filter.StudentID, formatTime(filter.DateFrom), formatTime(filter.DateTo))
	var cached []models.AnalyticsBehaviorSummary
	if s.cache != nil {
		if hit, err := s.cache.Get(ctx, cacheKey, &cached); err != nil {
			return nil, false, fmt.Errorf("get behavior cache: %w", err)
		} else if hit {
			return cached, true, nil
		}
	}

	start := time.Now()
	summaries, err := s.repo.BehaviorSummary(ctx, filter)
	if err != nil {
		return nil, false, err
	}
	if s.metrics != nil {
		s.metrics.ObserveDBQuery("analytics_behavior", time.Since(start))
	}
	if s.cache != nil {
		if err := s.cache.Set(ctx, cacheKey, summaries, 0); err != nil && s.logger != nil {
			s.logger.Warn("cache behavior", zap.Error(err))
		}
	}
	return summaries, false, nil
}

// SystemMetrics returns system instrumentation snapshot.
func (s *AnalyticsService) SystemMetrics() models.AnalyticsSystemMetrics {
	if s.metrics == nil {
		return models.AnalyticsSystemMetrics{}
	}
	return s.metrics.Snapshot()
}

func makeAnalyticsCacheKey(parts ...string) string {
	var builder strings.Builder
	builder.Grow(len(parts) * 16)
	builder.WriteString("analytics")
	for _, part := range parts {
		if part == "" {
			continue
		}
		builder.WriteByte(':')
		builder.WriteString(strings.ReplaceAll(part, ":", "|"))
	}
	return builder.String()
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
