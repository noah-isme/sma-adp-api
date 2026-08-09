package archives

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// -----------------------------------------------------------------------------
// 1. REPOSITORY QUERY BOUNDARY CONDITIONS & EMPTY FILTERS & LARGE PAGE SIZES
// -----------------------------------------------------------------------------

func TestRepository_QueryBoundaryConditions(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()

	// Seed multiple test documents
	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := 1; i <= 20; i++ {
		doc := &ArchiveDocument{
			ID:               uuid.New(),
			Filename:         fmt.Sprintf("doc_%02d.pdf", i),
			OriginalFilename: fmt.Sprintf("orig_%02d.pdf", i),
			MimeType:         "application/pdf",
			SizeBytes:        int64(i * 100),
			Checksum:         fmt.Sprintf("hash_%d", i),
			StoragePath:      fmt.Sprintf("/path/%d", i),
			StorageTier:      StorageTierHot,
			Category:         CategoryStudentRecord,
			Tags:             []string{fmt.Sprintf("tag%d", i), "common"},
			Metadata:         JSONMap{"index": i},
			OCRStatus:        OCRStatusCompleted,
			OCRText:          fmt.Sprintf("Extracted content for document number %d", i),
			RetainUntil:      baseTime.AddDate(1, 0, 0),
			UploadedBy:       uuid.New(),
			UploadedAt:       baseTime.Add(time.Duration(i) * time.Hour),
		}
		if i%2 == 0 {
			doc.Category = CategoryGradeReport
			doc.StorageTier = StorageTierWarm
			doc.LegalHold = true
			doc.LegalHoldReason = "Audit"
		}
		if i%5 == 0 {
			doc.OCRStatus = OCRStatusPending
			doc.OCRText = ""
		}
		if err := repo.CreateDocument(ctx, doc); err != nil {
			t.Fatalf("failed to seed document %d: %v", i, err)
		}
	}

	t.Run("Empty Filter (ArchiveFilter{})", func(t *testing.T) {
		docs, total, err := repo.ListDocuments(ctx, ArchiveFilter{})
		if err != nil {
			t.Fatalf("ListDocuments(EmptyFilter) returned error: %v", err)
		}
		if total != 20 {
			t.Errorf("expected total 20, got %d", total)
		}
		if len(docs) != 20 {
			t.Errorf("expected 20 documents returned, got %d", len(docs))
		}
	})

	t.Run("Query Filter - Matching & Case Insensitivity", func(t *testing.T) {
		filter := ArchiveFilter{Query: "DOC_01"}
		docs, total, err := repo.ListDocuments(ctx, filter)
		if err != nil {
			t.Fatalf("ListDocuments(Query) error: %v", err)
		}
		if total != 1 || len(docs) != 1 {
			t.Errorf("expected 1 match for DOC_01, got total=%d, len=%d", total, len(docs))
		}
		if len(docs) > 0 && docs[0].Filename != "doc_01.pdf" {
			t.Errorf("expected filename doc_01.pdf, got %s", docs[0].Filename)
		}
	})

	t.Run("Query Filter - Matching OCR Text", func(t *testing.T) {
		filter := ArchiveFilter{Query: "content for document number 3"}
		docs, total, err := repo.ListDocuments(ctx, filter)
		if err != nil {
			t.Fatalf("ListDocuments(OCR query) error: %v", err)
		}
		if total != 1 || len(docs) != 1 {
			t.Errorf("expected 1 match for OCR query, got total=%d, len=%d", total, len(docs))
		}
	})

	t.Run("Query Filter - SQL Injection & Special Characters", func(t *testing.T) {
		specialQueries := []string{
			"' OR '1'='1",
			"'; DROP TABLE archive_documents; --",
			"%pdf%",
			"\\",
			"\" OR \"a\"=\"a",
			"🎉 unicode search 🚀",
		}
		for _, sq := range specialQueries {
			filter := ArchiveFilter{Query: sq}
			docs, total, err := repo.ListDocuments(ctx, filter)
			if err != nil {
				t.Errorf("ListDocuments query '%s' caused unexpected error: %v", sq, err)
			}
			// None of these match our seeded documents, but must execute safely without crashing
			t.Logf("Query '%s' returned total=%d docs=%d", sq, total, len(docs))
		}
	})

	t.Run("Category Filter", func(t *testing.T) {
		filter := ArchiveFilter{Category: CategoryGradeReport}
		docs, total, err := repo.ListDocuments(ctx, filter)
		if err != nil {
			t.Fatalf("ListDocuments(Category) error: %v", err)
		}
		if total != 10 || len(docs) != 10 {
			t.Errorf("expected 10 GRADE_REPORT docs, got total=%d, len=%d", total, len(docs))
		}
	})

	t.Run("StorageTier Filter", func(t *testing.T) {
		filter := ArchiveFilter{StorageTier: StorageTierWarm}
		docs, total, err := repo.ListDocuments(ctx, filter)
		if err != nil {
			t.Fatalf("ListDocuments(StorageTier) error: %v", err)
		}
		if total != 10 || len(docs) != 10 {
			t.Errorf("expected 10 WARM docs, got total=%d, len=%d", total, len(docs))
		}
	})

	t.Run("LegalHoldOnly Filter", func(t *testing.T) {
		filter := ArchiveFilter{LegalHoldOnly: true}
		docs, total, err := repo.ListDocuments(ctx, filter)
		if err != nil {
			t.Fatalf("ListDocuments(LegalHoldOnly) error: %v", err)
		}
		if total != 10 || len(docs) != 10 {
			t.Errorf("expected 10 LegalHold docs, got total=%d, len=%d", total, len(docs))
		}
	})

	t.Run("OCRCompleted Filter", func(t *testing.T) {
		filter := ArchiveFilter{OCRCompleted: true}
		docs, total, err := repo.ListDocuments(ctx, filter)
		if err != nil {
			t.Fatalf("ListDocuments(OCRCompleted) error: %v", err)
		}
		// 20 total, i%5==0 (4 docs) pending OCR => 16 completed
		if total != 16 || len(docs) != 16 {
			t.Errorf("expected 16 OCRCompleted docs, got total=%d, len=%d", total, len(docs))
		}
	})

	t.Run("MemoryRepository Filter Feature Support Analysis (Tags & Dates & Pagination)", func(t *testing.T) {
		// MemoryRepository implementation check:
		// 1. Tags filtering:
		tagFilter := ArchiveFilter{Tags: []string{"tag1"}}
		docs, _, err := repo.ListDocuments(ctx, tagFilter)
		if err != nil {
			t.Fatalf("ListDocuments(Tags) error: %v", err)
		}
		// MemoryRepository does NOT filter by tags, so it returns all 20 docs.
		if len(docs) != 20 {
			t.Logf("MemoryRepository filtered by tags: len=%d", len(docs))
		} else {
			t.Logf("DISCREPANCY: MemoryRepository ignores ArchiveFilter.Tags (returned %d docs instead of 1)", len(docs))
		}

		// 2. DateFrom / DateTo filtering:
		from := baseTime.Add(5 * time.Hour)
		dateFilter := ArchiveFilter{DateFrom: &from}
		docs, _, err = repo.ListDocuments(ctx, dateFilter)
		if err != nil {
			t.Fatalf("ListDocuments(DateFrom) error: %v", err)
		}
		if len(docs) != 20 {
			t.Logf("MemoryRepository filtered by DateFrom: len=%d", len(docs))
		} else {
			t.Logf("DISCREPANCY: MemoryRepository ignores ArchiveFilter.DateFrom/DateTo (returned %d docs)", len(docs))
		}

		// 3. Pagination (Limit / Offset):
		pageFilter := ArchiveFilter{Limit: 5, Offset: 0}
		docs, _, err = repo.ListDocuments(ctx, pageFilter)
		if err != nil {
			t.Fatalf("ListDocuments(Limit/Offset) error: %v", err)
		}
		if len(docs) != 20 {
			t.Logf("MemoryRepository applied Limit: len=%d", len(docs))
		} else {
			t.Logf("DISCREPANCY: MemoryRepository ignores ArchiveFilter.Limit/Offset (returned %d docs instead of 5)", len(docs))
		}
	})

	t.Run("Large Page Size & Out-of-Bounds Pagination", func(t *testing.T) {
		extremeFilters := []ArchiveFilter{
			{Limit: 1000000},
			{Limit: math.MaxInt32},
			{Limit: -1},
			{Offset: 1000000},
			{Offset: -100},
			{Limit: 0, Offset: 0},
		}
		for idx, ef := range extremeFilters {
			docs, total, err := repo.ListDocuments(ctx, ef)
			if err != nil {
				t.Errorf("Extreme filter #%d (limit=%d, offset=%d) returned error: %v", idx, ef.Limit, ef.Offset, err)
			} else {
				t.Logf("Extreme filter #%d succeeded: total=%d, returned_docs=%d", idx, total, len(docs))
			}
		}
	})
}

