package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type semesterScheduleRepository interface {
	CreateVersioned(ctx context.Context, exec sqlx.ExtContext, schedule *models.SemesterSchedule) error
	ListByTermClass(ctx context.Context, termID, classID string) ([]models.SemesterSchedule, error)
	FindByID(ctx context.Context, id string) (*models.SemesterSchedule, error)
	Delete(ctx context.Context, id string) error
	UpdateStatus(ctx context.Context, exec sqlx.ExtContext, id string, status models.SemesterScheduleStatus, meta types.JSONText) error
}

type semesterScheduleSlotRepository interface {
	UpsertBatch(ctx context.Context, exec sqlx.ExtContext, slots []models.SemesterScheduleSlot) error
	ListBySchedule(ctx context.Context, scheduleID string) ([]models.SemesterScheduleSlot, error)
}

type teacherAssignmentFetcher interface {
	ListByClassAndTerm(ctx context.Context, classID, termID string) ([]models.TeacherAssignment, error)
}

type teacherPreferenceFetcher interface {
	GetByTeacher(ctx context.Context, teacherID string) (*models.TeacherPreference, error)
}

type scheduleFeeder interface {
	ListByTeacher(ctx context.Context, teacherID string) ([]models.Schedule, error)
	ListByClass(ctx context.Context, classID string) ([]models.Schedule, error)
	FindConflicts(ctx context.Context, termID, dayOfWeek, timeSlot string) ([]models.Schedule, error)
	BulkCreateWithTx(ctx context.Context, tx *sqlx.Tx, schedules []models.Schedule) error
}

type schedulerClassReader interface {
	FindByID(ctx context.Context, id string) (*models.Class, error)
}

type schedulerTermReader interface {
	FindByID(ctx context.Context, id string) (*models.Term, error)
}

type schedulerSubjectReader interface {
	FindByID(ctx context.Context, id string) (*models.Subject, error)
}

type txProvider interface {
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
}

type scheduleConflictChecker interface {
	Check(ctx context.Context, termID, classID string, slots []dto.ScheduleSlotProposal) ([]models.ScheduleConflict, error)
}

// ScheduleGeneratorService builds timetable proposals and persists semester schedules.
type ScheduleGeneratorService struct {
	terms       schedulerTermReader
	classes     schedulerClassReader
	subjects    schedulerSubjectReader
	assignments teacherAssignmentFetcher
	prefs       teacherPreferenceFetcher
	schedules   scheduleFeeder
	semesters   semesterScheduleRepository
	slots       semesterScheduleSlotRepository
	conflicts   scheduleConflictChecker
	tx          txProvider
	validator   *validator.Validate
	logger      *zap.Logger
	store       *proposalStore
}

// ScheduleGeneratorConfig governs generator behaviour.
type ScheduleGeneratorConfig struct {
	ProposalTTL time.Duration
}

