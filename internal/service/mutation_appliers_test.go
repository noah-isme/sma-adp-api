package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

type studentMutationRepoStub struct {
	detail    *models.StudentDetail
	updated   *models.Student
	existsNIS bool
}

func (s *studentMutationRepoStub) FindByID(ctx context.Context, id string) (*models.StudentDetail, error) {
	return s.detail, nil
}

func (s *studentMutationRepoStub) ExistsByNIS(ctx context.Context, nis string, excludeID string) (bool, error) {
	return s.existsNIS, nil
}

func (s *studentMutationRepoStub) Update(ctx context.Context, student *models.Student) error {
	s.updated = student
	return nil
}

func TestStudentMutationApplierApply(t *testing.T) {
	repo := &studentMutationRepoStub{
		detail: &models.StudentDetail{
			Student: models.Student{
				ID:        "stu-1",
				NIS:       "123",
				FullName:  "Adi",
				Gender:    "M",
				BirthDate: time.Date(2005, 1, 2, 0, 0, 0, 0, time.UTC),
				Address:   "Jl lama",
				Phone:     "0800",
				Active:    true,
			},
		},
	}
	applier := NewStudentMutationApplier(repo, nil)
	payload := map[string]interface{}{
		"fullName":  "Adi Baru",
		"address":   "Jl baru",
		"birthDate": "2005-02-03",
		"active":    false,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	snapshot, err := applier.Apply(context.Background(), &models.Mutation{
		EntityID:         "stu-1",
		RequestedChanges: raw,
	})
	require.NoError(t, err)
	require.NotNil(t, repo.updated)
	require.Equal(t, "Adi Baru", repo.updated.FullName)
	require.False(t, repo.updated.Active)
	require.Contains(t, string(snapshot), "Adi Baru")
}

func TestStudentMutationApplierRejectsDuplicateNIS(t *testing.T) {
	repo := &studentMutationRepoStub{
		detail: &models.StudentDetail{
			Student: models.Student{
				ID:  "stu-1",
				NIS: "123",
			},
		},
		existsNIS: true,
	}
	applier := NewStudentMutationApplier(repo, nil)
	payload := []byte(`{"nis":"999"}`)
	_, err := applier.Apply(context.Background(), &models.Mutation{
		EntityID:         "stu-1",
		RequestedChanges: payload,
	})
	require.Error(t, err)
}
