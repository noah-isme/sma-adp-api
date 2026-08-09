package archives

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 1. POSTGRES SEARCH ENGINE FILTERING TESTS
// =============================================================================

// TestPostgresSearchEngine_TagArrayMatching tests single and multiple tag array filtering.
func TestPostgresSearchEngine_TagArrayMatching(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewPostgresSearchEngine(repo)

	doc1ID := uuid.New()
	doc2ID := uuid.New()
	doc3ID := uuid.New()

	now := time.Now().UTC()

	// Seed doc1 with tags ["finance", "audit", "2026"]
	doc1 := &ArchiveDocument{
		ID:               doc1ID,
		Filename:         "finance_audit_2026.pdf",
		OriginalFilename: "finance_audit_2026.pdf",
		MimeType:         "application/pdf",
		SizeBytes:        1024,
		Checksum:         "hash1",
		StorageTier:      StorageTierHot,
		Category:         CategoryFinancialDoc,
		Tags:             []string{"finance", "audit", "2026"},
		OCRStatus:        OCRStatusCompleted,
		OCRText:          "Annual Financial Audit Report for Fiscal Year 2026.",
		UploadedAt:       now,
	}

	// Seed doc2 with tags ["finance", "tax"]
	doc2 := &ArchiveDocument{
		ID:               doc2ID,
		Filename:         "finance_tax.pdf",
		OriginalFilename: "finance_tax.pdf",
		MimeType:         "application/pdf",
		SizeBytes:        2048,
		Checksum:         "hash2",
		StorageTier:      StorageTierHot,
		Category:         CategoryFinancialDoc,
		Tags:             []string{"finance", "tax"},
		OCRStatus:        OCRStatusCompleted,
		OCRText:          "Tax return summary for corporate entities.",
		UploadedAt:       now.Add(-1 * time.Hour),
	}

	// Seed doc3 with tags ["student", "transcript"]
	doc3 := &ArchiveDocument{
		ID:               doc3ID,
		Filename:         "student_record.pdf",
		OriginalFilename: "student_record.pdf",
		MimeType:         "application/pdf",
		SizeBytes:        512,
		Checksum:         "hash3",
		StorageTier:      StorageTierHot,
		Category:         CategoryStudentRecord,
		Tags:             []string{"student", "transcript"},
		OCRStatus:        OCRStatusCompleted,
		OCRText:          "Student transcript for academic records.",
		UploadedAt:       now.Add(-2 * time.Hour),
	}

	require.NoError(t, repo.CreateDocument(ctx, doc1))
	require.NoError(t, repo.CreateDocument(ctx, doc2))
	require.NoError(t, repo.CreateDocument(ctx, doc3))

	t.Run("Single Tag Matching", func(t *testing.T) {
		res, err := engine.Search(ctx, SearchRequest{
			Tags:  []string{"finance"},
			Page:  1,
			Limit: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(2), res.Total)
		require.Len(t, res.Data, 2)
	})

	t.Run("Multiple Tags Matching (All Required)", func(t *testing.T) {
		res, err := engine.Search(ctx, SearchRequest{
			Tags:  []string{"finance", "audit"},
			Page:  1,
			Limit: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.Total)
		require.Len(t, res.Data, 1)
		assert.Equal(t, doc1ID, res.Data[0].ID)
	})

	t.Run("Multiple Tags Matching - Partial Failure", func(t *testing.T) {
		res, err := engine.Search(ctx, SearchRequest{
			Tags:  []string{"finance", "nonexistent_tag"},
			Page:  1,
			Limit: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(0), res.Total)
		assert.Empty(t, res.Data)
	})
}

// TestPostgresSearchEngine_CategoryLegalHoldDateAndPagination tests category, legal hold, date bounds, and pagination.
func TestPostgresSearchEngine_CategoryLegalHoldDateAndPagination(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	engine := NewPostgresSearchEngine(repo)

	baseTime := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	// Create 5 documents with varying categories, legal hold, and timestamps
	docs := []*ArchiveDocument{
		{
			ID:          uuid.New(),
			Filename:    "doc_01.pdf",
			Category:    CategoryStudentRecord,
			LegalHold:   true,
			UploadedAt:  baseTime.Add(1 * time.Hour),
			OCRStatus:   OCRStatusCompleted,
			OCRText:     "Student grade transcript data",
			StorageTier: StorageTierHot,
		},
		{
			ID:          uuid.New(),
			Filename:    "doc_02.pdf",
			Category:    CategoryStudentRecord,
			LegalHold:   false,
			UploadedAt:  baseTime.Add(2 * time.Hour),
			OCRStatus:   OCRStatusCompleted,
			OCRText:     "Student attendance record data",
			StorageTier: StorageTierHot,
		},
		{
			ID:          uuid.New(),
			Filename:    "doc_03.pdf",
			Category:    CategoryFinancialDoc,
			LegalHold:   true,
			UploadedAt:  baseTime.Add(3 * time.Hour),
			OCRStatus:   OCRStatusCompleted,
			OCRText:     "Financial invoice statement",
			StorageTier: StorageTierHot,
		},
		{
			ID:          uuid.New(),
			Filename:    "doc_04.pdf",
			Category:    CategoryFinancialDoc,
			LegalHold:   false,
			UploadedAt:  baseTime.Add(4 * time.Hour),
			OCRStatus:   OCRStatusCompleted,
			OCRText:     "Financial balance sheet",
			StorageTier: StorageTierHot,
		},
		{
			ID:          uuid.New(),
			Filename:    "doc_05.pdf",
			Category:    CategoryLegalDoc,
			LegalHold:   true,
			UploadedAt:  baseTime.Add(5 * time.Hour),
			OCRStatus:   OCRStatusCompleted,
			OCRText:     "Legal contract document",
			StorageTier: StorageTierHot,
		},
	}

	for _, doc := range docs {
		require.NoError(t, repo.CreateDocument(ctx, doc))
	}

	t.Run("Category Filter", func(t *testing.T) {
		res, err := engine.Search(ctx, SearchRequest{
			Category: CategoryStudentRecord,
			Page:     1,
			Limit:    10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(2), res.Total)
		for _, hit := range res.Data {
			assert.Equal(t, CategoryStudentRecord, hit.Category)
		}
	})

	t.Run("Legal Hold Filter", func(t *testing.T) {
		res, err := engine.Search(ctx, SearchRequest{
			LegalHoldOnly: true,
			Page:          1,
			Limit:         10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(3), res.Total)
		for _, hit := range res.Data {
			assert.True(t, hit.LegalHold)
		}
	})

	t.Run("Date Range Bounds", func(t *testing.T) {
		from := baseTime.Add(2 * time.Hour)
		to := baseTime.Add(4 * time.Hour)

		res, err := engine.Search(ctx, SearchRequest{
			DateFrom: &from,
			DateTo:   &to,
			Page:     1,
			Limit:    10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(3), res.Total) // doc_02, doc_03, doc_04
	})

	t.Run("Pagination - Page 1 Limit 2", func(t *testing.T) {
		res, err := engine.Search(ctx, SearchRequest{
			Page:  1,
			Limit: 2,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(5), res.Total)
		assert.Len(t, res.Data, 2)
		assert.Equal(t, 1, res.Page)
		assert.Equal(t, 2, res.Limit)
	})

	t.Run("Pagination - Page 2 Limit 2", func(t *testing.T) {
		res, err := engine.Search(ctx, SearchRequest{
			Page:  2,
			Limit: 2,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(5), res.Total)
		assert.Len(t, res.Data, 2)
	})

	t.Run("Pagination - Page 3 Limit 2", func(t *testing.T) {
		res, err := engine.Search(ctx, SearchRequest{
			Page:  3,
			Limit: 2,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(5), res.Total)
		assert.Len(t, res.Data, 1)
	})

	t.Run("Pagination - Page Out of Bounds", func(t *testing.T) {
		res, err := engine.Search(ctx, SearchRequest{
			Page:  10,
			Limit: 2,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(5), res.Total)
		assert.Empty(t, res.Data)
	})

	t.Run("Nil Repo Handling", func(t *testing.T) {
		nilEngine := NewPostgresSearchEngine(nil)
		res, err := nilEngine.Search(ctx, SearchRequest{Query: "test"})
		require.NoError(t, err)
		assert.Equal(t, int64(0), res.Total)
		assert.Empty(t, res.Data)
	})
}

// TestSnippetExtraction_Comprehensive tests snippet extraction across locations, Unicode, and missing terms.
func TestSnippetExtraction_Comprehensive(t *testing.T) {
	t.Run("Term at Beginning", func(t *testing.T) {
		text := "Mathematics is a core subject required for science majors."
		snippet := extractSnippet(text, "Mathematics")
		assert.True(t, len(snippet) > 0)
		assert.Contains(t, snippet, "<em>Mathematics</em>")
		assert.False(t, snippet[:3] == "...") // No prefix ellipsis at start
	})

	t.Run("Term in Middle", func(t *testing.T) {
		text := "This document contains confidential information regarding the Annual Mathematics Competition of 2026."
		snippet := extractSnippet(text, "Mathematics")
		assert.Contains(t, snippet, "<em>Mathematics</em>")
		assert.Contains(t, snippet, "...")
	})

	t.Run("Term at End", func(t *testing.T) {
		text := "All student records must be verified by the dean of Mathematics"
		snippet := extractSnippet(text, "Mathematics")
		assert.Contains(t, snippet, "<em>Mathematics</em>")
	})

	t.Run("Multiple Query Terms Location", func(t *testing.T) {
		text := "First chapter of Mathematics. Second chapter of Science. Final review of Mathematics."
		snippet := extractSnippet(text, "Mathematics")
		assert.Contains(t, snippet, "<em>Mathematics</em>")
	})

	t.Run("Unicode Text - Indonesian & Diacritics", func(t *testing.T) {
		text := "Laporan Hasil Ujian Nasional Siswa PT Merdeka: Nilai Akhir Bahasa Indonesia & Café Résumé 🚀"
		snippet := extractSnippet(text, "Bahasa")
		assert.Contains(t, snippet, "<em>Bahasa</em>")
		assert.Contains(t, snippet, "Indonesia")
	})

	t.Run("Unicode Text - CJK Characters", func(t *testing.T) {
		text := "档案管理系统 成绩单 日本語テキスト 2026"
		snippet := extractSnippet(text, "成绩单")
		assert.Contains(t, snippet, "<em>成绩单</em>")
	})

	t.Run("Missing Term - Long Text Truncation", func(t *testing.T) {
		longText := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam."
		snippet := extractSnippet(longText, "NonExistentKeyword")
		assert.True(t, len(snippet) > 0)
		assert.Contains(t, snippet, "...")
		assert.NotContains(t, snippet, "<em>")
	})

	t.Run("Missing Term - Short Text", func(t *testing.T) {
		shortText := "Short text only."
		snippet := extractSnippet(shortText, "Missing")
		assert.Equal(t, shortText, snippet)
	})

	t.Run("Empty Query String", func(t *testing.T) {
		longText := "12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"
		snippet := extractSnippet(longText, "")
		assert.Contains(t, snippet, "...")
	})

	t.Run("Empty Full Text", func(t *testing.T) {
		snippet := extractSnippet("", "query")
		assert.Equal(t, "", snippet)
	})
}

// =============================================================================
// 2. HYBRID SEARCH ENGINE FAILOVER TESTS
// =============================================================================

// MockMeiliErrServer sets up a mock Meilisearch HTTP server returning error codes or timeouts.
type MockMeiliErrServer struct {
	server *httptest.Server
	status int
}

func NewMockMeiliErrServer(statusCode int) *MockMeiliErrServer {
	mock := &MockMeiliErrServer{status: statusCode}
	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(`{"message": "Meilisearch internal server error"}`))
	}))
	return mock
}

func (m *MockMeiliErrServer) URL() string {
	return m.server.URL
}

func (m *MockMeiliErrServer) Close() {
	m.server.Close()
}

// TestHybridSearchEngine_FailoverScenarios tests failover when Meilisearch is unconfigured, unreachable, or errors out.
func TestHybridSearchEngine_FailoverScenarios(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	fallbackEngine := NewPostgresSearchEngine(repo)

	// Seed document into fallback repo
	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:         docID,
		Filename:   "failover_doc.pdf",
		Category:   CategoryFinancialDoc,
		OCRStatus:  OCRStatusCompleted,
		OCRText:    "Failover content for Meilisearch fallback test.",
		UploadedAt: time.Now(),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	t.Run("Unconfigured Meilisearch Host (Empty Host)", func(t *testing.T) {
		meiliUncfg := NewMeiliSearchEngineWithFallback(MeiliConfig{Host: ""}, fallbackEngine)
		hybrid := NewHybridSearchEngine(meiliUncfg, fallbackEngine)

		// Search should succeed via fallback
		res, err := hybrid.Search(ctx, SearchRequest{
			Query: "Failover",
			Page:  1,
			Limit: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.Total)
		require.Len(t, res.Data, 1)
		assert.Equal(t, docID, res.Data[0].ID)

		// Indexing should delegate to fallback
		require.NoError(t, hybrid.IndexDocument(ctx, doc))

		// Delete should delegate to fallback
		require.NoError(t, hybrid.DeleteDocumentIndex(ctx, docID))
	})

	t.Run("Unreachable Meilisearch Host (Server Down / Connection Refused)", func(t *testing.T) {
		// Use invalid port on localhost that refuses connection
		deadHost := "http://127.0.0.1:59999"
		meiliDead := NewMeiliSearchEngineWithFallback(MeiliConfig{Host: deadHost}, fallbackEngine)
		hybrid := NewHybridSearchEngine(meiliDead, fallbackEngine)

		// Indexing should not crash and delegate to fallback
		_ = hybrid.IndexDocument(ctx, doc)

		// Search should seamlessly fallback without returning error to caller
		res, err := hybrid.Search(ctx, SearchRequest{
			Query: "Failover",
			Page:  1,
			Limit: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.Total)
		assert.Equal(t, docID, res.Data[0].ID)
	})

	t.Run("Meilisearch HTTP Server 500 Internal Error Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		meili500 := NewMeiliSearchEngineWithFallback(MeiliConfig{Host: server.URL}, fallbackEngine)
		hybrid := NewHybridSearchEngine(meili500, fallbackEngine)

		res, err := hybrid.Search(ctx, SearchRequest{
			Query: "Failover",
			Page:  1,
			Limit: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.Total)
		assert.Equal(t, docID, res.Data[0].ID)
	})

	t.Run("Both Primary and Fallback Are Nil", func(t *testing.T) {
		emptyHybrid := NewHybridSearchEngine(nil, nil)

		res, err := emptyHybrid.Search(ctx, SearchRequest{Query: "test"})
		require.NoError(t, err)
		assert.Equal(t, int64(0), res.Total)
		assert.Empty(t, res.Data)

		assert.NoError(t, emptyHybrid.IndexDocument(ctx, doc))
		assert.NoError(t, emptyHybrid.DeleteDocumentIndex(ctx, docID))
	})
}

// Static error engine for custom primary mock testing.
type customErrSearchEngine struct {
	indexErr  error
	deleteErr error
	searchErr error
}

func (c *customErrSearchEngine) IndexDocument(ctx context.Context, doc *ArchiveDocument) error {
	return c.indexErr
}
func (c *customErrSearchEngine) DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error {
	return c.deleteErr
}
func (c *customErrSearchEngine) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	return nil, c.searchErr
}

// TestHybridSearchEngine_CustomErrorMock verifies failover logic when primary engine explicitly returns errors.
func TestHybridSearchEngine_CustomErrorMock(t *testing.T) {
	ctx := context.Background()
	memEngine := NewMemorySearchEngine()

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:         docID,
		Filename:   "custom_err_doc.pdf",
		OCRText:    "Custom error mock fallback verification text.",
		UploadedAt: time.Now(),
	}
	require.NoError(t, memEngine.IndexDocument(ctx, doc))

	primary := &customErrSearchEngine{
		indexErr:  errors.New("primary index error"),
		deleteErr: errors.New("primary delete error"),
		searchErr: errors.New("primary search error"),
	}

	hybrid := NewHybridSearchEngine(primary, memEngine)

	res, err := hybrid.Search(ctx, SearchRequest{
		Query: "Custom",
		Page:  1,
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.Total)
	assert.Equal(t, docID, res.Data[0].ID)

	assert.NoError(t, hybrid.IndexDocument(ctx, doc))
	assert.NoError(t, hybrid.DeleteDocumentIndex(ctx, docID))
}

// =============================================================================
// 3. FACTORY AND CONCURRENCY STRESS TESTS
// =============================================================================

// TestNewSearchEngine_FactoryWiring verifies factory initialization under different configs.
func TestNewSearchEngine_FactoryWiring(t *testing.T) {
	repo := NewMemoryRepository()

	// Empty config -> PostgresSearchEngine (with repo)
	se1 := NewSearchEngine(MeiliConfig{}, repo)
	assert.NotNil(t, se1)

	// Empty config with nil repo -> MemorySearchEngine
	se2 := NewSearchEngine(MeiliConfig{}, nil)
	assert.NotNil(t, se2)

	// Config with host -> MeiliSearchEngine
	se3 := NewSearchEngine(MeiliConfig{Host: "http://localhost:7700"}, repo)
	assert.NotNil(t, se3)
}

// TestSearchEngine_ConcurrentStress Tests concurrent search and failover execution under race detector.
func TestSearchEngine_ConcurrentStress(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	fallback := NewPostgresSearchEngine(repo)

	// Seed 20 documents
	for i := 0; i < 20; i++ {
		doc := &ArchiveDocument{
			ID:          uuid.New(),
			Filename:    fmt.Sprintf("stress_doc_%d.pdf", i),
			Category:    CategoryStudentRecord,
			Tags:        []string{"stress", fmt.Sprintf("tag_%d", i%5)},
			OCRStatus:   OCRStatusCompleted,
			OCRText:     fmt.Sprintf("Stress test OCR document content number %d", i),
			UploadedAt:  time.Now().Add(-1 * time.Duration(i) * time.Hour),
			StorageTier: StorageTierHot,
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))
	}

	deadMeili := NewMeiliSearchEngineWithFallback(MeiliConfig{Host: "http://127.0.0.1:59999"}, fallback)
	hybrid := NewHybridSearchEngine(deadMeili, fallback)

	var wg sync.WaitGroup
	numGoroutines := 20
	requestsPerGoroutine := 15

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()
			for r := 0; r < requestsPerGoroutine; r++ {
				req := SearchRequest{
					Query: "Stress",
					Tags:  []string{"stress"},
					Page:  1,
					Limit: 5,
				}
				res, err := hybrid.Search(ctx, req)
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.Equal(t, int64(20), res.Total)
			}
		}(g)
	}

	wg.Wait()
}

// =============================================================================
// 4. REMEDIATION VERIFICATION TESTS
// =============================================================================

// panickingRepoMock panics on GetDocumentByID to test secondary panic recovery.
type panickingRepoMock struct {
	Repository
}

func (m *panickingRepoMock) GetDocumentByID(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error) {
	panic("SIMULATED REPOSITORY PANIC IN GETDOCUMENTBYID")
}

func (m *panickingRepoMock) UpdateDocument(ctx context.Context, doc *ArchiveDocument) error {
	panic("SIMULATED REPOSITORY PANIC IN UPDATEDOCUMENT")
}

func TestRemediation_HandleJobFailurePanicRecovery(t *testing.T) {
	repo := &panickingRepoMock{}
	pool := NewGoOCRWorkerPool(1, 10, repo, nil)

	// handleJobFailure must recover cleanly from secondary panic
	assert.NotPanics(t, func() {
		pool.handleJobFailure(context.Background(), uuid.New(), errors.New("initial job error"))
	})
}

func TestRemediation_EnqueueClosedChannelRace(t *testing.T) {
	for iteration := 0; iteration < 5; iteration++ {
		pool := NewGoOCRWorkerPool(4, 50, NewMemoryRepository(), nil)
		pool.Start()

		var wg sync.WaitGroup
		// Launch 10 goroutines enqueuing jobs concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					_ = pool.Enqueue(uuid.New(), "", "text/plain")
				}
			}()
		}

		// Concurrently stop the pool
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(1 * time.Millisecond)
			pool.Stop()
		}()

		wg.Wait()
	}
}

