package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type analyticsSummaryProvider interface {
	Attendance(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, bool, error)
	Grades(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, bool, error)
	Behavior(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, bool, error)
}

type analyticsSummaryRepository interface {
	AttendanceSummary(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error)
	GradeSummary(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error)
	BehaviorSummary(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error)
}

type calendarLister interface {
	List(ctx context.Context, req CalendarListRequest) ([]models.CalendarEvent, *models.Pagination, error)
}

type announcementLister interface {
	List(ctx context.Context, req AnnouncementListRequest) ([]models.Announcement, *models.Pagination, error)
}

type scheduleLister interface {
	ListByTeacher(ctx context.Context, teacherID string) ([]models.Schedule, error)
}

type assignmentLister interface {
	ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error)
}

// DashboardServiceConfig tunes dashboard behaviour.
type DashboardServiceConfig struct {
	CacheTTL               time.Duration
	LowAttendanceThreshold float64
	GradeOutlierThreshold  float64
	UpcomingEventsLimit    int
	BehaviorLeaderboardMax int
}

// DashboardService orchestrates composition of dashboard payloads.
type DashboardService struct {
	analytics     analyticsSummaryProvider
	analyticsRepo analyticsSummaryRepository
	calendar      calendarLister
	announcements announcementLister
	schedules     scheduleLister
	assignments   assignmentLister
	cache         *CacheService
	logger        *zap.Logger
	now           func() time.Time
	cfg           DashboardServiceConfig
}

// DashboardServiceParams groups constructor dependencies.
type DashboardServiceParams struct {
	Analytics     analyticsSummaryProvider
	AnalyticsRepo analyticsSummaryRepository
	Calendar      calendarLister
	Announcements announcementLister
	Schedules     scheduleLister
	Assignments   assignmentLister
	Cache         *CacheService
	Logger        *zap.Logger
	Config        DashboardServiceConfig
}

// NewDashboardService constructs a DashboardService with sane defaults.
func NewDashboardService(params DashboardServiceParams) *DashboardService {
	cfg := params.Config
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.LowAttendanceThreshold <= 0 {
		cfg.LowAttendanceThreshold = 90
	}
	if cfg.GradeOutlierThreshold <= 0 {
		cfg.GradeOutlierThreshold = 70
	}
	if cfg.UpcomingEventsLimit <= 0 {
		cfg.UpcomingEventsLimit = 3
	}
	if cfg.BehaviorLeaderboardMax <= 0 {
		cfg.BehaviorLeaderboardMax = 5
	}
	logger := params.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DashboardService{
		analytics:     params.Analytics,
		analyticsRepo: params.AnalyticsRepo,
		calendar:      params.Calendar,
		announcements: params.Announcements,
		schedules:     params.Schedules,
		assignments:   params.Assignments,
		cache:         params.Cache,
		logger:        logger,
		now:           time.Now,
		cfg:           cfg,
	}
}

// Admin returns admin dashboard summary and indicates cache utilisation.
func (s *DashboardService) Admin(ctx context.Context, termID string) (*dto.AdminDashboardResponse, bool, error) {
	if termID == "" {
		return nil, false, appErrors.Clone(appErrors.ErrValidation, "termId is required")
	}
	cacheKey := fmt.Sprintf("dash:admin:%s", termID)
	if summary, hit, err := s.tryAdminCache(ctx, cacheKey); err != nil {
		return nil, false, err
	} else if hit {
		return summary, true, nil
	}

	summary, err := s.composeAdminSummary(ctx, termID)
	if err != nil {
		return nil, false, err
	}
	s.persistCache(ctx, cacheKey, summary)
	return summary, false, nil
}

// Teacher returns teacher dashboard data constrained by term and date.
func (s *DashboardService) Teacher(ctx context.Context, teacherID, termID string, date time.Time) (*dto.TeacherDashboardResponse, bool, error) {
	if teacherID == "" {
		return nil, false, appErrors.Clone(appErrors.ErrValidation, "teacherId is required")
	}
	if termID == "" {
		return nil, false, appErrors.Clone(appErrors.ErrValidation, "termId is required")
	}
	date = date.UTC()
	cacheKey := fmt.Sprintf("dash:teacher:%s:%s:%s", teacherID, termID, date.Format("2006-01-02"))
	if summary, hit, err := s.tryTeacherCache(ctx, cacheKey); err != nil {
		return nil, false, err
	} else if hit {
		return summary, true, nil
	}

	summary, err := s.composeTeacherSummary(ctx, teacherID, termID, date)
	if err != nil {
		return nil, false, err
	}
	s.persistCache(ctx, cacheKey, summary)
	return summary, false, nil
}

