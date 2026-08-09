package archives

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSnippet(t *testing.T) {
	t.Run("empty text", func(t *testing.T) {
		assert.Equal(t, "", extractSnippet("", "query"))
	})

	t.Run("empty query", func(t *testing.T) {
		shortText := "Short document snippet."
		assert.Equal(t, shortText, extractSnippet(shortText, ""))

		longText := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
		snippet := extractSnippet(longText, "")
		assert.True(t, len(snippet) > 0)
		assert.Contains(t, snippet, "...")
	})

	t.Run("query match highlighted", func(t *testing.T) {
		fullText := "Official Academic Transcript for Student ID stu_001. Final Grade: A in Mathematics and Science."
		snippet := extractSnippet(fullText, "Mathematics")
		assert.Contains(t, snippet, "<em>Mathematics</em>")
	})

	t.Run("query case insensitivity", func(t *testing.T) {
		fullText := "Official Academic Transcript for Student ID stu_001. Final Grade: A in Mathematics and Science."
		snippet := extractSnippet(fullText, "mathematics")
		assert.Contains(t, snippet, "<em>Mathematics</em>")
	})

	t.Run("query not found", func(t *testing.T) {
		fullText := "Official Academic Transcript for Student ID stu_001."
		snippet := extractSnippet(fullText, "Physics")
		assert.Equal(t, fullText, snippet)
	})

	t.Run("unicode safe extraction", func(t *testing.T) {
		fullText := "Dokumen Resmi: Nilai Akhir Bahasa Indonesia & Café Résumé."
		snippet := extractSnippet(fullText, "Bahasa")
		assert.Contains(t, snippet, "<em>Bahasa</em>")
	})
}

func TestMemorySearchEngine(t *testing.T) {
	ctx := context.Background()
	engine := NewMemorySearchEngine()

	docID1 := uuid.New()
	docID2 := uuid.New()

	doc1 := &ArchiveDocument{
		ID:               docID1,
		Filename:         "report_2025.pdf",
		OriginalFilename: "report_2025.pdf",
		MimeType:         "application/pdf",
		SizeBytes:        1024,
		Checksum:         "abc123hash",
		StorageTier:      StorageTierHot,
		Category:         CategoryFinancialDoc,
		Tags:             []string{"finance", "2025", "audit"},
		OCRStatus:        OCRStatusCompleted,
		OCRText:          "Annual Financial Report 2025. Total revenue increased by 20%.",
		RetainUntil:      time.Now().AddDate(5, 0, 0),
		LegalHold:        false,
		UploadedBy:       uuid.New(),
		UploadedAt:       time.Now(),
	}

	doc2 := &ArchiveDocument{
		ID:               docID2,
		Filename:         "student_transcript.pdf",
		OriginalFilename: "student_transcript.pdf",
		MimeType:         "application/pdf",
		SizeBytes:        2048,
		Checksum:         "def456hash",
		StorageTier:      StorageTierWarm,
		Category:         CategoryStudentRecord,
		Tags:             []string{"student", "transcript"},
		OCRStatus:        OCRStatusCompleted,
		OCRText:          "Official Student Record for John Doe. Grade in Mathematics: A.",
		RetainUntil:      time.Now().AddDate(7, 0, 0),
		LegalHold:        true,
		UploadedBy:       uuid.New(),
		UploadedAt:       time.Now(),
	}

	// 1. Indexing
	require.NoError(t, engine.IndexDocument(ctx, doc1))
	require.NoError(t, engine.IndexDocument(ctx, doc2))

	// 2. Search positive query
	res, err := engine.Search(ctx, SearchRequest{
		Query: "revenue",
		Page:  1,
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.Total)
	require.Len(t, res.Data, 1)
	assert.Equal(t, docID1, res.Data[0].ID)
	assert.Contains(t, res.Data[0].Snippet, "<em>revenue</em>")

	// 3. Search category filter
	resCat, err := engine.Search(ctx, SearchRequest{
		Category: CategoryStudentRecord,
		Page:     1,
		Limit:    10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resCat.Total)
	assert.Equal(t, docID2, resCat.Data[0].ID)

	// 4. Search legal hold filter
	resHold, err := engine.Search(ctx, SearchRequest{
		LegalHoldOnly: true,
		Page:          1,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resHold.Total)
	assert.Equal(t, docID2, resHold.Data[0].ID)

	// 5. Delete index
	require.NoError(t, engine.DeleteDocumentIndex(ctx, docID1))
	resAfterDel, err := engine.Search(ctx, SearchRequest{
		Query: "revenue",
		Page:  1,
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resAfterDel.Total)
}

type errSearchEngine struct{}

func (e *errSearchEngine) IndexDocument(ctx context.Context, doc *ArchiveDocument) error {
	return errors.New("meilisearch connection error")
}
func (e *errSearchEngine) DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error {
	return errors.New("meilisearch connection error")
}
func (e *errSearchEngine) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	return nil, errors.New("meilisearch host down")
}

func TestHybridSearchEngineFallback(t *testing.T) {
	ctx := context.Background()

	primaryErrEngine := &errSearchEngine{}
	fallbackEngine := NewMemorySearchEngine()

	hybrid := NewHybridSearchEngine(primaryErrEngine, fallbackEngine)

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:       docID,
		Filename: "fallback_doc.pdf",
		Category: CategoryOther,
		OCRText:  "Fallback content for testing hybrid decorator.",
	}

	// Indexing shouldn't fail even if primary errors
	errIndex := hybrid.IndexDocument(ctx, doc)
	assert.NoError(t, errIndex)

	// Search should automatically fall back to fallbackEngine without error
	res, err := hybrid.Search(ctx, SearchRequest{
		Query: "Fallback",
		Page:  1,
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.Total)
	assert.Equal(t, docID, res.Data[0].ID)
}

func TestNewSearchEngineFactory(t *testing.T) {
	repo := NewMemoryRepository()

	// Default config with empty host returns Postgres/Memory search engine
	se1 := NewSearchEngine(MeiliConfig{}, repo)
	assert.NotNil(t, se1)

	// Config with host returns HybridSearchEngine
	se2 := NewSearchEngine(MeiliConfig{Host: "http://localhost:7700"}, repo)
	assert.NotNil(t, se2)
}

func TestMeiliSearchEngineFallback(t *testing.T) {
	ctx := context.Background()
	memEngine := NewMemorySearchEngine()
	cfg := MeiliConfig{Host: "", Index: "archives"}

	meiliEngine := NewMeiliSearchEngineWithFallback(cfg, memEngine)

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:        docID,
		Filename:  "test_doc.pdf",
		Category:  CategoryFinancialDoc,
		OCRText:   "Meilisearch fallback indexing test text.",
		UploadedAt: time.Now(),
	}

	require.NoError(t, meiliEngine.IndexDocument(ctx, doc))

	res, err := meiliEngine.Search(ctx, SearchRequest{
		Query: "fallback",
		Page:  1,
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.Total)
	assert.Equal(t, docID, res.Data[0].ID)

	require.NoError(t, meiliEngine.DeleteDocumentIndex(ctx, docID))
	resAfter, err := meiliEngine.Search(ctx, SearchRequest{
		Query: "fallback",
		Page:  1,
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resAfter.Total)
}

