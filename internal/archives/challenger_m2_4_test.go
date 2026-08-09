package archives

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// 1. MEILISEARCH FILTER ESCAPING ADVERSARIAL TESTS
// -----------------------------------------------------------------------------

func TestMeilisearch_FilterEscaping_TrailingBackslash_UnclosedQuoteBug(t *testing.T) {
	var capturedFilter string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		capturedFilter = buf.String()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hits": [], "totalHits": 0}`))
	}))
	defer server.Close()

	engine := NewMeiliSearchEngine(MeiliConfig{Host: server.URL, Index: "test"})

	// Input ending with a backslash: "folder\"
	req := SearchRequest{
		Query:    "test",
		Category: DocumentCategory("folder\\"),
		Tags:     []string{"tag\\"},
		Page:     1,
		Limit:    10,
	}

	res, err := engine.Search(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	t.Logf("Captured Meilisearch payload: %s", capturedFilter)

	// Check if trailing backslash escapes the closing single quote
	// In category = 'folder\' AND ..., 'folder\' leaves the single quote escaped!
	// Proper escaping must produce category = 'folder\\' AND tags = 'tag\\'
	assert.NotContains(t, capturedFilter, "category = 'folder\\'", "Filter value with trailing backslash must not escape the closing quote")
	assert.NotContains(t, capturedFilter, "tags = 'tag\\'", "Filter tag with trailing backslash must not escape the closing quote")
}

func TestMeilisearch_FilterEscaping_BackslashSingleQuoteCombination(t *testing.T) {
	var capturedFilter string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		capturedFilter = buf.String()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hits": [], "totalHits": 0}`))
	}))
	defer server.Close()

	engine := NewMeiliSearchEngine(MeiliConfig{Host: server.URL, Index: "test"})

	req := SearchRequest{
		Query:    "test",
		Category: DocumentCategory("Lawyer\\'s Brief"),
	}

	_, err := engine.Search(context.Background(), req)
	require.NoError(t, err)

	t.Logf("Captured Meilisearch payload for Lawyer\\'s Brief: %s", capturedFilter)
}

// -----------------------------------------------------------------------------
// 2. REPOSITORY PANIC HANDLING IN OCR_WORKER.GO
// -----------------------------------------------------------------------------

type ExtremePanicRepo struct {
	Repository
	panicCount int64
}

func (r *ExtremePanicRepo) GetDocumentByID(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error) {
	atomic.AddInt64(&r.panicCount, 1)
	panic("CRITICAL DB CONNECTION CRASH ON GET_DOCUMENT")
}

func (r *ExtremePanicRepo) UpdateDocument(ctx context.Context, doc *ArchiveDocument) error {
	atomic.AddInt64(&r.panicCount, 1)
	panic("CRITICAL DB CONNECTION CRASH ON UPDATE_DOCUMENT")
}

func TestOCRWorkerPool_RepositoryPanicHandling_WorkersSurvive(t *testing.T) {
	panicRepo := &ExtremePanicRepo{}
	searchEngine := NewMemorySearchEngine()

	pool := NewGoOCRWorkerPool(4, 50, panicRepo, searchEngine)
	pool.Start()

	// Enqueue 20 jobs that will trigger panics on GetDocumentByID
	for i := 0; i < 20; i++ {
		err := pool.Enqueue(uuid.New(), "", "text/plain")
		require.NoError(t, err)
	}

	// Wait for jobs to process
	time.Sleep(200 * time.Millisecond)

	status := pool.Status()
	assert.Equal(t, int64(20), status.ProcessedCount, "All jobs should be processed despite panics")
	assert.Equal(t, 0, status.WorkersActive, "No worker goroutines should remain active/hung")

	// Now enqueue a valid job with a working repo to verify workers are still alive
	workingRepo := NewMemoryRepository()
	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "survivor.txt",
		MimeType:    "text/plain",
		OCRStatus:   OCRStatusPending,
		Category:    CategoryLegalDoc,
		RetainUntil: time.Now().Add(24 * time.Hour),
		UploadedAt:  time.Now(),
	}
	require.NoError(t, workingRepo.CreateDocument(context.Background(), doc))

	// Reassign repo on pool (simulating DB recovery)
	pool.repo = workingRepo

	require.NoError(t, pool.Enqueue(docID, "", "text/plain"))
	time.Sleep(100 * time.Millisecond)

	pool.Stop()

	updatedDoc, err := workingRepo.GetDocumentByID(context.Background(), docID)
	require.NoError(t, err)
	assert.Equal(t, OCRStatusCompleted, updatedDoc.OCRStatus, "Worker pool must remain fully functional after repository panics")
}

// -----------------------------------------------------------------------------
// 3. CUSTOM PARSER REGISTRY DISPATCH TESTS
// -----------------------------------------------------------------------------

type PanickingParser struct{}

func (p *PanickingParser) Parse(ctx context.Context, data []byte, mimeType string) (string, error) {
	panic("CUSTOM PARSER EXPLODED IN PARSE")
}

func TestOCRWorkerPool_CustomParserPanic_HandledGracefully(t *testing.T) {
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	pool := NewGoOCRWorkerPool(2, 10, repo, searchEngine)

	customRegistry := NewParserRegistry()
	customRegistry.Register(&PanickingParser{}, []string{"application/x-panic"}, []string{".panic"})
	pool.SetParserRegistry(customRegistry)

	pool.Start()

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "explode.panic",
		MimeType:    "application/x-panic",
		OCRStatus:   OCRStatusPending,
		Category:    CategoryLegalDoc,
		RetainUntil: time.Now().Add(24 * time.Hour),
		UploadedAt:  time.Now(),
	}
	require.NoError(t, repo.CreateDocument(context.Background(), doc))

	tmpFile, err := os.CreateTemp("", "test_panic_*.panic")
	require.NoError(t, err)
	_, _ = tmpFile.WriteString("some payload")
	_ = tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	require.NoError(t, pool.Enqueue(docID, tmpFile.Name(), "application/x-panic"))
	time.Sleep(150 * time.Millisecond)

	pool.Stop()

	updatedDoc, err := repo.GetDocumentByID(context.Background(), docID)
	require.NoError(t, err)
	assert.Equal(t, OCRStatusFailed, updatedDoc.OCRStatus, "Document status should be FAILED when parser panics")
}
