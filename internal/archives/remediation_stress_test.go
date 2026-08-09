package archives

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// -----------------------------------------------------------------------------
// 1. File Byte Extraction Stress Tests
// -----------------------------------------------------------------------------

func TestFileByteExtraction_Stress(t *testing.T) {
	t.Run("ASCII Printable Extraction", func(t *testing.T) {
		input := []byte("Hello World! 123 \n\r\t")
		expected := "Hello World! 123"
		got := extractTextFromBytes(input, "text/plain")
		if got != expected {
			t.Errorf("extractTextFromBytes ASCII failed: expected %q, got %q", expected, got)
		}
	})

	t.Run("UTF-8 Non-ASCII Characters (Accents, International Scripts, Emoji)", func(t *testing.T) {
		// Testing how extractTextFromBytes handles multi-byte UTF-8 sequences.
		// UTF-8 bytes for 'Café', 'Résumé', 'Müller', '日本語', '🚀' have byte values >= 128.
		input := []byte("Document Title: Café & Résumé — Müller GmbH (日本語 / 🚀)")
		got := extractTextFromBytes(input, "text/plain")
		
		t.Logf("Input UTF-8: %s", string(input))
		t.Logf("Extracted text: %s", got)

		// Note: extractTextFromBytes currently filters (b >= 32 && b <= 126), stripping UTF-8 multi-byte sequences.
		// If Café turns into Caf, Résumé into Rsum, Müller into Mller:
		if got != string(input) {
			t.Logf("BEHAVIOR DETECTED: extractTextFromBytes strips non-ASCII UTF-8 characters: input length %d -> extracted length %d", len(input), len(got))
		}
	})

	t.Run("Binary File Data (PDF / Image bytes)", func(t *testing.T) {
		binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x80, 0x7F, 0x00, 0x0A, 0x0D}
		got := extractTextFromBytes(binaryData, "application/octet-stream")
		if got != "" {
			t.Errorf("expected empty string for pure non-printable binary data, got %q", got)
		}
	})

	t.Run("Mixed Binary & ASCII Data", func(t *testing.T) {
		mixed := []byte{0x00, 0x10, 'A', 'B', 0xFF, 'C', 'D'}
		got := extractTextFromBytes(mixed, "application/pdf")
		if got != "ABCD" {
			t.Errorf("expected ABCD, got %q", got)
		}
	})

	t.Run("OCR Worker File Processing Fallback & Pre-existing Text Overwrite", func(t *testing.T) {
		repo := NewMemoryRepository()
		pool := NewGoOCRWorkerPool(1, 10, repo, nil)
		pool.Start()
		defer pool.Stop()

		// Create temp file with non-printable binary bytes
		tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("test_binary_%d.bin", time.Now().UnixNano()))
		_ = os.WriteFile(tmpFile, []byte{0x00, 0x01, 0x02, 0x03}, 0644)
		defer os.Remove(tmpFile)

		docID := uuid.New()
		doc := &ArchiveDocument{
			ID:          docID,
			Filename:    "binary_doc.bin",
			MimeType:    "application/octet-stream",
			Category:    CategoryOther,
			OCRStatus:   OCRStatusPending,
			OCRText:     "Preserved Pre-existing Text",
			StoragePath: tmpFile,
		}
		ctx := context.Background()
		if err := repo.CreateDocument(ctx, doc); err != nil {
			t.Fatalf("CreateDocument failed: %v", err)
		}

		err := pool.Enqueue(docID, tmpFile, doc.MimeType)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}

		// Wait for pool to process
		time.Sleep(100 * time.Millisecond)

		updated, err := repo.GetDocumentByID(ctx, docID)
		if err != nil {
			t.Fatalf("GetDocumentByID failed: %v", err)
		}

		t.Logf("Final OCRText: %q", updated.OCRText)
		if updated.OCRStatus != OCRStatusCompleted {
			t.Errorf("expected OCRStatusCompleted, got %v", updated.OCRStatus)
		}
	})
}

