package archives

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// StorageTierManager manages storage tier classification (HOT, WARM, COLD) and migrations based on document age.
type StorageTierManager struct {
	repo         Repository
	searchEngine SearchEngine
	mu           sync.Mutex
}

// StorageTierMigrator is an alias for StorageTierManager.
type StorageTierMigrator = StorageTierManager

// NewStorageTierManager initializes a new StorageTierManager.
func NewStorageTierManager(repo Repository, searchEngine SearchEngine) *StorageTierManager {
	return &StorageTierManager{
		repo:         repo,
		searchEngine: searchEngine,
	}
}

// NewStorageTierMigrator is a constructor alias for NewStorageTierManager.
func NewStorageTierMigrator(repo Repository, searchEngine SearchEngine) *StorageTierMigrator {
	return NewStorageTierManager(repo, searchEngine)
}

// SetSearchEngine sets or updates the search engine instance.
func (m *StorageTierManager) SetSearchEngine(se SearchEngine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.searchEngine = se
}

func (m *StorageTierManager) getSearchEngine() SearchEngine {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.searchEngine
}

func determineTargetTierForDuration(dur time.Duration) StorageTier {
	if dur > 730*24*time.Hour {
		return StorageTierCold
	}
	if dur > 90*24*time.Hour {
		return StorageTierWarm
	}
	return StorageTierHot
}

// EvaluateAndMigrateTiers scans candidate documents in HOT and WARM tiers, performing migrations based on age rules.
// - HOT (<= 90 days) -> WARM (90d - 2y) or COLD (> 2y)
// - WARM -> COLD (> 2y)
func (m *StorageTierManager) EvaluateAndMigrateTiers(ctx context.Context) (int, error) {
	if m.repo == nil {
		return 0, nil
	}

	migratedCount := 0
	const batchSize = 200

	now := time.Now().UTC()

	// 1. Process HOT candidates older than 90 days in batches
	for {
		hotCandidates, err := m.repo.GetDocumentsForTierMigration(ctx, StorageTierHot, 90)
		if err != nil {
			return migratedCount, fmt.Errorf("fetch hot tier candidates: %w", err)
		}
		if len(hotCandidates) == 0 {
			break
		}

		migratedInBatch := 0
		for _, doc := range hotCandidates {
			if doc.DeletedAt != nil {
				continue
			}

			refTime := doc.UploadedAt
			if refTime.IsZero() {
				refTime = doc.UpdatedAt
			}
			dur := now.Sub(refTime.UTC()).Truncate(time.Second)
			if dur <= 90*24*time.Hour {
				continue
			}

			targetTier := determineTargetTierForDuration(dur)

			if err := m.MigrateDocumentTier(ctx, doc, targetTier); err == nil {
				migratedCount++
				migratedInBatch++
			}
		}

		if len(hotCandidates) < batchSize || migratedInBatch == 0 {
			break
		}
	}

	// 2. Process WARM candidates older than 730 days (2 years) in batches
	for {
		warmCandidates, err := m.repo.GetDocumentsForTierMigration(ctx, StorageTierWarm, 730)
		if err != nil {
			return migratedCount, fmt.Errorf("fetch warm tier candidates: %w", err)
		}
		if len(warmCandidates) == 0 {
			break
		}

		migratedInBatch := 0
		for _, doc := range warmCandidates {
			if doc.DeletedAt != nil {
				continue
			}

			refTime := doc.UploadedAt
			if refTime.IsZero() {
				refTime = doc.UpdatedAt
			}
			dur := now.Sub(refTime.UTC()).Truncate(time.Second)
			if dur <= 730*24*time.Hour {
				continue
			}

			targetTier := StorageTierCold

			if err := m.MigrateDocumentTier(ctx, doc, targetTier); err == nil {
				migratedCount++
				migratedInBatch++
			}
		}

		if len(warmCandidates) < batchSize || migratedInBatch == 0 {
			break
		}
	}

	return migratedCount, nil
}

