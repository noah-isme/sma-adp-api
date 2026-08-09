package archives

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

type engineState int

const (
	engineStopped engineState = iota
	engineRunning
	engineStopping
)

// RetentionEngine handles background policy evaluation, auto-deletion, legal holds, and storage tier migrations.
type RetentionEngine struct {
	repo         Repository
	searchEngine SearchEngine
	tierMigrator *StorageTierMigrator
	interval     time.Duration
	ticker       *time.Ticker
	stopCh       chan struct{}
	doneCh       chan struct{}
	mu           sync.Mutex
	evalMu       sync.Mutex
	state        engineState
}

// NewRetentionEngine constructs a RetentionEngine instance.
func NewRetentionEngine(repo Repository) *RetentionEngine {
	engine := &RetentionEngine{
		repo:     repo,
		interval: 24 * time.Hour,
		stopCh:   make(chan struct{}),
	}
	if repo != nil {
		engine.tierMigrator = NewStorageTierMigrator(repo, nil)
	}
	return engine
}

// SetSearchEngine sets search engine for index management and tier migrator.
func (e *RetentionEngine) SetSearchEngine(se SearchEngine) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.searchEngine = se
	if e.tierMigrator != nil {
		e.tierMigrator.SetSearchEngine(se)
	}
}

// SetStorageTierMigrator sets storage tier migrator.
func (e *RetentionEngine) SetStorageTierMigrator(m *StorageTierMigrator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tierMigrator = m
}

// SetInterval sets evaluation ticker interval.
func (e *RetentionEngine) SetInterval(interval time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.interval = interval
}

// Start begins background ticker evaluation loop.
func (e *RetentionEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.state != engineStopped {
		e.mu.Unlock()
		return nil
	}
	e.state = engineRunning
	ticker := time.NewTicker(e.interval)
	e.ticker = ticker
	stopCh := make(chan struct{})
	e.stopCh = stopCh
	doneCh := make(chan struct{})
	e.doneCh = doneCh
	e.mu.Unlock()

	go func(stop chan struct{}, done chan struct{}, t *time.Ticker) {
		defer func() {
			close(done)
			e.mu.Lock()
			if e.doneCh == done {
				e.state = engineStopped
			}
			e.mu.Unlock()
		}()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Recover from panic in ticker loop to keep worker alive
						}
					}()
					_ = e.EvaluatePolicies(ctx)
					_ = e.MigrateStorageTiers(ctx)
				}()
			}
		}
	}(stopCh, doneCh, ticker)

	return nil
}

// Stop terminates background evaluation loop gracefully.
func (e *RetentionEngine) Stop() {
	e.mu.Lock()
	if e.state == engineStopped {
		e.mu.Unlock()
		return
	}
	e.state = engineStopping
	if e.ticker != nil {
		e.ticker.Stop()
	}
	select {
	case <-e.stopCh:
	default:
		close(e.stopCh)
	}
	done := e.doneCh
	e.mu.Unlock()

	if done != nil {
		<-done
	}

	e.mu.Lock()
	if e.doneCh == done {
		e.state = engineStopped
	}
	e.mu.Unlock()
}

// TriggerRun forces an immediate synchronous retention evaluation and storage tier migration run.
func (e *RetentionEngine) TriggerRun(ctx context.Context) error {
	if err := e.EvaluatePolicies(ctx); err != nil {
		return fmt.Errorf("evaluate policies: %w", err)
	}
	if err := e.MigrateStorageTiers(ctx); err != nil {
		return fmt.Errorf("migrate storage tiers: %w", err)
	}
	return nil
}

func (e *RetentionEngine) hasAuditAction(ctx context.Context, docID uuid.UUID, action string) bool {
	logs, err := e.repo.GetAuditLogsByDocument(ctx, docID)
	if err != nil {
		return false
	}
	for _, log := range logs {
		if log.Action == action {
			return true
		}
	}
	return false
}

