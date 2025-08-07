package jira

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ericfisherdev/GoJira/internal/monitoring"
	"github.com/rs/zerolog/log"
)

// BatchProcessor handles batching and optimization of Jira API calls
type BatchProcessor struct {
	client      *Client
	monitor     *monitoring.DetailedPerformanceMonitor
	config      BatchConfig
	queue       chan BatchOperation
	workers     int
	stopCh      chan struct{}
	wg          sync.WaitGroup
	rateLimiter *RateLimiter
	stats       BatchStats
	mu          sync.RWMutex
}

// BatchConfig configures batch processing behavior
type BatchConfig struct {
	WorkerCount        int           `json:"workerCount"`        // Number of concurrent workers
	QueueSize          int           `json:"queueSize"`          // Size of operation queue
	BatchSize          int           `json:"batchSize"`          // Number of operations per batch
	FlushInterval      time.Duration `json:"flushInterval"`      // How often to flush partial batches
	MaxRetries         int           `json:"maxRetries"`         // Maximum retry attempts
	RetryDelay         time.Duration `json:"retryDelay"`         // Initial delay between retries
	RequestsPerSecond  float64       `json:"requestsPerSecond"`  // Rate limit for API calls
	EnableRateLimiting bool          `json:"enableRateLimiting"` // Whether to enable rate limiting
	EnableBatching     bool          `json:"enableBatching"`     // Whether to batch compatible operations
	TimeoutDuration    time.Duration `json:"timeoutDuration"`    // Operation timeout
}

// BatchOperation represents a single operation that can be batched
type BatchOperation struct {
	ID          string
	Type        BatchOperationType
	Data        interface{}
	Callback    func(result interface{}, err error)
	Priority    int
	CreatedAt   time.Time
	Retries     int
	Context     context.Context
}

// BatchOperationType defines the type of batch operation
type BatchOperationType string

const (
	BatchTypeGetIssue       BatchOperationType = "GET_ISSUE"
	BatchTypeUpdateIssue    BatchOperationType = "UPDATE_ISSUE"
	BatchTypeSearchIssues   BatchOperationType = "SEARCH_ISSUES"
	BatchTypeGetProject     BatchOperationType = "GET_PROJECT"
	BatchTypeCreateIssue    BatchOperationType = "CREATE_ISSUE"
	BatchTypeGetTransitions BatchOperationType = "GET_TRANSITIONS"
	BatchTypeAddComment     BatchOperationType = "ADD_COMMENT"
)