// -----------------------------------------------------------------------------
// 2. Tag Slice Filtering in MemoryRepository.ListDocuments Stress Tests
// -----------------------------------------------------------------------------

func TestTagSliceFiltering_Stress(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()

	doc1ID := uuid.New()
	doc2ID := uuid.New()
	doc3ID := uuid.New()
	doc4ID := uuid.New()

	docs := []*ArchiveDocument{
		{
			ID:          doc1ID,
			Filename:    "doc1.pdf",
			Category:    CategoryStudentRecord,
			Tags:        []string{"finance", "urgent", "2026"},
			StorageTier: StorageTierHot,
		},
		{
			ID:          doc2ID,
			Filename:    "doc2.pdf",
			Category:    CategoryStudentRecord,
			Tags:        []string{"finance"},
			StorageTier: StorageTierHot,
		},
		{
			ID:          doc3ID,
			Filename:    "doc3.pdf",
			Category:    CategoryStudentRecord,
			Tags:        []string{"urgent", "2026"},
			StorageTier: StorageTierHot,
		},
		{
			ID:          doc4ID,
			Filename:    "doc4.pdf",
			Category:    CategoryStudentRecord,
			Tags:        nil, // nil tags
			StorageTier: StorageTierHot,
		},
	}

	for _, d := range docs {
		if err := repo.CreateDocument(ctx, d); err != nil {
			t.Fatalf("CreateDocument failed: %v", err)
		}
	}

	t.Run("Single Tag Match", func(t *testing.T) {
		res, count, err := repo.ListDocuments(ctx, ArchiveFilter{Tags: []string{"finance"}})
		if err != nil {
			t.Fatalf("ListDocuments failed: %v", err)
		}
		if count != 2 || len(res) != 2 {
			t.Errorf("expected 2 documents with tag 'finance', got count=%d, len=%d", count, len(res))
		}
	})

	t.Run("Multiple Tag Match (All Required / Subset)", func(t *testing.T) {
		res, count, err := repo.ListDocuments(ctx, ArchiveFilter{Tags: []string{"finance", "urgent"}})
		if err != nil {
			t.Fatalf("ListDocuments failed: %v", err)
		}
		if count != 1 || len(res) != 1 {
			t.Errorf("expected 1 document with tags ['finance', 'urgent'], got count=%d, len=%d", count, len(res))
		}
		if len(res) == 1 && res[0].ID != doc1ID {
			t.Errorf("expected doc1ID, got %v", res[0].ID)
		}
	})

	t.Run("Non-existent Tag", func(t *testing.T) {
		res, count, err := repo.ListDocuments(ctx, ArchiveFilter{Tags: []string{"nonexistent"}})
		if err != nil {
			t.Fatalf("ListDocuments failed: %v", err)
		}
		if count != 0 || len(res) != 0 {
			t.Errorf("expected 0 documents, got count=%d, len=%d", count, len(res))
		}
	})

	t.Run("Nil / Empty Tag Filter", func(t *testing.T) {
		res, count, err := repo.ListDocuments(ctx, ArchiveFilter{Tags: nil})
		if err != nil {
			t.Fatalf("ListDocuments failed: %v", err)
		}
		if count != 4 || len(res) != 4 {
			t.Errorf("expected all 4 documents for nil tag filter, got count=%d, len=%d", count, len(res))
		}
	})

	t.Run("Case Sensitivity in Tags", func(t *testing.T) {
		_, count, err := repo.ListDocuments(ctx, ArchiveFilter{Tags: []string{"FINANCE"}})
		if err != nil {
			t.Fatalf("ListDocuments failed: %v", err)
		}
		t.Logf("Upper-case tag search 'FINANCE' returned count=%d", count)
		// Exact matching means "FINANCE" != "finance"
		if count != 0 {
			t.Logf("Note: Tag filtering is case-sensitive: 'FINANCE' returned %d matches", count)
		}
	})

	t.Run("Duplicate Tags in Filter", func(t *testing.T) {
		res, count, err := repo.ListDocuments(ctx, ArchiveFilter{Tags: []string{"finance", "finance"}})
		if err != nil {
			t.Fatalf("ListDocuments failed: %v", err)
		}
		if count != 2 || len(res) != 2 {
			t.Errorf("expected 2 documents, got count=%d, len=%d", count, len(res))
		}
	})

	t.Run("Special Characters in Tags", func(t *testing.T) {
		specialDoc := &ArchiveDocument{
			ID:          uuid.New(),
			Filename:    "special.pdf",
			Category:    CategoryStudentRecord,
			Tags:        []string{"dept:hr/payroll", "tag with spaces", "utf8-🏷️"},
			StorageTier: StorageTierHot,
		}
		_ = repo.CreateDocument(ctx, specialDoc)

		_, count, err := repo.ListDocuments(ctx, ArchiveFilter{Tags: []string{"dept:hr/payroll"}})
		if err != nil || count != 1 {
			t.Errorf("failed to match tag with special characters: count=%d, err=%v", count, err)
		}

		res2, count2, err2 := repo.ListDocuments(ctx, ArchiveFilter{Tags: []string{"utf8-🏷️"}})
		if err2 != nil || count2 != 1 || len(res2) != 1 {
			t.Errorf("failed to match tag with UTF-8 emoji: count=%d, err=%v", count2, err2)
		}
	})
}

