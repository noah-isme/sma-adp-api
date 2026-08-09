package archives

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBulkProcessor_StreamBulkZip(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	signer := NewHMACSignedURLSigner("secret", "/api/v1/archives")
	processor := NewBulkProcessor(repo, searchEngine, signer)

	tempDir := filepath.Join(os.TempDir(), "bulk_test_zip")
	_ = os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	file1Path := filepath.Join(tempDir, "doc1.txt")
	_ = os.WriteFile(file1Path, []byte("Content of Doc 1"), 0644)

	doc1 := &ArchiveDocument{
		ID:          uuid.New(),
		Filename:    "doc1.txt",
		StoragePath: file1Path,
		Category:    CategoryStudentRecord,
		UploadedAt:  time.Now(),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc1))

	var buf bytes.Buffer
	err := processor.StreamBulkZip(ctx, []uuid.UUID{doc1.ID}, &buf)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, zr.File, 2) // doc1.txt + manifest/audit
	assert.Equal(t, "doc1.txt", zr.File[0].Name)
}

func TestBulkProcessor_DeleteWithLegalHold(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	signer := NewHMACSignedURLSigner("secret", "/api/v1/archives")
	processor := NewBulkProcessor(repo, searchEngine, signer)
	userID := uuid.New()

	// Doc 1: normal doc
	doc1 := &ArchiveDocument{
		ID:        uuid.New(),
		Filename:  "normal.pdf",
		Category:  CategoryStudentRecord,
		LegalHold: false,
	}
	// Doc 2: under legal hold
	doc2 := &ArchiveDocument{
		ID:              uuid.New(),
		Filename:        "legal_hold.pdf",
		Category:        CategoryLegalDoc,
		LegalHold:       true,
		LegalHoldReason: "Pending lawsuit",
	}

	require.NoError(t, repo.CreateDocument(ctx, doc1))
	require.NoError(t, repo.CreateDocument(ctx, doc2))

	req := BulkActionRequest{
		Action: "DELETE",
		IDs:    []uuid.UUID{doc1.ID, doc2.ID},
	}

	resp, err := processor.ProcessBulkAction(ctx, req, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.ProcessedCount)
	assert.Equal(t, 1, resp.SkippedCount)
	assert.Contains(t, resp.SkippedIDs, doc2.ID)

	// Verify doc1 soft deleted
	retrieved1, err := repo.GetDocumentByID(ctx, doc1.ID)
	require.NoError(t, err)
	assert.NotNil(t, retrieved1.DeletedAt)

	// Verify doc2 NOT deleted
	retrieved2, err := repo.GetDocumentByID(ctx, doc2.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved2.DeletedAt)

	// Verify audit log for doc2 has SKIPPED_LEGAL_HOLD
	logs, err := repo.GetAuditLogsByDocument(ctx, doc2.ID)
	require.NoError(t, err)
	hasSkippedAudit := false
	for _, l := range logs {
		if l.Action == AuditActionSkippedLegalHold {
			hasSkippedAudit = true
			break
		}
	}
	assert.True(t, hasSkippedAudit, "must log SKIPPED_LEGAL_HOLD for doc under legal hold")
}

func TestBulkProcessor_ChangeCategory(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	signer := NewHMACSignedURLSigner("secret", "/api/v1/archives")
	processor := NewBulkProcessor(repo, searchEngine, signer)
	userID := uuid.New()

	doc := &ArchiveDocument{
		ID:       uuid.New(),
		Filename: "report.pdf",
		Category: CategoryStudentRecord,
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	req := BulkActionRequest{
		Action: "CHANGE_CATEGORY",
		IDs:    []uuid.UUID{doc.ID},
		Parameters: map[string]string{
			"category": string(CategoryFinancialDoc),
		},
	}

	resp, err := processor.ProcessBulkAction(ctx, req, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.ProcessedCount)

	retrieved, err := repo.GetDocumentByID(ctx, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, CategoryFinancialDoc, retrieved.Category)
}

func TestBulkProcessor_ApplyRetention(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	signer := NewHMACSignedURLSigner("secret", "/api/v1/archives")
	processor := NewBulkProcessor(repo, searchEngine, signer)
	userID := uuid.New()

	policy, err := repo.GetDefaultPolicyByCategory(ctx, CategoryFinancialDoc)
	require.NoError(t, err)

	doc := &ArchiveDocument{
		ID:       uuid.New(),
		Filename: "invoice.pdf",
		Category: CategoryFinancialDoc,
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	req := BulkActionRequest{
		Action: "APPLY_RETENTION",
		IDs:    []uuid.UUID{doc.ID},
		Parameters: map[string]string{
			"retention_policy_id": policy.ID.String(),
		},
	}

	resp, err := processor.ProcessBulkAction(ctx, req, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.ProcessedCount)

	retrieved, err := repo.GetDocumentByID(ctx, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, &policy.ID, retrieved.RetentionPolicyID)
}
