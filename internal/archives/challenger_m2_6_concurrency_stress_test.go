package archives

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// 1. FULL PIPELINE MULTI-THREADED CONCURRENCY & STRESS TEST
// -----------------------------------------------------------------------------

func TestM2_FullPipeline_MultiThreadedStress(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	workerPool := NewGoOCRWorkerPool(8, 200, repo, searchEngine)
	workerPool.Start()
	defer workerPool.Stop()

	signer := NewHMACSignedURLSigner("stress_secret_key_12345", "/api/v1/archives")
	service := NewArchiveService(repo, searchEngine, workerPool, signer, nil)

	const (
		numUploaders = 15
		numSearchers = 10
		uploadsEach  = 20
	)

	var (
		uploadedDocIDs sync.Map
		totalUploaded  atomic.Int64
		totalSearches  atomic.Int64
		searchFailures atomic.Int64
		wg             sync.WaitGroup
	)

	// Phase 1: Launch Concurrent Uploaders and Concurrent Searchers
	for i := 0; i < numUploaders; i++ {
		wg.Add(1)
		go func(uploaderID int) {
			defer wg.Done()
			for j := 0; j < uploadsEach; j++ {
				filename := fmt.Sprintf("uploader_%d_doc_%d.txt", uploaderID, j)
				content := fmt.Sprintf("High-concurrency document content for uploader %d doc %d with keyword alpha_%d", uploaderID, j, uploaderID)
				category := CategoryStudentRecord
				if j%2 == 0 {
					category = CategoryGradeReport
				}
				tags := []string{"stress", fmt.Sprintf("group_%d", uploaderID)}
				userID := uuid.New()

				doc, err := service.UploadDocument(ctx, filename, category, tags, nil, []byte(content), userID)
				if assert.NoError(t, err) && doc != nil {
					uploadedDocIDs.Store(doc.ID, doc)
					totalUploaded.Add(1)
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	for s := 0; s < numSearchers; s++ {
		wg.Add(1)
		go func(searcherID int) {
			defer wg.Done()
			for k := 0; k < 30; k++ {
				query := fmt.Sprintf("alpha_%d", k%numUploaders)
				req := SearchRequest{
					Query:    query,
					Category: CategoryStudentRecord,
					Page:     1,
					Limit:    10,
				}
				res, err := service.Search(ctx, req)
				totalSearches.Add(1)
				if err != nil {
					searchFailures.Add(1)
				} else {
					_ = res
				}
				time.Sleep(2 * time.Millisecond)
			}
		}(s)
	}

	wg.Wait()

	assert.Equal(t, int64(numUploaders*uploadsEach), totalUploaded.Load(), "All upload operations should succeed")
	assert.Equal(t, int64(0), searchFailures.Load(), "Search operations should not fail under load")

	// Phase 2: Wait for OCR Worker Pool to drain and complete processing
	require.Eventually(t, func() bool {
		status := workerPool.Status()
		return status.ProcessedCount == int64(numUploaders*uploadsEach)
	}, 10*time.Second, 50*time.Millisecond, "OCR Worker Pool should process all uploaded documents")

	// Phase 3: Verify document statuses in repository
	docCount := 0
	uploadedDocIDs.Range(func(key, value interface{}) bool {
		docID := key.(uuid.UUID)
		doc, err := repo.GetDocumentByID(ctx, docID)
		if assert.NoError(t, err) {
			assert.Equal(t, OCRStatusCompleted, doc.OCRStatus, "OCR status for document %s should be COMPLETED", docID)
			assert.NotEmpty(t, doc.OCRText, "OCR text should be extracted")
			docCount++
		}
		return true
	})
	assert.Equal(t, numUploaders*uploadsEach, docCount)
}

// -----------------------------------------------------------------------------
// 2. OCR WORKER POOL CONCURRENT LIFECYCLE CHAOS TEST
// -----------------------------------------------------------------------------

func TestM2_OCRWorkerPool_ConcurrentLifecycleChaos(t *testing.T) {
	for cycle := 0; cycle < 10; cycle++ {
		repo := NewMemoryRepository()
		searchEngine := NewMemorySearchEngine()
		pool := NewGoOCRWorkerPool(4, 100, repo, searchEngine)
		pool.Start()

		var (
			wg             sync.WaitGroup
			enqueuedCount  atomic.Int64
			enqueueFailed  atomic.Int64
			stopExecuted   atomic.Bool
		)

		// 30 Producer Goroutines continuously enqueueing jobs
		for p := 0; p < 30; p++ {
			wg.Add(1)
			go func(producerID int) {
				defer wg.Done()
				for i := 0; i < 50; i++ {
					docID := uuid.New()
					doc := &ArchiveDocument{
						ID:         docID,
						Filename:   fmt.Sprintf("chaos_%d_%d.txt", producerID, i),
						MimeType:   "text/plain",
						OCRStatus:  OCRStatusPending,
						Category:   CategoryOther,
						UploadedAt: time.Now(),
					}
					_ = repo.CreateDocument(context.Background(), doc)

					err := pool.Enqueue(docID, "", "text/plain")
					if err == nil {
						enqueuedCount.Add(1)
					} else {
						enqueueFailed.Add(1)
					}
					time.Sleep(50 * time.Microsecond)
				}
			}(p)
		}

		// 10 Monitor Goroutines querying Status and updating ParserRegistry
		for m := 0; m < 10; m++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 20; i++ {
					_ = pool.Status()
					pool.SetParserRegistry(DefaultParserRegistry)
					time.Sleep(100 * time.Microsecond)
				}
			}()
		}

		// Shutdown Goroutine calling Stop mid-flight
		go func() {
			time.Sleep(2 * time.Millisecond)
			pool.Stop()
			stopExecuted.Store(true)
		}()

		// Secondary Stop Goroutines calling Stop redundantly
		for s := 0; s < 3; s++ {
			go func() {
				time.Sleep(3 * time.Millisecond)
				pool.Stop()
			}()
		}

		wg.Wait()

		// Verify pool state post-chaos
		status := pool.Status()
		assert.Equal(t, 0, status.WorkersActive, "No worker goroutines should remain active post-Stop")
		
		// Attempting Enqueue post-stop must safely return an error without panic
		err := pool.Enqueue(uuid.New(), "", "text/plain")
		assert.Error(t, err, "Enqueue on stopped worker pool must fail gracefully")
	}
}

// -----------------------------------------------------------------------------
// 3. MEILISEARCH CONCURRENT HTTP FAILURES & FALLBACK DEGRADATION TEST
// -----------------------------------------------------------------------------

func TestM2_Meilisearch_ConcurrentHTTPFailuresAndFallback(t *testing.T) {
	var requestCount atomic.Int64

	// Create test HTTP server simulating chaos (500 errors, network delays)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count%3 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
			return
		}
		if count%5 == 0 {
			time.Sleep(50 * time.Millisecond)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hits": [], "totalHits": 0, "processingTimeMs": 1}`))
	}))
	defer server.Close()

	memRepo := NewMemoryRepository()
	fallbackEngine := NewPostgresSearchEngine(memRepo)

	// Seed fallback repo with documents
	for i := 0; i < 10; i++ {
		doc := &ArchiveDocument{
			ID:         uuid.New(),
			Filename:   fmt.Sprintf("fallback_doc_%d.pdf", i),
			MimeType:   "application/pdf",
			Category:   CategoryStudentRecord,
			OCRStatus:  OCRStatusCompleted,
			OCRText:    fmt.Sprintf("Fallback search content number %d", i),
			UploadedAt: time.Now(),
		}
		_ = memRepo.CreateDocument(context.Background(), doc)
	}

	meiliEngine := NewMeiliSearchEngineWithFallback(
		MeiliConfig{Host: server.URL, Index: "test_index"},
		fallbackEngine,
	)

	var wg sync.WaitGroup
	const concurrentClients = 25
	var fallbackHitsCount atomic.Int64

	for c := 0; c < concurrentClients; c++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			for i := 0; i < 15; i++ {
				req := SearchRequest{
					Query:    "Fallback",
					Category: CategoryStudentRecord,
					Page:     1,
					Limit:    10,
				}
				res, err := meiliEngine.Search(context.Background(), req)
				if assert.NoError(t, err) && res != nil {
					if res.Total > 0 {
						fallbackHitsCount.Add(1)
					}
				}
			}
		}(c)
	}

	wg.Wait()
	assert.Greater(t, fallbackHitsCount.Load(), int64(0), "Fallback engine should successfully serve queries when Meilisearch fails or returns errors")
}

