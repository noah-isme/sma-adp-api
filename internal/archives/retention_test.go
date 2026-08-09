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

// TestRetentionEngine_StartStopTicker tests background evaluation loop lifecycle.
func TestRetentionEngine_StartStopTicker(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewRetentionEngine(repo)

	engine.SetInterval(10 * time.Millisecond)

	err := engine.Start(ctx)
	require.NoError(t, err)

	time.Sleep(35 * time.Millisecond)

	engine.Stop()
}

// TestRetentionEngine_AutoDeleteExpiredDocument tests policy evaluation auto-deletion.
func TestRetentionEngine_AutoDeleteExpiredDocument(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	engine := NewRetentionEngine(repo)
	engine.SetSearchEngine(searchEngine)

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_doc_001.pdf")
	err := os.WriteFile(filePath, []byte("Test content for retention auto delete"), 0644)
	require.NoError(t, err)

	docID := uuid.New()
	userID := uuid.New()
	pastRetain := time.Now().UTC().AddDate(-1, 0, 0) // Expired 1 year ago

	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "test_doc_001.pdf",
		MimeType:    "application/pdf",
		SizeBytes:   100,
		StoragePath: filePath,
		StorageTier: StorageTierHot,
		Category:    CategoryStudentRecord,
		RetainUntil: pastRetain,
		LegalHold:   false,
		UploadedBy:  userID,
		UploadedAt:  time.Now().UTC().AddDate(-2, 0, 0),
	}

	err = repo.CreateDocument(ctx, doc)
	require.NoError(t, err)

	// Index in search engine
	err = searchEngine.IndexDocument(ctx, doc)
	require.NoError(t, err)

	// Run retention evaluation
	err = engine.EvaluatePolicies(ctx)
	require.NoError(t, err)

	// Assert DB record soft-deleted
	updatedDoc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.NotNil(t, updatedDoc.DeletedAt, "document should be soft-deleted")

	// Assert physical file removed from storage
	_, statErr := os.Stat(filePath)
	assert.True(t, os.IsNotExist(statErr), "physical storage file should be deleted")

	// Assert audit log recorded
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
	assert.True(t, foundExpiredLog, "RETENTION_EXPIRED audit log should exist")
}

// TestRetention_ManualReviewPolicy tests policy evaluation when AutoDelete is false.
func TestRetention_ManualReviewPolicy(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewRetentionEngine(repo)

	policyID := uuid.New()
	policy := &RetentionPolicy{
		ID:                policyID,
		Name:              "Manual Review Policy",
		Category:          CategoryLegalDoc,
		RetentionYears:    5,
		AutoDelete:        false,
		LegalHoldOverride: false,
	}
	err := repo.CreateRetentionPolicy(ctx, policy)
	require.NoError(t, err)

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:                docID,
		Filename:          "legal_doc.pdf",
		StorageTier:       StorageTierHot,
		Category:          CategoryLegalDoc,
		RetentionPolicyID: &policyID,
		RetainUntil:       time.Now().UTC().AddDate(0, 0, -10), // Expired 10 days ago
		LegalHold:         false,
		UploadedAt:        time.Now().UTC().AddDate(-5, 0, 0),
	}
	err = repo.CreateDocument(ctx, doc)
	require.NoError(t, err)

	err = engine.EvaluatePolicies(ctx)
	require.NoError(t, err)

	// Assert document NOT deleted
	docAfter, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Nil(t, docAfter.DeletedAt, "document should NOT be deleted when AutoDelete=false")

	// Assert RETENTION_EXPIRED_MANUAL_REVIEW audit log created
	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	foundManualReviewLog := false
	for _, log := range logs {
		if log.Action == "RETENTION_EXPIRED_MANUAL_REVIEW" {
			foundManualReviewLog = true
			assert.Equal(t, "MANUAL_REVIEW_REQUIRED", log.Details["action"])
			break
		}
	}
	assert.True(t, foundManualReviewLog, "RETENTION_EXPIRED_MANUAL_REVIEW audit log should exist")
}

