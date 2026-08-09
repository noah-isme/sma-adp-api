package archives

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageTierMigrator_HotToWarm(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	migrator := NewStorageTierMigrator(repo, searchEngine)

	docID := uuid.New()
	uploadedAt := time.Now().UTC().AddDate(0, 0, -95) // 95 days ago

	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "old_student_record.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierHot,
		Category:    CategoryStudentRecord,
		UploadedAt:  uploadedAt,
		RetainUntil: time.Now().UTC().AddDate(5, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	migrated, err := migrator.MigrateStorageTiers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, migrated)

	updatedDoc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierWarm, updatedDoc.StorageTier)

	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, AuditActionTierMigration, logs[0].Action)
	assert.Equal(t, "HOT", logs[0].Details["from_tier"])
	assert.Equal(t, "WARM", logs[0].Details["to_tier"])
}

func TestStorageTierMigrator_WarmToCold(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	migrator := NewStorageTierMigrator(repo, searchEngine)

	docID := uuid.New()
	uploadedAt := time.Now().UTC().AddDate(0, 0, -740) // > 2 years ago

	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "ancient_record.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierWarm,
		Category:    CategoryStudentRecord,
		UploadedAt:  uploadedAt,
		RetainUntil: time.Now().UTC().AddDate(5, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	migrated, err := migrator.MigrateStorageTiers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, migrated)

	updatedDoc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierCold, updatedDoc.StorageTier)

	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, AuditActionTierMigration, logs[0].Action)
	assert.Equal(t, "WARM", logs[0].Details["from_tier"])
	assert.Equal(t, "COLD", logs[0].Details["to_tier"])
}

func TestStorageTierMigrator_HotToColdDirect(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	migrator := NewStorageTierMigrator(repo, searchEngine)

	docID := uuid.New()
	uploadedAt := time.Now().UTC().AddDate(0, 0, -800) // > 2 years ago, still HOT

	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "direct_jump.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierHot,
		Category:    CategoryStudentRecord,
		UploadedAt:  uploadedAt,
		RetainUntil: time.Now().UTC().AddDate(5, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	migrated, err := migrator.MigrateStorageTiers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, migrated)

	updatedDoc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierCold, updatedDoc.StorageTier)

	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, AuditActionTierMigration, logs[0].Action)
	assert.Equal(t, "HOT", logs[0].Details["from_tier"])
	assert.Equal(t, "COLD", logs[0].Details["to_tier"])
}

func TestStorageTierMigrator_RecentNoMigration(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	migrator := NewStorageTierMigrator(repo, searchEngine)

	docID := uuid.New()
	uploadedAt := time.Now().UTC().AddDate(0, 0, -10) // 10 days ago

	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "fresh_doc.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierHot,
		Category:    CategoryStudentRecord,
		UploadedAt:  uploadedAt,
		RetainUntil: time.Now().UTC().AddDate(5, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	migrated, err := migrator.MigrateStorageTiers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, migrated)

	updatedDoc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierHot, updatedDoc.StorageTier)

	logs, err := repo.GetAuditLogsByDocument(ctx, docID)
	require.NoError(t, err)
	assert.Len(t, logs, 0)
}

func TestStorageTierMigrator_PromoteOnAccess(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	migrator := NewStorageTierMigrator(repo, searchEngine)

	// 1. Promote WARM -> HOT
	warmDocID := uuid.New()
	warmDoc := &ArchiveDocument{
		ID:          warmDocID,
		Filename:    "warm_doc.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierWarm,
		Category:    CategoryStudentRecord,
		UploadedAt:  time.Now().UTC().AddDate(0, 0, -100),
	}
	require.NoError(t, repo.CreateDocument(ctx, warmDoc))

	err := migrator.PromoteOnAccess(ctx, warmDocID)
	require.NoError(t, err)

	updatedWarm, err := repo.GetDocumentByID(ctx, warmDocID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierHot, updatedWarm.StorageTier)

	// 2. Promote COLD -> WARM
	coldDocID := uuid.New()
	coldDoc := &ArchiveDocument{
		ID:          coldDocID,
		Filename:    "cold_doc.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierCold,
		Category:    CategoryStudentRecord,
		UploadedAt:  time.Now().UTC().AddDate(0, 0, -800),
	}
	require.NoError(t, repo.CreateDocument(ctx, coldDoc))

	err = migrator.PromoteOnAccess(ctx, coldDocID)
	require.NoError(t, err)

	updatedCold, err := repo.GetDocumentByID(ctx, coldDocID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierWarm, updatedCold.StorageTier)
}

