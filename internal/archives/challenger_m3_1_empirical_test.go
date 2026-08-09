package archives

import (
	"context"
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
// Empirical Test Suite for Milestone M3 (Retention & Storage Engine)
// -----------------------------------------------------------------------------

// TestM3_1_DocumentExpired_ActiveLegalHold_SkippedDeletion tests Edge Case 1:
// Expired document with active LegalHold (override=false) must skip deletion and record SKIPPED_LEGAL_HOLD audit log.
func TestM3_1_DocumentExpired_ActiveLegalHold_SkippedDeletion(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	engine := NewRetentionEngine(repo)
	engine.SetSearchEngine(searchEngine)

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "legal_hold_doc.pdf")
	require.NoError(t, os.WriteFile(filePath, []byte("Sensitive litigation data"), 0644))

	policyID := uuid.New()
	policy := &RetentionPolicy{
		ID:                policyID,
		Name:              "Standard Policy No Override",
		Category:          CategoryStudentRecord,
		RetentionYears:    5,
		AutoDelete:        true,
		LegalHoldOverride: false,
	}
	require.NoError(t, repo.CreateRetentionPolicy(ctx, policy))

	docID := uuid.New()
	userID := uuid.New()
	doc := &ArchiveDocument{
		ID:                docID,
		Filename:          "legal_hold_doc.pdf",
		MimeType:          "application/pdf",
		StoragePath:       filePath,
		StorageTier:       StorageTierHot,
		Category:          CategoryStudentRecord,
		RetentionPolicyID: &policyID,
		RetainUntil:       time.Now().UTC().AddDate(0, 0, -10), // Expired 10 days ago
		LegalHold:         true,
		LegalHoldReason:   "Subpoena #2026-X89",
		UploadedBy:        userID,
		UploadedAt:        time.Now().UTC().AddDate(-5, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))
	require.NoError(t, searchEngine.IndexDocument(ctx, doc))

	// Execute policy evaluation
	require.NoError(t, engine.EvaluatePolicies(ctx))

	// Assert 1: Document is NOT soft-deleted
	docAfter, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Nil(t, docAfter.DeletedAt, "Document under active legal hold must NOT be deleted")

	// Assert 2: Physical file still exists on disk
	_, statErr := os.Stat(filePath)
	assert.NoError(t, statErr, "Physical storage file must remain intact")

	// Assert 3: SKIPPED_LEGAL_HOLD audit log written
	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	require.NotEmpty(t, logs, "Audit log must be created")

	foundSkippedLog := false
	for _, log := range logs {
		if log.Action == "SKIPPED_LEGAL_HOLD" {
			foundSkippedLog = true
			assert.Equal(t, "Document under legal hold; auto-deletion skipped", log.Details["reason"])
			assert.Equal(t, "Subpoena #2026-X89", log.Details["legal_hold_reason"])
			break
		}
	}
	assert.True(t, foundSkippedLog, "SKIPPED_LEGAL_HOLD audit log entry must be present")
}

// TestM3_2_DocumentExpired_ActiveLegalHold_OverrideTrue_DeletesDocument tests Edge Case 2:
// Expired document with active LegalHold BUT policy LegalHoldOverride = true MUST be deleted.
func TestM3_2_DocumentExpired_ActiveLegalHold_OverrideTrue_DeletesDocument(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	engine := NewRetentionEngine(repo)
	engine.SetSearchEngine(searchEngine)

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "override_doc.pdf")
	require.NoError(t, os.WriteFile(filePath, []byte("Content to override"), 0644))

	policyID := uuid.New()
	policy := &RetentionPolicy{
		ID:                policyID,
		Name:              "Strict Compliance Override Policy",
		Category:          CategoryBehaviorNote,
		RetentionYears:    1,
		AutoDelete:        true,
		LegalHoldOverride: true, // Mandatory override enabled
	}
	require.NoError(t, repo.CreateRetentionPolicy(ctx, policy))

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:                docID,
		Filename:          "override_doc.pdf",
		MimeType:          "application/pdf",
		StoragePath:       filePath,
		StorageTier:       StorageTierHot,
		Category:          CategoryBehaviorNote,
		RetentionPolicyID: &policyID,
		RetainUntil:       time.Now().UTC().AddDate(0, 0, -5), // Expired 5 days ago
		LegalHold:         true,
		LegalHoldReason:   "Informal inquiry",
		UploadedAt:        time.Now().UTC().AddDate(-1, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))
	require.NoError(t, searchEngine.IndexDocument(ctx, doc))

	// Execute policy evaluation
	require.NoError(t, engine.EvaluatePolicies(ctx))

	// Assert 1: Document IS soft-deleted
	docAfter, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.NotNil(t, docAfter.DeletedAt, "Document MUST be deleted when policy has LegalHoldOverride = true")

	// Assert 2: Physical storage file removed
	_, statErr := os.Stat(filePath)
	assert.True(t, os.IsNotExist(statErr), "Physical storage file must be deleted")

	// Assert 3: RETENTION_EXPIRED audit log recorded
	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	foundExpiredLog := false
	for _, log := range logs {
		if log.Action == "RETENTION_EXPIRED" {
			foundExpiredLog = true
			assert.Equal(t, "AUTO_DELETE", log.Details["action"])
			break
		}
	}
	assert.True(t, foundExpiredLog, "RETENTION_EXPIRED audit log must be recorded")
}

