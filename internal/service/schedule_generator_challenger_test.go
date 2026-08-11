package service

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
)

type termLookupChallengerStub struct {
	validTerms map[string]bool
}

func (t termLookupChallengerStub) FindByID(ctx context.Context, id string) (*models.Term, error) {
	if t.validTerms != nil && t.validTerms[id] {
		return &models.Term{ID: id}, nil
	}
	return nil, sql.ErrNoRows
}

// TestChallenger_ScheduleGenerator_ZeroClassIDs tests zero class_ids and blank class_id
func TestChallenger_ScheduleGenerator_ZeroClassIDs(t *testing.T) {
	service := newSchedulerServiceFixture(t, schedulerFixtureConfig{})

	req := dto.GenerateScheduleRequest{
		TermID:          "term-1",
		ClassID:         "",
		ClassIDs:        []string{},
		TimeSlotsPerDay: 4,
		Days:            []int{1, 2},
		SubjectLoads: []dto.SubjectLoadRequest{
			{SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 2},
		},
	}

	resp, err := service.Generate(context.Background(), req)
	require.Error(t, err, "Should reject empty classIDs and blank classID")
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "classId or classIds required")
}

// TestChallenger_ScheduleGenerator_InvalidTermID tests non-existent term_id with valid termLookup stub
func TestChallenger_ScheduleGenerator_InvalidTermID(t *testing.T) {
	termsStub := termLookupChallengerStub{validTerms: map[string]bool{"term-valid-1": true}}
	assignments := assignmentRepoSchedulerStub{
		items: []models.TeacherAssignment{
			{SubjectID: "math", TeacherID: "teacher-1"},
		},
	}
	service := NewScheduleGeneratorService(
		termsStub,
		classLookupStub{},
		subjectLookupStub{subjects: map[string]struct{}{"math": {}}},
		assignments,
		preferenceRepoSchedulerStub{},
		scheduleFeederStub{},
		&semesterScheduleRepoStub{},
		&semesterScheduleSlotRepoStub{},
		nil,
		noopTxProvider{},
		validator.New(),
		zap.NewNop(),
		ScheduleGeneratorConfig{ProposalTTL: time.Hour},
	)

	req := dto.GenerateScheduleRequest{
		TermID:          "term-invalid-999",
		ClassID:         "class-1",
		TimeSlotsPerDay: 4,
		Days:            []int{1, 2},
		SubjectLoads: []dto.SubjectLoadRequest{
			{SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 8, Difficulty: 5},
		},
	}

	resp, err := service.Generate(context.Background(), req)
	require.Error(t, err, "Should fail when term does not exist")
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "term not found")
}

// TestChallenger_ScheduleGenerator_OverlappingTeacherSchedules tests teacher conflict across 3 classes
func TestChallenger_ScheduleGenerator_OverlappingTeacherSchedules(t *testing.T) {
	service := newSchedulerServiceFixture(t, schedulerFixtureConfig{})

	// 3 classes all wanting teacher-1 for math (4 slots each, total 12 slots for teacher-1).
	// Total available slots = 2 days * 4 slots/day = 8 slots total!
	// Overbooking teacher-1!
	req := dto.GenerateScheduleRequest{
		TermID:          "term-1",
		ClassIDs:        []string{"class-1", "class-2", "class-3"},
		TimeSlotsPerDay: 4,
		Days:            []int{1, 2},
		SubjectLoads: []dto.SubjectLoadRequest{
			{ClassID: "class-1", SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 4},
			{ClassID: "class-1", SubjectID: "science", TeacherID: "teacher-2", WeeklyCount: 4},
			{ClassID: "class-2", SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 4},
			{ClassID: "class-2", SubjectID: "science", TeacherID: "teacher-2", WeeklyCount: 4},
			{ClassID: "class-3", SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 4},
			{ClassID: "class-3", SubjectID: "science", TeacherID: "teacher-2", WeeklyCount: 4},
		},
	}

	resp, err := service.Generate(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Since teacher-1 has 12 demand slots but only 8 total time-slots exist, conflicts must be captured!
	assert.NotEmpty(t, resp.Conflicts, "Expected unfulfilled load conflicts for overbooked teacher-1")

	// Ensure NO teacher is double-booked in the generated slots
	teacherOccupancy := make(map[string]map[string]string) // teacherID -> "day-slot" -> classID
	for _, slot := range resp.Slots {
		key := fmt.Sprintf("%d-%d", slot.DayOfWeek, slot.TimeSlot)
		if teacherOccupancy[slot.TeacherID] == nil {
			teacherOccupancy[slot.TeacherID] = make(map[string]string)
		}
		existingClass, conflict := teacherOccupancy[slot.TeacherID][key]
		assert.False(t, conflict, "Teacher %s double-booked at day %d slot %d in class %s and %s", slot.TeacherID, slot.DayOfWeek, slot.TimeSlot, existingClass, slot.ClassID)
		teacherOccupancy[slot.TeacherID][key] = slot.ClassID
	}
}

// TestChallenger_ScheduleGenerator_TeacherUnavailableSlots tests teacher blocked slots
func TestChallenger_ScheduleGenerator_TeacherUnavailableSlots(t *testing.T) {
	service := newSchedulerServiceFixture(t, schedulerFixtureConfig{
		preferences: map[string]*models.TeacherPreference{
			"teacher-1": mockPreference("MONDAY", "1"),
		},
	})

	req := dto.GenerateScheduleRequest{
		TermID:          "term-1",
		ClassID:         "class-1",
		TimeSlotsPerDay: 2,
		Days:            []int{1, 2},
		SubjectLoads: []dto.SubjectLoadRequest{
			{SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 2},
			{SubjectID: "science", TeacherID: "teacher-2", WeeklyCount: 2},
		},
	}

	resp, err := service.Generate(context.Background(), req)
	require.NoError(t, err)
	for _, slot := range resp.Slots {
		if slot.TeacherID == "teacher-1" {
			assert.False(t, slot.DayOfWeek == 1 && slot.TimeSlot == 1, "Teacher-1 should not be placed in blocked MONDAY slot 1")
		}
	}
}

// TestChallenger_ScheduleGenerator_ConcurrentGenerateRace tests concurrent Generate requests
func TestChallenger_ScheduleGenerator_ConcurrentGenerateRace(t *testing.T) {
	service := newSchedulerServiceFixture(t, schedulerFixtureConfig{})

	var wg sync.WaitGroup
	concurrentReqs := 20

	for i := 0; i < concurrentReqs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := dto.GenerateScheduleRequest{
				TermID:          "term-1",
				ClassID:         "class-1",
				TimeSlotsPerDay: 2,
				Days:            []int{1, 2},
				SubjectLoads: []dto.SubjectLoadRequest{
					{SubjectID: "math", TeacherID: "teacher-1", WeeklyCount: 2},
					{SubjectID: "science", TeacherID: "teacher-2", WeeklyCount: 2},
				},
			}
			resp, err := service.Generate(context.Background(), req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.NotEmpty(t, resp.ProposalID)
		}(i)
	}

	wg.Wait()
}
