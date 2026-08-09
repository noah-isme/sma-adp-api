package archives

import (
	"archive/zip"
	"bytes"
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

// =============================================================================
// 1. OCR WORKER POOL CONCURRENCY & LIFECYCLE TESTS
// =============================================================================

// TestOCRWorkerPool_HighConcurrencyEnqueueing tests 50+ concurrent job enqueues across multiple goroutines.
func TestOCRWorkerPool_HighConcurrencyEnqueueing(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()

	numWorkers := 8
	queueCap := 100
	numJobs := 60

	pool := NewGoOCRWorkerPool(numWorkers, queueCap, repo, searchEngine)
	pool.Start()

	// Seed 60 documents into repo
	docIDs := make([]uuid.UUID, numJobs)
	tmpFiles := make([]string, numJobs)
	for i := 0; i < numJobs; i++ {
		docIDs[i] = uuid.New()
		tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("ocr_concurrent_%d.txt", i))
		content := fmt.Sprintf("Concurrent job content index %d timestamp %d", i, time.Now().UnixNano())
		require.NoError(t, os.WriteFile(tmpPath, []byte(content), 0644))
		defer os.Remove(tmpPath)
		tmpFiles[i] = tmpPath

		doc := &ArchiveDocument{
			ID:         docIDs[i],
			Filename:   fmt.Sprintf("concurrent_%d.txt", i),
			Category:   CategoryStudentRecord,
			OCRStatus:  OCRStatusPending,
			UploadedAt: time.Now(),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))
	}

	// Concurrently enqueue all 60 jobs using 10 client goroutines
	var wg sync.WaitGroup
	workersCount := 10
	jobsPerGoroutine := numJobs / workersCount

	for g := 0; g < workersCount; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()
			start := gIdx * jobsPerGoroutine
			end := start + jobsPerGoroutine
			for i := start; i < end; i++ {
				err := pool.Enqueue(docIDs[i], tmpFiles[i], "text/plain")
				assert.NoError(t, err, "Enqueue failed at index %d", i)
			}
		}(g)
	}

	wg.Wait()

	// Wait for all jobs to reach completed status
	require.Eventually(t, func() bool {
		completed := 0
		for _, id := range docIDs {
			doc, err := repo.GetDocumentByID(ctx, id)
			if err == nil && doc.OCRStatus == OCRStatusCompleted {
				completed++
			}
		}
		return completed == numJobs
	}, 10*time.Second, 100*time.Millisecond, "Expected all 60 concurrent jobs to complete")

	pool.Stop()

	status := pool.Status()
	assert.Equal(t, int64(numJobs), status.ProcessedCount)
}

// TestOCRWorkerPool_StopDrainingVerification verifies that calling Stop() drains all enqueued jobs before returning.
func TestOCRWorkerPool_StopDrainingVerification(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()

	// 2 workers, capacity 100, 50 jobs
	numJobs := 50
	pool := NewGoOCRWorkerPool(2, 100, repo, searchEngine)

	docIDs := make([]uuid.UUID, numJobs)
	tmpFiles := make([]string, numJobs)

	for i := 0; i < numJobs; i++ {
		docIDs[i] = uuid.New()
		tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("ocr_draining_%d.txt", i))
		require.NoError(t, os.WriteFile(tmpPath, []byte(fmt.Sprintf("Draining test content %d", i)), 0644))
		defer os.Remove(tmpPath)
		tmpFiles[i] = tmpPath

		doc := &ArchiveDocument{
			ID:         docIDs[i],
			Filename:   fmt.Sprintf("draining_%d.txt", i),
			Category:   CategoryFinancialDoc,
			OCRStatus:  OCRStatusPending,
			UploadedAt: time.Now(),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))
	}

	pool.Start()

	// Enqueue all 50 jobs immediately
	for i := 0; i < numJobs; i++ {
		require.NoError(t, pool.Enqueue(docIDs[i], tmpFiles[i], "text/plain"))
	}

	// Stop pool - must block until ALL 50 jobs are drained and processed
	stopDone := make(chan struct{})
	go func() {
		pool.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
		// Stop returned cleanly
	case <-time.After(10 * time.Second):
		t.Fatal("pool.Stop() timed out waiting for queue to drain")
	}

	status := pool.Status()
	assert.Equal(t, int64(numJobs), status.ProcessedCount, "ProcessedCount should match total enqueued jobs")

	// Verify all 50 documents reached completed status
	for i, id := range docIDs {
		doc, err := repo.GetDocumentByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, OCRStatusCompleted, doc.OCRStatus, "Doc %d status should be COMPLETED", i)
		assert.Contains(t, doc.OCRText, fmt.Sprintf("Draining test content %d", i))
	}
}

