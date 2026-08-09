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

func TestExtractTextFromBytes_Formats(t *testing.T) {
	t.Run("plain text with UTF-8 character preservation", func(t *testing.T) {
		input := []byte("Document Title: Café & Résumé — Bahasa Indonesia 🇮🇩")
		text, err := ExtractTextFromBytes(input, "text/plain", "notes.txt")
		require.NoError(t, err)
		assert.Contains(t, text, "Café")
		assert.Contains(t, text, "Résumé")
		assert.Contains(t, text, "Bahasa Indonesia")
	})

	t.Run("PDF text stream extraction", func(t *testing.T) {
		pdfData := []byte("%PDF-1.4\nBT\n(Hello PDF World) Tj\nET\n")
		text, err := ExtractTextFromBytes(pdfData, "application/pdf", "sample.pdf")
		require.NoError(t, err)
		assert.Contains(t, text, "Hello PDF World")
	})

	t.Run("DOCX text extraction", func(t *testing.T) {
		// Construct minimal in-memory DOCX ZIP
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		f, err := zw.Create("word/document.xml")
		require.NoError(t, err)
		_, err = f.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><w:document><w:body><w:p><w:t>Hello DOCX Document Content</w:t></w:p></w:body></w:document>`))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		text, err := ExtractTextFromBytes(buf.Bytes(), "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "test.docx")
		require.NoError(t, err)
		assert.Contains(t, text, "Hello DOCX Document Content")
	})

	t.Run("XLSX text extraction", func(t *testing.T) {
		// Construct minimal in-memory XLSX ZIP
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		f, err := zw.Create("xl/sharedStrings.xml")
		require.NoError(t, err)
		_, err = f.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><sst><si><t>Shared Excel Cell Value</t></si></sst>`))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		text, err := ExtractTextFromBytes(buf.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "test.xlsx")
		require.NoError(t, err)
		assert.Contains(t, text, "Shared Excel Cell Value")
	})

	t.Run("ZIP archive recursion", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		f, err := zw.Create("internal_doc.txt")
		require.NoError(t, err)
		_, err = f.Write([]byte("Archived Text File Inside ZIP"))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		text, err := ExtractTextFromBytes(buf.Bytes(), "application/zip", "archive.zip")
		require.NoError(t, err)
		assert.Contains(t, text, "internal_doc.txt")
		assert.Contains(t, text, "Archived Text File Inside ZIP")
	})
}

func TestGoOCRWorkerPool_Lifecycle(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()

	pool := NewGoOCRWorkerPool(2, 20, repo, searchEngine)
	pool.Start()
	defer pool.Stop()

	// Create test document in repo
	docID := uuid.New()
	tmpFile := filepath.Join(os.TempDir(), "ocr_test_doc.txt")
	require.NoError(t, os.WriteFile(tmpFile, []byte("Content for worker pool lifecycle test"), 0644))
	defer os.Remove(tmpFile)

	doc := &ArchiveDocument{
		ID:        docID,
		Filename:  "ocr_test_doc.txt",
		Category:  CategoryOther,
		OCRStatus: OCRStatusPending,
		UploadedAt: time.Now(),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	// Enqueue job
	require.NoError(t, pool.Enqueue(docID, tmpFile, "text/plain"))

	// Poll until OCR completes
	require.Eventually(t, func() bool {
		d, err := repo.GetDocumentByID(ctx, docID)
		if err != nil {
			return false
		}
		return d.OCRStatus == OCRStatusCompleted
	}, 3*time.Second, 50*time.Millisecond)

	updatedDoc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, OCRStatusCompleted, updatedDoc.OCRStatus)
	assert.Contains(t, updatedDoc.OCRText, "Content for worker pool lifecycle test")

	status := pool.Status()
	assert.GreaterOrEqual(t, status.ProcessedCount, int64(1))
}

func TestGoOCRWorkerPool_FailureAndPanicRecovery(t *testing.T) {
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()

	pool := NewGoOCRWorkerPool(1, 10, repo, searchEngine)
	pool.Start()
	defer pool.Stop()

	// Enqueue job for missing document
	missingDocID := uuid.New()
	// Should not crash pool
	_ = pool.Enqueue(missingDocID, "nonexistent.txt", "text/plain")

	time.Sleep(100 * time.Millisecond)

	status := pool.Status()
	assert.GreaterOrEqual(t, status.ProcessedCount, int64(1))
}

func TestGoOCRWorkerPool_GracefulStopDraining(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()

	pool := NewGoOCRWorkerPool(2, 50, repo, searchEngine)

	var docIDs []uuid.UUID
	for i := 0; i < 5; i++ {
		dID := uuid.New()
		docIDs = append(docIDs, dID)
		doc := &ArchiveDocument{
			ID:        dID,
			Filename:  "test.txt",
			Category:  CategoryOther,
			OCRStatus: OCRStatusPending,
			UploadedAt: time.Now(),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))
	}

	pool.Start()
	for _, id := range docIDs {
		_ = pool.Enqueue(id, "", "text/plain")
	}

	// Stop pool - should drain remaining queued jobs
	pool.Stop()

	status := pool.Status()
	assert.Equal(t, int64(5), status.ProcessedCount)
}