// TestRetention_LegalHoldProtection tests skipping deletion when LegalHold is active.
func TestRetention_LegalHoldProtection(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewRetentionEngine(repo)

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:              docID,
		Filename:        "litigation_file.pdf",
		StorageTier:     StorageTierHot,
		Category:        CategoryStudentRecord,
		RetainUntil:     time.Now().UTC().AddDate(0, 0, -5), // Expired 5 days ago
		LegalHold:       true,
		LegalHoldReason: "Pending litigation case #999",
		UploadedAt:      time.Now().UTC().AddDate(-3, 0, 0),
	}
	err := repo.CreateDocument(ctx, doc)
	require.NoError(t, err)

	err = engine.EvaluatePolicies(ctx)
	require.NoError(t, err)

	// Assert document NOT deleted
	docAfter, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Nil(t, docAfter.DeletedAt, "active legal hold must block retention deletion")

	// Assert SKIPPED_LEGAL_HOLD audit log
	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	foundSkippedLog := false
	for _, log := range logs {
		if log.Action == "SKIPPED_LEGAL_HOLD" {
			foundSkippedLog = true
			assert.Equal(t, "Pending litigation case #999", log.Details["legal_hold_reason"])
			break
		}
	}
	assert.True(t, foundSkippedLog, "SKIPPED_LEGAL_HOLD audit log should exist")
}

// TestRetention_LegalHoldOverride tests deletion when LegalHoldOverride is true.
func TestRetention_LegalHoldOverride(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewRetentionEngine(repo)

	policyID := uuid.New()
	policy := &RetentionPolicy{
		ID:                policyID,
		Name:              "Override Policy",
		Category:          CategoryBehaviorNote,
		RetentionYears:    1,
		AutoDelete:        true,
		LegalHoldOverride: true,
	}
	err := repo.CreateRetentionPolicy(ctx, policy)
	require.NoError(t, err)

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:                docID,
		Filename:          "behavior_note.pdf",
		StorageTier:       StorageTierHot,
		Category:          CategoryBehaviorNote,
		RetentionPolicyID: &policyID,
		RetainUntil:       time.Now().UTC().AddDate(0, 0, -2),
		LegalHold:         true,
		LegalHoldReason:   "Minor inquiry",
		UploadedAt:        time.Now().UTC().AddDate(-1, 0, 0),
	}
	err = repo.CreateDocument(ctx, doc)
	require.NoError(t, err)

	err = engine.EvaluatePolicies(ctx)
	require.NoError(t, err)

	// Assert document WAS deleted due to LegalHoldOverride
	docAfter, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.NotNil(t, docAfter.DeletedAt, "document should be deleted when policy has LegalHoldOverride=true")
}

// TestRetentionEngine_ApplyAndReleaseLegalHold tests legal hold toggle methods.
func TestRetentionEngine_ApplyAndReleaseLegalHold(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewRetentionEngine(repo)

	userID := uuid.New()
	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "doc_hold_test.pdf",
		StorageTier: StorageTierHot,
		Category:    CategoryStudentRecord,
		UploadedAt:  time.Now().UTC(),
	}
	err := repo.CreateDocument(ctx, doc)
	require.NoError(t, err)

	// Apply Legal Hold
	appliedDoc, err := engine.ApplyLegalHold(ctx, docID, "Audit hold #101", userID)
	require.NoError(t, err)
	assert.True(t, appliedDoc.LegalHold)
	assert.Equal(t, "Audit hold #101", appliedDoc.LegalHoldReason)

	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	foundApplyLog := false
	for _, log := range logs {
		if log.Action == "APPLY_LEGAL_HOLD" {
			foundApplyLog = true
			break
		}
	}
	assert.True(t, foundApplyLog, "APPLY_LEGAL_HOLD audit log should exist")

	// Release Legal Hold
	releasedDoc, err := engine.ReleaseLegalHold(ctx, docID, "Audit resolved", userID)
	require.NoError(t, err)
	assert.False(t, releasedDoc.LegalHold)

	logsAfter, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	foundReleaseLog := false
	for _, log := range logsAfter {
		if log.Action == "RELEASE_LEGAL_HOLD" {
			foundReleaseLog = true
			break
		}
	}
	assert.True(t, foundReleaseLog, "RELEASE_LEGAL_HOLD audit log should exist")
}