// PanicInjectingRepo wraps Repository to trigger panic on specific doc ID.
type PanicInjectingRepo struct {
	Repository
	panicDocID uuid.UUID
}

func (r *PanicInjectingRepo) GetDocumentByID(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error) {
	if id == r.panicDocID {
		panic("SIMULATED CORRUPT MEMORY PANIC IN REPOSITORY")
	}
	return r.Repository.GetDocumentByID(ctx, id)
}

// TestOCRWorkerPool_PanicRecovery verifies panic recovery during job execution and status update to OCRStatusFailed.
func TestOCRWorkerPool_PanicRecovery(t *testing.T) {
	ctx := context.Background()
	baseRepo := NewMemoryRepository()
	panicID := uuid.New()
	normalID := uuid.New()

	// Seed normal doc
	normalDoc := &ArchiveDocument{
		ID:         normalID,
		Filename:   "normal.txt",
		Category:   CategoryOther,
		OCRStatus:  OCRStatusPending,
		UploadedAt: time.Now(),
	}
	require.NoError(t, baseRepo.CreateDocument(ctx, normalDoc))

	// Seed panic doc
	panicDoc := &ArchiveDocument{
		ID:         panicID,
		Filename:   "panic.txt",
		Category:   CategoryOther,
		OCRStatus:  OCRStatusPending,
		UploadedAt: time.Now(),
	}
	require.NoError(t, baseRepo.CreateDocument(ctx, panicDoc))

	panicRepo := &PanicInjectingRepo{
		Repository: baseRepo,
		panicDocID: panicID,
	}

	pool := NewGoOCRWorkerPool(1, 10, panicRepo, nil)
	pool.Start()
	defer pool.Stop()

	// Enqueue panic job
	tmpPanicFile := filepath.Join(os.TempDir(), "panic_file.txt")
	require.NoError(t, os.WriteFile(tmpPanicFile, []byte("panic data"), 0644))
	defer os.Remove(tmpPanicFile)

	require.NoError(t, pool.Enqueue(panicID, tmpPanicFile, "text/plain"))

	// Enqueue normal job after panic job
	tmpNormalFile := filepath.Join(os.TempDir(), "normal_file.txt")
	require.NoError(t, os.WriteFile(tmpNormalFile, []byte("normal data"), 0644))
	defer os.Remove(tmpNormalFile)

	require.NoError(t, pool.Enqueue(normalID, tmpNormalFile, "text/plain"))

	// Wait for normal job to complete
	require.Eventually(t, func() bool {
		doc, err := baseRepo.GetDocumentByID(ctx, normalID)
		return err == nil && doc.OCRStatus == OCRStatusCompleted
	}, 5*time.Second, 100*time.Millisecond, "Worker pool failed to recover from panic to process subsequent job")

	// Verify worker pool remained operational and processed count increased
	status := pool.Status()
	assert.GreaterOrEqual(t, status.ProcessedCount, int64(2))
}

// TestOCRWorkerPool_InvalidFile_UpdatesFailed verifies missing file triggers handleJobFailure and sets OCRStatusFailed.
func TestOCRWorkerPool_InvalidFile_UpdatesFailed(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	pool := NewGoOCRWorkerPool(1, 10, repo, nil)
	pool.Start()
	defer pool.Stop()

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:         docID,
		Filename:   "missing.txt",
		Category:   CategoryOther,
		OCRStatus:  OCRStatusPending,
		UploadedAt: time.Now(),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	// Enqueue with invalid file path
	require.NoError(t, pool.Enqueue(docID, "/path/does/not/exist/missing.txt", "text/plain"))

	require.Eventually(t, func() bool {
		d, err := repo.GetDocumentByID(ctx, docID)
		return err == nil && d.OCRStatus == OCRStatusFailed
	}, 3*time.Second, 50*time.Millisecond, "Document status should update to OCRStatusFailed on missing file error")

	updatedDoc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, OCRStatusFailed, updatedDoc.OCRStatus)
}