// -----------------------------------------------------------------------------
// 2. UUID GENERATION STRESS TEST & BOUNDARY CONDITIONS
// -----------------------------------------------------------------------------

func TestUUIDGeneration_Stress(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()

	t.Run("Auto-generation of nil UUIDs in CreateDocument", func(t *testing.T) {
		doc := &ArchiveDocument{
			ID:               uuid.Nil, // Should be auto-populated
			Filename:         "auto_uuid.pdf",
			OriginalFilename: "auto_uuid.pdf",
			MimeType:         "application/pdf",
			SizeBytes:        100,
			Category:         CategoryOther,
		}
		if err := repo.CreateDocument(ctx, doc); err != nil {
			t.Fatalf("CreateDocument failed: %v", err)
		}
		if doc.ID == uuid.Nil {
			t.Errorf("CreateDocument failed to assign a new UUID when doc.ID was uuid.Nil")
		}
		if doc.ID.Version() != 4 {
			t.Errorf("expected UUID version 4, got version %d", doc.ID.Version())
		}

		// Verify retrieval works with the auto-generated UUID
		retrieved, err := repo.GetDocumentByID(ctx, doc.ID)
		if err != nil {
			t.Fatalf("GetDocumentByID failed for auto-generated UUID: %v", err)
		}
		if retrieved.ID != doc.ID {
			t.Errorf("retrieved ID %v != expected ID %v", retrieved.ID, doc.ID)
		}
	})

	t.Run("Auto-generation of nil UUIDs in CreateRetentionPolicy and CreateAuditLog", func(t *testing.T) {
		policy := &RetentionPolicy{
			ID:       uuid.Nil,
			Name:     "Test Auto UUID Policy",
			Category: CategoryOther,
		}
		if err := repo.CreateRetentionPolicy(ctx, policy); err != nil {
			t.Fatalf("CreateRetentionPolicy failed: %v", err)
		}
		if policy.ID == uuid.Nil {
			t.Errorf("CreateRetentionPolicy failed to generate UUID")
		}

		logEntry := &AuditLog{
			ID:         uuid.Nil,
			DocumentID: &policy.ID,
			Action:     "TEST",
		}
		if err := repo.CreateAuditLog(ctx, logEntry); err != nil {
			t.Fatalf("CreateAuditLog failed: %v", err)
		}
		if logEntry.ID == uuid.Nil {
			t.Errorf("CreateAuditLog failed to generate UUID")
		}
	})

	t.Run("UUID Collision & Uniqueness Stress Test (10,000 UUIDs)", func(t *testing.T) {
		const count = 10000
		seen := make(map[uuid.UUID]bool, count)
		for i := 0; i < count; i++ {
			id := uuid.New()
			if id == uuid.Nil {
				t.Fatalf("uuid.New() returned uuid.Nil at iteration %d", i)
			}
			if seen[id] {
				t.Fatalf("UUID collision detected at iteration %d: %s", i, id.String())
			}
			if id.Version() != 4 {
				t.Fatalf("UUID version is not 4 at iteration %d: %d", i, id.Version())
			}
			seen[id] = true
		}
		if len(seen) != count {
			t.Errorf("expected %d unique UUIDs, got %d", count, len(seen))
		}
	})

	t.Run("GetDocumentByID with uuid.Nil and Non-existent UUID", func(t *testing.T) {
		_, err := repo.GetDocumentByID(ctx, uuid.Nil)
		if err != ErrDocumentNotFound {
			t.Errorf("expected ErrDocumentNotFound for uuid.Nil, got %v", err)
		}

		randomID := uuid.New()
		_, err = repo.GetDocumentByID(ctx, randomID)
		if err != ErrDocumentNotFound {
			t.Errorf("expected ErrDocumentNotFound for random ID, got %v", err)
		}
	})
}

