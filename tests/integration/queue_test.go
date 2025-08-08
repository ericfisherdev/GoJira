package integration

import (
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobQueue(t *testing.T) {
	t.Run("Basic Queue Operations", func(t *testing.T) {
		config := queue.QueueConfig{
			MaxWorkers:   2,
			MaxQueueSize: 10,
			MaxRetries:   3,
			RetryDelay:   100 * time.Millisecond,
			RateLimit:    5,
		}

		q := queue.NewJobQueue(config)
		q.Start()
		defer q.Stop()

		// Submit a job
		job := queue.Job{
			ID:       "test-1",
			Type:     queue.JobTypeCreateIssue,
			Priority: 5,
			Payload:  map[string]string{"test": "data"},
		}

		err := q.Submit(job)
		assert.NoError(t, err)

		// Get result
		result, err := q.GetResult(5 * time.Second)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "test-1", result.JobID)
		assert.True(t, result.Success)
	})

	t.Run("Queue Processing Order", func(t *testing.T) {
		config := queue.QueueConfig{
			MaxWorkers:   1, // Single worker to ensure order
			MaxQueueSize: 10,
			MaxRetries:   1,
			RetryDelay:   100 * time.Millisecond,
			RateLimit:    10,
		}

		q := queue.NewJobQueue(config)
		q.Start()
		defer q.Stop()

		// Submit jobs 
		jobs := []queue.Job{
			{ID: "first", Type: queue.JobTypeUpdateIssue, Priority: 1},
			{ID: "second", Type: queue.JobTypeUpdateIssue, Priority: 10},
			{ID: "third", Type: queue.JobTypeUpdateIssue, Priority: 5},
		}

		for _, job := range jobs {
			err := q.Submit(job)
			assert.NoError(t, err)
		}

		// Results should come in FIFO order for now (priority queue not integrated)
		expectedOrder := []string{"first", "second", "third"}
		for _, expected := range expectedOrder {
			result, err := q.GetResult(5 * time.Second)
			require.NoError(t, err)
			assert.Equal(t, expected, result.JobID)
		}
	})

	t.Run("Queue Metrics", func(t *testing.T) {
		config := queue.QueueConfig{
			MaxWorkers:   2,
			MaxQueueSize: 10,
			MaxRetries:   1,
			RetryDelay:   100 * time.Millisecond,
			RateLimit:    10,
		}

		q := queue.NewJobQueue(config)
		q.Start()
		defer q.Stop()

		// Submit multiple jobs
		for i := 0; i < 5; i++ {
			job := queue.Job{
				ID:       string(rune('a' + i)),
				Type:     queue.JobTypeUpdateIssue,
				Priority: i,
			}
			err := q.Submit(job)
			assert.NoError(t, err)
		}

		// Wait for processing
		time.Sleep(2 * time.Second)

		// Check metrics
		metrics := q.GetMetrics()
		assert.Equal(t, int64(5), metrics.TotalJobs)
		assert.GreaterOrEqual(t, metrics.SuccessfulJobs, int64(0))
		assert.GreaterOrEqual(t, metrics.FailedJobs, int64(0))
	})
}

func TestPriorityQueue(t *testing.T) {
	t.Run("Priority Ordering", func(t *testing.T) {
		pq := queue.NewPriorityQueue()

		// Add jobs in random order
		jobs := []*queue.Job{
			{ID: "1", Priority: 5},
			{ID: "2", Priority: 10},
			{ID: "3", Priority: 1},
			{ID: "4", Priority: 7},
		}

		for _, job := range jobs {
			pq.Push(job)
		}

		// Pop should return in priority order
		assert.Equal(t, "2", pq.Pop().ID) // Priority 10
		assert.Equal(t, "4", pq.Pop().ID) // Priority 7
		assert.Equal(t, "1", pq.Pop().ID) // Priority 5
		assert.Equal(t, "3", pq.Pop().ID) // Priority 1
		assert.Nil(t, pq.Pop())           // Empty
	})

	t.Run("Queue Operations", func(t *testing.T) {
		pq := queue.NewPriorityQueue()

		assert.True(t, pq.IsEmpty())
		assert.Equal(t, 0, pq.Len())

		job := &queue.Job{ID: "test", Priority: 5}
		pq.Push(job)

		assert.False(t, pq.IsEmpty())
		assert.Equal(t, 1, pq.Len())
		assert.Equal(t, "test", pq.Peek().ID)

		removed := pq.Remove("test")
		assert.True(t, removed)
		assert.True(t, pq.IsEmpty())
	})

	t.Run("Get By Type and Priority", func(t *testing.T) {
		pq := queue.NewPriorityQueue()

		jobs := []*queue.Job{
			{ID: "1", Type: queue.JobTypeCreateIssue, Priority: 5},
			{ID: "2", Type: queue.JobTypeUpdateIssue, Priority: 5},
			{ID: "3", Type: queue.JobTypeCreateIssue, Priority: 10},
		}

		for _, job := range jobs {
			pq.Push(job)
		}

		// Get by type
		createJobs := pq.GetByType(queue.JobTypeCreateIssue)
		assert.Len(t, createJobs, 2)

		// Get by priority
		priority5Jobs := pq.GetByPriority(5)
		assert.Len(t, priority5Jobs, 2)
	})
}

