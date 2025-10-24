package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

type mockGradeConfigRepo struct {
	configs   map[string]*models.GradeConfig
	finalized map[string]bool
}

func (m *mockGradeConfigRepo) List(ctx context.Context, filter models.FinalGradeFilter) ([]models.GradeConfig, error) {
	return nil, nil
}

func (m *mockGradeConfigRepo) FindByID(ctx context.Context, id string) (*models.GradeConfig, error) {
	if m.configs != nil {
		if cfg, ok := m.configs[id]; ok {
			return cfg, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *mockGradeConfigRepo) FindByScope(ctx context.Context, classID, subjectID, termID string) (*models.GradeConfig, error) {
	for _, cfg := range m.configs {
		if cfg.ClassID == classID && cfg.SubjectID == subjectID && cfg.TermID == termID {
			return cfg, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *mockGradeConfigRepo) Exists(ctx context.Context, classID, subjectID, termID, excludeID string) (bool, error) {
	return false, nil
}

func (m *mockGradeConfigRepo) Create(ctx context.Context, config *models.GradeConfig) error {
	if m.configs == nil {
		m.configs = make(map[string]*models.GradeConfig)
	}
	config.ID = "cfg1"
	m.configs[config.ID] = config
	return nil
}

func (m *mockGradeConfigRepo) Update(ctx context.Context, config *models.GradeConfig) error {
	m.configs[config.ID] = config
	return nil
}

func (m *mockGradeConfigRepo) Finalize(ctx context.Context, id string, finalized bool) error {
	if m.finalized == nil {
		m.finalized = make(map[string]bool)
	}
	m.finalized[id] = finalized
	if cfg, ok := m.configs[id]; ok {
		cfg.Finalized = finalized
	}
	return nil
}

type mockComponentReader struct {
	components map[string]*models.GradeComponent
}

func (m *mockComponentReader) FindByID(ctx context.Context, id string) (*models.GradeComponent, error) {
	if c, ok := m.components[id]; ok {
		return c, nil
	}
	return nil, sql.ErrNoRows
}

func TestGradeConfigServiceCreate(t *testing.T) {
	repo := &mockGradeConfigRepo{}
	components := &mockComponentReader{components: map[string]*models.GradeComponent{"comp1": {ID: "comp1", Code: "TST", Name: "Test"}}}
	svc := NewGradeConfigService(repo, components, validator.New(), zap.NewNop())

	cfg, err := svc.Create(context.Background(), CreateGradeConfigRequest{
		ClassID: "class", SubjectID: "sub", TermID: "term", CalculationScheme: models.GradeSchemeWeighted,
		Components: []GradeConfigComponentRequest{{ComponentID: "comp1", Weight: 100}},
	})
	require.NoError(t, err)
	assert.Equal(t, "class", cfg.ClassID)
	assert.Len(t, cfg.Components, 1)
}

func TestGradeConfigServiceCreateInvalidWeights(t *testing.T) {
	repo := &mockGradeConfigRepo{}
	components := &mockComponentReader{components: map[string]*models.GradeComponent{"comp1": {ID: "comp1", Code: "TST", Name: "Test"}}}
	svc := NewGradeConfigService(repo, components, validator.New(), zap.NewNop())

	_, err := svc.Create(context.Background(), CreateGradeConfigRequest{
		ClassID: "class", SubjectID: "sub", TermID: "term", CalculationScheme: models.GradeSchemeWeighted,
		Components: []GradeConfigComponentRequest{{ComponentID: "comp1", Weight: 50}},
	})
	require.Error(t, err)
}

func TestGradeConfigServiceFinalize(t *testing.T) {
	repo := &mockGradeConfigRepo{configs: map[string]*models.GradeConfig{"cfg": {ID: "cfg", ClassID: "class", SubjectID: "sub", TermID: "term", CalculationScheme: models.GradeSchemeAverage}}}
	components := &mockComponentReader{components: map[string]*models.GradeComponent{"comp1": {ID: "comp1", Code: "TST", Name: "Test"}}}
	svc := NewGradeConfigService(repo, components, validator.New(), zap.NewNop())

	cfg, err := svc.Finalize(context.Background(), "cfg")
	require.NoError(t, err)
	assert.True(t, cfg.Finalized)
	assert.True(t, repo.finalized["cfg"])
}