// -----------------------------------------------------------------------------
// 3. MODEL JSON SERIALIZATION & DESERIALIZATION STRESS TESTS
// -----------------------------------------------------------------------------

func TestModel_JSONSerialization(t *testing.T) {
	t.Run("JSONMap Null & Nil Edge Cases", func(t *testing.T) {
		var nilMap JSONMap
		val, err := nilMap.Value()
		if err != nil {
			t.Fatalf("nilMap.Value() error: %v", err)
		}
		if val != "{}" {
			t.Errorf("expected '{}' for nil map Value(), got %v", val)
		}

		emptyMap := JSONMap{}
		val, err = emptyMap.Value()
		if err != nil {
			t.Fatalf("emptyMap.Value() error: %v", err)
		}
		valStr := string(val.([]byte))
		if valStr != "{}" {
			t.Errorf("expected '{}' for empty map Value(), got %s", valStr)
		}
	})

	t.Run("JSONMap Scan Input Types & Invalid Format", func(t *testing.T) {
		var m JSONMap

		// Scan nil
		if err := m.Scan(nil); err != nil {
			t.Errorf("Scan(nil) failed: %v", err)
		}
		if m == nil {
			t.Errorf("Scan(nil) resulted in nil map")
		}

		// Scan empty string
		if err := m.Scan(""); err != nil {
			t.Errorf("Scan(\"\") failed: %v", err)
		}

		// Scan empty byte slice
		if err := m.Scan([]byte("")); err != nil {
			t.Errorf("Scan([]byte{}) failed: %v", err)
		}

		// Scan malformed JSON bytes
		err := m.Scan([]byte("{ invalid json : "))
		if err == nil {
			t.Errorf("expected error scanning invalid JSON, got nil")
		}

		// Scan unsupported type (struct/int)
		err = m.Scan(struct{ A string }{"test"})
		if err == nil {
			t.Errorf("expected error scanning struct, got nil")
		}
	})

	t.Run("JSONMap Complex Nested Data Round-trip", func(t *testing.T) {
		complexData := JSONMap{
			"string": "hello world",
			"number": float64(123.456),
			"bool":   true,
			"null":   nil,
			"array":  []interface{}{"a", float64(1), false},
			"nested": map[string]interface{}{
				"deep": "value",
			},
		}

		val, err := complexData.Value()
		if err != nil {
			t.Fatalf("Value() error: %v", err)
		}

		var restored JSONMap
		if err := restored.Scan(val); err != nil {
			t.Fatalf("Scan() error: %v", err)
		}

		if restored["string"] != "hello world" {
			t.Errorf("string mismatch: %v", restored["string"])
		}
		if restored["number"] != float64(123.456) {
			t.Errorf("number mismatch: %v", restored["number"])
		}
		if restored["bool"] != true {
			t.Errorf("bool mismatch: %v", restored["bool"])
		}
		if restored["null"] != nil {
			t.Errorf("null mismatch: %v", restored["null"])
		}
	})

	t.Run("ArchiveDocument Full JSON Round-trip Serialization", func(t *testing.T) {
		docID := uuid.New()
		policyID := uuid.New()
		userID := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond)

		doc := ArchiveDocument{
			ID:                docID,
			Filename:          "test_archive.pdf",
			OriginalFilename:  "original_archive.pdf",
			MimeType:          "application/pdf",
			SizeBytes:         1048576,
			Checksum:          "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			StoragePath:       "/var/storage/2026/test_archive.pdf",
			StorageTier:       StorageTierHot,
			Category:          CategoryStudentRecord,
			Tags:              []string{"student", "transcript", "2026"},
			Metadata:          JSONMap{"gpa": float64(3.8), "honors": true},
			OCRText:           "This document certifies that the student completed graduation requirements.",
			OCRStatus:         OCRStatusCompleted,
			RetentionPolicyID: &policyID,
			RetainUntil:       now.AddDate(7, 0, 0),
			LegalHold:         true,
			LegalHoldReason:   "Pending court subpoena 2026-X",
			UploadedBy:        userID,
			UploadedAt:        now,
			UpdatedAt:         now,
			DeletedAt:         &now,
		}

		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("json.Marshal(ArchiveDocument) failed: %v", err)
		}

		// Verify expected JSON field keys exist
		jsonStr := string(data)
		expectedKeys := []string{
			`"id"`, `"filename"`, `"originalFilename"`, `"mimeType"`, `"sizeBytes"`,
			`"checksum"`, `"storagePath"`, `"storageTier"`, `"category"`, `"tags"`,
			`"metadata"`, `"ocrText"`, `"ocrStatus"`, `"retentionPolicyId"`, `"retainUntil"`,
			`"legalHold"`, `"legalHoldReason"`, `"uploadedBy"`, `"uploadedAt"`, `"updatedAt"`, `"deletedAt"`,
		}
		for _, k := range expectedKeys {
			if !strings.Contains(jsonStr, k) {
				t.Errorf("JSON output missing expected key: %s in %s", k, jsonStr)
			}
		}

		var restored ArchiveDocument
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("json.Unmarshal(ArchiveDocument) failed: %v", err)
		}

		if restored.ID != docID {
			t.Errorf("ID mismatch: %v vs %v", restored.ID, docID)
		}
		if restored.Filename != doc.Filename {
			t.Errorf("Filename mismatch: %s vs %s", restored.Filename, doc.Filename)
		}
		if restored.LegalHoldReason != doc.LegalHoldReason {
			t.Errorf("LegalHoldReason mismatch: %s vs %s", restored.LegalHoldReason, doc.LegalHoldReason)
		}
		if restored.DeletedAt == nil {
			t.Errorf("DeletedAt should not be nil")
		}
	})

	t.Run("ArchiveDocument Omitempty Field Handling", func(t *testing.T) {
		doc := ArchiveDocument{
			ID:          uuid.New(),
			Filename:    "sparse.pdf",
			MimeType:    "application/pdf",
			OCRText:     "",  // Should be omitted
			LegalHold:   false,
			LegalHoldReason: "", // Should be omitted
			DeletedAt:   nil, // Should be omitted
		}

		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		jsonStr := string(data)
		forbiddenKeys := []string{`"ocrText"`, `"legalHoldReason"`, `"deletedAt"`}
		for _, k := range forbiddenKeys {
			if strings.Contains(jsonStr, k) {
				t.Errorf("JSON output contains omitempty key that should be omitted: %s in %s", k, jsonStr)
			}
		}
	})

	t.Run("DTO and Search Model JSON Serialization", func(t *testing.T) {
		hit := SearchHit{
			ID:               uuid.New(),
			Filename:         "hit.pdf",
			OriginalFilename: "orig_hit.pdf",
			MimeType:         "application/pdf",
			SizeBytes:        2048,
			Checksum:         "hash",
			StorageTier:      StorageTierWarm,
			Category:         CategoryFinancialDoc,
			Tags:             []string{"fin", "tax"},
			Metadata:         JSONMap{"quarter": "Q1"},
			OCRStatus:        OCRStatusCompleted,
			OCRText:          "Invoice content",
			Snippet:          "Invoice <b>content</b> snippet",
			RetainUntil:      time.Now().UTC(),
			LegalHold:        false,
			UploadedBy:       uuid.New(),
			UploadedAt:       time.Now().UTC(),
		}

		result := SearchResult{
			Data:        []SearchHit{hit},
			Total:       1,
			Page:        1,
			Limit:       50,
			QueryTimeMs: 12,
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("json.Marshal(SearchResult) failed: %v", err)
		}

		var restored SearchResult
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("json.Unmarshal(SearchResult) failed: %v", err)
		}

		if restored.Total != 1 || len(restored.Data) != 1 {
			t.Fatalf("SearchResult fields corrupted in JSON round-trip")
		}
		if restored.Data[0].Snippet != "Invoice <b>content</b> snippet" {
			t.Errorf("SearchHit Snippet corrupted: %s", restored.Data[0].Snippet)
		}
	})
}

