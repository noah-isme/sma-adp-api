package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

type mockGradeComponentRepo struct {
	components map[string]*models.GradeComponent
}

func (m *mockGradeComponentRepo) List(context.Context, string) ([]models.GradeComponent, error) {
	result := make([]models.GradeComponent, 0, len(m.components))
	for _, component := range m.components {
		result = append(result, *component)
	}
	return result, nil
}

func (m *mockGradeComponentRepo) ExistsByCode(_ context.Context, code, excludeID string) (bool, error) {
	for id, component := range m.components {
		if id != excludeID && component.Code == code {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockGradeComponentRepo) Create(_ context.Context, component *models.GradeComponent) error {
	if component.ID == "" {
		component.ID = "created"
	}
	m.components[component.ID] = component
	return nil
}

func (m *mockGradeComponentRepo) FindByID(_ context.Context, id string) (*models.GradeComponent, error) {
	if component, ok := m.components[id]; ok {
		return component, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockGradeComponentRepo) FindByCode(_ context.Context, code string) (*models.GradeComponent, error) {
	for _, component := range m.components {
		if component.Code == code {
			return component, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *mockGradeComponentRepo) Update(_ context.Context, component *models.GradeComponent) error {
	if _, ok := m.components[component.ID]; !ok {
		return sql.ErrNoRows
	}
	m.components[component.ID] = component
	return nil
}

func (m *mockGradeComponentRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.components[id]; !ok {
		return sql.ErrNoRows
	}
	delete(m.components, id)
	return nil
}

func TestGradeComponentServiceUpdatePreservesCodeWhenOmitted(t *testing.T) {
	repo := &mockGradeComponentRepo{components: map[string]*models.GradeComponent{
		"component-1": {ID: "component-1", Code: "UTS", Name: "Old name"},
	}}
	svc := NewGradeComponentService(repo, validator.New(), zap.NewNop())

	updated, err := svc.Update(context.Background(), "component-1", UpdateGradeComponentRequest{Name: "New name"})
	require.NoError(t, err)
	require.Equal(t, "UTS", updated.Code)
	require.Equal(t, "New name", updated.Name)
}

func TestGradeComponentServiceDeleteRemovesFromActiveRepository(t *testing.T) {
	repo := &mockGradeComponentRepo{components: map[string]*models.GradeComponent{
		"component-1": {ID: "component-1", Code: "UTS", Name: "Midterm"},
	}}
	svc := NewGradeComponentService(repo, validator.New(), zap.NewNop())

	require.NoError(t, svc.Delete(context.Background(), "component-1"))
	_, err := repo.FindByID(context.Background(), "component-1")
	require.ErrorIs(t, err, sql.ErrNoRows)
}