// MigrateStorageTiers is an alias method delegating to EvaluateAndMigrateTiers.
func (m *StorageTierManager) MigrateStorageTiers(ctx context.Context) (int, error) {
	return m.EvaluateAndMigrateTiers(ctx)
}

// MigrateDocumentTier migrates a single document to targetTier and logs an audit record.
func (m *StorageTierManager) MigrateDocumentTier(ctx context.Context, doc *ArchiveDocument, targetTier StorageTier) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.migrateDocumentTierLocked(ctx, doc, targetTier)
}

// migrateDocumentTierLocked performs storage tier migration for a document assuming m.mu is already held.
func (m *StorageTierManager) migrateDocumentTierLocked(ctx context.Context, doc *ArchiveDocument, targetTier StorageTier) error {
	if m.repo == nil {
		return nil
	}

	freshDoc, err := m.repo.GetDocumentByID(ctx, doc.ID)
	if err != nil {
		return fmt.Errorf("re-fetch document for tier migration: %w", err)
	}
	if freshDoc == nil || freshDoc.DeletedAt != nil {
		return nil
	}

	if freshDoc.StorageTier == targetTier {
		return nil
	}

	// Check if document tier was modified (e.g. promoted via PromoteOnAccess) while candidate list was held
	if doc.StorageTier != "" && freshDoc.StorageTier != doc.StorageTier {
		return nil
	}

	refTime := freshDoc.UploadedAt
	if refTime.IsZero() {
		refTime = freshDoc.UpdatedAt
	}
	dur := time.Now().UTC().Sub(refTime.UTC())
	ageDays := int(dur.Hours() / 24.0)

	oldTier := freshDoc.StorageTier
	freshDoc.StorageTier = targetTier
	freshDoc.UpdatedAt = time.Now().UTC()

	// Simulate physical storage provider tier migration
	if err := m.simulateStorageProviderTierMigration(freshDoc.StoragePath, oldTier, targetTier); err != nil {
		// Log warning or continue; DB record and search index still updated
	}

	if err := m.repo.UpdateDocument(ctx, freshDoc); err != nil {
		return fmt.Errorf("update document tier in repo: %w", err)
	}

	se := m.searchEngine
	if se != nil {
		if err := se.IndexDocument(ctx, freshDoc); err != nil {
			return fmt.Errorf("re-index document tier in search engine: %w", err)
		}
	}

	docID := freshDoc.ID
	if err := m.repo.CreateAuditLog(ctx, &AuditLog{
		ID:         uuid.New(),
		DocumentID: &docID,
		Action:     AuditActionTierMigration, // "TIER_MIGRATION"
		UserID:     uuid.Nil,
		IPAddress:  "127.0.0.1",
		UserAgent:  "StorageTierManager/1.0",
		Details: JSONMap{
			"action":    "STORAGE_TIER_MIGRATED",
			"from_tier": string(oldTier),
			"to_tier":   string(targetTier),
			"age_days":  ageDays,
			"reason":    "AUTOMATED_TIER_MIGRATION",
		},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("create audit log for tier migration: %w", err)
	}

	return nil
}

// PromoteOnAccess promotes a document's storage tier upon access (WARM -> HOT, COLD -> WARM).
func (m *StorageTierManager) PromoteOnAccess(ctx context.Context, docID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.repo == nil {
		return nil
	}

	doc, err := m.repo.GetDocumentByID(ctx, docID)
	if err != nil {
		return err
	}

	var targetTier StorageTier
	switch doc.StorageTier {
	case StorageTierWarm:
		targetTier = StorageTierHot
	case StorageTierCold:
		targetTier = StorageTierWarm
	default:
		return nil // Already HOT
	}

	return m.migrateDocumentTierLocked(ctx, doc, targetTier)
}

// simulateStorageProviderTierMigration simulates object movement across S3 storage classes (Standard -> IA -> Glacier).
func (m *StorageTierManager) simulateStorageProviderTierMigration(path string, fromTier, toTier StorageTier) error {
	if path == "" {
		return nil
	}
	// Check if file exists; touch or verify file accessibility
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	// Simulated storage provider call succeeded
	return nil
}
