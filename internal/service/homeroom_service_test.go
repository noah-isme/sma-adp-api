package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type homeroomRepoStub struct {
	items        []dto.HomeroomItem
	teacherItems []dto.HomeroomItem
	getItem      *dto.HomeroomItem
	listErr      error
	teacherErr   error
	getErr       error
	upsertOld    *string
	upsertErr    error
	upsertParams []repository.HomeroomAssignmentParams
	teacherCalls int
}

func (s *homeroomRepoStub) List(ctx context.Context, filter dto.HomeroomFilter) ([]dto.HomeroomItem, error) {
	return s.items, s.listErr
}

func (s *homeroomRepoStub) ListForTeacher(ctx context.Context, teacherID string, filter dto.HomeroomFilter) ([]dto.HomeroomItem, error) {
	s.teacherCalls++
	return s.teacherItems, s.teacherErr
}

func (s *homeroomRepoStub) Get(ctx context.Context, classID, termID string) (*dto.HomeroomItem, error) {
	return s.getItem, s.getErr
}

func (s *homeroomRepoStub) Upsert(ctx context.Context, params repository.HomeroomAssignmentParams) (*string, error) {
	s.upsertParams = append(s.upsertParams, params)
	return s.upsertOld, s.upsertErr
}

type classRepoStub struct {
	classes map[string]*models.Class
	err     error
}

func (s classRepoStub) FindByID(ctx context.Context, id string) (*models.Class, error) {
	if s.err != nil {
		return nil, s.err
	}
	if class, ok := s.classes[id]; ok {
		return class, nil
	}
	return nil, sql.ErrNoRows
}

type termRepoStub struct {
	terms  map[string]*models.Term
	active *models.Term
	err    error
}

func (s termRepoStub) FindByID(ctx context.Context, id string) (*models.Term, error) {
	if term, ok := s.terms[id]; ok {
		return term, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	return nil, sql.ErrNoRows
}

func (s termRepoStub) FindActive(ctx context.Context) (*models.Term, error) {
	if s.active != nil {
		return s.active, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	return nil, sql.ErrNoRows
}

type subjectFinderStub struct {
	subject *models.Subject
	err     error
}

func (s subjectFinderStub) FindByCode(ctx context.Context, code string) (*models.Subject, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.subject != nil {
		return s.subject, nil
	}
	return nil, sql.ErrNoRows
}

type classAccessStub struct {
	allowed bool
	err     error
}

func (s classAccessStub) HasClassAccess(ctx context.Context, teacherID, classID, termID string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.allowed, nil
}

type auditRecorderStub struct {
	logs []*models.AuditLog
	err  error
}

func (s *auditRecorderStub) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	s.logs = append(s.logs, log)
	return s.err
}

func TestHomeroomServiceSetCreatesAssignment(t *testing.T) {
	repo := &homeroomRepoStub{
		getItem: &dto.HomeroomItem{ClassID: "class-1", TermID: "term-1", TermName: "Odd"},
	}
	classRepo := classRepoStub{classes: map[string]*models.Class{"class-1": {ID: "class-1"}}}
	termRepo := termRepoStub{terms: map[string]*models.Term{"term-1": {ID: "term-1"}}}
	teacherRepo := &teacherRepoStub{items: map[string]*models.Teacher{"teacher-1": {ID: "teacher-1", Active: true}}}
	subjectRepo := subjectFinderStub{subject: &models.Subject{ID: "subject-hm"}}
	access := classAccessStub{allowed: true}
	audit := &auditRecorderStub{}

	service := NewHomeroomService(repo, classRepo, termRepo, teacherRepo, subjectRepo, access, audit, nil, zap.NewNop())
	req := dto.SetHomeroomRequest{ClassID: "class-1", TermID: "term-1", TeacherID: "teacher-1"}
	result, err := service.Set(context.Background(), req, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})
	require.NoError(t, err)
	assert.Equal(t, "class-1", result.ClassID)
	require.Len(t, repo.upsertParams, 1)
	assert.Equal(t, "subject-hm", repo.upsertParams[0].SubjectID)
	require.Len(t, audit.logs, 1)
	assert.Equal(t, models.AuditActionHomeroomUpdate, audit.logs[0].Action)
}

func TestHomeroomServiceSetInactiveTeacher(t *testing.T) {
	repo := &homeroomRepoStub{}
	classRepo := classRepoStub{classes: map[string]*models.Class{"class-1": {ID: "class-1"}}}
	termRepo := termRepoStub{terms: map[string]*models.Term{"term-1": {ID: "term-1"}}}
	teacherRepo := &teacherRepoStub{items: map[string]*models.Teacher{"teacher-1": {ID: "teacher-1", Active: false}}}

	service := NewHomeroomService(repo, classRepo, termRepo, teacherRepo, subjectFinderStub{}, classAccessStub{allowed: true}, &auditRecorderStub{}, nil, zap.NewNop())
	_, err := service.Set(context.Background(), dto.SetHomeroomRequest{ClassID: "class-1", TermID: "term-1", TeacherID: "teacher-1"}, &models.JWTClaims{UserID: "admin", Role: models.RoleAdmin})
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrPreconditionFailed.Code, appErrors.FromError(err).Code)
}

func TestHomeroomServiceGetTeacherForbidden(t *testing.T) {
	repo := &homeroomRepoStub{}
	classRepo := classRepoStub{classes: map[string]*models.Class{"class-1": {ID: "class-1"}}}
	termRepo := termRepoStub{terms: map[string]*models.Term{"term-1": {ID: "term-1"}}}
	service := NewHomeroomService(repo, classRepo, termRepo, &teacherRepoStub{}, subjectFinderStub{}, classAccessStub{allowed: false}, &auditRecorderStub{}, nil, zap.NewNop())

	claims := &models.JWTClaims{UserID: "teacher-9", Role: models.RoleTeacher}
	_, err := service.Get(context.Background(), "class-1", "term-1", claims)
	require.Error(t, err)
	assert.Equal(t, appErrors.ErrForbidden.Code, appErrors.FromError(err).Code)
}

func TestHomeroomServiceListTeacherScoped(t *testing.T) {
	repo := &homeroomRepoStub{
		teacherItems: []dto.HomeroomItem{{ClassID: "class-1", TermID: "term-1"}},
	}
	classRepo := classRepoStub{classes: map[string]*models.Class{"class-1": {ID: "class-1"}}}
	termRepo := termRepoStub{
		active: &models.Term{ID: "term-1"},
		terms:  map[string]*models.Term{"term-1": {ID: "term-1"}},
	}
	service := NewHomeroomService(repo, classRepo, termRepo, &teacherRepoStub{}, subjectFinderStub{}, classAccessStub{allowed: true}, &auditRecorderStub{}, nil, zap.NewNop())

	items, err := service.List(context.Background(), dto.HomeroomFilter{}, &models.JWTClaims{UserID: "teacher-1", Role: models.RoleTeacher})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, 1, repo.teacherCalls)
}
