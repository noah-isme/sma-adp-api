package archives

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OCRJob defines a single background document extraction task.
type OCRJob struct {
	DocID    uuid.UUID
	FilePath string
	MimeType string
}

// WorkerPoolStatus represents current pool operational metrics.
type WorkerPoolStatus struct {
	WorkersActive  int
	QueueLength    int
	ProcessedCount int64
}

// OCRWorkerPool defines interface for async OCR processing pool.
type OCRWorkerPool interface {
	Start()
	Stop()
	Enqueue(docID uuid.UUID, filePath string, mimeType string) error
	Status() WorkerPoolStatus
}

// GoOCRWorkerPool implements in-memory channel-backed worker pool.
type GoOCRWorkerPool struct {
	workers        int
	jobQueue       chan OCRJob
	repo           Repository
	searchEngine   SearchEngine
	parserRegistry *ParserRegistry
	wg             sync.WaitGroup
	quit           chan struct{}
	mu             sync.Mutex
	processed      int64
	active         int
	stopped        bool
	timeout        time.Duration
}

// NewGoOCRWorkerPool initializes GoOCRWorkerPool with designated workers and queue capacity.
func NewGoOCRWorkerPool(workers int, queueSize int, repo Repository, searchEngine SearchEngine) *GoOCRWorkerPool {
	if workers < 1 {
		workers = 2
	}
	if queueSize < 1 {
		queueSize = 100
	}
	return &GoOCRWorkerPool{
		workers:        workers,
		jobQueue:       make(chan OCRJob, queueSize),
		repo:           repo,
		searchEngine:   searchEngine,
		parserRegistry: DefaultParserRegistry,
		quit:           make(chan struct{}),
		timeout:        30 * time.Second,
	}
}

// SetParserRegistry configures a custom ParserRegistry for the worker pool.
func (p *GoOCRWorkerPool) SetParserRegistry(registry *ParserRegistry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if registry != nil {
		p.parserRegistry = registry
	}
}

// Start launches worker goroutines.
func (p *GoOCRWorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.workerLoop(i)
	}
}

// Stop gracefully signals workers to stop and drains remaining jobs in queue.
func (p *GoOCRWorkerPool) Stop() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	p.mu.Unlock()

	close(p.quit)
	close(p.jobQueue)
	p.wg.Wait()
}

// Enqueue submits a document processing job to the buffered pool channel safely.
func (p *GoOCRWorkerPool) Enqueue(docID uuid.UUID, filePath string, mimeType string) (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return fmt.Errorf("OCR worker pool is stopped")
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("OCR worker pool is stopped")
		}
	}()

	select {
	case p.jobQueue <- OCRJob{DocID: docID, FilePath: filePath, MimeType: mimeType}:
		return nil
	default:
		return fmt.Errorf("OCR job queue full")
	}
}

// Status thread-safely returns current active workers, queue depth, and total processed jobs.
func (p *GoOCRWorkerPool) Status() WorkerPoolStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return WorkerPoolStatus{
		WorkersActive:  p.active,
		QueueLength:    len(p.jobQueue),
		ProcessedCount: p.processed,
	}
}

func (p *GoOCRWorkerPool) workerLoop(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.quit:
			for job := range p.jobQueue {
				p.processJob(job)
			}
			return
		case job, ok := <-p.jobQueue:
			if !ok {
				return
			}
			p.processJob(job)
		}
	}
}

func (p *GoOCRWorkerPool) processJob(job OCRJob) {
	p.mu.Lock()
	p.active++
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.active--
		p.processed++
		p.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			p.handleJobFailure(ctx, job.DocID, fmt.Errorf("panic caught in OCR worker loop: %v", r))
		}
	}()

	if p.repo == nil {
		return
	}

	doc, err := p.repo.GetDocumentByID(ctx, job.DocID)
	if err != nil || doc == nil {
		return
	}

	doc.OCRStatus = OCRStatusProcessing
	if err := p.repo.UpdateDocument(ctx, doc); err != nil {
		p.handleJobFailure(ctx, job.DocID, fmt.Errorf("update doc status processing: %w", err))
		return
	}

	p.mu.Lock()
	registry := p.parserRegistry
	p.mu.Unlock()
	if registry == nil {
		registry = DefaultParserRegistry
	}

	var extractedText string
	var extractErr error
	if job.FilePath != "" {
		data, readErr := os.ReadFile(job.FilePath)
		if readErr != nil {
			extractErr = fmt.Errorf("read file for OCR: %w", readErr)
		} else {
			extractedText, extractErr = registry.Parse(ctx, data, job.MimeType, doc.Filename)
		}
	} else {
		extractedText, extractErr = registry.Parse(ctx, nil, job.MimeType, doc.Filename)
	}

	if extractErr != nil {
		p.handleJobFailure(ctx, job.DocID, extractErr)
		return
	}

	if strings.TrimSpace(extractedText) == "" && doc.OCRText != "" {
		extractedText = doc.OCRText
	}

	if strings.TrimSpace(extractedText) == "" {
		extractedText = fmt.Sprintf("Document: %s (Category: %s, Mime: %s)", doc.Filename, doc.Category, doc.MimeType)
	}

	doc.OCRText = extractedText
	doc.OCRStatus = OCRStatusCompleted
	if err := p.repo.UpdateDocument(ctx, doc); err != nil {
		p.handleJobFailure(ctx, job.DocID, fmt.Errorf("update doc status completed: %w", err))
		return
	}

	if p.searchEngine != nil {
		_ = p.searchEngine.IndexDocument(ctx, doc)
	}
}

func (p *GoOCRWorkerPool) handleJobFailure(ctx context.Context, docID uuid.UUID, jobErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[OCR Worker] Secondary panic caught in handleJobFailure: %v", r)
		}
	}()
	log.Printf("[OCR Worker] Failure processing document %s: %v", docID, jobErr)
	if p.repo != nil {
		doc, err := p.repo.GetDocumentByID(ctx, docID)
		if err == nil && doc != nil {
			doc.OCRStatus = OCRStatusFailed
			_ = p.repo.UpdateDocument(ctx, doc)
		}
	}
}
