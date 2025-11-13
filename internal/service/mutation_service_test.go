package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/dto"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
)

type mutationRepoStub struct {
	mutations map[string]*models.Mutation
	filter    models.MutationFilter
}

func newMutationRepoStub() *mutationRepoStub {
	return &mutationRepoStub{mutations: make(map[string]*models.Mutation)}
}

func (m *mutationRepoStub) Create(ctx context.Context, mutation *models.Mutation) error {
	m.mutations[mutation.ID] = mutation
	return nil
}

func (m *mutationRepoStub) GetByID(ctx context.Context, id string) (*models.Mutation, error) {
	if mut, ok := m.mutations[id]; ok {
		copy := *mut
		return &copy, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mutationRepoStub) List(ctx context.Context, filter models.MutationFilter) ([]models.Mutation, error) {
	m.filter = filter
	result := make([]models.Mutation, 0, len(m.mutations))
	for _, mut := range m.mutations {
		result = append(result, *mut)
	}
	return result, nil
}

func (m *mutationRepoStub) UpdateStatusAndSnapshot(ctx context.Context, params repository.UpdateMutationParams) error {
	mut, ok := m.mutations[params.ID]
	if !ok {
		return sql.ErrNoRows
	}
	mut.Status = params.Status
	mut.ReviewedBy = &params.ReviewedBy
	mut.ReviewedAt = &params.ReviewedAt
	if params.Note != nil {
		mut.Note = params.Note
	}
	if len(params.CurrentSnapshot) > 0 {
		mut.CurrentSnapshot = params.CurrentSnapshot
	}
	return nil
}

type auditStub struct {
	logs []*models.AuditLog
}

func (a *auditStub) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	a.logs = append(a.logs, log)
	return nil
}

func TestMutationServiceRequestChange(t *testing.T) {
	repo := newMutationRepoStub()
	audit := &auditStub{}
	snapshot := MutationSnapshotProviderFunc(func(ctx context.Context, entity, entityID string) ([]byte, error) {
		return []byte(`{"before":true}`), nil
	})
	svc := NewMutationService(repo, audit, nil, WithMutationSnapshotProvider(snapshot))

	req := dto.CreateMutationRequest{
		Type:             models.MutationTypeStudentData,
		Entity:           "student",
		EntityID:         "student-1",
		Reason:           "typo",
		RequestedChanges: []byte(`{"name":"John"}`),
	}
	mutation, err := svc.RequestChange(context.Background(), req, "admin-1")
	require.NoError(t, err)
	require.Equal(t, models.MutationStatusPending, mutation.Status)
	require.Len(t, audit.logs, 1)
}

func TestMutationServiceReviewApprove(t *testing.T) {
	repo := newMutationRepoStub()
	audit := &auditStub{}
	mutation := &models.Mutation{
		ID:               "mut-1",
		Type:             models.MutationTypeStudentData,
		Entity:           "student",
		EntityID:         "student-1",
		Status:           models.MutationStatusPending,
		RequestedChanges: []byte(`{"name":"John"}`),
		CurrentSnapshot:  []byte(`{"name":"Jon"}`),
		RequestedBy:      "teacher-1",
	}
	repo.mutations[mutation.ID] = mutation
	appliers := map[string]MutationApplier{
		"student": MutationApplierFunc(func(ctx context.Context, mut *models.Mutation) ([]byte, error) {
			return []byte(`{"name":"John"}`), nil
		}),
	}
	svc := NewMutationService(repo, audit, nil, WithMutationAppliers(appliers))

	result, err := svc.Review(context.Background(), mutation.ID, dto.ReviewMutationRequest{
		Status: models.MutationStatusApproved,
		Note:   "ok",
	}, "super-1")
	require.NoError(t, err)
	require.Equal(t, models.MutationStatusApproved, result.Status)
	require.Len(t, audit.logs, 1)
}

func TestMutationServiceListTeacherFilters(t *testing.T) {
	repo := newMutationRepoStub()
	audit := &auditStub{}
	repo.mutations["mut-1"] = &models.Mutation{ID: "mut-1", RequestedBy: "teacher-1"}
	repo.mutations["mut-2"] = &models.Mutation{ID: "mut-2", RequestedBy: "teacher-2"}

	svc := NewMutationService(repo, audit, nil)
	claims := &models.JWTClaims{UserID: "teacher-1", Role: models.RoleTeacher}

	_, err := svc.List(context.Background(), dto.MutationQuery{}, claims)
	require.NoError(t, err)
	require.Equal(t, "teacher-1", repo.filter.RequestedBy)
}