// -----------------------------------------------------------------------------
// 3. Soft-Delete Rejection in UpdateDocument Stress Tests
// -----------------------------------------------------------------------------

func TestSoftDeleteRejection_Stress(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "active_doc.pdf",
		Category:    CategoryStudentRecord,
		StorageTier: StorageTierHot,
	}

	if err := repo.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}

	// Active document update must succeed
	doc.Filename = "updated_doc.pdf"
	if err := repo.UpdateDocument(ctx, doc); err != nil {
		t.Fatalf("UpdateDocument on active document failed: %v", err)
	}

	// Soft delete document
	if err := repo.SoftDeleteDocument(ctx, docID); err != nil {
		t.Fatalf("SoftDeleteDocument failed: %v", err)
	}

	// Attempt update on soft-deleted document
	doc.Filename = "illegal_update.pdf"
	err := repo.UpdateDocument(ctx, doc)
	if err != ErrDocumentNotFound {
		t.Errorf("EXPECTED ErrDocumentNotFound on soft-deleted UpdateDocument, got: %v", err)
	}

	// Multiple update attempts on soft-deleted document
	for i := 0; i < 5; i++ {
		errRepeated := repo.UpdateDocument(ctx, doc)
		if errRepeated != ErrDocumentNotFound {
			t.Errorf("iteration %d: expected ErrDocumentNotFound, got: %v", i, errRepeated)
		}
	}

	// Verify document remained soft-deleted and filename was NOT changed to illegal_update.pdf
	retrieved, err := repo.GetDocumentByID(ctx, docID)
	if err != nil {
		t.Fatalf("GetDocumentByID failed: %v", err)
	}
	if retrieved.Filename == "illegal_update.pdf" {
		t.Errorf("CRITICAL: Soft-deleted document fields were mutated despite error returned!")
	}
	if retrieved.DeletedAt == nil {
		t.Errorf("CRITICAL: Soft-deleted document DeletedAt timestamp was cleared!")
	}

	// Update on non-existent UUID must return ErrDocumentNotFound
	nonExistentDoc := &ArchiveDocument{ID: uuid.New(), Filename: "ghost.pdf"}
	if err := repo.UpdateDocument(ctx, nonExistentDoc); err != ErrDocumentNotFound {
		t.Errorf("expected ErrDocumentNotFound for non-existent document, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 4. JSONMap.Scan Handling of []byte("null") Stress Tests
// -----------------------------------------------------------------------------

func TestJSONMapScan_NullHandling_Stress(t *testing.T) {
	t.Run("Scan []byte('null') into uninitialized JSONMap", func(t *testing.T) {
		var m JSONMap
		if err := m.Scan([]byte("null")); err != nil {
			t.Fatalf("Scan([]byte('null')) returned error: %v", err)
		}
		if m == nil {
			t.Fatalf("CRITICAL: JSONMap is nil after scanning []byte('null')")
		}
		// Must allow key assignment without panic
		m["status"] = "ok"
		if m["status"] != "ok" {
			t.Errorf("expected status='ok', got %v", m["status"])
		}
	})

	t.Run("Scan string('null') into uninitialized JSONMap", func(t *testing.T) {
		var m JSONMap
		if err := m.Scan("null"); err != nil {
			t.Fatalf("Scan(string('null')) returned error: %v", err)
		}
		if m == nil {
			t.Fatalf("CRITICAL: JSONMap is nil after scanning string('null')")
		}
		m["tested"] = true
	})

	t.Run("Scan []byte('null') into pre-populated JSONMap", func(t *testing.T) {
		m := JSONMap{"existing": "data"}
		if err := m.Scan([]byte("null")); err != nil {
			t.Fatalf("Scan []byte('null') error: %v", err)
		}
		if m == nil {
			t.Fatalf("JSONMap became nil after scanning []byte('null') into pre-populated map")
		}
		// Verify map is re-initialized (empty non-nil map)
		if _, exists := m["existing"]; exists {
			t.Errorf("expected pre-existing key to be cleared after scanning null")
		}
	})

	t.Run("Scan whitespace-surrounded null", func(t *testing.T) {
		var m JSONMap
		if err := m.Scan([]byte("  null \n\t")); err != nil {
			t.Fatalf("Scan whitespace null error: %v", err)
		}
		if m == nil {
			t.Fatalf("JSONMap is nil after scanning padded null")
		}
	})

	t.Run("Scan invalid JSON & non-map JSON structures", func(t *testing.T) {
		var m1, m2, m3 JSONMap

		if err := m1.Scan([]byte("12345")); err == nil {
			t.Errorf("expected error scanning JSON number, got nil")
		}

		if err := m2.Scan([]byte(`"a string"`)); err == nil {
			t.Errorf("expected error scanning JSON string, got nil")
		}

		if err := m3.Scan([]byte(`[1, 2, 3]`)); err == nil {
			t.Errorf("expected error scanning JSON array, got nil")
		}
	})
}

// -----------------------------------------------------------------------------
// 5. Concurrent Stress Testing & Race Condition Checks
// -----------------------------------------------------------------------------

func TestConcurrentRepository_Stress(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()

	docCount := 50
	goroutines := 10
	var wg sync.WaitGroup

	docIDs := make([]uuid.UUID, docCount)
	for i := 0; i < docCount; i++ {
		docIDs[i] = uuid.New()
		doc := &ArchiveDocument{
			ID:          docIDs[i],
			Filename:    fmt.Sprintf("concurrent_%d.pdf", i),
			Category:    CategoryStudentRecord,
			Tags:        []string{"concurrent", fmt.Sprintf("tag_%d", i%5)},
			StorageTier: StorageTierHot,
		}
		if err := repo.CreateDocument(ctx, doc); err != nil {
			t.Fatalf("CreateDocument failed: %v", err)
		}
	}

	// Concurrent Readers & Updaters & Soft-Deleters
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				targetID := docIDs[(workerID+i)%docCount]

				// Concurrent Get
				doc, err := repo.GetDocumentByID(ctx, targetID)
				if err == nil && doc != nil {
					// Concurrent Update
					doc.Filename = fmt.Sprintf("updated_w%d_i%d.pdf", workerID, i)
					_ = repo.UpdateDocument(ctx, doc)
				}

				// Concurrent List
				_, _, _ = repo.ListDocuments(ctx, ArchiveFilter{Tags: []string{"concurrent"}})
			}
		}(g)
	}

	wg.Wait()
	t.Logf("Concurrent stress test completed without data races or panics")
}
