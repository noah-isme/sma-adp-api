package archives

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRepositoryInterfaceCompliance(t *testing.T) {
	var _ Repository = (*PostgresRepository)(nil)
	var _ Repository = (*MemoryRepository)(nil)
}

func TestMemoryRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()

	// 1. Retention Policies Seed Check
	policies, err := repo.ListRetentionPolicies(ctx)
	if err != nil {
		t.Fatalf("ListRetentionPolicies failed: %v", err)
	}
	if len(policies) != 8 {
		t.Errorf("expected 8 pre-seeded retention policies, got %d", len(policies))
	}

	// 2. GetDefaultPolicyByCategory
	pol, err := repo.GetDefaultPolicyByCategory(ctx, CategoryStudentRecord)
	if err != nil {
		t.Fatalf("GetDefaultPolicyByCategory failed: %v", err)
	}
	if pol.Category != CategoryStudentRecord {
		t.Errorf("expected category STUDENT_RECORD, got %s", pol.Category)
	}

	// 3. Create & Get Document
	docID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	doc := &ArchiveDocument{
		ID:                docID,
		Filename:          "test_file.pdf",
		OriginalFilename:  "original_file.pdf",
		MimeType:          "application/pdf",
		SizeBytes:         500,
		Checksum:          "dummychecksum",
		StoragePath:       "/storage/test_file.pdf",
		StorageTier:       StorageTierHot,
		Category:          CategoryStudentRecord,
		Tags:              []string{"test", "doc"},
		Metadata:          JSONMap{"author": "admin"},
		OCRStatus:         OCRStatusPending,
		RetentionPolicyID: &pol.ID,
		RetainUntil:       now.AddDate(1, 0, 0),
		LegalHold:         false,
		UploadedBy:        userID,
		UploadedAt:        now,
	}

	if err := repo.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}

	retrieved, err := repo.GetDocumentByID(ctx, docID)
	if err != nil {
		t.Fatalf("GetDocumentByID failed: %v", err)
	}
	if retrieved.Filename != "test_file.pdf" {
		t.Errorf("expected filename 'test_file.pdf', got %s", retrieved.Filename)
	}

	// 4. ListDocuments with filter
	filter := ArchiveFilter{
		Category: CategoryStudentRecord,
	}
	docs, count, err := repo.ListDocuments(ctx, filter)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if count < 1 || len(docs) < 1 {
		t.Errorf("expected at least 1 document in filter, got count=%d, len=%d", count, len(docs))
	}

	// ListDocuments with Tag Slice Filter
	tagFilterMatch := ArchiveFilter{Tags: []string{"test", "doc"}}
	tagDocsMatch, tagCountMatch, err := repo.ListDocuments(ctx, tagFilterMatch)
	if err != nil || tagCountMatch != 1 || len(tagDocsMatch) != 1 {
		t.Errorf("expected 1 document matching tags, got count=%d, len=%d, err=%v", tagCountMatch, len(tagDocsMatch), err)
	}

	tagFilterNoMatch := ArchiveFilter{Tags: []string{"nonexistent-tag"}}
	tagDocsNoMatch, tagCountNoMatch, err := repo.ListDocuments(ctx, tagFilterNoMatch)
	if err != nil || tagCountNoMatch != 0 || len(tagDocsNoMatch) != 0 {
		t.Errorf("expected 0 documents matching non-existent tag, got count=%d, len=%d", tagCountNoMatch, len(tagDocsNoMatch))
	}

	// 5. Update Document
	retrieved.LegalHold = true
	retrieved.LegalHoldReason = "Audit pending"
	if err := repo.UpdateDocument(ctx, retrieved); err != nil {
		t.Fatalf("UpdateDocument failed: %v", err)
	}

	updated, err := repo.GetDocumentByID(ctx, docID)
	if err != nil {
		t.Fatalf("GetDocumentByID after update failed: %v", err)
	}
	if !updated.LegalHold || updated.LegalHoldReason != "Audit pending" {
		t.Errorf("update failed to set legal hold fields properly")
	}

	// 6. Audit Log Creation and Fetching
	audit := &AuditLog{
		ID:         uuid.New(),
		DocumentID: &docID,
		Action:     AuditActionLegalHold,
		UserID:     userID,
		IPAddress:  "127.0.0.1",
		UserAgent:  "TestAgent/1.0",
		Details:    JSONMap{"reason": "Audit pending"},
		CreatedAt:  time.Now().UTC(),
	}
	if err := repo.CreateAuditLog(ctx, audit); err != nil {
		t.Fatalf("CreateAuditLog failed: %v", err)
	}

	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	if err != nil {
		t.Fatalf("GetAuditLogsByDocument failed: %v", err)
	}
	if len(logs) != 1 || logs[0].Action != AuditActionLegalHold {
		t.Errorf("expected 1 audit log with action %s, got %d logs", AuditActionLegalHold, len(logs))
	}

	// 7. Soft Delete
	if err := repo.SoftDeleteDocument(ctx, docID); err != nil {
		t.Fatalf("SoftDeleteDocument failed: %v", err)
	}
	softDeletedDoc, err := repo.GetDocumentByID(ctx, docID)
	if err != nil {
		t.Fatalf("GetDocumentByID after soft delete returned unexpected error: %v", err)
	}
	if softDeletedDoc.DeletedAt == nil {
		t.Errorf("expected non-nil DeletedAt timestamp after soft delete")
	}

	// Update on soft-deleted document must return ErrDocumentNotFound
	if err := repo.UpdateDocument(ctx, softDeletedDoc); err != ErrDocumentNotFound {
		t.Errorf("expected ErrDocumentNotFound when updating soft-deleted document, got %v", err)
	}

	// ListDocuments without IncludeDeleted should now return 0 active documents
	activeDocs, activeCount, err := repo.ListDocuments(ctx, ArchiveFilter{Category: CategoryStudentRecord, IncludeDeleted: false})
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if activeCount != 0 || len(activeDocs) != 0 {
		t.Errorf("expected 0 active documents after soft delete, got count=%d, len=%d", activeCount, len(activeDocs))
	}
}