// NewScheduleGeneratorService wires scheduler dependencies.
func NewScheduleGeneratorService(
	terms schedulerTermReader,
	classes schedulerClassReader,
	subjects schedulerSubjectReader,
	assignments teacherAssignmentFetcher,
	prefs teacherPreferenceFetcher,
	schedules scheduleFeeder,
	semesters semesterScheduleRepository,
	slots semesterScheduleSlotRepository,
	conflictChecker scheduleConflictChecker,
	tx txProvider,
	validate *validator.Validate,
	logger *zap.Logger,
	cfg ScheduleGeneratorConfig,
) *ScheduleGeneratorService {
	if validate == nil {
		validate = validator.New()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.ProposalTTL <= 0 {
		cfg.ProposalTTL = 30 * time.Minute
	}
	if conflictChecker == nil && schedules != nil {
		conflictChecker = &defaultScheduleConflictChecker{repo: schedules}
	}
	return &ScheduleGeneratorService{
		terms:       terms,
		classes:     classes,
		subjects:    subjects,
		assignments: assignments,
		prefs:       prefs,
		schedules:   schedules,
		semesters:   semesters,
		slots:       slots,
		conflicts:   conflictChecker,
		tx:          tx,
		validator:   validate,
		logger:      logger,
		store:       newProposalStore(cfg.ProposalTTL),
	}
}

// Generate orchestrates the constraint-based scheduling pipeline.
func (s *ScheduleGeneratorService) Generate(ctx context.Context, req dto.GenerateScheduleRequest) (*dto.GenerateScheduleResponse, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid schedule generation payload")
	}
	if err := s.ensureTermAndClass(ctx, req.TermID, req.ClassID); err != nil {
		return nil, err
	}

	days := normalizeDays(req.Days)
	if len(days) == 0 {
		return nil, appErrors.Clone(appErrors.ErrValidation, "days must contain at least one entry between 1-6")
	}
	expectedLoad := req.TimeSlotsPerDay * len(days)
	totalLoad := 0
	for _, item := range req.SubjectLoads {
		totalLoad += item.WeeklyCount
	}
	if totalLoad != expectedLoad {
		return nil, appErrors.Clone(appErrors.ErrValidation, fmt.Sprintf("subjectLoads weeklyCount (%d) must equal total weekly slots (%d)", totalLoad, expectedLoad))
	}

	assignments, err := s.assignments.ListByClassAndTerm(ctx, req.ClassID, req.TermID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher assignments")
	}
	if len(assignments) == 0 {
		return nil, appErrors.Clone(appErrors.ErrPreconditionFailed, "no teacher assignments defined for this class and term")
	}

	if err := s.ensureSubjectsExist(ctx, req.SubjectLoads); err != nil {
		return nil, err
	}

	assignmentMap := mapAssignments(assignments)
	if err := validateSubjectLoads(req.SubjectLoads, assignmentMap); err != nil {
		return nil, err
	}

	teacherAvailabilities, err := s.buildTeacherAvailability(ctx, req.TermID, assignmentMap, req.SubjectLoads)
	if err != nil {
		return nil, err
	}

	state := newSchedulerState(days, req.TimeSlotsPerDay, teacherAvailabilities)
	conflicts := s.seedSlots(state, req.SubjectLoads)
	improvements := state.repairGaps(12)

	slots := state.exportSlots()
	gapPenalty := calculateGapPenalty(days, req.TimeSlotsPerDay, slots)
	loadPenalty := calculateLoadPenalty(teacherAvailabilities)
	conflictPenalty := float64(len(conflicts))
	score := math.Max(0, 100-(conflictPenalty*100+gapPenalty*2+loadPenalty*5))

	proposal := scheduleProposal{
		ProposalID:      uuid.NewString(),
		TermID:          req.TermID,
		ClassID:         req.ClassID,
		Score:           score,
		Slots:           slots,
		Conflicts:       conflicts,
		Stats:           dto.ScheduleImprovementStats{Iterations: improvements, GapPenalty: gapPenalty, LoadPenalty: loadPenalty},
		TimeSlotsPerDay: req.TimeSlotsPerDay,
		Days:            days,
		SubjectLoads:    req.SubjectLoads,
		RequestedAt:     time.Now().UTC(),
		Meta: map[string]any{
			"hardConstraints": req.HardConstraints,
			"softConstraints": req.SoftConstraints,
		},
	}
	s.store.Save(proposal)

	resp := &dto.GenerateScheduleResponse{
		ProposalID: proposal.ProposalID,
		Score:      score,
		Slots:      slots,
		Conflicts:  conflicts,
		Stats:      proposal.Stats,
	}
	return resp, nil
}

