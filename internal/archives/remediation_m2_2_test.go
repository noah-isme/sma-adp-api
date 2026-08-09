package archives

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// 1. MEILISEARCH SINGLE-QUOTE ESCAPING & UNICODE BOUNDS TESTS
// -----------------------------------------------------------------------------

func TestMeilisearch_SingleQuoteFilterEscaping(t *testing.T) {
	var capturedFilter string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		capturedFilter = buf.String()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"hits": [],
			"totalHits": 0
		}`))
	}))
	defer server.Close()

	cfg := MeiliConfig{
		Host:  server.URL,
		Index: "test_archives",
	}

	engine := NewMeiliSearchEngine(cfg)
	ctx := context.Background()

	req := SearchRequest{
		Query:    "contract",
		Category: DocumentCategory("Lawyer's Brief"),
		Tags:     []string{"Attorney's Work", "Client's Request"},
		Page:     1,
		Limit:    10,
	}

	res, err := engine.Search(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Contains(t, capturedFilter, `category = 'Lawyer\\'s Brief'`)
	assert.Contains(t, capturedFilter, `tags = 'Attorney\\'s Work'`)
	assert.Contains(t, capturedFilter, `tags = 'Client\\'s Request'`)
}

func TestMeilisearch_FilterEscaping_BackslashAndQuote(t *testing.T) {
	// Direct function tests for escapeMeiliFilterVal
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `Folder\`,
			expected: `Folder\\`,
		},
		{
			input:    `O'Reilly`,
			expected: `O\'Reilly`,
		},
		{
			input:    `Folder\'s`,
			expected: `Folder\\\'s`,
		},
		{
			input:    `\path\to\'file\`,
			expected: `\\path\\to\\\'file\\`,
		},
	}

	for _, tt := range tests {
		escaped := escapeMeiliFilterVal(tt.input)
		assert.Equal(t, tt.expected, escaped, "escapeMeiliFilterVal(%q)", tt.input)
	}

	// End-to-end search filter payload verification
	var capturedFilter string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		capturedFilter = buf.String()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"hits": [],
			"totalHits": 0
		}`))
	}))
	defer server.Close()

	cfg := MeiliConfig{
		Host:  server.URL,
		Index: "test_archives",
	}

	engine := NewMeiliSearchEngine(cfg)
	ctx := context.Background()

	req := SearchRequest{
		Query:       "test",
		Category:    DocumentCategory(`Folder\`),
		StorageTier: StorageTier(`O'Reilly`),
		Tags:        []string{`Tag\'s`},
		Page:        1,
		Limit:       10,
	}

	res, err := engine.Search(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Filter strings:
	// category = 'Folder\\' (trailing backslash is escaped so closing quote is NOT escaped)
	assert.Contains(t, capturedFilter, `category = 'Folder\\\\'`)
	// storage_tier = 'O\'Reilly' (single quote is escaped)
	assert.Contains(t, capturedFilter, `storage_tier = 'O\\'Reilly'`)
	// tags = 'Tag\\\'s' (backslash escaped, single quote escaped)
	assert.Contains(t, capturedFilter, `tags = 'Tag\\\\\\'s'`)
}

func TestMeilisearch_FilterEscaping_LiteralClosureValidation(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Trailing single backslash",
			input:    `Folder\`,
			expected: `Folder\\`,
		},
		{
			name:     "Trailing double backslash",
			input:    `Folder\\`,
			expected: `Folder\\\\`,
		},
		{
			name:     "Single quote in middle",
			input:    `O'Reilly`,
			expected: `O\'Reilly`,
		},
		{
			name:     "Combined backslash and single quote",
			input:    `Folder\'s`,
			expected: `Folder\\\'s`,
		},
		{
			name:     "Complex path with backslashes and quotes",
			input:    `C:\Program Files\App's Data\`,
			expected: `C:\\Program Files\\App\'s Data\\`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			escaped := escapeMeiliFilterVal(tc.input)
			assert.Equal(t, tc.expected, escaped)

			// Formatted filter literal string: category = '<escaped>'
			formatted := "category = '" + escaped + "'"

			// Verify that the string literal ends with an unescaped single quote
			// Count trailing backslashes right before the closing quote
			inner := formatted[12 : len(formatted)-1] // stripped "category = '" and trailing "'"
			trailingBackslashCount := 0
			for i := len(inner) - 1; i >= 0; i-- {
				if inner[i] == '\\' {
					trailingBackslashCount++
				} else {
					break
				}
			}

			// If trailing backslash count is even (0, 2, 4...), the closing quote is NOT escaped
			assert.Equal(t, 0, trailingBackslashCount%2, "Closing quote must not be escaped by an odd number of backslashes for input %q", tc.input)
		})
	}
}

func TestExtractSnippet_TurkishRuneBounds_NoPanic(t *testing.T) {
	tests := []struct {
		name     string
		fullText string
		query    string
	}{
		{
			name:     "Turkish İstanbul capital I with dot",
			fullText: "Belge konusu: İstanbul Barosu Avukatlık Sözleşmesi ve Müvekkil Hakları.",
			query:    "istanbul",
		},
		{
			name:     "Turkish İstanbul exact capital query",
			fullText: "Belge konusu: İstanbul Barosu Avukatlık Sözleşmesi ve Müvekkil Hakları.",
			query:    "İstanbul",
		},
		{
			name:     "Mixed Turkish & CJK Unicode",
			fullText: "İstanbul / 成绩单 / Café & Résumé — Müller GmbH",
			query:    "成绩单",
		},
		{
			name:     "Long text Turkish İ at boundary",
			fullText: strings.Repeat("Örnek metin içerik. ", 10) + "İstanbul" + strings.Repeat(" son metin.", 10),
			query:    "istanbul",
		},
		{
			name:     "Query longer than text",
			fullText: "İ",
			query:    "İstanbul Barosu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				snippet := extractSnippet(tt.fullText, tt.query)
				assert.NotEmpty(t, snippet)
			})
		})
	}
}

