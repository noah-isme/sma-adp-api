package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Job represents a queued background task.
type Job struct {
	ID       string
	Type     string
	Payload  interface{}
	Attempt  int
	Enqueued time.Time
}

// Handler processes a job.
type Handler func(context.Context, Job) error

// QueueConfig configures worker pool behaviour.
type QueueConfig struct {
	Workers    int
	BufferSize int
	MaxRetries int
	RetryDelay time.Duration
	Logger     *zap.Logger
}

// Queue is a lightweight in-memory job dispatcher backed by goroutines.
type Queue struct {
	name    string
	handler Handler

	workers    int
	bufferSize int
	maxRetries int
	retryDelay time.Duration
	logger     *zap.Logger

	jobs    chan Job
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	started bool
}

// NewQueue builds a new queue with the provided handler.
func NewQueue(name string, handler Handler, cfg QueueConfig) *Queue {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = cfg.Workers * 4
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	return &Queue{
		name:       name,
		handler:    handler,
		workers:    cfg.Workers,
		bufferSize: cfg.BufferSize,
		maxRetries: cfg.MaxRetries,
		retryDelay: cfg.RetryDelay,
		logger:     cfg.Logger,
		jobs:       make(chan Job, cfg.BufferSize),
	}
}

// Start begins worker consumption. Safe to call once.
func (q *Queue) Start(ctx context.Context) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.started {
		return
	}
	q.ctx, q.cancel = context.WithCancel(ctx)
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i + 1)
	}
	q.started = true
	q.logger.Sugar().Infow("queue started", "queue", q.name, "workers", q.workers)
}

// Stop cancels workers and waits for them to exit.
func (q *Queue) Stop() {
	q.mu.Lock()
	if !q.started {
		q.mu.Unlock()
		return
	}
	q.cancel()
	q.mu.Unlock()
	q.wg.Wait()
	q.logger.Sugar().Infow("queue stopped", "queue", q.name)
}

// Enqueue pushes a job onto the queue.
func (q *Queue) Enqueue(job Job) error {
	q.mu.Lock()
	ctx := q.ctx
	started := q.started
	q.mu.Unlock()

	if !started {
		return fmt.Errorf("queue %s not started", q.name)
	}
	if job.Enqueued.IsZero() {
		job.Enqueued = time.Now().UTC()
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("queue %s stopped: %w", q.name, ctx.Err())
	case q.jobs <- job:
		return nil
	}
}

func (q *Queue) worker(workerID int) {
	defer q.wg.Done()
	for {
		select {
		case <-q.ctx.Done():
			return
		case job := <-q.jobs:
			if err := q.handler(q.ctx, job); err != nil {
				q.handleFailure(job, err)
			}
		}
	}
}

func (q *Queue) handleFailure(job Job, err error) {
	job.Attempt++
	if job.Attempt > q.maxRetries {
		q.logger.Sugar().Errorw("job exceeded retries", "queue", q.name, "job_id", job.ID, "type", job.Type, "error", err)
		return
	}
	q.logger.Sugar().Warnw("job failed, retrying", "queue", q.name, "job_id", job.ID, "type", job.Type, "attempt", job.Attempt, "error", err)

	go func(j Job) {
		timer := time.NewTimer(q.retryDelay)
		defer timer.Stop()
		select {
		case <-q.ctx.Done():
			return
		case <-timer.C:
			if err := q.Enqueue(j); err != nil {
				q.logger.Sugar().Errorw("failed to requeue job", "queue", q.name, "job_id", j.ID, "error", err)
			}
		}
	}(job)
}
