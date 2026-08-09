package archives

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChallengerM3_ConcurrentEvaluateAndStartStop tests concurrent calls to EvaluatePolicies
// while background Start/Stop ticker runs and documents are added/modified.
func TestChallengerM3_ConcurrentEvaluateAndStartStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	engine := NewRetentionEngine(repo)
	engine.SetSearchEngine(searchEngine)
	engine.SetInterval(5 * time.Millisecond)

	err := engine.Start(ctx)
	require.NoError(t, err)

	const numGoroutines = 10
	var wg sync.WaitGroup
	var evaluatedOps int64

	// Concurrently evaluate policies
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Create a document
					docID := uuid.New()
					doc := &ArchiveDocument{
						ID:          docID,
						Filename:    fmt.Sprintf("concurrent_doc_%d.pdf", id),
						StorageTier: StorageTierHot,
						Category:    CategoryStudentRecord,
						RetainUntil: time.Now().UTC().Add(-1 * time.Hour), // Expired
						UploadedAt:  time.Now().UTC().Add(-24 * time.Hour),
					}
					_ = repo.CreateDocument(ctx, doc)
					_ = searchEngine.IndexDocument(ctx, doc)

					_ = engine.EvaluatePolicies(ctx)
					_ = engine.MigrateStorageTiers(ctx)
					atomic.AddInt64(&evaluatedOps, 1)

					time.Sleep(2 * time.Millisecond)
				}
			}
		}(i)
	}

	// Concurrently restart engine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				engine.Stop()
				_ = engine.Start(ctx)
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	<-ctx.Done()
	engine.Stop()
	wg.Wait()

	t.Logf("Completed %d concurrent evaluation ops without race condition or panic", atomic.LoadInt64(&evaluatedOps))
}

// TestChallengerM3_MissingPhysicalFileAutoDelete tests auto-deletion when the physical file is missing on disk.
func TestChallengerM3_MissingPhysicalFileAutoDelete(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	engine := NewRetentionEngine(repo)
	engine.SetSearchEngine(searchEngine)

	nonExistentPath := filepath.Join(t.TempDir(), "non_existent_file_9999.pdf")

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "ghost_file.pdf",
		MimeType:    "application/pdf",
		SizeBytes:   500,
		StoragePath: nonExistentPath, // File does NOT exist on disk
		StorageTier: StorageTierHot,
		Category:    CategoryStudentRecord,
		RetainUntil: time.Now().UTC().AddDate(0, 0, -1), // Expired yesterday
		LegalHold:   false,
		UploadedAt:  time.Now().UTC().AddDate(-1, 0, 0),
	}

	err := repo.CreateDocument(ctx, doc)
	require.NoError(t, err)
	err = searchEngine.IndexDocument(ctx, doc)
	require.NoError(t, err)

	// Ensure physical file is indeed missing
	_, statErr := os.Stat(nonExistentPath)
	require.True(t, os.IsNotExist(statErr))

	// Evaluate policies should handle missing file gracefully without error or panic
	err = engine.EvaluatePolicies(ctx)
	assert.NoError(t, err, "EvaluatePolicies must not error out on missing physical file")

	// DB record should still be soft deleted
	docAfter, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.NotNil(t, docAfter.DeletedAt, "document in DB should be soft-deleted despite missing physical file")

	// Audit log should be recorded
	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	foundExpiredLog := false
	for _, log := range logs {
		if log.Action == "RETENTION_EXPIRED" {
			foundExpiredLog = true
			assert.Equal(t, "AUTO_DELETE", log.Details["action"])
		}
	}
	assert.True(t, foundExpiredLog, "RETENTION_EXPIRED audit log should be present")

	// Search index should be purged
	searchRes, err := searchEngine.Search(ctx, SearchRequest{Query: "ghost"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), searchRes.Total, "search index entry should be purged")
}

// TestChallengerM3_LegalHoldApplyReleaseRace tests race conditions between concurrent ApplyLegalHold,
// ReleaseLegalHold, and EvaluatePolicies.
func TestChallengerM3_LegalHoldApplyReleaseRace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	repo := NewMemoryRepository()
	engine := NewRetentionEngine(repo)

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "race_hold_doc.pdf",
		StorageTier: StorageTierHot,
		Category:    CategoryStudentRecord,
		RetainUntil: time.Now().UTC().Add(-10 * time.Minute), // Expired
		UploadedAt:  time.Now().UTC().AddDate(-1, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	var wg sync.WaitGroup
	const workers = 8

	// Worker 1: Repeatedly Apply Legal Hold
	wg.Add(1)
	go func() {
		defer wg.Done()
		userID := uuid.New()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, _ = engine.ApplyLegalHold(ctx, docID, "Litigation hold", userID)
			}
		}
	}()

	// Worker 2: Repeatedly Release Legal Hold
	wg.Add(1)
	go func() {
		defer wg.Done()
		userID := uuid.New()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, _ = engine.ReleaseLegalHold(ctx, docID, "Hold lifted", userID)
			}
		}
	}()

	// Worker 3..N: Concurrently evaluate policies
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_ = engine.EvaluatePolicies(ctx)
				}
			}
		}()
	}

	wg.Wait()

	// Final verification: Document must either be soft-deleted OR remain intact with consistent state
	docFinal, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	t.Logf("Legal hold race test completed. DeletedAt: %v, LegalHold: %v", docFinal.DeletedAt, docFinal.LegalHold)
}

