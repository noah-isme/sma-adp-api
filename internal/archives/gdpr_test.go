package archives

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGDPRProcessor_Access(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	signer := NewHMACSignedURLSigner("secret", "/api/v1/archives")
	bulkProc := NewBulkProcessor(repo, searchEngine, signer)
	processor := NewGDPRProcessor(repo, searchEngine, signer, bulkProc)

	userID := uuid.New()
	tempDir := filepath.Join(os.TempDir(), "gdpr_test")
	_ = os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "stu_doc.pdf")
	_ = os.WriteFile(filePath, []byte("Sample PDF Content"), 0644)

	doc := &ArchiveDocument{
		ID:          uuid.New(),
		Filename:    "stu_doc.pdf",
		StoragePath: filePath,
		Category:    CategoryStudentRecord,
		UploadedBy:  userID,
		UploadedAt:  time.Now(),
		Metadata:    map[string]interface{}{"student_id": "stu_101"},
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	req := GDPRRequest{
		Type:           "ACCESS",
		SubjectID:      userID.String(),
		RequesterEmail: "privacy@school.edu",
	}

	resp, err := processor.ProcessGDPRRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "ACCESS", resp.Type)
	assert.Equal(t, "COMPLETED", resp.Status)
	assert.NotEmpty(t, resp.ExportURL)
}

func TestGDPRProcessor_Rectification(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	signer := NewHMACSignedURLSigner("secret", "/api/v1/archives")
	bulkProc := NewBulkProcessor(repo, searchEngine, signer)
	processor := NewGDPRProcessor(repo, searchEngine, signer, bulkProc)

	doc := &ArchiveDocument{
		ID:       uuid.New(),
		Filename: "grade_rep.pdf",
		Checksum: "abc123checksum",
		Category: CategoryGradeReport,
		Metadata: map[string]interface{}{"student_id": "stu_old"},
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	req := GDPRRequest{
		Type:       "RECTIFICATION",
		SubjectID:  "stu_old",
		DocumentID: &doc.ID,
		Corrections: map[string]string{
			"student_id": "stu_new",
			"term":       "Fall 2026",
		},
	}

	resp, err := processor.ProcessGDPRRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", resp.Status)

	retrieved, err := repo.GetDocumentByID(ctx, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, "abc123checksum", retrieved.Checksum, "checksum must be preserved")
	assert.Equal(t, "stu_new", retrieved.Metadata["student_id"])
	assert.Equal(t, "Fall 2026", retrieved.Metadata["term"])
}

func TestGDPRProcessor_Erasure(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	signer := NewHMACSignedURLSigner("secret", "/api/v1/archives")
	bulkProc := NewBulkProcessor(repo, searchEngine, signer)
	processor := NewGDPRProcessor(repo, searchEngine, signer, bulkProc)

	// 1. Erasure blocked by Legal Hold
	docLegalHold := &ArchiveDocument{
		ID:        uuid.New(),
		Filename:  "held.pdf",
		LegalHold: true,
	}
	require.NoError(t, repo.CreateDocument(ctx, docLegalHold))

	reqHold := GDPRRequest{
		Type:       "ERASURE",
		DocumentID: &docLegalHold.ID,
	}
	_, errHold := processor.ProcessGDPRRequest(ctx, reqHold)
	assert.ErrorIs(t, errHold, ErrLegalHoldActive)

	// 2. Erasure blocked by unexpired retention
	docUnexpired := &ArchiveDocument{
		ID:          uuid.New(),
		Filename:    "unexpired.pdf",
		RetainUntil: time.Now().AddDate(1, 0, 0),
		LegalHold:   false,
	}
	require.NoError(t, repo.CreateDocument(ctx, docUnexpired))

	reqUnexp := GDPRRequest{
		Type:       "ERASURE",
		DocumentID: &docUnexpired.ID,
	}
	_, errUnexp := processor.ProcessGDPRRequest(ctx, reqUnexp)
	assert.ErrorIs(t, errUnexp, ErrRetentionNotExpired)

	// 3. Erasure success (past retention date & no legal hold)
	docExpired := &ArchiveDocument{
		ID:          uuid.New(),
		Filename:    "expired.pdf",
		RetainUntil: time.Now().AddDate(-1, 0, 0),
		LegalHold:   false,
	}
	require.NoError(t, repo.CreateDocument(ctx, docExpired))

	reqSuccess := GDPRRequest{
		Type:       "ERASURE",
		DocumentID: &docExpired.ID,
	}
	respSuccess, errSuccess := processor.ProcessGDPRRequest(ctx, reqSuccess)
	require.NoError(t, errSuccess)
	assert.Equal(t, "COMPLETED", respSuccess.Status)

	retrieved, err := repo.GetDocumentByID(ctx, docExpired.ID)
	require.NoError(t, err)
	assert.NotNil(t, retrieved.DeletedAt)
}

func TestGDPRProcessor_Portability(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	signer := NewHMACSignedURLSigner("secret", "/api/v1/archives")
	bulkProc := NewBulkProcessor(repo, searchEngine, signer)
	processor := NewGDPRProcessor(repo, searchEngine, signer, bulkProc)

	userID := uuid.New()
	doc := &ArchiveDocument{
		ID:         uuid.New(),
		Filename:   "transcript.pdf",
		Category:   CategoryStudentRecord,
		UploadedBy: userID,
		UploadedAt: time.Now(),
		OCRText:    "Extracted Transcript Text",
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	req := GDPRRequest{
		Type:      "PORTABILITY",
		SubjectID: userID.String(),
	}

	resp, err := processor.ProcessGDPRRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "PORTABILITY", resp.Type)
	assert.Equal(t, "COMPLETED", resp.Status)
	assert.NotNil(t, resp.Data)
}
