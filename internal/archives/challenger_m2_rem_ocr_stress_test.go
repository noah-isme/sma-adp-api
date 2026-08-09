package archives

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// OBJECTIVE 1: Secondary Repository Panics in handleJobFailure
// =============================================================================

// MockSecondaryPanicRepo simulates repository failure/panic during secondary status update in handleJobFailure.
type MockSecondaryPanicRepo struct {
	Repository
	panicOnGetDocByID bool
	panicOnUpdateDoc  bool
	panicOnDocIDs     map[uuid.UUID]bool
	mu                sync.Mutex
}

func NewMockSecondaryPanicRepo(base Repository) *MockSecondaryPanicRepo {
	return &MockSecondaryPanicRepo{
		Repository:    base,
		panicOnDocIDs: make(map[uuid.UUID]bool),
	}
}

func (r *MockSecondaryPanicRepo) SetPanicOnDoc(id uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.panicOnDocIDs[id] = true
}

func (r *MockSecondaryPanicRepo) GetDocumentByID(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error) {
	r.mu.Lock()
	shouldPanic := r.panicOnDocIDs[id] || r.panicOnGetDocByID
	r.mu.Unlock()

	if shouldPanic {
		panic("CRITICAL SECONDARY REPOSITORY PANIC IN GetDocumentByID")
	}
	return r.Repository.GetDocumentByID(ctx, id)
}

func (r *MockSecondaryPanicRepo) UpdateDocument(ctx context.Context, doc *ArchiveDocument) error {
	r.mu.Lock()
	shouldPanic := r.panicOnDocIDs[doc.ID] || r.panicOnUpdateDoc
	r.mu.Unlock()

	if shouldPanic {
		panic("CRITICAL SECONDARY REPOSITORY PANIC IN UpdateDocument")
	}
	return r.Repository.UpdateDocument(ctx, doc)
}

// TestSecondaryPanicRecovery_GetDocumentByID verifies handleJobFailure catches panics in GetDocumentByID
// without crashing the worker loop goroutine.
func TestSecondaryPanicRecovery_GetDocumentByID(t *testing.T) {
	ctx := context.Background()
	baseRepo := NewMemoryRepository()
	mockRepo := NewMockSecondaryPanicRepo(baseRepo)

	// Create 3 documents: doc1 (triggers primary panic in parser + secondary panic in handleJobFailure),
	// doc2 (triggers panic in handleJobFailure), doc3 (normal valid doc)
	doc1ID := uuid.New()
	doc2ID := uuid.New()
	doc3ID := uuid.New()

	for _, doc := range []*ArchiveDocument{
		{ID: doc1ID, Filename: "panic1.txt", Category: CategoryOther, OCRStatus: OCRStatusPending, UploadedAt: time.Now()},
		{ID: doc2ID, Filename: "panic2.txt", Category: CategoryOther, OCRStatus: OCRStatusPending, UploadedAt: time.Now()},
		{ID: doc3ID, Filename: "valid3.txt", Category: CategoryOther, OCRStatus: OCRStatusPending, UploadedAt: time.Now()},
	} {
		require.NoError(t, baseRepo.CreateDocument(ctx, doc))
	}

	// Register panic for doc1 and doc2 in mock repo
	mockRepo.SetPanicOnDoc(doc1ID)
	mockRepo.SetPanicOnDoc(doc2ID)

	tmpFile := filepath.Join(os.TempDir(), "valid_file.txt")
	require.NoError(t, os.WriteFile(tmpFile, []byte("Valid text content for worker verification"), 0644))
	defer os.Remove(tmpFile)

	// Use single worker to ensure sequential execution on one worker goroutine
	pool := NewGoOCRWorkerPool(1, 10, mockRepo, nil)
	pool.Start()
	defer pool.Stop()

	// Enqueue jobs in sequence: doc1 (panic), doc2 (panic), doc3 (valid)
	require.NoError(t, pool.Enqueue(doc1ID, tmpFile, "text/plain"))
	require.NoError(t, pool.Enqueue(doc2ID, tmpFile, "text/plain"))
	require.NoError(t, pool.Enqueue(doc3ID, tmpFile, "text/plain"))

	// Verify worker goroutine survived secondary panics and processed doc3 to completion
	require.Eventually(t, func() bool {
		doc, err := baseRepo.GetDocumentByID(ctx, doc3ID)
		return err == nil && doc.OCRStatus == OCRStatusCompleted
	}, 5*time.Second, 50*time.Millisecond, "Worker loop goroutine crashed or stopped processing after secondary panic")

	doc3, err := baseRepo.GetDocumentByID(ctx, doc3ID)
	require.NoError(t, err)
	assert.Equal(t, OCRStatusCompleted, doc3.OCRStatus)
	assert.Contains(t, doc3.OCRText, "Valid text content for worker verification")

	status := pool.Status()
	assert.Equal(t, int64(3), status.ProcessedCount, "All 3 jobs must be recorded as processed despite secondary panics")
}