// FailingSearchEngine is a mock SearchEngine that fails on IndexDocument.
type FailingSearchEngine struct {
	Err error
}

func (f *FailingSearchEngine) IndexDocument(ctx context.Context, doc *ArchiveDocument) error {
	if f.Err != nil {
		return f.Err
	}
	return errors.New("indexing failed: connection timeout")
}

func (f *FailingSearchEngine) DeleteDocumentIndex(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (f *FailingSearchEngine) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	return &SearchResult{}, nil
}

func TestStorageTierManager_Adversarial_ExactBoundary730d1s(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	migrator := NewStorageTierMigrator(repo, searchEngine)

	now := time.Now().UTC()

	// 1. Doc aged 730d + 1s in WARM tier -> MUST migrate to COLD
	doc1ID := uuid.New()
	doc1 := &ArchiveDocument{
		ID:          doc1ID,
		Filename:    "doc_730d_1s_warm.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierWarm,
		Category:    CategoryStudentRecord,
		UploadedAt:  now.Add(-730 * 24 * time.Hour).Add(-1 * time.Second),
		RetainUntil: now.AddDate(5, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc1))

	// 2. Doc aged 730d - 1s in WARM tier -> MUST NOT migrate to COLD (remains WARM)
	doc2ID := uuid.New()
	doc2 := &ArchiveDocument{
		ID:          doc2ID,
		Filename:    "doc_730d_minus_1s_warm.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierWarm,
		Category:    CategoryStudentRecord,
		UploadedAt:  now.Add(-730 * 24 * time.Hour).Add(1 * time.Second),
		RetainUntil: now.AddDate(5, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc2))

	// 3. Doc aged 730d + 1s in HOT tier -> MUST migrate directly to COLD
	doc3ID := uuid.New()
	doc3 := &ArchiveDocument{
		ID:          doc3ID,
		Filename:    "doc_730d_1s_hot.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierHot,
		Category:    CategoryStudentRecord,
		UploadedAt:  now.Add(-730 * 24 * time.Hour).Add(-1 * time.Second),
		RetainUntil: now.AddDate(5, 0, 0),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc3))

	migrated, err := migrator.MigrateStorageTiers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, migrated, "Doc1 (WARM->COLD) and Doc3 (HOT->COLD) should migrate")

	updatedDoc1, err := repo.GetDocumentByID(ctx, doc1ID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierCold, updatedDoc1.StorageTier, "730d 1s old document in WARM tier must migrate to COLD")

	updatedDoc2, err := repo.GetDocumentByID(ctx, doc2ID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierWarm, updatedDoc2.StorageTier, "730d - 1s document must remain WARM")

	updatedDoc3, err := repo.GetDocumentByID(ctx, doc3ID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierCold, updatedDoc3.StorageTier, "730d 1s old document in HOT tier must migrate to COLD")
}

func TestStorageTierManager_Adversarial_ConcurrentPromoteOnAccess(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	searchEngine := NewMemorySearchEngine()
	migrator := NewStorageTierMigrator(repo, searchEngine)

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "concurrent_access.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierWarm,
		Category:    CategoryStudentRecord,
		UploadedAt:  time.Now().UTC().AddDate(0, 0, -100),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	const numGoroutines = 30
	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := migrator.PromoteOnAccess(ctx, docID)
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		assert.NoError(t, err, "PromoteOnAccess should succeed under concurrent calls")
	}

	updatedDoc, err := repo.GetDocumentByID(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, StorageTierHot, updatedDoc.StorageTier, "Document tier should be promoted to HOT")
}

func TestStorageTierManager_Adversarial_SearchEngineErrorHandling(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	failingSE := &FailingSearchEngine{Err: fmt.Errorf("meilisearch unreachable")}
	migrator := NewStorageTierMigrator(repo, failingSE)

	docID := uuid.New()
	doc := &ArchiveDocument{
		ID:          docID,
		Filename:    "indexing_failure.pdf",
		MimeType:    "application/pdf",
		StorageTier: StorageTierWarm,
		Category:    CategoryStudentRecord,
		UploadedAt:  time.Now().UTC().AddDate(0, 0, -100),
	}
	require.NoError(t, repo.CreateDocument(ctx, doc))

	// 1. Direct MigrateDocumentTier call with search engine error
	err := migrator.MigrateDocumentTier(ctx, doc, StorageTierCold)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "re-index document tier in search engine")

	// 2. PromoteOnAccess call with search engine error
	err = migrator.PromoteOnAccess(ctx, docID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "re-index document tier in search engine")
}

