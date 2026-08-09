package archives

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Challenger M3_3_1 Empirical Stress Test Suite
// -----------------------------------------------------------------------------

// TestChallengerM3_3_1_ConcurrencyStress stress tests concurrent invocations of
// PromoteOnAccess, MigrateDocumentTier, and EvaluatePolicies across multiple goroutines
// to detect data races, stale read overwrites, and duplicate audit log entries.
func TestChallengerM3_3_1_ConcurrencyStress(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	migrator := NewStorageTierMigrator(repo, searchEngine)
	engine := NewRetentionEngine(repo)
	engine.SetSearchEngine(searchEngine)

	const numDocs = 20
	docIDs := make([]uuid.UUID, numDocs)

	for i := 0; i < numDocs; i++ {
		id := uuid.New()
		docIDs[i] = id
		doc := &ArchiveDocument{
			ID:          id,
			Filename:    fmt.Sprintf("concurrency_doc_%d.pdf", i),
			StorageTier: StorageTierWarm,
			Category:    CategoryStudentRecord,
			UploadedAt:  time.Now().UTC().AddDate(0, 0, -100),
			RetainUntil: time.Now().UTC().AddDate(0, 0, -10), // Expired
			LegalHold:   false,
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))
	}

	const numWorkers = 15
	const iterations = 30
	var wg sync.WaitGroup

	// Worker Pool 1: PromoteOnAccess
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for iter := 0; iter < iterations; iter++ {
				docID := docIDs[(workerID+iter)%numDocs]
				_ = migrator.PromoteOnAccess(ctx, docID)
				time.Sleep(100 * time.Microsecond)
			}
		}(w)
	}

	// Worker Pool 2: MigrateDocumentTier (HOT -> WARM)
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for iter := 0; iter < iterations; iter++ {
				docID := docIDs[(workerID+iter)%numDocs]
				doc, err := repo.GetDocumentByID(ctx, docID)
				if err == nil && doc != nil {
					_ = migrator.MigrateDocumentTier(ctx, doc, StorageTierWarm)
				}
				time.Sleep(100 * time.Microsecond)
			}
		}(w)
	}

	// Worker Pool 3: EvaluatePolicies
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for iter := 0; iter < iterations; iter++ {
				_ = engine.EvaluatePolicies(ctx)
				time.Sleep(150 * time.Microsecond)
			}
		}(w)
	}

	wg.Wait()

	// Audit Log duplicate inspection
	duplicateAuditLogsFound := 0
	for _, id := range docIDs {
		logs, err := repo.GetAuditLogsByDocument(ctx, id)
		require.NoError(t, err)

		manualReviewCount := 0
		for _, log := range logs {
			if log.Action == AuditActionRetentionExpiredManualReview {
				manualReviewCount++
			}
		}
		if manualReviewCount > 1 {
			duplicateAuditLogsFound++
			t.Logf("FAIL: Document %s has %d duplicate RETENTION_EXPIRED_MANUAL_REVIEW audit log entries", id, manualReviewCount)
		}
	}

	assert.Equal(t, 0, duplicateAuditLogsFound, "Concurrent EvaluatePolicies must be idempotent and not produce duplicate audit logs")
}