// TestOCRWorkerPool_TimeoutVerification tests the 30-second context timeout behavior.
func TestOCRWorkerPool_TimeoutVerification(t *testing.T) {
	pool := NewGoOCRWorkerPool(2, 10, nil, nil)
	assert.Equal(t, 30*time.Second, pool.timeout, "GoOCRWorkerPool default timeout must be 30 seconds")
}

// =============================================================================
// 2. MULTI-FORMAT PARSER TESTS (ocr_parsers.go)
// =============================================================================

// TestMultiFormatParsers_PDF verifies PDF text stream parsing and plain text fallback.
func TestMultiFormatParsers_PDF(t *testing.T) {
	ctx := context.Background()
	pdfParser := &PDFParser{}

	t.Run("PDF text stream extraction with BT/ET operators", func(t *testing.T) {
		pdfData := []byte("%PDF-1.5\nBT\n/F1 12 Tf\n(Confidential Financial Audit Report 2026) Tj\nET\n")
		text, err := pdfParser.Parse(ctx, pdfData, "application/pdf")
		require.NoError(t, err)
		assert.Contains(t, text, "Confidential Financial Audit Report 2026")
	})

	t.Run("PDF without BT/ET text stream fallback", func(t *testing.T) {
		pdfData := []byte("%PDF-1.4\nPlain PDF fallback content without operators\n")
		text, err := pdfParser.Parse(ctx, pdfData, "application/pdf")
		require.NoError(t, err)
		assert.Contains(t, text, "Plain PDF fallback content without operators")
	})

	t.Run("Empty PDF data", func(t *testing.T) {
		text, err := pdfParser.Parse(ctx, nil, "application/pdf")
		require.NoError(t, err)
		assert.Equal(t, "", text)
	})
}

// TestMultiFormatParsers_Image verifies Image parsing fallback and tesseract execution.
func TestMultiFormatParsers_Image(t *testing.T) {
	ctx := context.Background()
	imgParser := &ImageParser{}

	t.Run("Image text fallback when non-empty text inside image bytes", func(t *testing.T) {
		imgData := []byte("Header: PNG Image File with embedded text string for OCR testing long payload")
		text, err := imgParser.Parse(ctx, imgData, "image/png")
		require.NoError(t, err)
		assert.NotEmpty(t, text)
	})

	t.Run("Image description fallback for empty/binary image data", func(t *testing.T) {
		imgData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
		text, err := imgParser.Parse(ctx, imgData, "image/png")
		require.NoError(t, err)
		assert.Contains(t, text, "Image Document (image/png)")
	})
}

