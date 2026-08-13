package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// AnalyticsRepository describes the persistence layer required by AnalyticsService.
type AnalyticsRepository interface {
	AttendanceSummary(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error)
	GradeSummary(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error)
	BehaviorSummary(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error)
}

// analyticsDrilldownRepository is intentionally separate from
// AnalyticsRepository. Dashboard and export test doubles only need the
// original summary methods, so adding Phase 5 methods must not break them.
type analyticsDrilldownRepository interface {
	ClassAnalytics(ctx context.Context, classID, termID string) (*models.AnalyticsClassAnalytics, error)
	StudentAnalytics(ctx context.Context, studentID, termID string) (*models.AnalyticsStudentAnalytics, error)
	SubjectAnalytics(ctx context.Context, subjectID, classID, termID string) (*models.AnalyticsSubjectAnalytics, error)
	Leaderboard(ctx context.Context, metric string, filter models.AnalyticsLeaderboardFilter) ([]models.AnalyticsLeaderboardEntry, error)
}

// AnalyticsObjectAuthorizer is an optional hook for object-level access. The
// gateway can provide a teacher-assignment-backed implementation while the
// service remains usable in tests and for administrator roles.
type AnalyticsObjectAuthorizer interface {
	AuthorizeAnalytics(ctx context.Context, claims *models.JWTClaims, resourceType, resourceID, termID string) error
}

// AnalyticsService provides read-optimised access to analytics datasets with cache integration.
type AnalyticsService struct {
	repo          AnalyticsRepository
	drilldownRepo analyticsDrilldownRepository
	authorizer    AnalyticsObjectAuthorizer
	cache         *CacheService
	metrics       *MetricsService
	logger        *zap.Logger
}

// NewAnalyticsService constructs an analytics service.
func NewAnalyticsService(repo AnalyticsRepository, cache *CacheService, metrics *MetricsService, logger *zap.Logger) *AnalyticsService {
	var drilldown analyticsDrilldownRepository
	if typed, ok := repo.(analyticsDrilldownRepository); ok {
		drilldown = typed
	}
	return &AnalyticsService{repo: repo, drilldownRepo: drilldown, cache: cache, metrics: metrics, logger: logger}
}

