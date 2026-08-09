package archives

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveLifecycle_Integration(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	clientIP := "192.168.1.50"

	// -------------------------------------------------------------------------
	// Environment & Subsystem Setup
	// -------------------------------------------------------------------------
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	workerPool := NewGoOCRWorkerPool(2, 50, repo, searchEngine)
	workerPool.Start()
	defer workerPool.Stop()

	signer := NewHMACSignedURLSigner("test_secret_key_12345", "/api/v1/archives")
	retentionEngine := NewRetentionEngine(repo)
	service := NewArchiveService(repo, searchEngine, workerPool, signer, retentionEngine)

	var docID uuid.UUID

	// -------------------------------------------------------------------------
	// Step 1: Archive Document Upload
	// -------------------------------------------------------------------------
	t.Run("Step 1 - Archive Document Upload", func(t *testing.T) {
		filename := "transcript_stu_001.pdf"
		category := CategoryStudentRecord
		tags := []string{"transcript", "grade-10"}
		metadata := map[string]any{"student_id": "stu_001", "term_id": "term_2025_1"}
		content := []byte("%PDF-1.4 Official Academic Transcript for Student ID stu_001. Final Grade: A in Mathematics and Science.")

		doc, err := service.UploadDocument(ctx, filename, category, tags, metadata, content, userID)
		require.NoError(t, err, "document upload should succeed")
		require.NotNil(t, doc, "uploaded document struct should not be nil")

		assert.NotEqual(t, uuid.Nil, doc.ID, "document ID must be assigned")
		assert.Equal(t, filename, doc.Filename)
		assert.Equal(t, category, doc.Category)
		assert.Equal(t, OCRStatusPending, doc.OCRStatus, "initial OCR status must be PENDING")
		assert.Equal(t, StorageTierHot, doc.StorageTier, "initial storage tier must be HOT")
		assert.False(t, doc.LegalHold, "initial legal hold must be false")
		assert.NotEmpty(t, doc.Checksum, "document checksum must be populated")

		docID = doc.ID
	})

	require.NotEqual(t, uuid.Nil, docID, "docID must be initialized")

	// -------------------------------------------------------------------------
	// Step 2: Async OCR Worker Pool Processing
	// -------------------------------------------------------------------------
	t.Run("Step 2 - Async OCR Worker Processing (PENDING -> COMPLETED)", func(t *testing.T) {
		var doc *ArchiveDocument
		var fetchErr error

		// Poll for async OCR completion (timeout after 5 seconds)
		require.Eventually(t, func() bool {
			doc, fetchErr = service.GetDocument(ctx, docID)
			if fetchErr != nil {
				return false
			}
			return doc.OCRStatus == OCRStatusCompleted
		}, 5*time.Second, 50*time.Millisecond, "OCR worker should process job and set status to COMPLETED")

		assert.NoError(t, fetchErr)
		assert.Equal(t, OCRStatusCompleted, doc.OCRStatus, "OCR status must transition to COMPLETED")
		assert.NotEmpty(t, doc.OCRText, "extracted OCR text must not be empty")
		assert.Contains(t, doc.OCRText, "Mathematics", "OCR text must contain extracted keywords")
	})

	// -------------------------------------------------------------------------
	// Step 3: Meilisearch Full-Text Search Indexing and Query Verification
	// -------------------------------------------------------------------------
	t.Run("Step 3 - Meilisearch Full-Text Search", func(t *testing.T) {
		// Verify positive search hit
		req := SearchRequest{
			Query:    "Mathematics",
			Category: CategoryStudentRecord,
			Page:     1,
			Limit:    10,
		}

		result, err := service.Search(ctx, req)
		require.NoError(t, err, "search query should succeed")
		require.NotNil(t, result, "search result should not be nil")
		assert.GreaterOrEqual(t, result.Total, int64(1), "search should find at least 1 document match")
		require.Len(t, result.Data, 1, "search result page should contain matching hit")

		hit := result.Data[0]
		assert.Equal(t, docID, hit.ID)
		assert.Equal(t, "transcript_stu_001.pdf", hit.Filename)
		assert.Contains(t, hit.Snippet, "<em>Mathematics</em>", "search snippet should highlight matching query term")

		// Verify negative search hit
		negReq := SearchRequest{
			Query:    "NonExistentQueryTerm9999",
			Category: CategoryStudentRecord,
			Page:     1,
			Limit:    10,
		}
		negResult, err := service.Search(ctx, negReq)
		require.NoError(t, err)
		assert.Equal(t, int64(0), negResult.Total, "negative query should return 0 search hits")
	})

	// -------------------------------------------------------------------------
	// Step 4: Signed URL Download Token Generation and Validation
	// -------------------------------------------------------------------------
	t.Run("Step 4 - Signed URL Token Generation & IP Binding Validation", func(t *testing.T) {
		downloadURLStr, err := service.GenerateDownloadURL(ctx, docID, clientIP, 30*time.Minute)
		require.NoError(t, err, "generating signed download URL should succeed")
		require.NotEmpty(t, downloadURLStr)

		parsedURL, err := url.Parse(downloadURLStr)
		require.NoError(t, err)
		token := parsedURL.Query().Get("token")
		require.NotEmpty(t, token, "download URL must contain token parameter")

		// 1. Validate with matching client IP (Success)
		validatedDocID, err := service.ValidateDownloadToken(token, clientIP)
		assert.NoError(t, err, "validating signed URL token with matching IP should succeed")
		assert.Equal(t, docID, validatedDocID, "validated document ID must match original")

		// 2. Validate with mismatched client IP (Failure - ErrIPMismatch)
		mismatchedIP := "10.0.0.1"
		_, errMismatch := service.ValidateDownloadToken(token, mismatchedIP)
		assert.ErrorIs(t, errMismatch, ErrIPMismatch, "validating with wrong IP should return ErrIPMismatch")

		// 3. Validate with tampered token string (Failure - ErrInvalidToken)
		tamperedToken := token + "tampered"
		_, errTampered := service.ValidateDownloadToken(tamperedToken, clientIP)
		assert.ErrorIs(t, errTampered, ErrInvalidToken, "validating tampered token should return ErrInvalidToken")
	})

	// -------------------------------------------------------------------------
	// Step 5: Legal Hold Application Toggle & Verification
	// -------------------------------------------------------------------------
	t.Run("Step 5 - Legal Hold Application Toggle & Verification", func(t *testing.T) {
		holdReason := "Litigation hold for audit reference #2026-99"

		updatedDoc, err := service.SetLegalHold(ctx, docID, true, holdReason, userID)
		require.NoError(t, err, "setting legal hold should succeed")
		assert.True(t, updatedDoc.LegalHold, "document legal_hold flag must be true")
		assert.Equal(t, holdReason, updatedDoc.LegalHoldReason)

		// Verify audit log record
		logs, err := repo.GetAuditLogsByDocument(ctx, docID)
		require.NoError(t, err)
		hasLegalHoldAudit := false
		for _, log := range logs {
			if log.Action == "APPLY_LEGAL_HOLD" {
				hasLegalHoldAudit = true
				break
			}
		}
		assert.True(t, hasLegalHoldAudit, "audit log for APPLY_LEGAL_HOLD must be recorded")
	})

	// -------------------------------------------------------------------------
	// Step 6: Retention Policy Evaluator Execution & Legal Hold Protection
	// -------------------------------------------------------------------------
	t.Run("Step 6 - Retention Evaluator Execution & Legal Hold Protection", func(t *testing.T) {
		// 1. Manually expire document retain_until date (set to 1 year in the past)
		doc, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)

		pastRetainUntil := time.Now().AddDate(-1, 0, 0)
		doc.RetainUntil = pastRetainUntil
		err = repo.UpdateDocument(ctx, doc)
		require.NoError(t, err)

		// 2. Trigger retention policy evaluation while legal hold is ACTIVE
		err = service.RunRetentionEvaluator(ctx)
		require.NoError(t, err)

		// Assert document is NOT deleted (legal hold protection active!)
		docAfterEval, err := service.GetDocument(ctx, docID)
		require.NoError(t, err)
		assert.Nil(t, docAfterEval.DeletedAt, "document under active legal hold must NOT be auto-deleted")

		// Assert SKIPPED_LEGAL_HOLD audit log entry exists
		logs, err := repo.GetAuditLogsByDocument(ctx, docID)
		require.NoError(t, err)
		hasSkippedAudit := false
		for _, log := range logs {
			if log.Action == "SKIPPED_LEGAL_HOLD" {
				hasSkippedAudit = true
				break
			}
		}
		assert.True(t, hasSkippedAudit, "audit log for SKIPPED_LEGAL_HOLD must be created")

		// 3. Test GDPR Erasure Protection on Active Legal Hold
		errGDPR := service.HandleGDPRRequest(ctx, "ERASURE", docID)
		assert.ErrorIs(t, errGDPR, ErrLegalHoldActive, "GDPR erasure request must be blocked when legal hold is active")

		// 4. Release legal hold and re-evaluate retention policy
		releasedDoc, err := service.SetLegalHold(ctx, docID, false, "Litigation resolved", userID)
		require.NoError(t, err)
		assert.False(t, releasedDoc.LegalHold)

		err = service.RunRetentionEvaluator(ctx)
		require.NoError(t, err)

		// Assert document is now soft-deleted
		deletedDoc, err := repo.GetDocumentByID(ctx, docID)
		require.NoError(t, err)
		assert.NotNil(t, deletedDoc.DeletedAt, "document should be auto-deleted after legal hold is released and retain_until is expired")

		// Assert RETENTION_EXPIRED audit log entry exists
		logsAfterRelease, err := repo.GetAuditLogsByDocument(ctx, docID)
		require.NoError(t, err)
		hasExpiredAudit := false
		for _, log := range logsAfterRelease {
			if log.Action == "RETENTION_EXPIRED" {
				hasExpiredAudit = true
				break
			}
		}
		assert.True(t, hasExpiredAudit, "audit log for RETENTION_EXPIRED must be created upon auto-deletion")
	})
}