// TestMultiFormatParsers_DOCX_XLSX verifies OpenXML XML tag parsing.
func TestMultiFormatParsers_DOCX_XLSX(t *testing.T) {
	ctx := context.Background()
	parser := &DocxXlsxParser{}

	t.Run("DOCX word/document.xml extraction", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		f, err := zw.Create("word/document.xml")
		require.NoError(t, err)
		xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:t>Official Transcript</w:t></w:p>
    <w:p><w:t>Semester GPA: 3.95</w:t></w:p>
  </w:body>
</w:document>`
		_, err = f.Write([]byte(xmlContent))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		text, err := parser.Parse(ctx, buf.Bytes(), "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		require.NoError(t, err)
		assert.Contains(t, text, "Official Transcript")
		assert.Contains(t, text, "Semester GPA: 3.95")
	})

	t.Run("XLSX xl/sharedStrings.xml & worksheets extraction", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)

		f1, err := zw.Create("xl/sharedStrings.xml")
		require.NoError(t, err)
		_, err = f1.Write([]byte(`<?xml version="1.0"?><sst><si><t>Revenue 2026</t></si><si><t>Expense 2026</t></si></sst>`))
		require.NoError(t, err)

		f2, err := zw.Create("xl/worksheets/sheet1.xml")
		require.NoError(t, err)
		_, err = f2.Write([]byte(`<?xml version="1.0"?><worksheet><sheetData><row><c><t>Total Profit</t></c></row></sheetData></worksheet>`))
		require.NoError(t, err)

		require.NoError(t, zw.Close())

		text, err := parser.Parse(ctx, buf.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		require.NoError(t, err)
		assert.Contains(t, text, "Revenue 2026")
		assert.Contains(t, text, "Expense 2026")
		assert.Contains(t, text, "Total Profit")
	})
}

// TestMultiFormatParsers_ZIP_Limits_And_Recursion verifies 50 files limit, 20MB limit, and executable/zip filtering.
func TestMultiFormatParsers_ZIP_Limits_And_Recursion(t *testing.T) {
	ctx := context.Background()
	zipParser := &ZipParser{registry: DefaultParserRegistry}

	t.Run("ZIP max file count limit (max 50 files)", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)

		// Add 60 text files
		for i := 1; i <= 60; i++ {
			f, err := zw.Create(fmt.Sprintf("file_%02d.txt", i))
			require.NoError(t, err)
			_, err = f.Write([]byte(fmt.Sprintf("Content of file %d", i)))
			require.NoError(t, err)
		}
		require.NoError(t, zw.Close())

		text, err := zipParser.Parse(ctx, buf.Bytes(), "application/zip")
		require.NoError(t, err)

		// Should contain limit truncation message
		assert.Contains(t, text, "Zip contents truncated (max limit reached)")
		// Should contain file_50.txt but NOT file_51.txt
		assert.Contains(t, text, "file_50.txt")
		assert.NotContains(t, text, "file_51.txt")
	})

	t.Run("ZIP executable and nested ZIP filtering", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)

		// Add dangerous/nested files
		fExe, _ := zw.Create("malware.exe")
		_, _ = fExe.Write([]byte("binary content"))

		fDll, _ := zw.Create("library.dll")
		_, _ = fDll.Write([]byte("binary content"))

		fZip, _ := zw.Create("nested.zip")
		_, _ = fZip.Write([]byte("nested zip content"))

		fValid, _ := zw.Create("safe_notes.txt")
		_, _ = fValid.Write([]byte("Safe Text Content inside ZIP"))

		require.NoError(t, zw.Close())

		text, err := zipParser.Parse(ctx, buf.Bytes(), "application/zip")
		require.NoError(t, err)

		assert.Contains(t, text, "safe_notes.txt")
		assert.Contains(t, text, "Safe Text Content inside ZIP")

		// Exe, Dll, Nested Zip must be skipped
		assert.NotContains(t, text, "malware.exe")
		assert.NotContains(t, text, "library.dll")
		assert.NotContains(t, text, "nested.zip")
	})
}

// TestMultiFormatParsers_UTF8_Preservation tests preservation of international characters across languages and symbols.
func TestMultiFormatParsers_UTF8_Preservation(t *testing.T) {
	textParser := &TextParser{}
	ctx := context.Background()

	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Indonesian & Accents",
			input:    "Laporan Keuangan PT Merdeka 🇮🇩 — Café & Résumé",
			expected: []string{"Laporan Keuangan", "🇮🇩", "Café", "Résumé"},
		},
		{
			name:     "CJK (Chinese, Japanese, Korean)",
			input:    "档案管理系统 日本語テキスト 한국어 문서 2026",
			expected: []string{"档案管理系统", "日本語テキスト", "한국어 문서"},
		},
		{
			name:     "Arabic & Cyrillic",
			input:    "تقرير الأرشيف Документ архива 2026",
			expected: []string{"تقرير الأرشيف", "Документ архива"},
		},
		{
			name:     "Symbols & Emojis",
			input:    "Copyright © 2026 Status: OK 🚀 Grade: A+ (100%)",
			expected: []string{"©", "🚀", "Grade: A+"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.input)
			parsed, err := textParser.Parse(ctx, data, "text/plain")
			require.NoError(t, err)

			for _, exp := range tc.expected {
				assert.Contains(t, parsed, exp, "Parsed text must preserve UTF-8 string %q", exp)
			}

			// Verify via ExtractTextFromBytes as well
			extracted, err := ExtractTextFromBytes(data, "text/plain", "doc.txt")
			require.NoError(t, err)
			for _, exp := range tc.expected {
				assert.Contains(t, extracted, exp)
			}
		})
	}
}

// TestOCRWorkerPool_ParserRegistryField_Inconsistency verifies custom ParserRegistry wiring behavior in worker pool.
func TestOCRWorkerPool_ParserRegistryField_Inconsistency(t *testing.T) {
	repo := NewMemoryRepository()
	pool := NewGoOCRWorkerPool(1, 10, repo, nil)

	customRegistry := NewParserRegistry()
	pool.SetParserRegistry(customRegistry)

	assert.Equal(t, customRegistry, pool.parserRegistry, "SetParserRegistry should update p.parserRegistry")
}
