package queue

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type JobType string

const (
	JobTypeCreateIssue    JobType = "CREATE_ISSUE"
	JobTypeUpdateIssue    JobType = "UPDATE_ISSUE"
	JobTypeTransition     JobType = "TRANSITION"
	JobTypeBulkUpdate     JobType = "BULK_UPDATE"
	JobTypeSprintMove     JobType = "SPRINT_MOVE"
	JobTypeWorkflowChange JobType = "WORKFLOW_CHANGE"
)

type Job struct {
	ID       string
	Type     JobType
	Priority int
	Payload  interface{}
	Retries  int
	Created  time.Time
}

type JobResult struct {
	JobID    string
	Success  bool
	Result   interface{}
	Error    error
	Duration time.Duration
}

type QueueConfig struct {
	MaxWorkers   int
	MaxQueueSize int
	MaxRetries   int
	RetryDelay   time.Duration
	RateLimit    int // requests per second
}

type JobQueue struct {
	jobs          chan Job
	priorityQueue *PriorityQueue
	workers       []*Worker
	results       chan JobResult
	wg            sync.WaitGroup
	stopCh        chan struct{}
	config        QueueConfig
	mu            sync.RWMutex
	metrics       *QueueMetrics
	usePriority   bool
}

type QueueMetrics struct {
	TotalJobs      int64
	SuccessfulJobs int64
	FailedJobs     int64
	RetryCount     int64
	mu             sync.RWMutex
}

func NewJobQueue(config QueueConfig) *JobQueue {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 5
	}
	if config.MaxQueueSize <= 0 {
		config.MaxQueueSize = 100
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = time.Second
	}
	if config.RateLimit <= 0 {
		config.RateLimit = 10
	}

	q := &JobQueue{
		jobs:    make(chan Job, config.MaxQueueSize),
		results: make(chan JobResult, config.MaxQueueSize),
		stopCh:  make(chan struct{}),
		config:  config,
		metrics: &QueueMetrics{},
	}

	// Create workers
	q.workers = make([]*Worker, config.MaxWorkers)
	for i := 0; i < config.MaxWorkers; i++ {
		q.workers[i] = NewWorker(i, q)
	}

	return q
}

func (q *JobQueue) Start() {
	log.Info().
		Int("workers", q.config.MaxWorkers).
		Int("queueSize", q.config.MaxQueueSize).
		Int("rateLimit", q.config.RateLimit).
		Msg("Starting job queue")

	// Start rate limiter
	rateLimiter := time.NewTicker(time.Second / time.Duration(q.config.RateLimit))
	
	// Start workers
	for _, worker := range q.workers {
		q.wg.Add(1)
		go worker.Start(&q.wg, rateLimiter.C)
	}

	// Start metrics reporter
	go q.reportMetrics()
	
	// Keep rate limiter alive in background
	go func() {
		<-q.stopCh
		rateLimiter.Stop()
	}()
}

func (q *JobQueue) Submit(job Job) error {
	if job.ID == "" {
		job.ID = fmt.Sprintf("%s-%d", job.Type, time.Now().UnixNano())
	}
	if job.Created.IsZero() {
		job.Created = time.Now()
	}

	select {
	case q.jobs <- job:
		q.updateMetrics(func(m *QueueMetrics) {
			m.TotalJobs++
		})
		log.Debug().
			Str("jobId", job.ID).
			Str("type", string(job.Type)).
			Int("priority", job.Priority).
			Msg("Job submitted to queue")
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("queue is full, cannot submit job %s", job.ID)
	}
}

func (q *JobQueue) GetResult(timeout time.Duration) (*JobResult, error) {
	select {
	case result := <-q.results:
		return &result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for result")
	}
}

func (q *JobQueue) Stop() {
	log.Info().Msg("Stopping job queue")
	close(q.stopCh)
	q.wg.Wait()
	close(q.jobs)
	close(q.results)
}

func (q *JobQueue) GetMetrics() QueueMetrics {
	q.metrics.mu.RLock()
	defer q.metrics.mu.RUnlock()
	return *q.metrics
}

func (q *JobQueue) updateMetrics(fn func(*QueueMetrics)) {
	q.metrics.mu.Lock()
	defer q.metrics.mu.Unlock()
	fn(q.metrics)
}

func (q *JobQueue) reportMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			metrics := q.GetMetrics()
			log.Info().
				Int64("total", metrics.TotalJobs).
				Int64("successful", metrics.SuccessfulJobs).
				Int64("failed", metrics.FailedJobs).
				Int64("retries", metrics.RetryCount).
				Msg("Queue metrics")
		case <-q.stopCh:
			return
		}
	}
}

func (q *JobQueue) QueueSize() int {
	return len(q.jobs)
}

func (q *JobQueue) ResultsSize() int {
	return len(q.results)
}