// EvaluatePolicies scans expired documents and executes auto-deletion, legal hold checks, or manual review logging.
func (e *RetentionEngine) EvaluatePolicies(ctx context.Context) error {
	e.evalMu.Lock()
	defer e.evalMu.Unlock()

	if e.repo == nil {
		return nil
	}

	const batchSize = 100
	processedIDs := make(map[uuid.UUID]bool)

	for {
		expiredDocs, err := e.repo.GetExpiredDocuments(ctx, batchSize)
		if err != nil {
			return err
		}
		if len(expiredDocs) == 0 {
			break
		}

		newProcessedInBatch := 0
		deletedInBatch := 0

		for _, doc := range expiredDocs {
			if doc.DeletedAt != nil || processedIDs[doc.ID] {
				continue
			}
			processedIDs[doc.ID] = true
			newProcessedInBatch++

			legalHoldOverride := false
			autoDelete := false

			var policy *RetentionPolicy
			var policyErr error

			if doc.RetentionPolicyID != nil {
				policy, policyErr = e.repo.GetRetentionPolicyByID(ctx, *doc.RetentionPolicyID)
			}

			if (policy == nil || policyErr != nil) && doc.Category != "" {
				policy, policyErr = e.repo.GetDefaultPolicyByCategory(ctx, doc.Category)
			}

			if policyErr == nil && policy != nil {
				legalHoldOverride = policy.LegalHoldOverride
				autoDelete = policy.AutoDelete
			} else {
				autoDelete = false
			}

			docIDPtr := doc.ID

			// 1. Check Legal Hold mechanics
			if doc.LegalHold && !legalHoldOverride {
				// Skip deletion and log SKIPPED_LEGAL_HOLD audit entry if not already logged
				if !e.hasAuditAction(ctx, doc.ID, AuditActionSkippedLegalHold) {
					_ = e.repo.CreateAuditLog(ctx, &AuditLog{
						ID:         uuid.New(),
						DocumentID: &docIDPtr,
						Action:     AuditActionSkippedLegalHold,
						UserID:     uuid.Nil,
						IPAddress:  "127.0.0.1",
						UserAgent:  "RetentionEngine/1.0",
						Details: JSONMap{
							"reason":            "Document under legal hold; auto-deletion skipped",
							"legal_hold_reason": doc.LegalHoldReason,
							"retain_until":      doc.RetainUntil.Format(time.RFC3339),
						},
						CreatedAt: time.Now().UTC(),
					})
				}
				continue
			}

			// 2. Process Auto-Delete or Manual Review
			if autoDelete {
				// Re-check document legal hold status to prevent TOCTOU race conditions
				freshDoc, freshErr := e.repo.GetDocumentByID(ctx, doc.ID)
				if freshErr != nil || freshDoc == nil || freshDoc.DeletedAt != nil {
					continue
				}
				if freshDoc.LegalHold && !legalHoldOverride {
					if !e.hasAuditAction(ctx, freshDoc.ID, AuditActionSkippedLegalHold) {
						_ = e.repo.CreateAuditLog(ctx, &AuditLog{
							ID:         uuid.New(),
							DocumentID: &docIDPtr,
							Action:     AuditActionSkippedLegalHold,
							UserID:     uuid.Nil,
							IPAddress:  "127.0.0.1",
							UserAgent:  "RetentionEngine/1.0",
							Details: JSONMap{
								"reason":            "Document under legal hold; auto-deletion skipped",
								"legal_hold_reason": freshDoc.LegalHoldReason,
								"retain_until":      freshDoc.RetainUntil.Format(time.RFC3339),
							},
							CreatedAt: time.Now().UTC(),
						})
					}
					continue
				}

				if freshDoc.StoragePath != "" {
					_ = os.Remove(freshDoc.StoragePath)
				}

				err := e.repo.SoftDeleteDocument(ctx, freshDoc.ID)
				if err == nil {
					deletedInBatch++
					e.mu.Lock()
					se := e.searchEngine
					e.mu.Unlock()
					if se != nil {
						_ = se.DeleteDocumentIndex(ctx, freshDoc.ID)
					}

					_ = e.repo.CreateAuditLog(ctx, &AuditLog{
						ID:         uuid.New(),
						DocumentID: &docIDPtr,
						Action:     AuditActionRetentionExpired,
						UserID:     uuid.Nil,
						IPAddress:  "127.0.0.1",
						UserAgent:  "RetentionEngine/1.0",
						Details: JSONMap{
							"action":       "AUTO_DELETE",
							"status":       "AUTO_DELETED",
							"retain_until": freshDoc.RetainUntil.Format(time.RFC3339),
						},
						CreatedAt: time.Now().UTC(),
					})
				}
			} else {
				if !e.hasAuditAction(ctx, doc.ID, AuditActionRetentionExpiredManualReview) {
					_ = e.repo.CreateAuditLog(ctx, &AuditLog{
						ID:         uuid.New(),
						DocumentID: &docIDPtr,
						Action:     AuditActionRetentionExpiredManualReview,
						UserID:     uuid.Nil,
						IPAddress:  "127.0.0.1",
						UserAgent:  "RetentionEngine/1.0",
						Details: JSONMap{
							"action":       "MANUAL_REVIEW_REQUIRED",
							"retain_until": doc.RetainUntil.Format(time.RFC3339),
						},
						CreatedAt: time.Now().UTC(),
					})
				}
			}
		}

		if len(expiredDocs) < batchSize || newProcessedInBatch == 0 {
			break
		}
	}

	return nil
}