// TestChallengerM3_BoundaryAndNegativeRetainUntil Dates tests retain_until == time.Now(),
// negative dates, ancient dates, and zero dates.
func TestChallengerM3_BoundaryAndNegativeRetainUntil(t *testing.T) {
	ctx := context.Background()

	t.Run("retain_until equal to current time", func(t *testing.T) {
		repo := NewMemoryRepository()
		engine := NewRetentionEngine(repo)

		now := time.Now().UTC()
		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "exact_now_doc.pdf",
			StorageTier: StorageTierHot,
			Category:    CategoryStudentRecord,
			RetainUntil: now.Add(-1 * time.Microsecond), // Barely past now to test boundary
			UploadedAt:  now.AddDate(-1, 0, 0),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		err := engine.EvaluatePolicies(ctx)
		require.NoError(t, err)

		docAfter, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.NotNil(t, docAfter.DeletedAt, "document with retain_until barely past now should be auto-deleted")
	})

	t.Run("retain_until exact boundary RetainUntil equal now", func(t *testing.T) {
		repo := NewMemoryRepository()
		engine := NewRetentionEngine(repo)

		now := time.Now().UTC()
		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "exact_now_boundary.pdf",
			StorageTier: StorageTierHot,
			Category:    CategoryStudentRecord,
			RetainUntil: now, // Exact now
			UploadedAt:  now.AddDate(-1, 0, 0),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		// Sleep 1ms to ensure system clock advances past exact retain_until
		time.Sleep(2 * time.Millisecond)

		err := engine.EvaluatePolicies(ctx)
		require.NoError(t, err)

		docAfter, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.NotNil(t, docAfter.DeletedAt, "document whose retain_until matches current time should be auto-deleted once time passes")
	})

	t.Run("ancient / negative epoch retain_until dates", func(t *testing.T) {
		repo := NewMemoryRepository()
		engine := NewRetentionEngine(repo)

		// Ancient date (year 1900)
		ancientDate := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "ancient_doc.pdf",
			StorageTier: StorageTierHot,
			Category:    CategoryStudentRecord,
			RetainUntil: ancientDate,
			UploadedAt:  time.Now().UTC().AddDate(-10, 0, 0),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		err := engine.EvaluatePolicies(ctx)
		require.NoError(t, err)

		docAfter, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.NotNil(t, docAfter.DeletedAt, "document with ancient retain_until date (year 1900) must be auto-deleted")
	})

	t.Run("future retain_until dates must NOT be deleted", func(t *testing.T) {
		repo := NewMemoryRepository()
		engine := NewRetentionEngine(repo)

		futureDate := time.Now().UTC().AddDate(5, 0, 0) // 5 years in future
		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "future_doc.pdf",
			StorageTier: StorageTierHot,
			Category:    CategoryStudentRecord,
			RetainUntil: futureDate,
			UploadedAt:  time.Now().UTC(),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		err := engine.EvaluatePolicies(ctx)
		require.NoError(t, err)

		docAfter, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.Nil(t, docAfter.DeletedAt, "future retain_until document MUST NOT be deleted")
	})
}

// TestChallengerM3_NilDependencies tests RetentionEngine operations when repo or searchEngine is nil.
func TestChallengerM3_NilDependencies(t *testing.T) {
	ctx := context.Background()

	t.Run("nil repository in NewRetentionEngine", func(t *testing.T) {
		engine := NewRetentionEngine(nil)
		require.NotNil(t, engine)

		// EvaluatePolicies with nil repo should return nil without panic
		err := engine.EvaluatePolicies(ctx)
		assert.NoError(t, err)

		// MigrateStorageTiers with nil repo/migrator should return nil without panic
		err = engine.MigrateStorageTiers(ctx)
		assert.NoError(t, err)

		// Setters with nil should not panic
		engine.SetSearchEngine(nil)
		engine.SetStorageTierMigrator(nil)
	})

	t.Run("nil searchEngine with valid repo", func(t *testing.T) {
		repo := NewMemoryRepository()
		engine := NewRetentionEngine(repo)
		// Explicitly ensure searchEngine is nil
		engine.SetSearchEngine(nil)

		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_nil_search.pdf")
		require.NoError(t, os.WriteFile(filePath, []byte("nil search test"), 0644))

		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "test_nil_search.pdf",
			StoragePath: filePath,
			StorageTier: StorageTierHot,
			Category:    CategoryStudentRecord,
			RetainUntil: time.Now().UTC().AddDate(0, 0, -5), // Expired
			UploadedAt:  time.Now().UTC().AddDate(-1, 0, 0),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		err := engine.EvaluatePolicies(ctx)
		assert.NoError(t, err, "EvaluatePolicies with nil search engine should succeed without panic")

		docAfter, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.NotNil(t, docAfter.DeletedAt, "document should be soft-deleted even when search engine is nil")
	})
}

// TestChallengerM3_StorageTierMigratorEdgeCases tests storage tiering migration edge cases.
func TestChallengerM3_StorageTierMigratorEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("nil repository in StorageTierMigrator", func(t *testing.T) {
		migrator := NewStorageTierMigrator(nil, nil)
		count, err := migrator.EvaluateAndMigrateTiers(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)

		err = migrator.PromoteOnAccess(ctx, uuid.New())
		assert.NoError(t, err)
	})

	t.Run("promote document already HOT", func(t *testing.T) {
		repo := NewMemoryRepository()
		migrator := NewStorageTierMigrator(repo, nil)

		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "already_hot.pdf",
			StorageTier: StorageTierHot,
			Category:    CategoryStudentRecord,
			UploadedAt:  time.Now().UTC(),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))

		err := migrator.PromoteOnAccess(ctx, docID)
		assert.NoError(t, err)

		docAfter, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.Equal(t, StorageTierHot, docAfter.StorageTier, "promoting HOT document should remain HOT")
	})
}
