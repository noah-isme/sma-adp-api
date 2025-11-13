package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
)

func newHomeroomRepoMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	sqlxDB := sqlx.NewDb(db, "postgres")
	cleanup := func() {
		_ = sqlxDB.Close()
		db.Close()
	}
	return sqlxDB, mock, cleanup
}

func TestHomeroomRepositoryList(t *testing.T) {
	db, mock, cleanup := newHomeroomRepoMock(t)
	defer cleanup()
	repo := NewHomeroomRepository(db)

	rows := sqlmock.NewRows([]string{"class_id", "class_name", "term_id", "term_name", "homeroom_teacher_id", "homeroom_teacher_name"}).
		AddRow("class-1", "X IPA 1", "term-1", "Ganjil", sql.NullString{String: "teacher-1", Valid: true}, sql.NullString{String: "Bu Ani", Valid: true})

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT
	c.id AS class_id,
	c.name AS class_name,
	t.id AS term_id,
	t.name AS term_name,
	ta.teacher_id AS homeroom_teacher_id,
	tr.full_name AS homeroom_teacher_name
FROM classes c
JOIN terms t ON t.id = $1`)).
		WithArgs("term-1").
		WillReturnRows(rows)

	items, err := repo.List(context.Background(), dto.HomeroomFilter{TermID: "term-1"})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "class-1", items[0].ClassID)
	assert.Equal(t, "term-1", items[0].TermID)
	assert.NotNil(t, items[0].HomeroomTeacherID)
	assert.Equal(t, "teacher-1", *items[0].HomeroomTeacherID)
}

func TestHomeroomRepositoryListForTeacher(t *testing.T) {
	db, mock, cleanup := newHomeroomRepoMock(t)
	defer cleanup()
	repo := NewHomeroomRepository(db)

	rows := sqlmock.NewRows([]string{"class_id", "class_name", "term_id", "term_name", "homeroom_teacher_id", "homeroom_teacher_name"}).
		AddRow("class-1", "X IPA 1", "term-1", "Ganjil", sql.NullString{Valid: false}, sql.NullString{Valid: false})

	mock.ExpectQuery(regexp.QuoteMeta("SELECT")). // coarse match
							WithArgs("term-1", "teacher-1").
							WillReturnRows(rows)

	items, err := repo.ListForTeacher(context.Background(), "teacher-1", dto.HomeroomFilter{TermID: "term-1"})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Nil(t, items[0].HomeroomTeacherID)
}

func TestHomeroomRepositoryGetNone(t *testing.T) {
	db, mock, cleanup := newHomeroomRepoMock(t)
	defer cleanup()
	repo := NewHomeroomRepository(db)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WithArgs("class-99", "term-1").
		WillReturnError(sql.ErrNoRows)

	item, err := repo.Get(context.Background(), "class-99", "term-1")
	require.NoError(t, err)
	assert.Nil(t, item)
}

func TestHomeroomRepositoryUpsertInsert(t *testing.T) {
	db, mock, cleanup := newHomeroomRepoMock(t)
	defer cleanup()
	repo := NewHomeroomRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, teacher_id FROM teacher_assignments")).
		WithArgs("class-1", "term-1").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO teacher_assignments")).
		WithArgs(sqlmock.AnyArg(), "teacher-1", "class-1", "homeroom-subject", "term-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	prev, err := repo.Upsert(context.Background(), HomeroomAssignmentParams{
		ClassID:   "class-1",
		TermID:    "term-1",
		TeacherID: "teacher-1",
		SubjectID: "homeroom-subject",
	})
	require.NoError(t, err)
	assert.Nil(t, prev)
}

func TestHomeroomRepositoryUpsertUpdate(t *testing.T) {
	db, mock, cleanup := newHomeroomRepoMock(t)
	defer cleanup()
	repo := NewHomeroomRepository(db)

	rows := sqlmock.NewRows([]string{"id", "teacher_id"}).
		AddRow("assign-1", "teacher-old")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, teacher_id FROM teacher_assignments")).
		WithArgs("class-1", "term-1").
		WillReturnRows(rows)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE teacher_assignments SET teacher_id = $1, subject_id = $2, role = 'HOMEROOM' WHERE id = $3")).
		WithArgs("teacher-1", "homeroom-subject", "assign-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	prev, err := repo.Upsert(context.Background(), HomeroomAssignmentParams{
		ClassID:   "class-1",
		TermID:    "term-1",
		TeacherID: "teacher-1",
		SubjectID: "homeroom-subject",
	})
	require.NoError(t, err)
	require.NotNil(t, prev)
	assert.Equal(t, "teacher-old", *prev)
}
