package archives

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ADVERSARIAL CHALLENGER M2_2 TEST SUITE
// =============================================================================

// TestAdversarial_ZeroByteFiles verifies that zero-byte files of all formats are handled gracefully without errors or panics.
func TestAdversarial_ZeroByteFiles(t *testing.T) {
	ctx := context.Background()
	registry := NewParserRegistry()

	formats := []struct {
		name     string
		mime     string
		filename string
	}{
		{"Zero-byte Text", "text/plain", "empty.txt"},
		{"Zero-byte PDF", "application/pdf", "empty.pdf"},
		{"Zero-byte DOCX", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "empty.docx"},
		{"Zero-byte XLSX", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "empty.xlsx"},
		{"Zero-byte Image", "image/png", "empty.png"},
		{"Zero-byte ZIP", "application/zip", "empty.zip"},
	}

	for _, f := range formats {
		t.Run(f.name, func(t *testing.T) {
			text, err := registry.Parse(ctx, []byte{}, f.mime, f.filename)
			require.NoError(t, err, "Zero-byte file parsing must not return error for %s", f.name)
			assert.Equal(t, "", text, "Zero-byte file should return empty string")

			// Also verify via ExtractTextFromBytes
			extracted, err := ExtractTextFromBytes([]byte{}, f.mime, f.filename)
			require.NoError(t, err)
			assert.Equal(t, "", extracted)
		})
	}
}

// TestAdversarial_ZeroByteFiles_WorkerPool verifies OCRWorkerPool behavior when processing zero-byte files.
func TestAdversarial_ZeroByteFiles_WorkerPool(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	pool := NewGoOCRWorkerPool(2, 10, repo, searchEngine)
	pool.Start()
	defer pool.Stop()

	// Create zero-byte file on disk
	zeroPath := filepath.Join(t.TempDir(), "zero_bytes.docx")
	require.NoError(t, os.WriteFile(zeroPath, []byte{}, 0644))

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:         docID,
		Filename:   "zero_bytes.docx",
		Category:   CategoryFinancialDoc,
		OCRStatus:  OCRStatusPending,
		UploadedAt: time.Now(),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	require.NoError(t, pool.Enqueue(docID, zeroPath, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"))

	require.Eventually(t, func() bool {
		d, err := repo.GetDocumentByID(ctx, docID)
		return err == nil && d.OCRStatus == OCRStatusCompleted
	}, 5*time.Second, 50*time.Millisecond, "Zero-byte file should successfully complete OCR with fallback text")

	updatedDoc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, OCRStatusCompleted, updatedDoc.OCRStatus)
	assert.Contains(t, updatedDoc.OCRText, "Document: zero_bytes.docx")
}

// TestAdversarial_NestedZipArchives tests unpacking ZIP files containing nested ZIP archives and zero-byte files.
func TestAdversarial_NestedZipArchives(t *testing.T) {
	ctx := context.Background()
	registry := NewParserRegistry()

	t.Run("ZIP with nested sub-ZIP and text files", func(t *testing.T) {
		// Create inner zip
		innerBuf := new(bytes.Buffer)
		innerZw := zip.NewWriter(innerBuf)
		f1, err := innerZw.Create("inner_file.txt")
		require.NoError(t, err)
		_, err = f1.Write([]byte("Inner ZIP secret content"))
		require.NoError(t, err)
		require.NoError(t, innerZw.Close())

		// Create outer zip
		outerBuf := new(bytes.Buffer)
		outerZw := zip.NewWriter(outerBuf)

		// Add outer text file
		fOuter, err := outerZw.Create("outer_notes.txt")
		require.NoError(t, err)
		_, err = fOuter.Write([]byte("Outer ZIP readable content"))
		require.NoError(t, err)

		// Add nested zip file
		fSubZip, err := outerZw.Create("nested_archive.zip")
		require.NoError(t, err)
		_, err = fSubZip.Write(innerBuf.Bytes())
		require.NoError(t, err)

		require.NoError(t, outerZw.Close())

		text, err := registry.Parse(ctx, outerBuf.Bytes(), "application/zip", "outer.zip")
		require.NoError(t, err)
		assert.Contains(t, text, "outer_notes.txt")
		assert.Contains(t, text, "Outer ZIP readable content")
		// Nested zip should be skipped cleanly without panic or error
		assert.NotContains(t, text, "nested_archive.zip")
	})

	t.Run("ZIP containing zero-byte member files", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)

		fEmpty, err := zw.Create("empty_member.txt")
		require.NoError(t, err)
		_, err = fEmpty.Write([]byte{})
		require.NoError(t, err)

		fValid, err := zw.Create("valid_member.txt")
		require.NoError(t, err)
		_, err = fValid.Write([]byte("Valid content inside ZIP"))
		require.NoError(t, err)

		require.NoError(t, zw.Close())

		text, err := registry.Parse(ctx, buf.Bytes(), "application/zip", "archive.zip")
		require.NoError(t, err)
		assert.Contains(t, text, "valid_member.txt")
		assert.Contains(t, text, "Valid content inside ZIP")
	})

	t.Run("Deeply nested multi-level ZIP archive structure", func(t *testing.T) {
		// Create level 3 zip
		l3Buf := new(bytes.Buffer)
		l3Zw := zip.NewWriter(l3Buf)
		f3, _ := l3Zw.Create("deep.txt")
		_, _ = f3.Write([]byte("Deep level 3 content"))
		_ = l3Zw.Close()

		// Create level 2 zip containing level 3
		l2Buf := new(bytes.Buffer)
		l2Zw := zip.NewWriter(l2Buf)
		f2, _ := l2Zw.Create("level3.zip")
		_, _ = f2.Write(l3Buf.Bytes())
		_ = l2Zw.Close()

		// Create level 1 zip containing level 2
		l1Buf := new(bytes.Buffer)
		l1Zw := zip.NewWriter(l1Buf)
		f1, _ := l1Zw.Create("level2.zip")
		_, _ = f1.Write(l2Buf.Bytes())
		f1Text, _ := l1Zw.Create("level1_readme.txt")
		_, _ = f1Text.Write([]byte("Level 1 Root Readme"))
		_ = l1Zw.Close()

		text, err := registry.Parse(ctx, l1Buf.Bytes(), "application/zip", "level1.zip")
		require.NoError(t, err)
		assert.Contains(t, text, "level1_readme.txt")
		assert.Contains(t, text, "Level 1 Root Readme")
		// Nested zips are skipped to protect against zip bomb recursion
		assert.NotContains(t, text, "level2.zip")
	})
}

