package archives

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Challenger M3 Remediation Iteration 2 Adversarial Test Suite
// Author: challenger_m3_rem2_2
// -----------------------------------------------------------------------------

// MockFailingSearchEngine returns error on IndexDocument
type MockFailingSearchEngine struct {
	IndexErr error
}

func (m *MockFailingSearchEngine) IndexDocument(ctx context.Context, doc *ArchiveDocument) error {
	if m.IndexErr != nil {
		return m.IndexErr
	}
	return errors.New("search engine indexing error")
}

func (m *MockFailingSearchEngine) DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *MockFailingSearchEngine) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	return &SearchResult{}, nil
}

// PanickingRepository wraps Repository and injects panics during ticker evaluation
type PanickingRepository struct {
	Repository
	ShouldPanic atomic.Bool
}

func (p *PanickingRepository) GetExpiredDocuments(ctx context.Context, limit int) ([]*ArchiveDocument, error) {
	if p.ShouldPanic.Load() {
		panic("simulated panic in GetExpiredDocuments")
	}
	return p.Repository.GetExpiredDocuments(ctx, limit)
}

func (p *PanickingRepository) GetDocumentsForTierMigration(ctx context.Context, currentTier StorageTier, olderThanDays int) ([]*ArchiveDocument, error) {
	if p.ShouldPanic.Load() {
		panic("simulated panic in GetDocumentsForTierMigration")
	}
	return p.Repository.GetDocumentsForTierMigration(ctx, currentTier, olderThanDays)
}

// -----------------------------------------------------------------------------
// Test 1: Concurrent PromoteOnAccess during background EvaluateAndMigrateTiers
// -----------------------------------------------------------------------------
func TestChallengerM3_Rem2_2_ConcurrentPromoteOnAccess_DuringEvaluateAndMigrateTiers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	migrator := NewStorageTierManager(repo, searchEngine)

	const numHotDocs = 50
	const numColdDocs = 50

	hotIDs := make([]uuid.UUID, numHotDocs)
	coldIDs := make([]uuid.UUID, numColdDocs)

	// Create HOT docs older than 90d (eligible for WARM migration)
	for i := 0; i < numHotDocs; i++ {
		id := uuid.New()
		hotIDs[i] = id
		doc := &ArchiveDocument{
			ID:          id,
			Filename:    fmt.Sprintf("hot_doc_%d.pdf", i),
			StorageTier: StorageTierHot,
			Category:    CategoryStudentRecord,
			UploadedAt:  time.Now().UTC().AddDate(0, 0, -100),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))
	}

	// Create COLD docs older than 730d (eligible for PromoteOnAccess COLD -> WARM)
	for i := 0; i < numColdDocs; i++ {
		id := uuid.New()
		coldIDs[i] = id
		doc := &ArchiveDocument{
			ID:          id,
			Filename:    fmt.Sprintf("cold_doc_%d.pdf", i),
			StorageTier: StorageTierCold,
			Category:    CategoryStudentRecord,
			UploadedAt:  time.Now().UTC().AddDate(0, 0, -800),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))
	}

	var wg sync.WaitGroup
	const bgEvaluatorGoroutines = 5
	const promoteGoroutines = 20

	// Launch background tier evaluators
	for g := 0; g < bgEvaluatorGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_, _ = migrator.EvaluateAndMigrateTiers(ctx)
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	// Launch concurrent PromoteOnAccess calls on COLD and WARM docs
	for g := 0; g < promoteGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				coldTarget := coldIDs[(goroutineID+i)%numColdDocs]
				hotTarget := hotIDs[(goroutineID+i)%numHotDocs]

				_ = migrator.PromoteOnAccess(ctx, coldTarget)
				_ = migrator.PromoteOnAccess(ctx, hotTarget)
				time.Sleep(500 * time.Microsecond)
			}
		}(g)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("Concurrent PromoteOnAccess and EvaluateAndMigrateTiers finished successfully without deadlock or blocking")
	case <-time.After(5 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Concurrent PromoteOnAccess during background EvaluateAndMigrateTiers timed out!")
	}

	// Verify all documents remain in valid state
	for _, id := range coldIDs {
		doc, err := repo.GetDocumentByID(ctx, id)
		require.NoError(t, err)
		assert.Contains(t, []StorageTier{StorageTierCold, StorageTierWarm, StorageTierHot}, doc.StorageTier)
	}
}