func TestRemediation_ZipBombReadLimit(t *testing.T) {
	// Create a zip archive in memory with a file larger than maxZipTotalBytes (20MB)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.Create("large_file.txt")
	require.NoError(t, err)

	// Write 25MB of data
	chunk := bytes.Repeat([]byte("A"), 1024*1024)
	for i := 0; i < 25; i++ {
		_, err := fw.Write(chunk)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	parser := &ZipParser{registry: DefaultParserRegistry}
	res, err := parser.Parse(context.Background(), buf.Bytes(), "application/zip")
	require.NoError(t, err)
	assert.Contains(t, res, "Zip contents truncated (max limit reached)")
}

type customTestParser struct{}

func (c *customTestParser) Parse(ctx context.Context, data []byte, mimeType string) (string, error) {
	return "CUSTOM_REGISTRY_PARSED_TEXT", nil
}

func TestRemediation_WorkerUsesCustomParserRegistry(t *testing.T) {
	repo := NewMemoryRepository()
	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:         docID,
		Filename:   "custom_test.txt",
		MimeType:   "text/plain",
		OCRStatus:  OCRStatusPending,
		UploadedAt: time.Now(),
	}
	require.NoError(t, repo.CreateDocument(context.Background(), doc))

	customRegistry := NewParserRegistry()
	customRegistry.Register(&customTestParser{}, []string{"text/plain"}, []string{".txt"})

	pool := NewGoOCRWorkerPool(1, 10, repo, nil)
	pool.SetParserRegistry(customRegistry)

	// Create temp file
	tmpFile, err := os.CreateTemp("", "custom_ocr_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, _ = tmpFile.WriteString("Sample file content")
	_ = tmpFile.Close()

	pool.processJob(OCRJob{
		DocID:    docID,
		FilePath: tmpFile.Name(),
		MimeType: "text/plain",
	})

	updatedDoc, err := repo.GetDocumentByID(context.Background(), docID)
	require.NoError(t, err)
	assert.Equal(t, OCRStatusCompleted, updatedDoc.OCRStatus)
	assert.Equal(t, "CUSTOM_REGISTRY_PARSED_TEXT", updatedDoc.OCRText)
}

func TestRemediation_UnicodeSnippetAlignment(t *testing.T) {
	t.Run("Turkish capital İ exact alignment", func(t *testing.T) {
		fullText := "İstanbul is a beautiful historic city in Turkey."
		snippet := extractSnippet(fullText, "İstanbul")
		assert.Equal(t, "<em>İstanbul</em> is a beautiful historic city in Turkey.", snippet)
	})

	t.Run("Turkish lowercase query istanbul matching capital İ", func(t *testing.T) {
		fullText := "Welcome to İstanbul today!"
		snippet := extractSnippet(fullText, "istanbul")
		assert.Equal(t, "Welcome to <em>İstanbul</em> today!", snippet)
	})
}

type countingFallbackEngine struct {
	indexCount int
	mu         sync.Mutex
}

func (c *countingFallbackEngine) IndexDocument(ctx context.Context, doc *ArchiveDocument) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.indexCount++
	return nil
}
func (c *countingFallbackEngine) DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (c *countingFallbackEngine) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	return nil, nil
}

func TestRemediation_NoRedundantFallbackIndexing(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"taskUid": 1, "status": "enqueued"}`))
	}))
	defer mockServer.Close()

	fallback := &countingFallbackEngine{}
	meiliEngine := NewMeiliSearchEngineWithFallback(MeiliConfig{
		Host:  mockServer.URL,
		Index: "test_index",
	}, fallback)

	doc := &ArchiveDocument{
		ID:         uuid.New(),
		Filename:   "meili_success.pdf",
		UploadedAt: time.Now(),
	}

	err := meiliEngine.IndexDocument(context.Background(), doc)
	require.NoError(t, err)

	fallback.mu.Lock()
	count := fallback.indexCount
	fallback.mu.Unlock()

	// When Meilisearch indexing succeeds, fallback index must NOT be called redundantly
	assert.Equal(t, 0, count)
}