// Save persists a validated proposal as a semester schedule and optionally daily schedules.
func (s *ScheduleGeneratorService) Save(ctx context.Context, req dto.SaveScheduleRequest) (string, error) {
	if err := s.validator.Struct(req); err != nil {
		return "", appErrors.Wrap(err, appErrors.ErrValidation.Code, appErrors.ErrValidation.Status, "invalid save schedule payload")
	}
	proposal, ok := s.store.Get(req.ProposalID)
	if !ok {
		return "", appErrors.Clone(appErrors.ErrNotFound, "proposal not found or expired")
	}
	if len(proposal.Conflicts) > 0 {
		return "", appErrors.Clone(appErrors.ErrConflict, "proposal contains unresolved conflicts")
	}
	if s.tx == nil {
		return "", appErrors.Clone(appErrors.ErrInternal, "transaction provider missing")
	}

	tx, err := s.tx.BeginTxx(ctx, nil)
	if err != nil {
		return "", appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to begin transaction")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	metaPayload := map[string]any{
		"score":      proposal.Score,
		"stats":      proposal.Stats,
		"generated":  proposal.RequestedAt,
		"days":       proposal.Days,
		"timeSlots":  proposal.TimeSlotsPerDay,
		"algorithm":  "heuristic_v1",
		"subjectMap": proposal.SubjectLoads,
	}
	metaBytes, marshalErr := json.Marshal(metaPayload)
	if marshalErr != nil {
		err = appErrors.Wrap(marshalErr, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to encode schedule metadata")
		return "", err
	}

	record := &models.SemesterSchedule{
		TermID:  proposal.TermID,
		ClassID: proposal.ClassID,
		Status:  models.SemesterScheduleStatusDraft,
		Meta:    types.JSONText(metaBytes),
	}

	if err = s.semesters.CreateVersioned(ctx, tx, record); err != nil {
		err = appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to create semester schedule")
		return "", err
	}

	slotModels := make([]models.SemesterScheduleSlot, 0, len(proposal.Slots))
	for _, slot := range proposal.Slots {
		slotModels = append(slotModels, models.SemesterScheduleSlot{
			SemesterScheduleID: record.ID,
			DayOfWeek:          slot.DayOfWeek,
			TimeSlot:           slot.TimeSlot,
			SubjectID:          slot.SubjectID,
			TeacherID:          slot.TeacherID,
			Room:               slot.Room,
		})
	}

	if err = s.slots.UpsertBatch(ctx, tx, slotModels); err != nil {
		err = appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to persist semester schedule slots")
		return "", err
	}

	if req.CommitToDaily {
		if s.conflicts == nil {
			err = appErrors.Clone(appErrors.ErrInternal, "schedule conflict checker unavailable")
			return "", err
		}
		conflicts, conflictErr := s.conflicts.Check(ctx, proposal.TermID, proposal.ClassID, proposal.Slots)
		if conflictErr != nil {
			err = conflictErr
			return "", err
		}
		if len(conflicts) > 0 {
			err = appErrors.Wrap(&models.ScheduleConflictError{Type: "CONFLICT", Message: "detected conflicts when committing to daily schedules", Errors: conflicts}, appErrors.ErrConflict.Code, appErrors.ErrConflict.Status, "conflict detected")
			return "", err
		}

		daily := make([]models.Schedule, 0, len(proposal.Slots))
		for _, slot := range proposal.Slots {
			daily = append(daily, models.Schedule{
				TermID:    proposal.TermID,
				ClassID:   proposal.ClassID,
				SubjectID: slot.SubjectID,
				TeacherID: slot.TeacherID,
				DayOfWeek: dayIndexToName(slot.DayOfWeek),
				TimeSlot:  strconv.Itoa(slot.TimeSlot),
				Room:      slotRoomValue(slot),
			})
		}
		if err = s.schedules.BulkCreateWithTx(ctx, tx, daily); err != nil {
			err = appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to commit daily schedules")
			return "", err
		}
		if err = s.semesters.UpdateStatus(ctx, tx, record.ID, models.SemesterScheduleStatusPublished, nil); err != nil {
			err = appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update schedule status")
			return "", err
		}
	}

	if err = tx.Commit(); err != nil {
		err = appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to commit schedule transaction")
		return "", err
	}

	s.store.Delete(req.ProposalID)
	return record.ID, nil
}

// List returns semester schedules for a class-term tuple.
func (s *ScheduleGeneratorService) List(ctx context.Context, query dto.SemesterScheduleQuery) ([]models.SemesterSchedule, error) {
	if query.TermID == "" || query.ClassID == "" {
		return nil, appErrors.Clone(appErrors.ErrValidation, "termId and classId are required")
	}
	list, err := s.semesters.ListByTermClass(ctx, query.TermID, query.ClassID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list semester schedules")
	}
	return list, nil
}

// GetSlots returns slot detail for a stored schedule.
func (s *ScheduleGeneratorService) GetSlots(ctx context.Context, scheduleID string) ([]models.SemesterScheduleSlot, error) {
	if scheduleID == "" {
		return nil, appErrors.Clone(appErrors.ErrValidation, "schedule id is required")
	}
	if _, err := s.semesters.FindByID(ctx, scheduleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Clone(appErrors.ErrNotFound, "semester schedule not found")
		}
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load semester schedule")
	}
	slots, err := s.slots.ListBySchedule(ctx, scheduleID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to list semester schedule slots")
	}
	return slots, nil
}