// BatchStats tracks batch processing performance
type BatchStats struct {
	TotalOperations   int64         `json:"totalOperations"`
	BatchedOperations int64         `json:"batchedOperations"`
	SuccessfulOps     int64         `json:"successfulOps"`
	FailedOps         int64         `json:"failedOps"`
	RetryCount        int64         `json:"retryCount"`
	AverageLatency    time.Duration `json:"averageLatency"`
	MaxLatency        time.Duration `json:"maxLatency"`
	QueueDepth        int           `json:"queueDepth"`
	BatchEfficiency   float64       `json:"batchEfficiency"` // Percentage of operations that were batched
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	rate     float64
	capacity int
	tokens   float64
	lastTime time.Time
	mu       sync.Mutex
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(client *Client, monitor *monitoring.DetailedPerformanceMonitor, config BatchConfig) *BatchProcessor {
	if config.WorkerCount == 0 {
		config.WorkerCount = 5
	}
	if config.QueueSize == 0 {
		config.QueueSize = 1000
	}
	if config.BatchSize == 0 {
		config.BatchSize = 10
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}
	if config.RequestsPerSecond == 0 {
		config.RequestsPerSecond = 10.0
	}
	if config.TimeoutDuration == 0 {
		config.TimeoutDuration = 30 * time.Second
	}

	var rateLimiter *RateLimiter
	if config.EnableRateLimiting {
		rateLimiter = NewRateLimiter(config.RequestsPerSecond, int(config.RequestsPerSecond*2))
	}

	bp := &BatchProcessor{
		client:      client,
		monitor:     monitor,
		config:      config,
		queue:       make(chan BatchOperation, config.QueueSize),
		workers:     config.WorkerCount,
		stopCh:      make(chan struct{}),
		rateLimiter: rateLimiter,
		stats:       BatchStats{},
	}

	bp.start()
	return bp
}

// SubmitOperation submits an operation for batch processing
func (bp *BatchProcessor) SubmitOperation(op BatchOperation) error {
	if bp.config.TimeoutDuration > 0 {
		var cancel context.CancelFunc
		op.Context, cancel = context.WithTimeout(op.Context, bp.config.TimeoutDuration)
		defer cancel()
	}

	op.CreatedAt = time.Now()
	bp.updateStats(func(s *BatchStats) {
		s.TotalOperations++
		s.QueueDepth = len(bp.queue)
	})

	select {
	case bp.queue <- op:
		return nil
	case <-op.Context.Done():
		return fmt.Errorf("operation cancelled: %v", op.Context.Err())
	default:
		return fmt.Errorf("queue full, cannot submit operation")
	}
}

// GetIssueAsync submits an async get issue operation
func (bp *BatchProcessor) GetIssueAsync(ctx context.Context, issueKey string, expand []string, callback func(*Issue, error)) error {
	op := BatchOperation{
		ID:       fmt.Sprintf("get-issue-%s-%d", issueKey, time.Now().UnixNano()),
		Type:     BatchTypeGetIssue,
		Data:     map[string]interface{}{"key": issueKey, "expand": expand},
		Context:  ctx,
		Priority: 1,
		Callback: func(result interface{}, err error) {
			if err != nil {
				callback(nil, err)
				return
			}
			if issue, ok := result.(*Issue); ok {
				callback(issue, nil)
			} else {
				callback(nil, fmt.Errorf("invalid result type"))
			}
		},
	}

	return bp.SubmitOperation(op)
}

// SearchIssuesAsync submits an async search operation
func (bp *BatchProcessor) SearchIssuesAsync(ctx context.Context, jql string, startAt, maxResults int, expand []string, callback func(*SearchResult, error)) error {
	op := BatchOperation{
		ID:   fmt.Sprintf("search-%d", time.Now().UnixNano()),
		Type: BatchTypeSearchIssues,
		Data: map[string]interface{}{
			"jql":        jql,
			"startAt":    startAt,
			"maxResults": maxResults,
			"expand":     expand,
		},
		Context:  ctx,
		Priority: 2,
		Callback: func(result interface{}, err error) {
			if err != nil {
				callback(nil, err)
				return
			}
			if searchResult, ok := result.(*SearchResult); ok {
				callback(searchResult, nil)
			} else {
				callback(nil, fmt.Errorf("invalid result type"))
			}
		},
	}

	return bp.SubmitOperation(op)
}

// UpdateIssueAsync submits an async update operation
func (bp *BatchProcessor) UpdateIssueAsync(ctx context.Context, issueKey string, update *UpdateIssueRequest, callback func(error)) error {
	op := BatchOperation{
		ID:       fmt.Sprintf("update-issue-%s-%d", issueKey, time.Now().UnixNano()),
		Type:     BatchTypeUpdateIssue,
		Data:     map[string]interface{}{"key": issueKey, "update": update},
		Context:  ctx,
		Priority: 3,
		Callback: func(result interface{}, err error) {
			callback(err)
		},
	}

	return bp.SubmitOperation(op)
}

// GetStats returns current batch processing statistics
func (bp *BatchProcessor) GetStats() BatchStats {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	
	stats := bp.stats
	stats.QueueDepth = len(bp.queue)
	
	if stats.TotalOperations > 0 {
		stats.BatchEfficiency = float64(stats.BatchedOperations) / float64(stats.TotalOperations) * 100
	}
	
	return stats
}

// Close shuts down the batch processor
func (bp *BatchProcessor) Close() error {
	log.Info().Msg("Shutting down batch processor")
	
	close(bp.stopCh)
	bp.wg.Wait()
	
	// Process any remaining operations in the queue
	close(bp.queue)
	for op := range bp.queue {
		if op.Callback != nil {
			op.Callback(nil, fmt.Errorf("batch processor closed"))
		}
	}

	log.Info().
		Int64("totalOps", bp.stats.TotalOperations).
		Int64("successful", bp.stats.SuccessfulOps).
		Int64("failed", bp.stats.FailedOps).
		Float64("batchEfficiency", bp.stats.BatchEfficiency).
		Msg("Batch processor closed")

	return nil
}

// start initializes worker goroutines
func (bp *BatchProcessor) start() {
	for i := 0; i < bp.workers; i++ {
		bp.wg.Add(1)
		go bp.worker(i)
	}

	// Start batch flusher if batching is enabled
	if bp.config.EnableBatching {
		bp.wg.Add(1)
		go bp.batchFlusher()
	}
}

// worker processes operations from the queue
func (bp *BatchProcessor) worker(workerID int) {
	defer bp.wg.Done()

	log.Debug().Int("workerId", workerID).Msg("Batch worker started")

	for {
		select {
		case op := <-bp.queue:
			bp.processOperation(op, workerID)
		case <-bp.stopCh:
			log.Debug().Int("workerId", workerID).Msg("Batch worker stopped")
			return
		}
	}
}

// processOperation processes a single operation
func (bp *BatchProcessor) processOperation(op BatchOperation, workerID int) {
	startTime := time.Now()
	
	// Apply rate limiting
	if bp.rateLimiter != nil {
		bp.rateLimiter.Wait()
	}

	// Process based on operation type
	var result interface{}
	var err error

	timer := bp.monitor.StartTimer(string(op.Type))

	switch op.Type {
	case BatchTypeGetIssue:
		result, err = bp.processGetIssue(op)
	case BatchTypeSearchIssues:
		result, err = bp.processSearchIssues(op)
	case BatchTypeUpdateIssue:
		result, err = bp.processUpdateIssue(op)
	case BatchTypeGetProject:
		result, err = bp.processGetProject(op)
	case BatchTypeCreateIssue:
		result, err = bp.processCreateIssue(op)
	case BatchTypeGetTransitions:
		result, err = bp.processGetTransitions(op)
	case BatchTypeAddComment:
		result, err = bp.processAddComment(op)
	default:
		err = fmt.Errorf("unknown operation type: %s", op.Type)
	}

	latency := time.Since(startTime)

	// Update statistics
	bp.updateStats(func(s *BatchStats) {
		if err != nil {
			s.FailedOps++
			timer.Error()
		} else {
			s.SuccessfulOps++
			timer.Success()
		}
		
		if latency > s.MaxLatency {
			s.MaxLatency = latency
		}
		
		// Update average latency
		total := s.SuccessfulOps + s.FailedOps
		if total > 0 {
			s.AverageLatency = time.Duration((int64(s.AverageLatency)*int64(total-1) + int64(latency)) / int64(total))
		}
	})

	// Handle retry logic
	if err != nil && op.Retries < bp.config.MaxRetries {
		op.Retries++
		bp.updateStats(func(s *BatchStats) { s.RetryCount++ })
		
		// Exponential backoff
		delay := bp.config.RetryDelay * time.Duration(1<<op.Retries)
		time.Sleep(delay)
		
		// Requeue the operation
		select {
		case bp.queue <- op:
			return
		case <-bp.stopCh:
			if op.Callback != nil {
				op.Callback(nil, fmt.Errorf("operation cancelled during retry"))
			}
			return
		}
	}

	// Call the callback with the result
	if op.Callback != nil {
		op.Callback(result, err)
	}

	log.Debug().
		Int("workerId", workerID).
		Str("opId", op.ID).
		Str("type", string(op.Type)).
		Dur("latency", latency).
		Bool("success", err == nil).
		Int("retries", op.Retries).
		Msg("Operation processed")
}

// Operation processors

func (bp *BatchProcessor) processGetIssue(op BatchOperation) (interface{}, error) {
	data := op.Data.(map[string]interface{})
	key := data["key"].(string)
	expand := data["expand"].([]string)
	
	return bp.client.GetIssue(op.Context, key, expand)
}

func (bp *BatchProcessor) processSearchIssues(op BatchOperation) (interface{}, error) {
	data := op.Data.(map[string]interface{})
	jql := data["jql"].(string)
	startAt := data["startAt"].(int)
	maxResults := data["maxResults"].(int)
	expand := data["expand"].([]string)
	
	return bp.client.SearchIssues(op.Context, jql, startAt, maxResults, expand)
}

func (bp *BatchProcessor) processUpdateIssue(op BatchOperation) (interface{}, error) {
	data := op.Data.(map[string]interface{})
	key := data["key"].(string)
	update := data["update"].(*UpdateIssueRequest)
	
	return nil, bp.client.UpdateIssue(op.Context, key, update)
}

func (bp *BatchProcessor) processGetProject(op BatchOperation) (interface{}, error) {
	data := op.Data.(map[string]interface{})
	key := data["key"].(string)
	
	return bp.client.GetProject(op.Context, key)
}

func (bp *BatchProcessor) processCreateIssue(op BatchOperation) (interface{}, error) {
	data := op.Data.(map[string]interface{})
	issue := data["issue"].(*CreateIssueRequest)
	
	return bp.client.CreateIssue(op.Context, issue)
}

func (bp *BatchProcessor) processGetTransitions(op BatchOperation) (interface{}, error) {
	data := op.Data.(map[string]interface{})
	key := data["key"].(string)
	
	return bp.client.GetIssueTransitions(op.Context, key)
}

func (bp *BatchProcessor) processAddComment(op BatchOperation) (interface{}, error) {
	data := op.Data.(map[string]interface{})
	key := data["key"].(string)
	comment := data["comment"].(*CreateCommentRequest)
	
	return bp.client.AddComment(op.Context, key, comment)
}

func (bp *BatchProcessor) updateStats(updateFn func(*BatchStats)) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	updateFn(&bp.stats)
}