// TestChallengerM3_3_1_BoundaryAgeTransitions tests document tier boundary conditions
// around 90 days and 730 days (e.g. 730 days + 1 second).
func TestChallengerM3_3_1_BoundaryAgeTransitions(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	migrator := NewStorageTierMigrator(repo, searchEngine)

	now := time.Now().UTC()

	// 1. HOT tier <= 90 days boundary
	doc90dMinus1s := &ArchiveDocument{
		ID:          uuid.New(),
		Filename:    "doc_90d_minus_1s.pdf",
		StorageTier: StorageTierHot,
		UploadedAt:  now.Add(-90 * 24 * time.Hour).Add(1 * time.Second),
	}
	doc90dExact := &ArchiveDocument{
		ID:          uuid.New(),
		Filename:    "doc_90d_exact.pdf",
		StorageTier: StorageTierHot,
		UploadedAt:  now.Add(-90 * 24 * time.Hour),
	}
	doc90dPlus1s := &ArchiveDocument{
		ID:          uuid.New(),
		Filename:    "doc_90d_plus_1s.pdf",
		StorageTier: StorageTierHot,
		UploadedAt:  now.Add(-90 * 24 * time.Hour).Add(-1 * time.Second),
	}

	// 2. WARM tier <= 730 days boundary
	doc730dMinus1s := &ArchiveDocument{
		ID:          uuid.New(),
		Filename:    "doc_730d_minus_1s.pdf",
		StorageTier: StorageTierWarm,
		UploadedAt:  now.Add(-730 * 24 * time.Hour).Add(1 * time.Second),
	}
	doc730dExact := &ArchiveDocument{
		ID:          uuid.New(),
		Filename:    "doc_730d_exact.pdf",
		StorageTier: StorageTierWarm,
		UploadedAt:  now.Add(-730 * 24 * time.Hour),
	}
	doc730dPlus1s := &ArchiveDocument{
		ID:          uuid.New(),
		Filename:    "doc_730d_plus_1s.pdf",
		StorageTier: StorageTierWarm,
		UploadedAt:  now.Add(-730 * 24 * time.Hour).Add(-1 * time.Second),
	}

	require.NoError(t, repo.CreateDocument(ctx, doc90dMinus1s))
	require.NoError(t, repo.CreateDocument(ctx, doc90dExact))
	require.NoError(t, repo.CreateDocument(ctx, doc90dPlus1s))

	require.NoError(t, repo.CreateDocument(ctx, doc730dMinus1s))
	require.NoError(t, repo.CreateDocument(ctx, doc730dExact))
	require.NoError(t, repo.CreateDocument(ctx, doc730dPlus1s))

	migrated, err := migrator.EvaluateAndMigrateTiers(ctx)
	require.NoError(t, err)

	// Verification of 90-day HOT -> WARM boundary
	res90Minus, err := repo.GetDocumentByID(ctx, doc90dMinus1s.ID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierHot, res90Minus.StorageTier, "90d - 1s document must stay HOT")

	res90Exact, err := repo.GetDocumentByID(ctx, doc90dExact.ID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierHot, res90Exact.StorageTier, "90d exact document must stay HOT")

	res90Plus, err := repo.GetDocumentByID(ctx, doc90dPlus1s.ID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierWarm, res90Plus.StorageTier, "90d + 1s document MUST migrate to WARM")

	// Verification of 730-day WARM -> COLD boundary
	res730Minus, err := repo.GetDocumentByID(ctx, doc730dMinus1s.ID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierWarm, res730Minus.StorageTier, "730d - 1s document must stay WARM")

	res730Exact, err := repo.GetDocumentByID(ctx, doc730dExact.ID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierWarm, res730Exact.StorageTier, "730d exact document must stay WARM")

	res730Plus, err := repo.GetDocumentByID(ctx, doc730dPlus1s.ID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierCold, res730Plus.StorageTier, "730d + 1s document MUST migrate to COLD")

	t.Logf("Migrated document count: %d (Expected: 2)", migrated)
}