// -----------------------------------------------------------------------------
// 4. EMPIRICAL CHALLENGES & SECURITY / EDGE CASE HARNESSES
// -----------------------------------------------------------------------------

func TestSignedURL_SecurityBypass_Empirical(t *testing.T) {
	signer := NewHMACSignedURLSigner("test_secret", "/api/v1/archives")
	docID := uuid.New()
	boundIP := "192.168.1.100"

	// Generate a URL restricted to boundIP (192.168.1.100)
	signedURL, err := signer.GenerateSignedURL(docID, "sensitive.pdf", boundIP, 10*time.Minute)
	if err != nil {
		t.Fatalf("GenerateSignedURL failed: %v", err)
	}

	// Extract token from URL string
	parts := strings.Split(signedURL, "token=")
	if len(parts) != 2 {
		t.Fatalf("invalid signed URL format: %s", signedURL)
	}
	token := parts[1]

	// Case A: Correct IP -> Must succeed
	id, err := signer.ValidateSignedURLToken(token, boundIP)
	if err != nil || id != docID {
		t.Errorf("validating token with correct bound IP failed: err=%v, id=%v", err, id)
	}

	// Case B: Different IP -> Must fail with ErrIPMismatch
	attackerIP := "10.0.0.5"
	_, errMismatch := signer.ValidateSignedURLToken(token, attackerIP)
	if errMismatch != ErrIPMismatch {
		t.Errorf("expected ErrIPMismatch for attacker IP %s, got %v", attackerIP, errMismatch)
	}

	// Case C: Empty Client IP -> SECURITY VULNERABILITY CHECK
	// If caller passes empty string "" as clientIP, does it bypass the bound IP check?
	validIDWithEmptyIP, errEmptyIP := signer.ValidateSignedURLToken(token, "")
	if errEmptyIP == nil {
		t.Errorf("SECURITY VULNERABILITY DETECTED: Passing empty clientIP bypasses IP binding restriction on signed URL! Granted docID=%v", validIDWithEmptyIP)
	} else {
		t.Logf("Empty clientIP correctly rejected: %v", errEmptyIP)
	}
}