// Delete removes a draft schedule version.
func (s *ScheduleGeneratorService) Delete(ctx context.Context, scheduleID string) error {
	record, err := s.semesters.FindByID(ctx, scheduleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.Clone(appErrors.ErrNotFound, "semester schedule not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load semester schedule")
	}
	if record.Status != models.SemesterScheduleStatusDraft {
		return appErrors.Clone(appErrors.ErrConflict, "only draft schedules can be deleted")
	}
	if err := s.semesters.Delete(ctx, scheduleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.Clone(appErrors.ErrNotFound, "semester schedule not found")
		}
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to delete semester schedule")
	}
	return nil
}

func (s *ScheduleGeneratorService) ensureTermAndClass(ctx context.Context, termID, classID string) error {
	if s.terms != nil {
		if _, err := s.terms.FindByID(ctx, termID); err != nil {
			if err == sql.ErrNoRows {
				return appErrors.Clone(appErrors.ErrNotFound, "term not found")
			}
			return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load term")
		}
	}
	if s.classes != nil {
		if _, err := s.classes.FindByID(ctx, classID); err != nil {
			if err == sql.ErrNoRows {
				return appErrors.Clone(appErrors.ErrNotFound, "class not found")
			}
			return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load class")
		}
	}
	return nil
}

func (s *ScheduleGeneratorService) ensureSubjectsExist(ctx context.Context, loads []dto.SubjectLoadRequest) error {
	if s.subjects == nil {
		return nil
	}
	checked := make(map[string]bool, len(loads))
	for _, load := range loads {
		if checked[load.SubjectID] {
			continue
		}
		if _, err := s.subjects.FindByID(ctx, load.SubjectID); err != nil {
			if err == sql.ErrNoRows {
				return appErrors.Clone(appErrors.ErrNotFound, fmt.Sprintf("subject %s not found", load.SubjectID))
			}
			return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load subject")
		}
		checked[load.SubjectID] = true
	}
	return nil
}

func (s *ScheduleGeneratorService) buildTeacherAvailability(
	ctx context.Context,
	termID string,
	assignments map[string]map[string]bool,
	loads []dto.SubjectLoadRequest,
) (map[string]*teacherAvailability, error) {
	teachers := map[string]struct{}{}
	for _, teacherMap := range assignments {
		for teacherID := range teacherMap {
			teachers[teacherID] = struct{}{}
		}
	}
	for _, load := range loads {
		teachers[load.TeacherID] = struct{}{}
	}

	result := make(map[string]*teacherAvailability, len(teachers))
	for teacherID := range teachers {
		var pref *models.TeacherPreference
		var err error
		if s.prefs != nil {
			pref, err = s.prefs.GetByTeacher(ctx, teacherID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher preferences")
			}
		}
		availability := newTeacherAvailability()
		if pref != nil {
			availability.MaxLoadPerDay = pref.MaxLoadPerDay
			availability.MaxLoadPerWeek = pref.MaxLoadPerWeek
			if len(pref.Unavailable) > 0 {
				var windows []models.TeacherUnavailableSlot
				_ = json.Unmarshal(pref.Unavailable, &windows) // safe best-effort
				for _, window := range windows {
					day := dayStringToIndex(window.DayOfWeek)
					if day == 0 {
						continue
					}
					for _, slot := range expandTimeRange(window.TimeRange) {
						availability.Block(day, slot)
					}
				}
			}
		}

		if s.schedules != nil {
			existing, err := s.schedules.ListByTeacher(ctx, teacherID)
			if err != nil {
				return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load teacher schedules")
			}
			for _, sched := range existing {
				if sched.TermID != termID {
					continue
				}
				day := dayStringToIndex(sched.DayOfWeek)
				slot := parseTimeSlot(sched.TimeSlot)
				if day == 0 || slot == 0 {
					continue
				}
				availability.Block(day, slot)
			}
		}
		result[teacherID] = availability
	}
	return result, nil
}