// -----------------------------------------------------------------------------
// Test 2: Search engine indexing error propagation during storage tier migration
// -----------------------------------------------------------------------------
func TestChallengerM3_Rem2_2_SearchEngineIndexingErrorPropagation(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	customErr := errors.New("meilisearch cluster connection refused")
	failingSearchEngine := &MockFailingSearchEngine{IndexErr: customErr}

	migrator := NewStorageTierManager(repo, failingSearchEngine)

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "error_propagation_doc.pdf",
		StorageTier: StorageTierWarm,
		Category:    CategoryStudentRecord,
		UploadedAt:  time.Now().UTC().AddDate(0, 0, -120),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	// Direct MigrateDocumentTier call
	err := migrator.MigrateDocumentTier(ctx, doc, StorageTierHot)
	require.Error(t, err, "MigrateDocumentTier must return error when search indexing fails")
	assert.ErrorIs(t, err, customErr, "Returned error must wrap original search indexing error")
	assert.Contains(t, err.Error(), "re-index document tier in search engine", "Error message should include context")

	// PromoteOnAccess call with failing search engine
	coldDocID := uuid.New()
	coldDoc := &ArchiveDocument{
		ID:          coldDocID,
		Filename:    "cold_err_doc.pdf",
		StorageTier: StorageTierCold,
		Category:    CategoryStudentRecord,
		UploadedAt:  time.Now().UTC().AddDate(0, 0, -800),
	}
	require.NoError(t, repo.CreateDocument(ctx, coldDoc))

	errPromote := migrator.PromoteOnAccess(ctx, coldDocID)
	require.Error(t, errPromote, "PromoteOnAccess must return error when search indexing fails")
	assert.ErrorIs(t, errPromote, customErr, "PromoteOnAccess must propagate search indexing error")
}

// -----------------------------------------------------------------------------
// Test 3: Panic safety in background ticker loop
// -----------------------------------------------------------------------------
func TestChallengerM3_Rem2_2_BackgroundTickerPanicSafety(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseRepo := NewMemoryRepository()
	panickingRepo := &PanickingRepository{Repository: baseRepo}
	panickingRepo.ShouldPanic.Store(true)

	engine := NewRetentionEngine(panickingRepo)
	engine.SetInterval(5 * time.Millisecond)

	require.NoError(t, engine.Start(ctx), "Start should succeed")

	// Allow multiple ticks to occur while repo is configured to panic
	time.Sleep(30 * time.Millisecond)

	// Switch off panics and verify background worker goroutine is still alive
	panickingRepo.ShouldPanic.Store(false)

	// Add an expired doc to baseRepo
	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "post_panic_doc.pdf",
		StorageTier: StorageTierHot,
		Category:    CategoryStudentRecord,
		RetainUntil: time.Now().UTC().AddDate(0, 0, -10),
		UploadedAt:  time.Now().UTC().AddDate(-1, 0, 0),
	}
	require.NoError(t, baseRepo.CreateDocument(ctx, doc))

	// Policy for auto-delete
	policy := &RetentionPolicy{
		ID:                uuid.New(),
		Category:          CategoryStudentRecord,
		AutoDelete:        true,
		LegalHoldOverride: false,
	}
	require.NoError(t, baseRepo.CreateRetentionPolicy(ctx, policy))

	// Trigger run or wait for ticker
	time.Sleep(20 * time.Millisecond)
	_ = engine.TriggerRun(ctx)

	// Check if doc was processed
	processedDoc, err := baseRepo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.NotNil(t, processedDoc.DeletedAt, "Background worker must recover from panics and continue processing subsequent ticks")

	// Stop engine cleanly
	engine.Stop()
}