// -----------------------------------------------------------------------------
// 4. MULTI-FORMAT PARSER MALFORMED & BOUNDARY STRESS TEST
// -----------------------------------------------------------------------------

func TestM2_MultiFormatParser_MalformedAndBoundaryStress(t *testing.T) {
	registry := NewParserRegistry()
	ctx := context.Background()

	// Sub-test 1: Zip Parser Malformed Zip & Size Limits
	t.Run("Zip Parser Boundary & Malformed Input", func(t *testing.T) {
		// Case A: Corrupted Byte Slice
		text, err := registry.Parse(ctx, []byte("NOT_A_ZIP_FILE_CORRUPTED_BYTES"), "application/zip", "bad.zip")
		assert.Error(t, err)
		assert.Empty(t, text)

		// Case B: Valid Zip containing nested valid and binary files
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)

		f1, _ := zw.Create("hello.txt")
		_, _ = f1.Write([]byte("Hello world inside zip"))

		f2, _ := zw.Create("binary.bin")
		_, _ = f2.Write([]byte{0x00, 0xFF, 0xFE, 0xFD})

		_ = zw.Close()

		zipText, err := registry.Parse(ctx, buf.Bytes(), "application/zip", "valid.zip")
		assert.NoError(t, err)
		assert.Contains(t, zipText, "Hello world inside zip")
	})

	// Sub-test 2: Docx/Xlsx Parser OpenXML Tag Extraction & Invalid Zip
	t.Run("Docx/Xlsx Parser OpenXML Tag Extraction", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)

		f1, _ := zw.Create("word/document.xml")
		_, _ = f1.Write([]byte(`<?xml version="1.0"?><w:document><w:body><w:p><w:t>Extracted OpenXML Document Text</w:t></w:p></w:body></w:document>`))

		_ = zw.Close()

		docxText, err := registry.Parse(ctx, buf.Bytes(), "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "test.docx")
		assert.NoError(t, err)
		assert.Equal(t, "Extracted OpenXML Document Text", docxText)
	})

	// Sub-test 3: PDF Parser BT/ET Stream Extraction
	t.Run("PDF Parser BT/ET Operator Parsing", func(t *testing.T) {
		pdfContent := "%PDF-1.4\n1 0 obj\nBT\n(PDF Text Extraction Stream) Tj\nET\nendobj"
		pdfText, err := registry.Parse(ctx, []byte(pdfContent), "application/pdf", "sample.pdf")
		assert.NoError(t, err)
		assert.Contains(t, pdfText, "PDF Text Extraction Stream")
	})

	// Sub-test 4: Text Parser Unicode Handling
	t.Run("Text Parser Valid and Invalid UTF-8", func(t *testing.T) {
		// Valid UTF-8 with multibyte characters
		utf8Input := "Student transcript from 🏫 University of Tokyo 東京 🇯🇵"
		parsedUtf8, err := registry.Parse(ctx, []byte(utf8Input), "text/plain", "doc.txt")
		assert.NoError(t, err)
		assert.Equal(t, utf8Input, parsedUtf8)

		// Invalid non-UTF8 byte sequence
		invalidBytes := []byte{'H', 'e', 'l', 'l', 'o', 0x80, 0x81, 'W', 'o', 'r', 'l', 'd'}
		parsedNonUtf8, err := registry.Parse(ctx, invalidBytes, "text/plain", "doc.txt")
		assert.NoError(t, err)
		assert.Contains(t, parsedNonUtf8, "Hello")
		assert.Contains(t, parsedNonUtf8, "World")
	})
}