func (s *ScheduleGeneratorService) seedSlots(state *schedulerState, loads []dto.SubjectLoadRequest) []dto.ProposalConflict {
	conflicts := make([]dto.ProposalConflict, 0)
	sorted := make([]dto.SubjectLoadRequest, len(loads))
	copy(sorted, loads)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Difficulty == sorted[j].Difficulty {
			return sorted[i].WeeklyCount > sorted[j].WeeklyCount
		}
		return sorted[i].Difficulty > sorted[j].Difficulty
	})

	for _, load := range sorted {
		for i := 0; i < load.WeeklyCount; i++ {
			if state.Assign(load) {
				continue
			}
			conflicts = append(conflicts, dto.ProposalConflict{
				Type:    "UNFULFILLED_LOAD",
				Message: fmt.Sprintf("unable to schedule subject %s for teacher %s", load.SubjectID, load.TeacherID),
				Meta: map[string]any{
					"subjectId": load.SubjectID,
					"teacherId": load.TeacherID,
				},
			})
		}
	}
	return conflicts
}

func mapAssignments(items []models.TeacherAssignment) map[string]map[string]bool {
	result := make(map[string]map[string]bool)
	for _, item := range items {
		if result[item.SubjectID] == nil {
			result[item.SubjectID] = make(map[string]bool)
		}
		result[item.SubjectID][item.TeacherID] = true
	}
	return result
}

func validateSubjectLoads(loads []dto.SubjectLoadRequest, assignments map[string]map[string]bool) error {
	for _, load := range loads {
		if load.WeeklyCount <= 0 {
			return appErrors.Clone(appErrors.ErrValidation, fmt.Sprintf("subject %s weeklyCount must be > 0", load.SubjectID))
		}
		if load.SubjectID == "" || load.TeacherID == "" {
			return appErrors.Clone(appErrors.ErrValidation, "subjectId and teacherId are required for subjectLoads")
		}
		if teachers, ok := assignments[load.SubjectID]; ok {
			if !teachers[load.TeacherID] {
				return appErrors.Clone(appErrors.ErrValidation, fmt.Sprintf("teacher %s is not assigned to subject %s", load.TeacherID, load.SubjectID))
			}
		}
	}
	return nil
}

// --- Proposal cache ---

type scheduleProposal struct {
	ProposalID      string
	TermID          string
	ClassID         string
	Score           float64
	Slots           []dto.ScheduleSlotProposal
	Conflicts       []dto.ProposalConflict
	Stats           dto.ScheduleImprovementStats
	TimeSlotsPerDay int
	Days            []int
	SubjectLoads    []dto.SubjectLoadRequest
	RequestedAt     time.Time
	Meta            map[string]any
}

type proposalStore struct {
	ttl   time.Duration
	mu    sync.RWMutex
	items map[string]scheduleProposal
}

func newProposalStore(ttl time.Duration) *proposalStore {
	return &proposalStore{
		ttl:   ttl,
		items: make(map[string]scheduleProposal),
	}
}

func (s *proposalStore) Save(proposal scheduleProposal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[proposal.ProposalID] = proposal
}