// MigrateStorageTiers delegates evaluation to storage tier migrator.
func (e *RetentionEngine) MigrateStorageTiers(ctx context.Context) error {
	e.mu.Lock()
	migrator := e.tierMigrator
	e.mu.Unlock()

	if migrator != nil {
		_, err := migrator.EvaluateAndMigrateTiers(ctx)
		return err
	}
	return nil
}

// ApplyLegalHold applies a legal hold lock to a document and logs APPLY_LEGAL_HOLD audit entry.
func (e *RetentionEngine) ApplyLegalHold(ctx context.Context, docID uuid.UUID, reason string, userID uuid.UUID) (*ArchiveDocument, error) {
	doc, err := e.repo.GetDocumentByID(ctx, docID)
	if err != nil {
		return nil, err
	}

	doc.LegalHold = true
	doc.LegalHoldReason = reason
	doc.UpdatedAt = time.Now().UTC()

	if err := e.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, err
	}

	_ = e.repo.CreateAuditLog(ctx, &AuditLog{
		ID:         uuid.New(),
		DocumentID: &docID,
		Action:     "APPLY_LEGAL_HOLD",
		UserID:     userID,
		IPAddress:  "127.0.0.1",
		UserAgent:  "RetentionEngine/1.0",
		Details: JSONMap{
			"legal_hold": true,
			"reason":     reason,
		},
		CreatedAt: time.Now().UTC(),
	})

	return doc, nil
}

// ReleaseLegalHold removes a legal hold lock from a document and logs RELEASE_LEGAL_HOLD audit entry.
func (e *RetentionEngine) ReleaseLegalHold(ctx context.Context, docID uuid.UUID, reason string, userID uuid.UUID) (*ArchiveDocument, error) {
	doc, err := e.repo.GetDocumentByID(ctx, docID)
	if err != nil {
		return nil, err
	}

	doc.LegalHold = false
	doc.LegalHoldReason = reason
	doc.UpdatedAt = time.Now().UTC()

	if err := e.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, err
	}

	_ = e.repo.CreateAuditLog(ctx, &AuditLog{
		ID:         uuid.New(),
		DocumentID: &docID,
		Action:     "RELEASE_LEGAL_HOLD",
		UserID:     userID,
		IPAddress:  "127.0.0.1",
		UserAgent:  "RetentionEngine/1.0",
		Details: JSONMap{
			"legal_hold": false,
			"reason":     reason,
		},
		CreatedAt: time.Now().UTC(),
	})

	return doc, nil
}