func (bp *BatchProcessor) batchFlusher() {
	defer bp.wg.Done()
	
	ticker := time.NewTicker(bp.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Implementation for batching similar operations would go here
			// This is a simplified version focusing on the infrastructure
		case <-bp.stopCh:
			return
		}
	}
}

// RateLimiter implementation

// NewRateLimiter creates a new token bucket rate limiter
func NewRateLimiter(rate float64, capacity int) *RateLimiter {
	return &RateLimiter{
		rate:     rate,
		capacity: capacity,
		tokens:   float64(capacity),
		lastTime: time.Now(),
	}
}

// Wait waits until a token is available
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime)
	rl.lastTime = now

	// Add tokens based on elapsed time
	rl.tokens += elapsed.Seconds() * rl.rate
	if rl.tokens > float64(rl.capacity) {
		rl.tokens = float64(rl.capacity)
	}

	// Wait if no tokens available
	if rl.tokens < 1.0 {
		waitTime := time.Duration((1.0-rl.tokens)/rl.rate) * time.Second
		time.Sleep(waitTime)
		rl.tokens = 0
	} else {
		rl.tokens -= 1.0
	}
}

// TryAcquire attempts to acquire a token without waiting
func (rl *RateLimiter) TryAcquire() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime)
	rl.lastTime = now

	// Add tokens based on elapsed time
	rl.tokens += elapsed.Seconds() * rl.rate
	if rl.tokens > float64(rl.capacity) {
		rl.tokens = float64(rl.capacity)
	}

	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}

	return false
}