// SetObjectAuthorizer installs an optional object-level authorization hook.
// It is safe to call with nil to disable the hook (for trusted admin paths).
func (s *AnalyticsService) SetObjectAuthorizer(authorizer AnalyticsObjectAuthorizer) {
	if s == nil {
		return
	}
	s.authorizer = authorizer
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

// Class returns class-level analytics for a term.
func (s *AnalyticsService) Class(ctx context.Context, classID, termID string) (*models.AnalyticsClassAnalytics, bool, error) {
	return s.ClassForClaims(ctx, classID, termID, nil)
}

// ClassForClaims returns class-level analytics after invoking the optional
// object authorizer. Authorization is performed before cache lookup so a
// caller cannot infer another teacher's cached class data.
func (s *AnalyticsService) ClassForClaims(ctx context.Context, classID, termID string, claims *models.JWTClaims) (*models.AnalyticsClassAnalytics, bool, error) {
	if err := validateAnalyticsResource(classID, "class_id"); err != nil {
		return nil, false, err
	}
	if err := validateAnalyticsTerm(termID); err != nil {
		return nil, false, err
	}
	if err := s.authorize(ctx, claims, "class", classID, termID); err != nil {
		return nil, false, err
	}
	if s.drilldownRepo == nil {
		return nil, false, appErrors.Clone(appErrors.ErrInternal, "analytics drilldown repository unavailable")
	}

	key := makeAnalyticsCacheKey("class", classID, termID)
	var cached models.AnalyticsClassAnalytics
	if hit, err := s.cacheGet(ctx, key, &cached); err != nil {
		return nil, false, err
	} else if hit {
		return &cached, true, nil
	}

	start := time.Now()
	result, err := s.drilldownRepo.ClassAnalytics(ctx, classID, termID)
	if err != nil {
		return nil, false, analyticsRepositoryError(err, "class analytics not found", "failed to load class analytics")
	}
	if s.metrics != nil {
		s.metrics.ObserveDBQuery("analytics_class", time.Since(start))
	}
	s.cacheSet(ctx, key, result, time.Hour, "cache class analytics")
	return result, false, nil
}

// Student returns student-level analytics for a term.
func (s *AnalyticsService) Student(ctx context.Context, studentID, termID string) (*models.AnalyticsStudentAnalytics, bool, error) {
	return s.StudentForClaims(ctx, studentID, termID, nil)
}

// StudentForClaims returns student analytics with optional object
// authorization. A student self-scope check is performed by the handler,
// while teacher/parent/class checks can be supplied through the hook.
func (s *AnalyticsService) StudentForClaims(ctx context.Context, studentID, termID string, claims *models.JWTClaims) (*models.AnalyticsStudentAnalytics, bool, error) {
	if err := validateAnalyticsResource(studentID, "student_id"); err != nil {
		return nil, false, err
	}
	if err := validateAnalyticsTerm(termID); err != nil {
		return nil, false, err
	}
	if err := s.authorize(ctx, claims, "student", studentID, termID); err != nil {
		return nil, false, err
	}
	if s.drilldownRepo == nil {
		return nil, false, appErrors.Clone(appErrors.ErrInternal, "analytics drilldown repository unavailable")
	}

	key := makeAnalyticsCacheKey("student", studentID, termID)
	var cached models.AnalyticsStudentAnalytics
	if hit, err := s.cacheGet(ctx, key, &cached); err != nil {
		return nil, false, err
	} else if hit {
		return &cached, true, nil
	}

	start := time.Now()
	result, err := s.drilldownRepo.StudentAnalytics(ctx, studentID, termID)
	if err != nil {
		return nil, false, analyticsRepositoryError(err, "student analytics not found", "failed to load student analytics")
	}
	if s.metrics != nil {
		s.metrics.ObserveDBQuery("analytics_student", time.Since(start))
	}
	s.cacheSet(ctx, key, result, time.Hour, "cache student analytics")
	return result, false, nil
}

// Subject returns subject-level analytics for a term, optionally scoped to a class.
func (s *AnalyticsService) Subject(ctx context.Context, subjectID, classID, termID string) (*models.AnalyticsSubjectAnalytics, bool, error) {
	return s.SubjectForClaims(ctx, subjectID, classID, termID, nil)
}

// SubjectForClaims returns subject analytics with optional object authorization.
func (s *AnalyticsService) SubjectForClaims(ctx context.Context, subjectID, classID, termID string, claims *models.JWTClaims) (*models.AnalyticsSubjectAnalytics, bool, error) {
	if err := validateAnalyticsResource(subjectID, "subject_id"); err != nil {
		return nil, false, err
	}
	if err := validateAnalyticsTerm(termID); err != nil {
		return nil, false, err
	}
	if err := s.authorizeSubject(ctx, claims, subjectID, classID, termID); err != nil {
		return nil, false, err
	}
	if s.drilldownRepo == nil {
		return nil, false, appErrors.Clone(appErrors.ErrInternal, "analytics drilldown repository unavailable")
	}

	key := makeAnalyticsCacheKey("subject", subjectID, classID, termID)
	var cached models.AnalyticsSubjectAnalytics
	if hit, err := s.cacheGet(ctx, key, &cached); err != nil {
		return nil, false, err
	} else if hit {
		return &cached, true, nil
	}

	start := time.Now()
	result, err := s.drilldownRepo.SubjectAnalytics(ctx, subjectID, classID, termID)
	if err != nil {
		return nil, false, analyticsRepositoryError(err, "subject analytics not found", "failed to load subject analytics")
	}
	if s.metrics != nil {
		s.metrics.ObserveDBQuery("analytics_subject", time.Since(start))
	}
	s.cacheSet(ctx, key, result, time.Hour, "cache subject analytics")
	return result, false, nil
}

// Leaderboard returns a bounded, deterministic leaderboard for one metric.
func (s *AnalyticsService) Leaderboard(ctx context.Context, metric string, filter models.AnalyticsLeaderboardFilter) (*models.AnalyticsLeaderboard, bool, error) {
	return s.LeaderboardForClaims(ctx, metric, filter, nil)
}

// LeaderboardForClaims returns a leaderboard after invoking the optional
// object authorizer. A class_id of empty scope means all classes in the term.
func (s *AnalyticsService) LeaderboardForClaims(ctx context.Context, metric string, filter models.AnalyticsLeaderboardFilter, claims *models.JWTClaims) (*models.AnalyticsLeaderboard, bool, error) {
	metric = strings.ToLower(strings.TrimSpace(metric))
	if metric != "gpa" && metric != "attendance" && metric != "behavior" {
		return nil, false, appErrors.Clone(appErrors.ErrValidation, "metric must be gpa, attendance, or behavior")
	}
	if err := validateAnalyticsTerm(filter.TermID); err != nil {
		return nil, false, err
	}
	if filter.Limit == 0 {
		filter.Limit = 10
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		return nil, false, appErrors.Clone(appErrors.ErrValidation, "limit must be between 1 and 100")
	}
	if filter.ClassID != "" {
		if err := s.authorize(ctx, claims, "class", filter.ClassID, filter.TermID); err != nil {
			return nil, false, err
		}
	}
	if s.drilldownRepo == nil {
		return nil, false, appErrors.Clone(appErrors.ErrInternal, "analytics drilldown repository unavailable")
	}

	key := makeAnalyticsCacheKey("leaderboard", metric, filter.TermID, filter.ClassID, fmt.Sprintf("%d", filter.Limit))
	var cached models.AnalyticsLeaderboard
	if hit, err := s.cacheGet(ctx, key, &cached); err != nil {
		return nil, false, err
	} else if hit {
		return &cached, true, nil
	}

	start := time.Now()
	entries, err := s.drilldownRepo.Leaderboard(ctx, metric, filter)
	if err != nil {
		return nil, false, analyticsRepositoryError(err, "leaderboard not found", "failed to load analytics leaderboard")
	}
	// Reassign ranks after every repository/cache path to guarantee sequential
	// deterministic ranks even if a backend view uses a different rank window.
	for i := range entries {
		entries[i].Rank = i + 1
	}
	if entries == nil {
		entries = []models.AnalyticsLeaderboardEntry{}
	}
	if s.metrics != nil {
		s.metrics.ObserveDBQuery("analytics_leaderboard_"+metric, time.Since(start))
	}
	result := &models.AnalyticsLeaderboard{TermID: filter.TermID, ClassID: filter.ClassID, Metric: metric, Leaderboard: entries}
	s.cacheSet(ctx, key, result, 30*time.Minute, "cache analytics leaderboard")
	return result, false, nil
}

// LeaderboardGPA is a convenience wrapper for route wiring.
func (s *AnalyticsService) LeaderboardGPA(ctx context.Context, filter models.AnalyticsLeaderboardFilter) (*models.AnalyticsLeaderboard, bool, error) {
	return s.Leaderboard(ctx, "gpa", filter)
}

// LeaderboardAttendance is a convenience wrapper for route wiring.
func (s *AnalyticsService) LeaderboardAttendance(ctx context.Context, filter models.AnalyticsLeaderboardFilter) (*models.AnalyticsLeaderboard, bool, error) {
	return s.Leaderboard(ctx, "attendance", filter)
}

// LeaderboardBehavior is a convenience wrapper for route wiring.
func (s *AnalyticsService) LeaderboardBehavior(ctx context.Context, filter models.AnalyticsLeaderboardFilter) (*models.AnalyticsLeaderboard, bool, error) {
	return s.Leaderboard(ctx, "behavior", filter)
}

func (s *AnalyticsService) authorize(ctx context.Context, claims *models.JWTClaims, resourceType, resourceID, termID string) error {
	if s.authorizer == nil {
		return nil
	}
	return s.authorizer.AuthorizeAnalytics(ctx, claims, resourceType, resourceID, termID)
}

// authorizeSubject preserves the original object-authorizer contract while
// allowing the database-backed authorizer to enforce the complete
// teacher/class/subject/term tuple when a subject drilldown is class-scoped.
// Falling back to AuthorizeAnalytics keeps lightweight test doubles and
// trusted callers compatible with the pre-scope interface.
func (s *AnalyticsService) authorizeSubject(ctx context.Context, claims *models.JWTClaims, subjectID, classID, termID string) error {
	if scoped, ok := s.authorizer.(interface {
		AuthorizeAnalyticsSubject(context.Context, *models.JWTClaims, string, string, string) error
	}); ok {
		return scoped.AuthorizeAnalyticsSubject(ctx, claims, subjectID, classID, termID)
	}
	if err := s.authorize(ctx, claims, "subject", subjectID, termID); err != nil {
		return err
	}
	if classID != "" {
		return s.authorize(ctx, claims, "class", classID, termID)
	}
	return nil
}

func (s *AnalyticsService) cacheGet(ctx context.Context, key string, destination interface{}) (bool, error) {
	if s.cache == nil {
		return false, nil
	}
	if hit, err := s.cache.Get(ctx, key, destination); err != nil {
		return false, fmt.Errorf("get analytics cache: %w", err)
	} else if hit {
		return true, nil
	}
	return false, nil
}

func (s *AnalyticsService) cacheSet(ctx context.Context, key string, value interface{}, ttl time.Duration, operation string) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Set(ctx, key, value, ttl); err != nil && s.logger != nil {
		s.logger.Warn(operation, zap.String("key", key), zap.Error(err))
	}
}

func validateAnalyticsTerm(termID string) error {
	if strings.TrimSpace(termID) == "" {
		return appErrors.Clone(appErrors.ErrValidation, "term_id is required")
	}
	return nil
}

func validateAnalyticsResource(value, name string) error {
	if strings.TrimSpace(value) == "" {
		return appErrors.Clone(appErrors.ErrValidation, name+" is required")
	}
	return nil
}

func analyticsRepositoryError(err error, notFoundMessage, internalMessage string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return appErrors.Clone(appErrors.ErrNotFound, notFoundMessage)
	}
	return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, internalMessage)
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