func (s *proposalStore) Get(id string) (scheduleProposal, bool) {
	s.mu.RLock()
	proposal, ok := s.items[id]
	s.mu.RUnlock()
	if !ok {
		return scheduleProposal{}, false
	}
	if time.Since(proposal.RequestedAt) > s.ttl {
		s.Delete(id)
		return scheduleProposal{}, false
	}
	return proposal, true
}

func (s *proposalStore) Delete(id string) {
	s.mu.Lock()
	delete(s.items, id)
	s.mu.Unlock()
}

// --- Scheduler state & helpers ---

type slotKey struct {
	Day  int
	Time int
}

type schedulerState struct {
	days           []int
	timeSlots      int
	classSlots     map[slotKey]dto.ScheduleSlotProposal
	dayLoad        map[int]int
	teacherLoads   map[string]*teacherAvailability
	preferredCache map[string][]int
}

func newSchedulerState(days []int, timeSlots int, loads map[string]*teacherAvailability) *schedulerState {
	return &schedulerState{
		days:           days,
		timeSlots:      timeSlots,
		classSlots:     make(map[slotKey]dto.ScheduleSlotProposal),
		dayLoad:        make(map[int]int),
		teacherLoads:   loads,
		preferredCache: make(map[string][]int),
	}
}

func (s *schedulerState) Assign(load dto.SubjectLoadRequest) bool {
	dayOrder := make([]int, len(s.days))
	copy(dayOrder, s.days)
	sort.Slice(dayOrder, func(i, j int) bool {
		return s.dayLoad[dayOrder[i]] < s.dayLoad[dayOrder[j]]
	})

	candidateTimes := s.candidateTimes(load)
	for _, day := range dayOrder {
		for _, slot := range candidateTimes {
			if s.canPlace(load.TeacherID, day, slot) {
				s.place(load, day, slot)
				return true
			}
		}
	}
	return false
}

func (s *schedulerState) candidateTimes(load dto.SubjectLoadRequest) []int {
	var result []int
	seen := make(map[int]bool)
	for _, slot := range load.Preferred {
		if slot < 1 || slot > s.timeSlots || seen[slot] {
			continue
		}
		result = append(result, slot)
		seen[slot] = true
	}
	for slot := 1; slot <= s.timeSlots; slot++ {
		if seen[slot] {
			continue
		}
		result = append(result, slot)
	}
	return result
}

func (s *schedulerState) canPlace(teacherID string, day, slot int) bool {
	if day < 1 || slot < 1 || slot > s.timeSlots {
		return false
	}
	key := slotKey{Day: day, Time: slot}
	if _, exists := s.classSlots[key]; exists {
		return false
	}
	teacher := s.teacherLoads[teacherID]
	if teacher == nil {
		return false
	}
	return teacher.CanTeach(day, slot)
}

func (s *schedulerState) place(load dto.SubjectLoadRequest, day, slot int) {
	key := slotKey{Day: day, Time: slot}
	s.classSlots[key] = dto.ScheduleSlotProposal{
		DayOfWeek: day,
		TimeSlot:  slot,
		SubjectID: load.SubjectID,
		TeacherID: load.TeacherID,
	}
	s.teacherLoads[load.TeacherID].Reserve(day, slot)
	s.dayLoad[day]++
}

func (s *schedulerState) repairGaps(maxIterations int) int {
	iterations := 0
	for iterations < maxIterations {
		moved := false
		for _, day := range s.days {
			times := s.timesForDay(day)
			if len(times) < 2 {
				continue
			}
			for i := 0; i < len(times)-1; i++ {
				current := times[i]
				next := times[i+1]
				if next-current <= 1 {
					continue
				}
				target := current + 1
				slot := s.classSlots[slotKey{Day: day, Time: next}]
				if s.canPlace(slot.TeacherID, day, target) {
					s.moveSlot(day, next, target)
					moved = true
					break
				}
			}
			if moved {
				break
			}
		}
		if !moved {
			break
		}
		iterations++
	}
	return iterations
}

func (s *schedulerState) timesForDay(day int) []int {
	var times []int
	for key := range s.classSlots {
		if key.Day == day {
			times = append(times, key.Time)
		}
	}
	sort.Ints(times)
	return times
}