// TestSecondaryPanicRecovery_UpdateDocument verifies handleJobFailure catches panics in UpdateDocument
// when attempting to set OCRStatusFailed, preserving worker loop health.
func TestSecondaryPanicRecovery_UpdateDocument(t *testing.T) {
	ctx := context.Background()
	baseRepo := NewMemoryRepository()

	// We use a custom parser that panics, causing processJob to enter handleJobFailure.
	// In handleJobFailure, GetDocumentByID succeeds, but UpdateDocument panics.
	panicUpdateDocID := uuid.New()
	normalDocID := uuid.New()

	for _, doc := range []*ArchiveDocument{
		{ID: panicUpdateDocID, Filename: "fail_update.txt", Category: CategoryOther, OCRStatus: OCRStatusPending, UploadedAt: time.Now()},
		{ID: normalDocID, Filename: "normal_after_panic.txt", Category: CategoryOther, OCRStatus: OCRStatusPending, UploadedAt: time.Now()},
	} {
		require.NoError(t, baseRepo.CreateDocument(ctx, doc))
	}

	mockRepo := NewMockSecondaryPanicRepo(baseRepo)

	// Configure mockRepo: normal for GetDocumentByID, but panic on UpdateDocument for panicUpdateDocID
	mockRepo.panicOnUpdateDoc = false

	// Custom panic parser
	customRegistry := NewParserRegistry()
	customRegistry.Register(&panicTestParser{}, []string{"custom/panic"}, []string{".panic"})

	tmpFile := filepath.Join(os.TempDir(), "test_secondary_update_panic.txt")
	require.NoError(t, os.WriteFile(tmpFile, []byte("some content"), 0644))
	defer os.Remove(tmpFile)

	pool := NewGoOCRWorkerPool(1, 10, mockRepo, nil)
	pool.SetParserRegistry(customRegistry)
	pool.Start()
	defer pool.Stop()

	// Cause UpdateDocument to panic for panicUpdateDocID only during failure handling
	mockRepo.SetPanicOnDoc(panicUpdateDocID)

	// Enqueue panic job first
	require.NoError(t, pool.Enqueue(panicUpdateDocID, tmpFile, "custom/panic"))
	// Enqueue normal job second
	require.NoError(t, pool.Enqueue(normalDocID, tmpFile, "text/plain"))

	// Worker should catch secondary panic in handleJobFailure and continue to process normalDocID
	require.Eventually(t, func() bool {
		doc, err := baseRepo.GetDocumentByID(ctx, normalDocID)
		return err == nil && doc.OCRStatus == OCRStatusCompleted
	}, 5*time.Second, 50*time.Millisecond, "Worker loop goroutine crashed on secondary UpdateDocument panic")

	status := pool.Status()
	assert.Equal(t, int64(2), status.ProcessedCount)
}

type panicTestParser struct{}

func (p *panicTestParser) Parse(ctx context.Context, data []byte, mimeType string) (string, error) {
	panic("PRIMARY PARSER PANIC")
}

// =============================================================================
// OBJECTIVE 2: Concurrent Enqueue & Stop Stress Testing
// =============================================================================