func (s *DashboardService) tryAdminCache(ctx context.Context, key string) (*dto.AdminDashboardResponse, bool, error) {
	if s.cache == nil {
		return nil, false, nil
	}
	var cached dto.AdminDashboardResponse
	hit, err := s.cache.Get(ctx, key, &cached)
	if err != nil {
		return nil, false, err
	}
	if hit {
		return &cached, true, nil
	}
	return nil, false, nil
}

func (s *DashboardService) tryTeacherCache(ctx context.Context, key string) (*dto.TeacherDashboardResponse, bool, error) {
	if s.cache == nil {
		return nil, false, nil
	}
	var cached dto.TeacherDashboardResponse
	hit, err := s.cache.Get(ctx, key, &cached)
	if err != nil {
		return nil, false, err
	}
	if hit {
		return &cached, true, nil
	}
	return nil, false, nil
}

func (s *DashboardService) persistCache(ctx context.Context, key string, value interface{}) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Set(ctx, key, value, s.cfg.CacheTTL); err != nil && s.logger != nil {
		s.logger.Warn("dashboard cache write failed", zap.String("key", key), zap.Error(err))
	}
}

func (s *DashboardService) composeAdminSummary(ctx context.Context, termID string) (*dto.AdminDashboardResponse, error) {
	attendanceSummaries, err := s.loadAttendance(ctx, models.AnalyticsAttendanceFilter{TermID: termID})
	if err != nil {
		return nil, err
	}
	gradeSummaries, err := s.loadGrades(ctx, models.AnalyticsGradeFilter{TermID: termID})
	if err != nil {
		return nil, err
	}
	behaviorSummaries, err := s.loadBehavior(ctx, models.AnalyticsBehaviorFilter{TermID: termID})
	if err != nil {
		return nil, err
	}

	summary := &dto.AdminDashboardResponse{
		TermID:     termID,
		Attendance: s.buildAdminAttendance(attendanceSummaries),
		Grades:     s.buildAdminGrades(gradeSummaries),
		Behavior:   s.buildAdminBehavior(behaviorSummaries),
		Ops:        s.buildOpsHighlights(ctx),
	}
	return summary, nil
}

