package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

func TestScheduleGeneratorServiceGenerateSuccess(t *testing.T) {
	service := newSchedulerServiceFixture(t, schedulerFixtureConfig{})

	resp, err := service.Generate(context.Background(), dto.GenerateScheduleRequest{
		TermID:          "term-1",
		ClassID:         "class-1",
		TimeSlotsPerDay: 2,
		Days:            []int{1, 2},
		SubjectLoads: []dto.SubjectLoadRequest{
			{SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 2, Difficulty: 5},
			{SubjectID: "science", TeacherID: "teacher-2", WeeklyCount: 2, Difficulty: 3},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 4, len(resp.Slots))
	assert.Empty(t, resp.Conflicts)
	assert.Greater(t, resp.Score, 0.0)
}

func TestScheduleGeneratorServiceGenerateHonoursUnavailable(t *testing.T) {
	service := newSchedulerServiceFixture(t, schedulerFixtureConfig{
		preferences: map[string]*models.TeacherPreference{
			"teacher-1": mockPreference("MONDAY", "1"),
		},
	})

	resp, err := service.Generate(context.Background(), dto.GenerateScheduleRequest{
		TermID:          "term-1",
		ClassID:         "class-1",
		TimeSlotsPerDay: 2,
		Days:            []int{1, 2},
		SubjectLoads: []dto.SubjectLoadRequest{
			{SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 2},
			{SubjectID: "science", TeacherID: "teacher-2", WeeklyCount: 2},
		},
	})
	require.NoError(t, err)
	for _, slot := range resp.Slots {
		if slot.TeacherID == "teacher-1" && slot.DayOfWeek == 1 {
			assert.NotEqual(t, 1, slot.TimeSlot, "blocked slot should not be used by teacher-1")
		}
	}
}

func TestScheduleGeneratorServiceSaveDraft(t *testing.T) {
	txProvider, mock := newTxProviderMock(t)
	service := newSchedulerServiceFixture(t, schedulerFixtureConfig{tx: txProvider})

	resp, err := service.Generate(context.Background(), dto.GenerateScheduleRequest{
		TermID:          "term-1",
		ClassID:         "class-1",
		TimeSlotsPerDay: 2,
		Days:            []int{1, 2},
		SubjectLoads: []dto.SubjectLoadRequest{
			{SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 2},
			{SubjectID: "science", TeacherID: "teacher-2", WeeklyCount: 2},
		},
	})
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectCommit()

	id, err := service.Save(context.Background(), dto.SaveScheduleRequest{ProposalID: resp.ProposalID})
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduleGeneratorServiceSaveConflict(t *testing.T) {
	txProvider, mock := newTxProviderMock(t)
	conflictErr := &models.ScheduleConflictError{Type: "CLASS", Message: "conflict"}
	conflictChecker := conflictCheckerStub{
		conflicts: []models.ScheduleConflict{{Dimension: "CLASS"}},
		err:       appErrors.Wrap(conflictErr, appErrors.ErrConflict.Code, appErrors.ErrConflict.Status, "conflict"),
	}
	service := newSchedulerServiceFixture(t, schedulerFixtureConfig{
		tx:        txProvider,
		conflicts: conflictChecker,
	})

	resp, err := service.Generate(context.Background(), dto.GenerateScheduleRequest{
		TermID:          "term-1",
		ClassID:         "class-1",
		TimeSlotsPerDay: 2,
		Days:            []int{1, 2},
		SubjectLoads: []dto.SubjectLoadRequest{
			{SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 2},
			{SubjectID: "science", TeacherID: "teacher-2", WeeklyCount: 2},
		},
	})
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err = service.Save(context.Background(), dto.SaveScheduleRequest{ProposalID: resp.ProposalID, CommitToDaily: true})
	require.Error(t, err)
	appErr := appErrors.FromError(err)
	assert.Equal(t, appErrors.ErrConflict.Code, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- Fixtures ---

type schedulerFixtureConfig struct {
	preferences map[string]*models.TeacherPreference
	tx          txProvider
	conflicts   scheduleConflictChecker
}

func newSchedulerServiceFixture(t *testing.T, cfg schedulerFixtureConfig) *ScheduleGeneratorService {
	assignments := assignmentRepoSchedulerStub{
		items: []models.TeacherAssignment{
			{SubjectID: "math", TeacherID: "teacher-1"},
			{SubjectID: "science", TeacherID: "teacher-2"},
		},
	}
	prefs := preferenceRepoSchedulerStub{items: cfg.preferences}
	semesters := &semesterScheduleRepoStub{}
	slots := &semesterScheduleSlotRepoStub{}
	subjects := subjectLookupStub{subjects: map[string]struct{}{"math": {}, "science": {}}}
	terms := termLookupStub{}
	classes := classLookupStub{}
	schedules := scheduleFeederStub{}
	conflicts := cfg.conflicts
	if conflicts == nil {
		conflicts = &defaultScheduleConflictChecker{repo: schedules}
	}
	tx := cfg.tx
	if tx == nil {
		tx = noopTxProvider{}
	}

	return NewScheduleGeneratorService(
		terms,
		classes,
		subjects,
		assignments,
		prefs,
		schedules,
		semesters,
		slots,
		conflicts,
		tx,
		validator.New(),
		zap.NewNop(),
		ScheduleGeneratorConfig{ProposalTTL: time.Hour},
	)
}

type assignmentRepoSchedulerStub struct {
	items []models.TeacherAssignment
}

func (s assignmentRepoSchedulerStub) ListByClassAndTerm(ctx context.Context, classID, termID string) ([]models.TeacherAssignment, error) {
	return s.items, nil
}

type preferenceRepoSchedulerStub struct {
	items map[string]*models.TeacherPreference
}

func (s preferenceRepoSchedulerStub) GetByTeacher(ctx context.Context, teacherID string) (*models.TeacherPreference, error) {
	if s.items == nil {
		return nil, sql.ErrNoRows
	}
	if pref, ok := s.items[teacherID]; ok {
		return pref, nil
	}
	return nil, sql.ErrNoRows
}

type semesterScheduleRepoStub struct {
	items []models.SemesterSchedule
}

func (s *semesterScheduleRepoStub) CreateVersioned(ctx context.Context, exec sqlx.ExtContext, schedule *models.SemesterSchedule) error {
	schedule.ID = uuidString(len(s.items) + 1)
	schedule.Version = len(s.items) + 1
	s.items = append(s.items, *schedule)
	return nil
}

func (s *semesterScheduleRepoStub) ListByTermClass(ctx context.Context, termID, classID string) ([]models.SemesterSchedule, error) {
	return s.items, nil
}

func (s *semesterScheduleRepoStub) FindByID(ctx context.Context, id string) (*models.SemesterSchedule, error) {
	for _, item := range s.items {
		if item.ID == id {
			return &item, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *semesterScheduleRepoStub) Delete(ctx context.Context, id string) error {
	for idx, item := range s.items {
		if item.ID == id {
			s.items = append(s.items[:idx], s.items[idx+1:]...)
			return nil
		}
	}
	return sql.ErrNoRows
}

func (s *semesterScheduleRepoStub) UpdateStatus(ctx context.Context, exec sqlx.ExtContext, id string, status models.SemesterScheduleStatus, meta types.JSONText) error {
	for idx := range s.items {
		if s.items[idx].ID == id {
			s.items[idx].Status = status
			return nil
		}
	}
	return sql.ErrNoRows
}

type semesterScheduleSlotRepoStub struct {
	items map[string][]models.SemesterScheduleSlot
}

func (s *semesterScheduleSlotRepoStub) UpsertBatch(ctx context.Context, exec sqlx.ExtContext, slots []models.SemesterScheduleSlot) error {
	if s.items == nil {
		s.items = make(map[string][]models.SemesterScheduleSlot)
	}
	for _, slot := range slots {
		s.items[slot.SemesterScheduleID] = append(s.items[slot.SemesterScheduleID], slot)
	}
	return nil
}

func (s *semesterScheduleSlotRepoStub) ListBySchedule(ctx context.Context, scheduleID string) ([]models.SemesterScheduleSlot, error) {
	return s.items[scheduleID], nil
}

type subjectLookupStub struct {
	subjects map[string]struct{}
}

func (s subjectLookupStub) FindByID(ctx context.Context, id string) (*models.Subject, error) {
	if _, ok := s.subjects[id]; !ok {
		return nil, sql.ErrNoRows
	}
	return &models.Subject{ID: id}, nil
}

type termLookupStub struct{}

func (termLookupStub) FindByID(ctx context.Context, id string) (*models.Term, error) {
	return &models.Term{ID: id}, nil
}

type classLookupStub struct{}

func (classLookupStub) FindByID(ctx context.Context, id string) (*models.Class, error) {
	return &models.Class{ID: id}, nil
}

type scheduleFeederStub struct {
	teacherSchedules map[string][]models.Schedule
}

func (s scheduleFeederStub) ListByTeacher(ctx context.Context, teacherID string) ([]models.Schedule, error) {
	return s.teacherSchedules[teacherID], nil
}

func (scheduleFeederStub) ListByClass(ctx context.Context, classID string) ([]models.Schedule, error) {
	return nil, nil
}

func (scheduleFeederStub) FindConflicts(ctx context.Context, termID, dayOfWeek, timeSlot string) ([]models.Schedule, error) {
	return nil, nil
}

func (scheduleFeederStub) BulkCreateWithTx(ctx context.Context, tx *sqlx.Tx, schedules []models.Schedule) error {
	return nil
}

type noopTxProvider struct{}

func (noopTxProvider) BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error) {
	return nil, appErrors.Clone(appErrors.ErrInternal, "transaction provider unavailable")
}

type conflictCheckerStub struct {
	conflicts []models.ScheduleConflict
	err       error
}

func (c conflictCheckerStub) Check(ctx context.Context, termID, classID string, slots []dto.ScheduleSlotProposal) ([]models.ScheduleConflict, error) {
	return c.conflicts, c.err
}

type txProviderMock struct {
	db   *sqlx.DB
	mock sqlmock.Sqlmock
}

func newTxProviderMock(t *testing.T) (txProvider, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	sqlxdb := sqlx.NewDb(db, "sqlmock")
	t.Cleanup(func() { db.Close() })
	return &txProviderMock{db: sqlxdb, mock: mock}, mock
}

func (t *txProviderMock) BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error) {
	return t.db.BeginTxx(ctx, opts)
}

func uuidString(v int) string {
	return fmt.Sprintf("sched-%d", v)
}

func mockPreference(day, slot string) *models.TeacherPreference {
	payload, _ := json.Marshal([]models.TeacherUnavailableSlot{{DayOfWeek: day, TimeRange: slot}})
	return &models.TeacherPreference{
		TeacherID:      "teacher-1",
		MaxLoadPerDay:  0,
		MaxLoadPerWeek: 0,
		Unavailable:    payload,
	}
}