// -----------------------------------------------------------------------------
// 2. OCR WORKER POOL CONCURRENT ENQUEUE/STOP & PANIC ESCALATION TESTS
// -----------------------------------------------------------------------------

func TestOCRWorkerPool_ConcurrentEnqueueAndStop_NoPanic(t *testing.T) {
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	pool := NewGoOCRWorkerPool(4, 20, repo, searchEngine)
	pool.Start()

	var wg sync.WaitGroup
	workersCount := 10

	// Launch concurrent enqueuers
	for i := 0; i < workersCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = pool.Enqueue(uuid.New(), "", "text/plain")
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Stop pool concurrently while enqueuers are active
	time.Sleep(5 * time.Millisecond)
	assert.NotPanics(t, func() {
		pool.Stop()
	})

	wg.Wait()
}

type PanickingMockRepo struct {
	Repository
	panicOnUpdate bool
}

func (m *PanickingMockRepo) GetDocumentByID(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error) {
	return &ArchiveDocument{
		ID:          id,
		Filename:    "panic_doc.txt",
		MimeType:    "text/plain",
		OCRStatus:   OCRStatusPending,
		Category:    CategoryLegalDoc,
		RetainUntil: time.Now().Add(24 * time.Hour),
		UploadedAt:  time.Now(),
	}, nil
}

func (m *PanickingMockRepo) UpdateDocument(ctx context.Context, doc *ArchiveDocument) error {
	if m.panicOnUpdate {
		panic("CRITICAL REPOSITORY PANIC IN UPDATE DOCUMENT")
	}
	return nil
}

func TestOCRWorkerPool_PanicEscalationInHandleJobFailure_NoWorkerCrash(t *testing.T) {
	panickingRepo := &PanickingMockRepo{panicOnUpdate: true}
	searchEngine := NewMemorySearchEngine()

	pool := NewGoOCRWorkerPool(2, 10, panickingRepo, searchEngine)
	pool.Start()
	defer pool.Stop()

	docID := uuid.New()
	err := pool.Enqueue(docID, "", "text/plain")
	require.NoError(t, err)

	// Wait for worker loop to process the job and recover from the repository panic
	time.Sleep(100 * time.Millisecond)

	status := pool.Status()
	assert.GreaterOrEqual(t, status.ProcessedCount, int64(1))
}

type CustomTestParser struct {
	called bool
	mu     sync.Mutex
}

func (p *CustomTestParser) Parse(ctx context.Context, data []byte, mimeType string) (string, error) {
	p.mu.Lock()
	p.called = true
	p.mu.Unlock()
	return "CUSTOM PARSER EXTRACTED CONTENT", nil
}

func TestOCRWorkerPool_UsesCustomParserRegistry(t *testing.T) {
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	pool := NewGoOCRWorkerPool(2, 10, repo, searchEngine)

	customParser := &CustomTestParser{}
	customRegistry := NewParserRegistry()
	customRegistry.Register(customParser, []string{"text/custom"}, []string{".custom"})

	pool.SetParserRegistry(customRegistry)
	pool.Start()
	defer pool.Stop()

	// Save dummy document in repo
	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "test.custom",
		MimeType:    "text/custom",
		OCRStatus:   OCRStatusPending,
		Category:    CategoryLegalDoc,
		RetainUntil: time.Now().Add(24 * time.Hour),
		UploadedAt:  time.Now(),
	}
	require.NoError(t, repo.CreateDocument(context.Background(), doc))

	tmpFile, err := os.CreateTemp("", "test_custom_*.custom")
	require.NoError(t, err)
	_, _ = tmpFile.WriteString("raw file data")
	_ = tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	require.NoError(t, pool.Enqueue(docID, tmpFile.Name(), "text/custom"))

	time.Sleep(100 * time.Millisecond)

	customParser.mu.Lock()
	wasCalled := customParser.called
	customParser.mu.Unlock()

	assert.True(t, wasCalled, "Expected custom parser to be called by GoOCRWorkerPool")

	updatedDoc, err := repo.GetDocumentByID(context.Background(), docID)
	require.NoError(t, err)
	assert.Equal(t, "CUSTOM PARSER EXTRACTED CONTENT", updatedDoc.OCRText)
}

// -----------------------------------------------------------------------------
// 3. ZIP PARSER ZIP BOMB VULNERABILITY PROTECTION TESTS
// -----------------------------------------------------------------------------

func TestZipParser_ZipBombProtection_LimitsStreamReading(t *testing.T) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// Create a zip entry claiming large uncompressed size (simulated zip bomb)
	w, err := zw.Create("large_bomb.txt")
	require.NoError(t, err)

	// Write 25 MB of repeated text payload to exceed 20MB limit
	chunk := bytes.Repeat([]byte("A"), 1024*1024) // 1 MB chunk
	for i := 0; i < 25; i++ {
		_, err := w.Write(chunk)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	zipData := buf.Bytes()
	zipParser := &ZipParser{registry: DefaultParserRegistry}

	ctx := context.Background()
	extracted, err := zipParser.Parse(ctx, zipData, "application/zip")
	require.NoError(t, err)

	assert.Contains(t, extracted, "Zip contents truncated (max limit reached)")
	// Assert total output string length is bounded cleanly
	assert.LessOrEqual(t, int64(len(extracted)), int64(maxZipTotalBytes+4096))
}