// TestAdversarial_CorruptedDocxXlsxStructures tests parsing of malformed DOCX/XLSX containers and XML data.
func TestAdversarial_CorruptedDocxXlsxStructures(t *testing.T) {
	ctx := context.Background()
	parser := &DocxXlsxParser{}

	t.Run("Random garbage bytes with DOCX mime type", func(t *testing.T) {
		garbage := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
		_, err := parser.Parse(ctx, garbage, "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		require.Error(t, err, "Corrupted DOCX zip should return error")
		assert.Contains(t, err.Error(), "read OpenXML zip")
	})

	t.Run("Valid ZIP missing word/document.xml for DOCX", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		f, err := zw.Create("other/file.txt")
		require.NoError(t, err)
		_, err = f.Write([]byte("some text"))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		text, err := parser.Parse(ctx, buf.Bytes(), "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		require.NoError(t, err)
		assert.Equal(t, "", text, "DOCX without word/document.xml should return empty string")
	})

	t.Run("DOCX with corrupted/malformed XML in word/document.xml", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		f, err := zw.Create("word/document.xml")
		require.NoError(t, err)
		// Unclosed XML tags
		_, err = f.Write([]byte(`<w:document><w:body><w:p><w:t>Partial valid text</w:t><unclosed tag here...`))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		text, err := parser.Parse(ctx, buf.Bytes(), "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		require.NoError(t, err)
		assert.Contains(t, text, "Partial valid text")
	})

	t.Run("Random garbage bytes with XLSX mime type", func(t *testing.T) {
		garbage := []byte("Not a zip file at all!")
		_, err := parser.Parse(ctx, garbage, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read OpenXML zip")
	})

	t.Run("Valid ZIP missing xl/sharedStrings.xml and worksheets for XLSX", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		f, err := zw.Create("docProps/core.xml")
		require.NoError(t, err)
		_, err = f.Write([]byte("<xml></xml>"))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		text, err := parser.Parse(ctx, buf.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		require.NoError(t, err)
		assert.Equal(t, "", text)
	})
}

// TestAdversarial_NonASCIIBinaryFiles tests parsing of arbitrary non-ASCII binary blobs, ELF binaries, and high-entropy data.
func TestAdversarial_NonASCIIBinaryFiles(t *testing.T) {
	ctx := context.Background()
	textParser := &TextParser{}

	t.Run("Invalid UTF-8 sequence with control bytes", func(t *testing.T) {
		invalidUTF8 := []byte{0xFF, 0xFE, 0xFD, 0x80, 0x81, 0x82, 0x01, 0x02, 0x48, 0x65, 0x6C, 0x6C, 0x6F, 0x00, 0xFF}
		text, err := textParser.Parse(ctx, invalidUTF8, "application/octet-stream")
		require.NoError(t, err)
		assert.Contains(t, text, "Hello")
	})

	t.Run("ELF Binary Header (\x7fELF)", func(t *testing.T) {
		elfHeader := []byte{0x7F, 0x45, 0x4C, 0x46, 0x02, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		text, err := textParser.Parse(ctx, elfHeader, "application/octet-stream")
		require.NoError(t, err)
		assert.Contains(t, text, "ELF")
	})

	t.Run("High-entropy binary data without printable ASCII", func(t *testing.T) {
		nonPrintable := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x0E, 0x0F, 0x10, 0x1F, 0x7F}
		text, err := textParser.Parse(ctx, nonPrintable, "application/octet-stream")
		require.NoError(t, err)
		assert.Equal(t, "", text)
	})

	t.Run("Binary file processed via DefaultParserRegistry with unknown mime", func(t *testing.T) {
		binData := []byte{0x10, 0x20, 0x30, 0x40, 0x54, 0x65, 0x73, 0x74, 0x20, 0x44, 0x61, 0x74, 0x61, 0x80, 0x90}
		text, err := ExtractTextFromBytes(binData, "application/x-custom-binary", "binary.bin")
		require.NoError(t, err)
		assert.Contains(t, text, "Test Data")
	})
}

// TestAdversarial_MeilisearchFallbackScenarios tests fallback behavior when Meilisearch URL is empty, unresolvable, offline, or timing out.
func TestAdversarial_MeilisearchFallbackScenarios(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	fallbackEngine := NewPostgresSearchEngine(repo)

	// Seed test document
	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:               docID,
		Filename:         "fallback_test_doc.pdf",
		OriginalFilename: "fallback_test_doc.pdf",
		MimeType:         "application/pdf",
		Category:         CategoryLegalDoc,
		StorageTier:      StorageTierHot,
		OCRStatus:        OCRStatusCompleted,
		OCRText:          "Legal Compliance Audit Report for Fallback Testing",
		UploadedAt:       time.Now(),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	t.Run("1. Empty Meilisearch URL Config", func(t *testing.T) {
		engine := NewSearchEngine(MeiliConfig{Host: ""}, repo)
		require.NotNil(t, engine)

		// Search should work via Postgres fallback
		res, err := engine.Search(ctx, SearchRequest{Query: "Compliance"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.Total)
		require.Len(t, res.Data, 1)
		assert.Equal(t, docID, res.Data[0].ID)

		// Indexing should succeed (noop or fallback)
		assert.NoError(t, engine.IndexDocument(ctx, doc))

		// Deleting index should succeed
		assert.NoError(t, engine.DeleteDocumentIndex(ctx, docID))
	})

	t.Run("2. Unresolvable Domain Name Meilisearch URL", func(t *testing.T) {
		unresolvableHost := "http://unresolvable-meilisearch-domain-xyz-999.invalid:7700"
		meiliEngine := NewMeiliSearchEngineWithFallback(MeiliConfig{Host: unresolvableHost}, fallbackEngine)

		// Search should fall back without failing
		res, err := meiliEngine.Search(ctx, SearchRequest{Query: "Audit"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.Total)
		assert.Equal(t, docID, res.Data[0].ID)

		// Indexing should fall back without failing
		err = meiliEngine.IndexDocument(ctx, doc)
		assert.NoError(t, err)
	})

	t.Run("3. Connection Refused Meilisearch Port", func(t *testing.T) {
		refusedHost := "http://127.0.0.1:59998"
		meiliEngine := NewMeiliSearchEngineWithFallback(MeiliConfig{Host: refusedHost}, fallbackEngine)

		res, err := meiliEngine.Search(ctx, SearchRequest{Query: "Compliance"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.Total)

		err = meiliEngine.IndexDocument(ctx, doc)
		assert.NoError(t, err)
	})

	t.Run("4. Meilisearch HTTP Server returning 502 Bad Gateway / 504 Gateway Timeout", func(t *testing.T) {
		server502 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"message": "502 Bad Gateway"}`))
		}))
		defer server502.Close()

		meili502 := NewMeiliSearchEngineWithFallback(MeiliConfig{Host: server502.URL}, fallbackEngine)

		res, err := meili502.Search(ctx, SearchRequest{Query: "Legal"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.Total)
		assert.Equal(t, docID, res.Data[0].ID)

		err = meili502.IndexDocument(ctx, doc)
		assert.NoError(t, err)
	})

	t.Run("5. Context Cancellation / Timeout during Search", func(t *testing.T) {
		// Slow mock server
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(500 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer slowServer.Close()

		meiliSlow := NewMeiliSearchEngineWithFallback(MeiliConfig{Host: slowServer.URL}, fallbackEngine)

		cancelCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Search with cancelled context should fall back to fallback engine
		res, err := meiliSlow.Search(cancelCtx, SearchRequest{Query: "Compliance"})
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, int64(1), res.Total)
	})
}