// TestRetentionEngine_Adversarial_ConcurrentStartStop verifies Retention Engine thread safety
// under rapid concurrent Start() and Stop() cycles without data races or panics.
func TestRetentionEngine_Adversarial_ConcurrentStartStop(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewRetentionEngine(repo)
	engine.SetInterval(1 * time.Millisecond)

	const numGoroutines = 20
	const iterations = 50
	var wg sync.WaitGroup

	// Concurrently call Start and Stop from multiple goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if (id+j)%2 == 0 {
					_ = engine.Start(ctx)
				} else {
					engine.Stop()
				}
				time.Sleep(100 * time.Microsecond)
			}
		}(i)
	}

	wg.Wait()
	engine.Stop()

	// Rapid start/stop sequence
	for cycle := 0; cycle < 20; cycle++ {
		var cycleWg sync.WaitGroup
		cycleWg.Add(2)
		go func() {
			defer cycleWg.Done()
			_ = engine.Start(ctx)
		}()
		go func() {
			defer cycleWg.Done()
			engine.Stop()
		}()
		cycleWg.Wait()
	}
	engine.Stop()
}

// TestRetentionEngine_Adversarial_EvaluatePolicies_Idempotency verifies idempotency of EvaluatePolicies.
// Running EvaluatePolicies multiple times on non-auto-delete and legal-hold documents must result in
// exactly 1 audit log entry per document.
func TestRetentionEngine_Adversarial_EvaluatePolicies_Idempotency(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewRetentionEngine(repo)

	policyID := uuid.New()
	policy := &RetentionPolicy{
		ID:                policyID,
		Name:              "Manual Review Only Policy",
		Category:          CategoryLegalDoc,
		RetentionYears:    1,
		AutoDelete:        false,
		LegalHoldOverride: false,
	}
	err := repo.CreateRetentionPolicy(ctx, policy)
	require.NoError(t, err)

	// Document 1: Non-auto-delete document
	doc1ID := uuid.New()
	doc1 := &ArchiveDocument{
		ID:                doc1ID,
		Filename:          "manual_review_doc.pdf",
		StorageTier:       StorageTierHot,
		Category:          CategoryLegalDoc,
		RetentionPolicyID: &policyID,
		RetainUntil:       time.Now().UTC().AddDate(0, 0, -10),
		LegalHold:         false,
		UploadedAt:        time.Now().UTC().AddDate(-2, 0, 0),
	}
	err = repo.CreateDocument(ctx, doc1)
	require.NoError(t, err)

	// Document 2: Legal hold document (with AutoDelete=true in default category policy)
	doc2ID := uuid.New()
	doc2 := &ArchiveDocument{
		ID:              doc2ID,
		Filename:        "legal_hold_doc.pdf",
		StorageTier:     StorageTierHot,
		Category:        CategoryStudentRecord,
		RetainUntil:     time.Now().UTC().AddDate(0, 0, -10),
		LegalHold:       true,
		LegalHoldReason: "Ongoing lawsuit #101",
		UploadedAt:      time.Now().UTC().AddDate(-2, 0, 0),
	}
	err = repo.CreateDocument(ctx, doc2)
	require.NoError(t, err)

	// Run EvaluatePolicies 5 times sequentially
	for i := 0; i < 5; i++ {
		err = engine.EvaluatePolicies(ctx)
		require.NoError(t, err)
	}

	// Verify Document 1 has exactly 1 RETENTION_EXPIRED_MANUAL_REVIEW audit log
	logs1, err := repo.GetAuditLogsByDocument(ctx, doc1ID)
	require.NoError(t, err)
	manualReviewLogCount := 0
	for _, log := range logs1 {
		if log.Action == AuditActionRetentionExpiredManualReview {
			manualReviewLogCount++
		}
	}
	assert.Equal(t, 1, manualReviewLogCount, "Doc 1 should have exactly 1 RETENTION_EXPIRED_MANUAL_REVIEW audit log entry across 5 evaluations")

	// Verify Document 2 has exactly 1 SKIPPED_LEGAL_HOLD audit log
	logs2, err := repo.GetAuditLogsByDocument(ctx, doc2ID)
	require.NoError(t, err)
	skippedHoldLogCount := 0
	for _, log := range logs2 {
		if log.Action == AuditActionSkippedLegalHold {
			skippedHoldLogCount++
		}
	}
	assert.Equal(t, 1, skippedHoldLogCount, "Doc 2 should have exactly 1 SKIPPED_LEGAL_HOLD audit log entry across 5 evaluations")

	// Neither document should be deleted
	doc1After, err := repo.GetDocumentByID(ctx, doc1ID)
	require.NoError(t, err)
	assert.Nil(t, doc1After.DeletedAt, "Doc 1 (non-auto-delete) should not be deleted")

	doc2After, err := repo.GetDocumentByID(ctx, doc2ID)
	require.NoError(t, err)
	assert.Nil(t, doc2After.DeletedAt, "Doc 2 (legal-hold) should not be deleted")
}