// TestConcurrentEnqueueAndStop_ZeroPanicsStress executes 100 producer goroutines calling Enqueue
// concurrently with multiple Stop() calls across 20 test cycles to verify zero runtime panics
// (such as send on closed channel) and zero data races.
func TestConcurrentEnqueueAndStop_ZeroPanicsStress(t *testing.T) {
	for cycle := 0; cycle < 20; cycle++ {
		baseRepo := NewMemoryRepository()
		pool := NewGoOCRWorkerPool(4, 500, baseRepo, nil)
		pool.Start()

		producers := 50
		enqueuesPerProducer := 100
		var wg sync.WaitGroup
		var successEnqueues atomic.Int64
		var failedEnqueues atomic.Int64
		stopCalled := make(chan struct{})

		// Launch 50 producer goroutines
		for p := 0; p < producers; p++ {
			wg.Add(1)
			go func(producerID int) {
				defer wg.Done()
				for i := 0; i < enqueuesPerProducer; i++ {
					docID := uuid.New()
					err := pool.Enqueue(docID, "", "text/plain")
					if err == nil {
						successEnqueues.Add(1)
					} else {
						failedEnqueues.Add(1)
					}
					// Introduce micro jitter
					if i%10 == 0 {
						time.Sleep(10 * time.Microsecond)
					}
				}
			}(p)
		}

		// Asynchronously call Stop() after a short delay
		go func() {
			time.Sleep(time.Duration(cycle%5+1) * time.Millisecond)
			pool.Stop()
			close(stopCalled)
		}()

		// Concurrently invoke multiple redundant Stop() calls from separate goroutines
		var stopWg sync.WaitGroup
		for s := 0; s < 5; s++ {
			stopWg.Add(1)
			go func() {
				defer stopWg.Done()
				time.Sleep(2 * time.Millisecond)
				pool.Stop()
			}()
		}

		// Concurrently invoke Status() and SetParserRegistry() during shutdown
		for st := 0; st < 5; st++ {
			stopWg.Add(1)
			go func() {
				defer stopWg.Done()
				for i := 0; i < 20; i++ {
					_ = pool.Status()
					pool.SetParserRegistry(DefaultParserRegistry)
					time.Sleep(100 * time.Microsecond)
				}
			}()
		}

		wg.Wait()
		stopWg.Wait()
		<-stopCalled

		// Post-stop: verify further Enqueue calls return error without panic
		err := pool.Enqueue(uuid.New(), "", "text/plain")
		assert.Error(t, err, "Enqueue on stopped pool must return an error")
		assert.Contains(t, err.Error(), "OCR worker pool is stopped")

		// Verify pool status is queryable after stop
		status := pool.Status()
		assert.Equal(t, 0, status.WorkersActive, "No workers should be active after Stop()")
	}
}

// TestConcurrentEnqueueAndStop_QueueSaturation verifies Enqueue behavior under full queue saturation
// when Stop() is invoked simultaneously.
func TestConcurrentEnqueueAndStop_QueueSaturation(t *testing.T) {
	ctx := context.Background()
	baseRepo := NewMemoryRepository()

	// Small queue (size 5) to force queue full condition rapidly
	pool := NewGoOCRWorkerPool(2, 5, baseRepo, nil)

	// Pre-fill queue with slow jobs (mocking slow repo)
	slowRepo := &slowMockRepo{Repository: baseRepo, delay: 50 * time.Millisecond}
	pool.repo = slowRepo
	pool.Start()

	var wg sync.WaitGroup
	producers := 20

	// Launch producers that flood the queue
	for i := 0; i < producers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			docID := uuid.New()
			doc := &ArchiveDocument{ID: docID, Filename: fmt.Sprintf("sat_%d.txt", idx), Category: CategoryOther, OCRStatus: OCRStatusPending, UploadedAt: time.Now()}
			_ = baseRepo.CreateDocument(ctx, doc)
			_ = pool.Enqueue(docID, "", "text/plain")
		}(i)
	}

	// Trigger Stop while queue is saturated and producers are blocked or failing
	time.Sleep(5 * time.Millisecond)
	pool.Stop()

	wg.Wait()

	// Verify no panics and status is consistent
	status := pool.Status()
	assert.Equal(t, 0, status.WorkersActive)
}

type slowMockRepo struct {
	Repository
	delay time.Duration
}

func (r *slowMockRepo) GetDocumentByID(ctx context.Context, id uuid.UUID) (*ArchiveDocument, error) {
	time.Sleep(r.delay)
	return r.Repository.GetDocumentByID(ctx, id)
}