func (s *DashboardService) composeTeacherSummary(ctx context.Context, teacherID, termID string, date time.Time) (*dto.TeacherDashboardResponse, error) {
	if s.assignments == nil {
		return nil, appErrors.Clone(appErrors.ErrInternal, "assignment service unavailable")
	}
	assignments, err := s.assignments.ListByTeacher(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	classSet := map[string]struct{}{}
	for _, assignment := range assignments {
		if assignment.TermID == termID {
			classSet[assignment.ClassID] = struct{}{}
		}
	}
	classIDs := make([]string, 0, len(classSet))
	for classID := range classSet {
		classIDs = append(classIDs, classID)
	}
	sort.Strings(classIDs)

	attendanceSummaries, err := s.loadAttendance(ctx, models.AnalyticsAttendanceFilter{TermID: termID})
	if err != nil {
		return nil, err
	}
	gradeSummaries, err := s.loadGrades(ctx, models.AnalyticsGradeFilter{TermID: termID})
	if err != nil {
		return nil, err
	}

	classAttendance := map[string]float64{}
	for _, summary := range attendanceSummaries {
		classAttendance[summary.ClassID] = summary.Percentage
	}
	classGrades := s.averageGradeByClass(gradeSummaries)

	classSnapshots := make([]dto.TeacherClassSummary, 0, len(classIDs))
	alerts := dto.TeacherAlerts{}
	for _, classID := range classIDs {
		attendanceRate := classAttendance[classID]
		averageGrade := classGrades[classID]
		classSnapshots = append(classSnapshots, dto.TeacherClassSummary{
			ClassID:        classID,
			AttendanceRate: attendanceRate,
			AverageGrade:   averageGrade,
		})
		if attendanceRate > 0 && attendanceRate < s.cfg.LowAttendanceThreshold {
			alerts.LowAttendanceClasses = append(alerts.LowAttendanceClasses, classID)
		}
		if averageGrade > 0 && averageGrade < s.cfg.GradeOutlierThreshold {
			alerts.GradeOutliers = append(alerts.GradeOutliers, classID)
		}
	}

	today := dto.TeacherScheduleSummary{Date: date.Format("2006-01-02")}
	if s.schedules != nil {
		schedules, err := s.schedules.ListByTeacher(ctx, teacherID)
		if err != nil {
			return nil, err
		}
		day := strings.ToUpper(date.Weekday().String())
		for _, sched := range schedules {
			if sched.TermID != termID || strings.ToUpper(sched.DayOfWeek) != day {
				continue
			}
			today.Schedules = append(today.Schedules, dto.TeacherScheduleSlot{
				ClassID:   sched.ClassID,
				SubjectID: sched.SubjectID,
				TimeSlot:  parseTimeSlotInt(sched.TimeSlot),
				Room:      normaliseRoom(sched.Room),
			})
		}
		sort.Slice(today.Schedules, func(i, j int) bool {
			return today.Schedules[i].TimeSlot < today.Schedules[j].TimeSlot
		})
	}

	return &dto.TeacherDashboardResponse{
		TeacherID: teacherID,
		Today:     today,
		Classes:   classSnapshots,
		Alerts:    alerts,
	}, nil
}

func (s *DashboardService) loadAttendance(ctx context.Context, filter models.AnalyticsAttendanceFilter) ([]models.AnalyticsAttendanceSummary, error) {
	if s.analytics != nil {
		if summaries, _, err := s.analytics.Attendance(ctx, filter); err == nil {
			return summaries, nil
		} else {
			s.logger.Warn("analytics attendance failed, fallback to repository", zap.Error(err))
		}
	}
	if s.analyticsRepo != nil {
		return s.analyticsRepo.AttendanceSummary(ctx, filter)
	}
	return nil, appErrors.Clone(appErrors.ErrInternal, "attendance analytics unavailable")
}

func (s *DashboardService) loadGrades(ctx context.Context, filter models.AnalyticsGradeFilter) ([]models.AnalyticsGradeSummary, error) {
	if s.analytics != nil {
		if summaries, _, err := s.analytics.Grades(ctx, filter); err == nil {
			return summaries, nil
		} else {
			s.logger.Warn("analytics grades failed, fallback to repository", zap.Error(err))
		}
	}
	if s.analyticsRepo != nil {
		return s.analyticsRepo.GradeSummary(ctx, filter)
	}
	return nil, appErrors.Clone(appErrors.ErrInternal, "grade analytics unavailable")
}

func (s *DashboardService) loadBehavior(ctx context.Context, filter models.AnalyticsBehaviorFilter) ([]models.AnalyticsBehaviorSummary, error) {
	if s.analytics != nil {
		if summaries, _, err := s.analytics.Behavior(ctx, filter); err == nil {
			return summaries, nil
		} else {
			s.logger.Warn("analytics behavior failed, fallback to repository", zap.Error(err))
		}
	}
	if s.analyticsRepo != nil {
		return s.analyticsRepo.BehaviorSummary(ctx, filter)
	}
	return nil, appErrors.Clone(appErrors.ErrInternal, "behavior analytics unavailable")
}

func (s *DashboardService) buildAdminAttendance(summaries []models.AnalyticsAttendanceSummary) dto.AdminAttendanceSection {
	section := dto.AdminAttendanceSection{}
	if len(summaries) == 0 {
		return section
	}
	var present, absent int
	for _, summary := range summaries {
		present += summary.PresentCount
		absent += summary.AbsentCount
		section.ByClass = append(section.ByClass, dto.AttendanceByClass{
			ClassID: summary.ClassID,
			Rate:    summary.Percentage,
		})
	}
	if total := present + absent; total > 0 {
		section.OverallRate = (float64(present) / float64(total)) * 100
	}
	sort.Slice(section.ByClass, func(i, j int) bool {
		return section.ByClass[i].Rate > section.ByClass[j].Rate
	})
	return section
}

func (s *DashboardService) buildAdminGrades(summaries []models.AnalyticsGradeSummary) dto.AdminGradesSection {
	section := dto.AdminGradesSection{}
	if len(summaries) == 0 {
		section.Distribution = defaultDistributionBins()
		return section
	}
	classAvg := make(map[string]struct {
		total float64
		count int
	})
	distribution := map[string]int{
		"A": 0,
		"B": 0,
		"C": 0,
		"D": 0,
		"E": 0,
	}

	for _, summary := range summaries {
		acc := classAvg[summary.ClassID]
		acc.total += summary.AverageScore
		acc.count++
		classAvg[summary.ClassID] = acc

		if len(summary.Rank) > 0 {
			for _, rank := range summary.Rank {
				distribution[gradeBucket(rank.Score)]++
			}
		} else {
			distribution[gradeBucket(summary.AverageScore)]++
		}
	}

	for classID, acc := range classAvg {
		if acc.count == 0 {
			continue
		}
		section.AverageByClass = append(section.AverageByClass, dto.ClassAverageGrade{
			ClassID: classID,
			Average: acc.total / float64(acc.count),
		})
	}
	sort.Slice(section.AverageByClass, func(i, j int) bool {
		return section.AverageByClass[i].Average > section.AverageByClass[j].Average
	})

	for _, bucket := range []string{"A", "B", "C", "D", "E"} {
		section.Distribution = append(section.Distribution, dto.GradeDistributionBin{
			Bucket: bucket,
			Count:  distribution[bucket],
		})
	}
	return section
}

func (s *DashboardService) buildAdminBehavior(summaries []models.AnalyticsBehaviorSummary) dto.AdminBehaviorSection {
	section := dto.AdminBehaviorSection{}
	if len(summaries) == 0 {
		return section
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].TotalPositive == summaries[j].TotalPositive {
			return summaries[i].Balance > summaries[j].Balance
		}
		return summaries[i].TotalPositive > summaries[j].TotalPositive
	})
	for i := 0; i < len(summaries) && i < s.cfg.BehaviorLeaderboardMax; i++ {
		section.TopPositive = append(section.TopPositive, dto.BehaviorLeaderboardEntry{
			StudentID: summaries[i].StudentID,
			Points:    summaries[i].TotalPositive,
		})
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].TotalNegative == summaries[j].TotalNegative {
			return summaries[i].Balance < summaries[j].Balance
		}
		return summaries[i].TotalNegative > summaries[j].TotalNegative
	})
	for i := 0; i < len(summaries) && i < s.cfg.BehaviorLeaderboardMax; i++ {
		section.TopNegative = append(section.TopNegative, dto.BehaviorLeaderboardEntry{
			StudentID: summaries[i].StudentID,
			Points:    summaries[i].TotalNegative,
		})
	}
	return section
}