// TestM3_3_StorageTierAgeBoundaries tests Edge Case 3:
// Document age boundary tests for HOT (<= 90d), WARM (90d - 730d), and COLD (> 730d).
func TestM3_3_StorageTierAgeBoundaries(t *testing.T) {
	ctx := context.Background()

	t.Run("Boundary 89 Days Old - Stays HOT", func(t *testing.T) {
		repo := NewMemoryRepository()
		migrator := NewStorageTierMigrator(repo, nil)

		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "doc_89d.pdf",
			StorageTier: StorageTierHot,
			UploadedAt:  time.Now().UTC().AddDate(0, 0, -89),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		migrated, err := migrator.EvaluateAndMigrateTiers(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, migrated, "Document 89 days old must NOT be migrated")

		updated, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.Equal(t, StorageTierHot, updated.StorageTier)
	})

	t.Run("Boundary 90 Days Old - Stays HOT (<= 90d)", func(t *testing.T) {
		repo := NewMemoryRepository()
		migrator := NewStorageTierMigrator(repo, nil)

		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "doc_90d.pdf",
			StorageTier: StorageTierHot,
			UploadedAt:  time.Now().UTC().AddDate(0, 0, -90),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		migrated, err := migrator.EvaluateAndMigrateTiers(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, migrated, "Document exactly 90 days old must stay HOT (<= 90 days rule)")

		updated, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.Equal(t, StorageTierHot, updated.StorageTier)
	})

	t.Run("Boundary 91 Days Old - Migrates HOT to WARM", func(t *testing.T) {
		repo := NewMemoryRepository()
		migrator := NewStorageTierMigrator(repo, nil)

		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "doc_91d.pdf",
			StorageTier: StorageTierHot,
			UploadedAt:  time.Now().UTC().AddDate(0, 0, -91),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		migrated, err := migrator.EvaluateAndMigrateTiers(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, migrated, "Document 91 days old must be migrated HOT -> WARM")

		updated, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.Equal(t, StorageTierWarm, updated.StorageTier)

		logs, err := repo.GetAuditLogsByDocument(ctx, docID)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		assert.Equal(t, AuditActionTierMigration, logs[0].Action)
		assert.Equal(t, "HOT", logs[0].Details["from_tier"])
		assert.Equal(t, "WARM", logs[0].Details["to_tier"])
	})

	t.Run("Boundary 730 Days Old - Stays WARM (<= 730d)", func(t *testing.T) {
		repo := NewMemoryRepository()
		migrator := NewStorageTierMigrator(repo, nil)

		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "doc_730d.pdf",
			StorageTier: StorageTierWarm,
			UploadedAt:  time.Now().UTC().AddDate(0, 0, -730),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		migrated, err := migrator.EvaluateAndMigrateTiers(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, migrated, "Document 730 days old in WARM tier must NOT be migrated")

		updated, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.Equal(t, StorageTierWarm, updated.StorageTier)
	})

	t.Run("Boundary 731 Days Old - Migrates WARM to COLD (> 730d)", func(t *testing.T) {
		repo := NewMemoryRepository()
		migrator := NewStorageTierMigrator(repo, nil)

		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "doc_731d.pdf",
			StorageTier: StorageTierWarm,
			UploadedAt:  time.Now().UTC().AddDate(0, 0, -731),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		migrated, err := migrator.EvaluateAndMigrateTiers(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, migrated, "Document 731 days old in WARM tier must be migrated to COLD")

		updated, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.Equal(t, StorageTierCold, updated.StorageTier)
	})

	t.Run("Boundary 731 Days Old HOT - Migrates Directly HOT to COLD", func(t *testing.T) {
		repo := NewMemoryRepository()
		migrator := NewStorageTierMigrator(repo, nil)

		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "hot_doc_731d.pdf",
			StorageTier: StorageTierHot,
			UploadedAt:  time.Now().UTC().AddDate(0, 0, -731),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		migrated, err := migrator.EvaluateAndMigrateTiers(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, migrated, "Document 731 days old in HOT tier must migrate directly to COLD")

		updated, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.Equal(t, StorageTierCold, updated.StorageTier)
	})
}

