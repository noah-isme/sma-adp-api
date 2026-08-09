package archives

import (
	"context"
	"fmt"
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

func TestEmpirical_MeiliFilterEscapingEdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trailing backslash",
			input:    `abc\`,
			expected: `abc\\`,
		},
		{
			name:     "single quote",
			input:    `O'Connor`,
			expected: `O\'Connor`,
		},
		{
			name:     "backslash before quote",
			input:    `abc\'def`,
			expected: `abc\\\'def`,
		},
		{
			name:     "empty string",
			input:    ``,
			expected: ``,
		},
		{
			name:     "multiple backslashes",
			input:    `\\\\`,
			expected: `\\\\\\\\`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			escaped := escapeMeiliFilterVal(tc.input)
			assert.Equal(t, tc.expected, escaped)

			req := SearchRequest{
				Category: DocumentCategory(tc.input),
				Tags:     []string{tc.input},
			}

			expectedCategoryFilter := fmt.Sprintf("category = '%s'", tc.expected)
			expectedTagFilter := fmt.Sprintf("tags = '%s'", tc.expected)

			assert.Contains(t, fmt.Sprintf("category = '%s'", escapeMeiliFilterVal(string(req.Category))), expectedCategoryFilter)
			assert.Contains(t, fmt.Sprintf("tags = '%s'", escapeMeiliFilterVal(req.Tags[0])), expectedTagFilter)
		})
	}
}

func TestEmpirical_AsyncOCRPipeline_JobProcessing(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()

	pool := NewGoOCRWorkerPool(4, 50, repo, searchEngine)
	pool.Start()
	defer pool.Stop()

	docID := uuid.New()
	tmpFile := filepath.Join(os.TempDir(), "ocr_empirical_test.txt")
	testContent := "Empirical Async OCR Pipeline Content Verification"
	require.NoError(t, os.WriteFile(tmpFile, []byte(testContent), 0644))
	defer os.Remove(tmpFile)

	doc := &ArchiveDocument{
		ID:         docID,
		Filename:   "ocr_empirical_test.txt",
		Category:   CategoryOther,
		OCRStatus:  OCRStatusPending,
		UploadedAt: time.Now(),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	require.NoError(t, pool.Enqueue(docID, tmpFile, "text/plain"))

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
	assert.Contains(t, updatedDoc.OCRText, testContent)

	searchResults, err := searchEngine.Search(ctx, SearchRequest{Query: "Empirical"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), searchResults.Total)
	assert.Equal(t, docID, searchResults.Data[0].ID)
}

func TestEmpirical_AsyncOCRPipeline_ContextCancellation(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()

	slowRegistry := NewParserRegistry()
	slowRegistry.Register(&slowParser{}, []string{"text/slow"}, []string{".slow"})

	pool := NewGoOCRWorkerPool(1, 10, repo, searchEngine)
	pool.SetParserRegistry(slowRegistry)
	pool.timeout = 100 * time.Millisecond

	pool.Start()
	defer pool.Stop()

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:         docID,
		Filename:   "slow.slow",
		Category:   CategoryOther,
		OCRStatus:  OCRStatusPending,
		UploadedAt: time.Now(),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	tmpFile := filepath.Join(os.TempDir(), "slow_test.slow")
	require.NoError(t, os.WriteFile(tmpFile, []byte("slow data"), 0644))
	defer os.Remove(tmpFile)

	require.NoError(t, pool.Enqueue(docID, tmpFile, "text/slow"))

	require.Eventually(t, func() bool {
		d, err := repo.GetDocumentByID(ctx, docID)
		if err != nil {
			return false
		}
		return d.OCRStatus == OCRStatusFailed
	}, 3*time.Second, 50*time.Millisecond)

	failedDoc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, OCRStatusFailed, failedDoc.OCRStatus)
}

type slowParser struct{}

func (s *slowParser) Parse(ctx context.Context, data []byte, mimeType string) (string, error) {
	select {
	case <-time.After(2 * time.Second):
		return "completed", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestEmpirical_AsyncOCRPipeline_WorkerPoolShutdown(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()

	pool := NewGoOCRWorkerPool(4, 100, repo, searchEngine)

	numJobs := 20
	docIDs := make([]uuid.UUID, numJobs)
	for i := 0; i < numJobs; i++ {
		docIDs[i] = uuid.New()
		doc := &ArchiveDocument{
			ID:         docIDs[i],
			Filename:   fmt.Sprintf("doc_%d.txt", i),
			Category:   CategoryOther,
			OCRStatus:  OCRStatusPending,
			UploadedAt: time.Now(),
		}
		require.NoError(t, repo.CreateDocument(ctx, doc))
	}

	pool.Start()

	for i := 0; i < numJobs; i++ {
		err := pool.Enqueue(docIDs[i], "", "text/plain")
		require.NoError(t, err)
	}

	pool.Stop()

	errPostStop := pool.Enqueue(uuid.New(), "", "text/plain")
	assert.Error(t, errPostStop)
	assert.Contains(t, errPostStop.Error(), "stopped")

	status := pool.Status()
	assert.Equal(t, int64(numJobs), status.ProcessedCount)
}

func TestEmpirical_ErrorFallbackToPostgresSearch(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:               docID,
		Filename:         "postgres_fallback.pdf",
		OriginalFilename: "postgres_fallback.pdf",
		MimeType:         "application/pdf",
		SizeBytes:        1024,
		Checksum:         "hash123",
		StorageTier:      StorageTierHot,
		Category:         CategoryFinancialDoc,
		Tags:             []string{"finance"},
		OCRStatus:        OCRStatusCompleted,
		OCRText:          "Postgres fallback search verification text",
		RetainUntil:      time.Now().AddDate(1, 0, 0),
		UploadedBy:       uuid.New(),
		UploadedAt:       time.Now(),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	pgSearch := NewPostgresSearchEngine(repo)

	// 1. Mock Meilisearch HTTP server returning 500 error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	meiliWithPgFallback := NewMeiliSearchEngineWithFallback(MeiliConfig{
		Host:  ts.URL,
		Index: "archives",
	}, pgSearch)

	res, err := meiliWithPgFallback.Search(ctx, SearchRequest{Query: "Postgres"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, int64(1), res.Total)
	require.Len(t, res.Data, 1)
	assert.Equal(t, docID, res.Data[0].ID)
	assert.Contains(t, res.Data[0].OCRText, "Postgres fallback search verification text")

	// 2. Mock Meilisearch host down (unreachable)
	meiliDown := NewMeiliSearchEngineWithFallback(MeiliConfig{
		Host:  "http://127.0.0.1:59999",
		Index: "archives",
	}, pgSearch)

	resDown, err := meiliDown.Search(ctx, SearchRequest{Query: "verification"})
	require.NoError(t, err)
	require.NotNil(t, resDown)
	assert.Equal(t, int64(1), resDown.Total)
	assert.Equal(t, docID, resDown.Data[0].ID)

	// 3. Meilisearch unconfigured host (empty host)
	meiliUnconfigured := NewMeiliSearchEngineWithFallback(MeiliConfig{
		Host:  "",
		Index: "archives",
	}, pgSearch)

	resUnconfig, err := meiliUnconfigured.Search(ctx, SearchRequest{Query: "fallback"})
	require.NoError(t, err)
	require.NotNil(t, resUnconfig)
	assert.Equal(t, int64(1), resUnconfig.Total)
	assert.Equal(t, docID, resUnconfig.Data[0].ID)
}