func (s *schedulerState) moveSlot(day, fromSlot, toSlot int) {
	key := slotKey{Day: day, Time: fromSlot}
	slot := s.classSlots[key]
	delete(s.classSlots, key)
	s.teacherLoads[slot.TeacherID].Release(day, fromSlot)

	slot.TimeSlot = toSlot
	s.classSlots[slotKey{Day: day, Time: toSlot}] = slot
	s.teacherLoads[slot.TeacherID].Reserve(day, toSlot)
}

func (s *schedulerState) exportSlots() []dto.ScheduleSlotProposal {
	slots := make([]dto.ScheduleSlotProposal, 0, len(s.classSlots))
	for _, slot := range s.classSlots {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].DayOfWeek == slots[j].DayOfWeek {
			return slots[i].TimeSlot < slots[j].TimeSlot
		}
		return slots[i].DayOfWeek < slots[j].DayOfWeek
	})
	return slots
}

// --- Teacher availability ---

type teacherAvailability struct {
	MaxLoadPerDay  int
	MaxLoadPerWeek int
	perDay         map[int]int
	weekly         int
	blocked        map[int]map[int]bool
	assigned       map[int]map[int]bool
}

func newTeacherAvailability() *teacherAvailability {
	return &teacherAvailability{
		perDay:   make(map[int]int),
		blocked:  make(map[int]map[int]bool),
		assigned: make(map[int]map[int]bool),
	}
}

func (t *teacherAvailability) Block(day, slot int) {
	if t.blocked[day] == nil {
		t.blocked[day] = make(map[int]bool)
	}
	t.blocked[day][slot] = true
}

func (t *teacherAvailability) CanTeach(day, slot int) bool {
	if t.blocked[day] != nil && t.blocked[day][slot] {
		return false
	}
	if t.assigned[day] != nil && t.assigned[day][slot] {
		return false
	}
	if t.MaxLoadPerDay > 0 && t.perDay[day] >= t.MaxLoadPerDay {
		return false
	}
	if t.MaxLoadPerWeek > 0 && t.weekly >= t.MaxLoadPerWeek {
		return false
	}
	return true
}

func (t *teacherAvailability) Reserve(day, slot int) {
	if t.assigned[day] == nil {
		t.assigned[day] = make(map[int]bool)
	}
	t.assigned[day][slot] = true
	t.perDay[day]++
	t.weekly++
}

func (t *teacherAvailability) Release(day, slot int) {
	if t.assigned[day] != nil {
		delete(t.assigned[day], slot)
	}
	if t.perDay[day] > 0 {
		t.perDay[day]--
	}
	if t.weekly > 0 {
		t.weekly--
	}
}

// --- Metrics helpers ---

func calculateGapPenalty(days []int, slotsPerDay int, slots []dto.ScheduleSlotProposal) float64 {
	var penalty float64
	for _, day := range days {
		var times []int
		for _, slot := range slots {
			if slot.DayOfWeek == day {
				times = append(times, slot.TimeSlot)
			}
		}
		if len(times) <= 1 {
			continue
		}
		sort.Ints(times)
		for i := 0; i < len(times)-1; i++ {
			diff := times[i+1] - times[i]
			if diff > 1 {
				penalty += float64(diff - 1)
			}
		}
		penalty += float64(slotsPerDay - len(times))
	}
	return penalty
}

func calculateLoadPenalty(loads map[string]*teacherAvailability) float64 {
	var penalty float64
	for _, load := range loads {
		if load.MaxLoadPerWeek > 0 && load.weekly > load.MaxLoadPerWeek {
			penalty += float64(load.weekly - load.MaxLoadPerWeek)
		}
		for _, count := range load.perDay {
			if load.MaxLoadPerDay > 0 && count > load.MaxLoadPerDay {
				penalty += float64(count - load.MaxLoadPerDay)
			}
		}
	}
	return penalty
}