// TestM3_4_PromoteOnAccess tests tier promotion mechanics upon access.
func TestM3_4_PromoteOnAccess(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	migrator := NewStorageTierMigrator(repo, nil)

	// Case A: WARM -> HOT
	docWarmID := uuid.New()
	docWarm := &ArchiveDocument{
		ID:          docWarmID,
		StorageTier: StorageTierWarm,
		UploadedAt:  time.Now().UTC().AddDate(0, 0, -100),
	}
	require.NoError(t, repo.CreateDocument(ctx, docWarm))
	require.NoError(t, migrator.PromoteOnAccess(ctx, docWarmID))

	resWarm, err := repo.GetDocumentByID(ctx, docWarmID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierHot, resWarm.StorageTier, "WARM document should promote to HOT")

	// Case B: COLD -> WARM
	docColdID := uuid.New()
	docCold := &ArchiveDocument{
		ID:          docColdID,
		StorageTier: StorageTierCold,
		UploadedAt:  time.Now().UTC().AddDate(0, 0, -800),
	}
	require.NoError(t, repo.CreateDocument(ctx, docCold))
	require.NoError(t, migrator.PromoteOnAccess(ctx, docColdID))

	resCold, err := repo.GetDocumentByID(ctx, docColdID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierWarm, resCold.StorageTier, "COLD document should promote to WARM")

	// Case C: HOT -> HOT (no-op)
	docHotID := uuid.New()
	docHot := &ArchiveDocument{
		ID:          docHotID,
		StorageTier: StorageTierHot,
		UploadedAt:  time.Now().UTC().AddDate(0, 0, -10),
	}
	require.NoError(t, repo.CreateDocument(ctx, docHot))
	require.NoError(t, migrator.PromoteOnAccess(ctx, docHotID))

	resHot, err := repo.GetDocumentByID(ctx, docHotID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierHot, resHot.StorageTier, "HOT document remains HOT on access")
}

// TestM3_5_ConcurrentRetentionAndLegalHoldOperations stress tests concurrent retention operations.
func TestM3_5_ConcurrentRetentionAndLegalHoldOperations(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewRetentionEngine(repo)

	const numDocs = 50
	docIDs := make([]uuid.UUID, numDocs)

	for i := 0; i < numDocs; i++ {
		id := uuid.New()
		docIDs[i] = id
		doc := &ArchiveDocument{
			ID:          id,
			Filename:    "stress_doc.pdf",
			StorageTier: StorageTierHot,
			RetainUntil: time.Now().UTC().AddDate(0, 0, -1), // Expired
			LegalHold:   false,
			UploadedAt:  time.Now().UTC().AddDate(0, 0, -100),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))
	}

	var wg sync.WaitGroup
	userID := uuid.New()

	// Goroutine 1: Continuously evaluate policies
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = engine.EvaluatePolicies(ctx)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutine 2: Continuously migrate storage tiers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = engine.MigrateStorageTiers(ctx)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutines 3 & 4: Toggle Legal Holds concurrently
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for _, id := range docIDs {
				if workerID%2 == 0 {
					_, _ = engine.ApplyLegalHold(ctx, id, "Concurrent Legal Hold", userID)
				} else {
					_, _ = engine.ReleaseLegalHold(ctx, id, "Concurrent Release", userID)
				}
				time.Sleep(50 * time.Microsecond)
			}
		}(g)
	}

	wg.Wait()
}