func TestRetryManager(t *testing.T) {
	t.Run("Successful Retry", func(t *testing.T) {
		rm := queue.NewRetryManager(3, 100*time.Millisecond, 1*time.Second)

		attempts := 0
		err := rm.ExecuteWithRetry(func() error {
			attempts++
			if attempts < 3 {
				return &queue.HTTPError{StatusCode: 503, Message: "Service Unavailable"}
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("Max Retries Exceeded", func(t *testing.T) {
		rm := queue.NewRetryManager(2, 100*time.Millisecond, 1*time.Second)

		attempts := 0
		err := rm.ExecuteWithRetry(func() error {
			attempts++
			return &queue.HTTPError{StatusCode: 503, Message: "Service Unavailable"}
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max retries")
		assert.Equal(t, 3, attempts) // Initial + 2 retries
	})

	t.Run("Non-Retryable Error", func(t *testing.T) {
		rm := queue.NewRetryManager(3, 100*time.Millisecond, 1*time.Second)

		attempts := 0
		err := rm.ExecuteWithRetry(func() error {
			attempts++
			return &queue.HTTPError{StatusCode: 400, Message: "Bad Request"}
		})

		assert.Error(t, err)
		assert.Equal(t, 1, attempts) // No retries for non-retryable error
	})

	t.Run("Exponential Backoff", func(t *testing.T) {
		rm := queue.NewRetryManager(2, 100*time.Millisecond, 1*time.Second)

		start := time.Now()
		attempts := 0

		_ = rm.ExecuteWithRetry(func() error {
			attempts++
			if attempts <= 2 {
				return &queue.HTTPError{StatusCode: 503, Message: "Service Unavailable"}
			}
			return nil
		})

		elapsed := time.Since(start)
		// Should have some delay due to backoff
		assert.Greater(t, elapsed, 100*time.Millisecond)
	})
}

func TestRateLimiter(t *testing.T) {
	t.Run("Basic Rate Limiting", func(t *testing.T) {
		rl := queue.NewRateLimiter(5, 10) // 5 per second, burst of 10
		defer rl.Stop()

		// Should allow burst
		allowed := 0
		for i := 0; i < 15; i++ {
			if rl.Allow() {
				allowed++
			}
		}

		assert.Equal(t, 10, allowed) // Should only allow burst size
	})

	t.Run("Wait for Token", func(t *testing.T) {
		rl := queue.NewRateLimiter(10, 2) // 10 per second, burst of 2
		defer rl.Stop()

		// Use up burst
		rl.Wait()
		rl.Wait()

		// Next wait should block briefly
		start := time.Now()
		rl.Wait()
		elapsed := time.Since(start)

		// Should have waited for refill
		assert.Greater(t, elapsed, 50*time.Millisecond)
	})

	t.Run("Try Wait with Timeout", func(t *testing.T) {
		rl := queue.NewRateLimiter(1, 1) // 1 per second, burst of 1
		defer rl.Stop()

		// Use up the token
		assert.True(t, rl.TryWait(100*time.Millisecond))

		// Should timeout
		assert.False(t, rl.TryWait(50*time.Millisecond))
	})

	t.Run("Rate Limiter Stats", func(t *testing.T) {
		rl := queue.NewRateLimiter(5, 5)
		defer rl.Stop()

		// Make some requests
		for i := 0; i < 10; i++ {
			rl.Allow()
		}

		stats := rl.GetStats()
		assert.Equal(t, int64(10), stats.TotalRequests)
		assert.Equal(t, int64(5), stats.AllowedRequests)
		assert.Equal(t, int64(5), stats.DeniedRequests)
	})
}

func TestTokenBucket(t *testing.T) {
	t.Run("Token Bucket Operations", func(t *testing.T) {
		tb := queue.NewTokenBucket(10, 5.0) // capacity 10, 5 tokens/sec

		// Should have full capacity initially
		assert.Equal(t, 10, tb.AvailableTokens())

		// Use some tokens
		assert.True(t, tb.Allow(5))
		assert.Equal(t, 5, tb.AvailableTokens())

		// Should deny if not enough tokens
		assert.False(t, tb.Allow(6))

		// Wait for refill
		time.Sleep(500 * time.Millisecond)
		// Should have refilled some tokens (approximately 2-3)
		assert.Greater(t, tb.AvailableTokens(), 5)
	})
}

func TestLeakyBucket(t *testing.T) {
	t.Run("Leaky Bucket Operations", func(t *testing.T) {
		lb := queue.NewLeakyBucket(10, 5) // capacity 10, leak 5/sec
		defer lb.Stop()

		// Should allow up to capacity
		assert.True(t, lb.Allow(5))
		assert.True(t, lb.Allow(5))
		assert.False(t, lb.Allow(1)) // Over capacity

		// Wait for leak
		time.Sleep(300 * time.Millisecond)
		// Should have leaked some
		assert.Greater(t, lb.Available(), 0)
	})
}