func TestSearchEngine_UnicodeExtractSnippet_Panic(t *testing.T) {
	engine := NewMemorySearchEngine()
	ctx := context.Background()

	// Turkish capital İ transforms to i\u0307 (2 bytes -> 3 bytes in ToLower)
	doc := &ArchiveDocument{
		ID:               uuid.New(),
		Filename:         "İSTANBUL_TRANSCRIPT.pdf",
		OriginalFilename: "İSTANBUL_TRANSCRIPT.pdf",
		MimeType:         "application/pdf",
		OCRStatus:        OCRStatusCompleted,
		OCRText:          "Student transcript issued in İSTANBUL university campus",
		StorageTier:      StorageTierHot,
		Category:         CategoryStudentRecord,
	}

	if err := engine.IndexDocument(ctx, doc); err != nil {
		t.Fatalf("IndexDocument failed: %v", err)
	}

	// Search query matching "istanbul"
	req := SearchRequest{
		Query:    "istanbul",
		Category: CategoryStudentRecord,
		Page:     1,
		Limit:    10,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("CRITICAL PANIC DETECTED in extractSnippet during Unicode search: %v", r)
		}
	}()

	res, err := engine.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Unicode search completed safely: total hits=%d", res.Total)
}

func TestGDPRRequest_AuditLog_And_Types(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewArchiveService(repo, nil, nil, nil, nil)

	docID := uuid.New()
	userID := uuid.New()

	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "gdpr_test.pdf",
		Category:    CategoryStudentRecord,
		UploadedBy:  userID,
		RetainUntil: time.Now().AddDate(-1, 0, 0), // Expired retention
	}
	_ = repo.CreateDocument(ctx, doc)

	// Process GDPR ERASURE request
	err := service.HandleGDPRRequest(ctx, "ERASURE", docID)
	if err != nil {
		t.Fatalf("GDPR ERASURE failed: %v", err)
	}

	// Check if audit log was created for GDPR request
	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	if err != nil {
		t.Fatalf("GetAuditLogsByDocument failed: %v", err)
	}

	hasGDPRAudit := false
	for _, log := range logs {
		if log.Action == AuditActionGDPRRequest || strings.Contains(log.Action, "GDPR") || log.Action == "DELETE" {
			hasGDPRAudit = true
			break
		}
	}

	if !hasGDPRAudit {
		t.Logf("DISCREPANCY DETECTED: HandleGDPRRequest executed ERASURE soft-delete but created 0 audit log entries")
	}
}

func TestOCRWorkerPool_StoppedState_Enqueue(t *testing.T) {
	repo := NewMemoryRepository()
	pool := NewGoOCRWorkerPool(1, 10, repo, nil)

	pool.Start()
	pool.Stop()

	// Enqueue after Stop()
	err := pool.Enqueue(uuid.New(), "/tmp/dummy.pdf", "application/pdf")
	if err == nil {
		t.Logf("DISCREPANCY: Enqueue on stopped OCRWorkerPool returned nil error, but job will never be processed")
	}
}