// Configuration presets

// DefaultBatchConfig returns a sensible default batch configuration
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		WorkerCount:        5,
		QueueSize:          1000,
		BatchSize:          10,
		FlushInterval:      time.Second,
		MaxRetries:         3,
		RetryDelay:         time.Second,
		RequestsPerSecond:  10.0,
		EnableRateLimiting: true,
		EnableBatching:     true,
		TimeoutDuration:    30 * time.Second,
	}
}

// HighThroughputBatchConfig returns configuration optimized for high throughput
func HighThroughputBatchConfig() BatchConfig {
	return BatchConfig{
		WorkerCount:        10,
		QueueSize:          2000,
		BatchSize:          20,
		FlushInterval:      500 * time.Millisecond,
		MaxRetries:         2,
		RetryDelay:         500 * time.Millisecond,
		RequestsPerSecond:  25.0,
		EnableRateLimiting: true,
		EnableBatching:     true,
		TimeoutDuration:    15 * time.Second,
	}
}

// LowLatencyBatchConfig returns configuration optimized for low latency
func LowLatencyBatchConfig() BatchConfig {
	return BatchConfig{
		WorkerCount:        3,
		QueueSize:          100,
		BatchSize:          5,
		FlushInterval:      100 * time.Millisecond,
		MaxRetries:         1,
		RetryDelay:         250 * time.Millisecond,
		RequestsPerSecond:  15.0,
		EnableRateLimiting: true,
		EnableBatching:     false, // Disable batching for lowest latency
		TimeoutDuration:    10 * time.Second,
	}
}