// TestChallengerM3_3_1_LegalHoldOverride tests that legal hold blocks deletion
// unless OverrideLegalHold is explicitly set to true.
func TestChallengerM3_3_1_LegalHoldOverride(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	engine := NewRetentionEngine(repo)
	engine.SetSearchEngine(searchEngine)

	tempDir := t.TempDir()

	// Case 1: Legal Hold active, policy Override = false -> Auto-delete MUST be blocked
	policy1ID := uuid.New()
	policy1 := &RetentionPolicy{
		ID:                policy1ID,
		Name:              "No Override Policy",
		Category:          CategoryStudentRecord,
		RetentionYears:    5,
		AutoDelete:        true,
		LegalHoldOverride: false,
	}
	require.NoError(t, repo.CreateRetentionPolicy(ctx, policy1))

	file1Path := filepath.Join(tempDir, "hold_no_override.pdf")
	require.NoError(t, os.WriteFile(file1Path, []byte("Content 1"), 0644))

	doc1ID := uuid.New()
	doc1 := &ArchiveDocument{
		ID:                doc1ID,
		Filename:          "hold_no_override.pdf",
		StoragePath:       file1Path,
		StorageTier:       StorageTierHot,
		Category:          CategoryStudentRecord,
		RetentionPolicyID: &policy1ID,
		RetainUntil:       time.Now().UTC().AddDate(0, 0, -10),
		LegalHold:         true,
		LegalHoldReason:   "Pending Court Order #500",
		UploadedAt:        time.Now().UTC().AddDate(-6, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc1))

	// Case 2: Legal Hold active, policy Override = true -> Auto-delete MUST proceed
	policy2ID := uuid.New()
	policy2 := &RetentionPolicy{
		ID:                policy2ID,
		Name:              "Mandatory Override Policy",
		Category:          CategoryBehaviorNote,
		RetentionYears:    1,
		AutoDelete:        true,
		LegalHoldOverride: true,
	}
	require.NoError(t, repo.CreateRetentionPolicy(ctx, policy2))

	file2Path := filepath.Join(tempDir, "hold_with_override.pdf")
	require.NoError(t, os.WriteFile(file2Path, []byte("Content 2"), 0644))

	doc2ID := uuid.New()
	doc2 := &ArchiveDocument{
		ID:                doc2ID,
		Filename:          "hold_with_override.pdf",
		StoragePath:       file2Path,
		StorageTier:       StorageTierHot,
		Category:          CategoryBehaviorNote,
		RetentionPolicyID: &policy2ID,
		RetainUntil:       time.Now().UTC().AddDate(0, 0, -10),
		LegalHold:         true,
		LegalHoldReason:   "Minor inquiry",
		UploadedAt:        time.Now().UTC().AddDate(-2, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc2))

	// Execute policy evaluation
	require.NoError(t, engine.EvaluatePolicies(ctx))

	// Verify Doc 1 (Override=false) -> NOT deleted
	doc1After, err := repo.GetDocumentByID(ctx, doc1ID)
	require.NoError(t, err)
	assert.Nil(t, doc1After.DeletedAt, "Doc 1 with LegalHold=true and Override=false must NOT be deleted")

	_, stat1 := os.Stat(file1Path)
	assert.NoError(t, stat1, "Doc 1 physical storage file must remain intact")

	logs1, err := repo.GetAuditLogsByDocument(ctx, doc1ID)
	require.NoError(t, err)
	foundSkippedLog := false
	for _, l := range logs1 {
		if l.Action == AuditActionSkippedLegalHold {
			foundSkippedLog = true
			break
		}
	}
	assert.True(t, foundSkippedLog, "SKIPPED_LEGAL_HOLD audit log must be created for Doc 1")

	// Verify Doc 2 (Override=true) -> IS deleted
	doc2After, err := repo.GetDocumentByID(ctx, doc2ID)
	require.NoError(t, err)
	assert.NotNil(t, doc2After.DeletedAt, "Doc 2 with LegalHold=true and Override=true MUST be deleted")

	_, stat2 := os.Stat(file2Path)
	assert.True(t, os.IsNotExist(stat2), "Doc 2 physical storage file must be removed")

	logs2, err := repo.GetAuditLogsByDocument(ctx, doc2ID)
	require.NoError(t, err)
	foundExpiredLog := false
	for _, l := range logs2 {
		if l.Action == AuditActionRetentionExpired {
			foundExpiredLog = true
			break
		}
	}
	assert.True(t, foundExpiredLog, "RETENTION_EXPIRED audit log must be created for Doc 2")
}

// TestChallengerM3_3_1_RetentionEngine_StartStopCycle repeatedly starts and stops
// the retention engine in rapid succession to ensure thread safety, zero goroutine leaks,
// and zero WaitGroup or channel races.
func TestChallengerM3_3_1_RetentionEngine_StartStopCycle(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewRetentionEngine(repo)
	engine.SetInterval(1 * time.Millisecond)

	const cycles = 50
	for c := 0; c < cycles; c++ {
		require.NoError(t, engine.Start(ctx), "Engine Start() should succeed on cycle %d", c)
		time.Sleep(500 * time.Microsecond)
		engine.Stop()
	}

	// Verify engine can be restarted cleanly after rapid cycling
	require.NoError(t, engine.Start(ctx), "Engine should start cleanly after rapid cycling")
	engine.Stop()
}