func expandTimeRange(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.Contains(raw, "-") {
		parts := strings.SplitN(raw, "-", 2)
		start := parseTimeSlot(parts[0])
		end := parseTimeSlot(parts[1])
		if start == 0 || end == 0 || end < start {
			return nil
		}
		var slots []int
		for i := start; i <= end; i++ {
			slots = append(slots, i)
		}
		return slots
	}
	value := parseTimeSlot(raw)
	if value == 0 {
		return nil
	}
	return []int{value}
}

func parseTimeSlot(raw string) int {
	raw = strings.TrimSpace(raw)
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}

func normalizeDays(days []int) []int {
	unique := make(map[int]struct{})
	for _, day := range days {
		if day < 1 || day > 7 {
			continue
		}
		unique[day] = struct{}{}
	}
	result := make([]int, 0, len(unique))
	for day := range unique {
		result = append(result, day)
	}
	sort.Ints(result)
	return result
}

var dayIndexMap = map[int]string{
	1: "MONDAY",
	2: "TUESDAY",
	3: "WEDNESDAY",
	4: "THURSDAY",
	5: "FRIDAY",
	6: "SATURDAY",
	7: "SUNDAY",
}

var dayNameIndex = map[string]int{
	"MONDAY":    1,
	"TUESDAY":   2,
	"WEDNESDAY": 3,
	"THURSDAY":  4,
	"FRIDAY":    5,
	"SATURDAY":  6,
	"SUNDAY":    7,
}

func dayIndexToName(day int) string {
	if name, ok := dayIndexMap[day]; ok {
		return name
	}
	return "MONDAY"
}

func dayStringToIndex(name string) int {
	return dayNameIndex[strings.ToUpper(strings.TrimSpace(name))]
}

func slotRoomValue(slot dto.ScheduleSlotProposal) string {
	if slot.Room == nil {
		return ""
	}
	return *slot.Room
}

// --- Conflict checker ---

type defaultScheduleConflictChecker struct {
	repo scheduleFeeder
}

func (d *defaultScheduleConflictChecker) Check(ctx context.Context, termID, classID string, slots []dto.ScheduleSlotProposal) ([]models.ScheduleConflict, error) {
	var conflicts []models.ScheduleConflict
	for _, slot := range slots {
		existing, err := d.repo.FindConflicts(ctx, termID, dayIndexToName(slot.DayOfWeek), strconv.Itoa(slot.TimeSlot))
		if err != nil {
			return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to check conflicts")
		}
		for _, sched := range existing {
			if sched.ClassID == classID {
				conflicts = append(conflicts, models.ScheduleConflict{
					ScheduleID: sched.ID,
					TermID:     sched.TermID,
					ClassID:    sched.ClassID,
					SubjectID:  sched.SubjectID,
					TeacherID:  sched.TeacherID,
					DayOfWeek:  sched.DayOfWeek,
					TimeSlot:   sched.TimeSlot,
					Room:       sched.Room,
					Dimension:  "CLASS",
				})
			}
			if sched.TeacherID == slot.TeacherID {
				conflicts = append(conflicts, models.ScheduleConflict{
					ScheduleID: sched.ID,
					TermID:     sched.TermID,
					ClassID:    sched.ClassID,
					SubjectID:  sched.SubjectID,
					TeacherID:  sched.TeacherID,
					DayOfWeek:  sched.DayOfWeek,
					TimeSlot:   sched.TimeSlot,
					Room:       sched.Room,
					Dimension:  "TEACHER",
				})
			}
			if sched.Room != "" && slot.Room != nil && *slot.Room != "" && sched.Room == *slot.Room {
				conflicts = append(conflicts, models.ScheduleConflict{
					ScheduleID: sched.ID,
					TermID:     sched.TermID,
					ClassID:    sched.ClassID,
					SubjectID:  sched.SubjectID,
					TeacherID:  sched.TeacherID,
					DayOfWeek:  sched.DayOfWeek,
					TimeSlot:   sched.TimeSlot,
					Room:       sched.Room,
					Dimension:  "ROOM",
				})
			}
		}
	}
	return conflicts, nil
}