func (s *DashboardService) buildOpsHighlights(ctx context.Context) dto.AdminOperationsHighlight {
	highlights := dto.AdminOperationsHighlight{}
	if s.calendar != nil {
		start := s.now().UTC()
		end := start.Add(7 * 24 * time.Hour)
		req := CalendarListRequest{StartDate: &start, EndDate: &end, Page: 1, PageSize: s.cfg.UpcomingEventsLimit}
		if events, _, err := s.calendar.List(ctx, req); err != nil {
			s.logger.Warn("calendar highlight fetch failed", zap.Error(err))
		} else {
			for i, event := range events {
				if i >= s.cfg.UpcomingEventsLimit {
					break
				}
				highlights.UpcomingEvents = append(highlights.UpcomingEvents, dto.OpsEvent{
					ID:    event.ID,
					Title: event.Title,
					Date:  event.StartDate.Format("2006-01-02"),
				})
			}
		}
	}
	if s.announcements != nil {
		req := AnnouncementListRequest{
			AudienceRoles: []models.UserRole{models.RoleAdmin, models.RoleSuperAdmin},
			Page:          1,
			PageSize:      1,
		}
		if _, pagination, err := s.announcements.List(ctx, req); err != nil {
			s.logger.Warn("announcement highlight fetch failed", zap.Error(err))
		} else if pagination != nil {
			highlights.OpenAnnouncements = pagination.TotalCount
		}
	}
	return highlights
}

func (s *DashboardService) averageGradeByClass(summaries []models.AnalyticsGradeSummary) map[string]float64 {
	result := make(map[string]float64)
	if len(summaries) == 0 {
		return result
	}
	acc := make(map[string]struct {
		total float64
		count int
	})
	for _, summary := range summaries {
		current := acc[summary.ClassID]
		current.total += summary.AverageScore
		current.count++
		acc[summary.ClassID] = current
	}
	for classID, data := range acc {
		if data.count == 0 {
			continue
		}
		result[classID] = data.total / float64(data.count)
	}
	return result
}

func gradeBucket(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "E"
	}
}

func defaultDistributionBins() []dto.GradeDistributionBin {
	return []dto.GradeDistributionBin{
		{Bucket: "A"}, {Bucket: "B"}, {Bucket: "C"}, {Bucket: "D"}, {Bucket: "E"},
	}
}

func parseTimeSlotInt(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}

func normaliseRoom(room string) *string {
	room = strings.TrimSpace(room)
	if room == "" {
		return nil
	}
	return &room
}
