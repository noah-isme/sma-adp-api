package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

func newSemesterScheduleSlotRepoMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return sqlx.NewDb(db, "sqlmock"), mock, func() { db.Close() }
}

func TestSemesterScheduleSlotRepositoryUpsertBatch(t *testing.T) {
	db, mock, cleanup := newSemesterScheduleSlotRepoMock(t)
	defer cleanup()
	repo := NewSemesterScheduleSlotRepository(db)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO semester_schedule_slots")).
		WithArgs(sqlmock.AnyArg(), "sched-1", 1, 1, "sub-1", "teacher-1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO semester_schedule_slots")).
		WithArgs(sqlmock.AnyArg(), "sched-1", 1, 2, "sub-2", "teacher-2", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	slots := []models.SemesterScheduleSlot{
		{
			SemesterScheduleID: "sched-1",
			DayOfWeek:          1,
			TimeSlot:           1,
			SubjectID:          "sub-1",
			TeacherID:          "teacher-1",
		},
		{
			SemesterScheduleID: "sched-1",
			DayOfWeek:          1,
			TimeSlot:           2,
			SubjectID:          "sub-2",
			TeacherID:          "teacher-2",
		},
	}

	require.NoError(t, repo.UpsertBatch(context.Background(), nil, slots))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSemesterScheduleSlotRepositoryListBySchedule(t *testing.T) {
	db, mock, cleanup := newSemesterScheduleSlotRepoMock(t)
	defer cleanup()
	repo := NewSemesterScheduleSlotRepository(db)

	rows := sqlmock.NewRows([]string{"id", "semester_schedule_id", "day_of_week", "time_slot", "subject_id", "teacher_id", "room", "created_at"}).
		AddRow("slot-1", "sched-1", 1, 1, "sub-1", "teacher-1", nil, time.Now())
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, semester_schedule_id, day_of_week, time_slot, subject_id, teacher_id, room, created_at FROM semester_schedule_slots WHERE semester_schedule_id = $1 ORDER BY day_of_week ASC, time_slot ASC")).
		WithArgs("sched-1").
		WillReturnRows(rows)

	slots, err := repo.ListBySchedule(context.Background(), "sched-1")
	require.NoError(t, err)
	assert.Len(t, slots, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}