// -----------------------------------------------------------------------------
// 5. SEARCH FILTER HIGH-CONCURRENCY ADVERSARIAL INPUTS TEST
// -----------------------------------------------------------------------------

func TestM2_SearchFilter_HighConcurrencyAdversarialInputs(t *testing.T) {
	memEngine := NewMemorySearchEngine()
	ctx := context.Background()

	// Seed engine with documents containing special characters
	docs := []*ArchiveDocument{
		{ID: uuid.New(), Filename: "normal_doc.pdf", Category: CategoryStudentRecord, OCRText: "Normal text content", UploadedAt: time.Now()},
		{ID: uuid.New(), Filename: "O'Reilly_Guide.pdf", Category: DocumentCategory("O'Reilly\\'s Category"), Tags: []string{"Tag\\With\\Backslash", "O'Reilly"}, OCRText: "Special characters test", UploadedAt: time.Now()},
	}
	for _, doc := range docs {
		_ = memEngine.IndexDocument(ctx, doc)
	}

	adversarialQueries := []struct {
		query    string
		category DocumentCategory
		tags     []string
	}{
		{"normal", CategoryStudentRecord, nil},
		{"O'Reilly", DocumentCategory("O'Reilly\\'s Category"), []string{"Tag\\With\\Backslash"}},
		{"' OR '1'='1", DocumentCategory("Folder\\"), []string{"Tag\\\\'s"}},
		{"\\path\\to\\file\\", DocumentCategory("Category\\"), []string{"Tag\\"}},
		{"🎉 unicode emoji 🚀", CategoryOther, nil},
	}

	var wg sync.WaitGroup
	const concurrency = 20

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for _, item := range adversarialQueries {
				req := SearchRequest{
					Query:    item.query,
					Category: item.category,
					Tags:     item.tags,
					Page:     1,
					Limit:    10,
				}
				res, err := memEngine.Search(ctx, req)
				assert.NoError(t, err)
				assert.NotNil(t, res)
			}
		}(i)
	}

	wg.Wait()
}
