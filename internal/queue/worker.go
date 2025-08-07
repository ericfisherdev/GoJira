package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Worker struct {
	id          int
	queue       *JobQueue
	retryMgr    *RetryManager
	rateLimiter <-chan time.Time
}

func NewWorker(id int, queue *JobQueue) *Worker {
	return &Worker{
		id:    id,
		queue: queue,
		retryMgr: NewRetryManager(
			queue.config.MaxRetries,
			queue.config.RetryDelay,
			30*time.Second,
		),
	}
}

func (w *Worker) Start(wg *sync.WaitGroup, rateLimiter <-chan time.Time) {
	defer wg.Done()
	w.rateLimiter = rateLimiter

	log.Info().Int("workerId", w.id).Msg("Worker started")

	for {
		select {
		case job, ok := <-w.queue.jobs:
			if !ok {
				log.Info().Int("workerId", w.id).Msg("Worker stopping - jobs channel closed")
				return
			}

			// Wait for rate limiter (with timeout to prevent blocking)
			select {
			case <-w.rateLimiter:
				// Process job
				w.processJob(job)
			case <-time.After(100 * time.Millisecond):
				// Process anyway if rate limiter is slow
				w.processJob(job)
			}

		case <-w.queue.stopCh:
			log.Info().Int("workerId", w.id).Msg("Worker stopping - stop signal received")
			return
		}
	}
}

func (w *Worker) processJob(job Job) {
	startTime := time.Now()

	log.Debug().
		Int("workerId", w.id).
		Str("jobId", job.ID).
		Str("type", string(job.Type)).
		Msg("Processing job")

	var result interface{}
	var err error

	// Execute with retry
	err = w.retryMgr.ExecuteWithRetry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err = w.executeJob(ctx, job)
		return err
	})

	duration := time.Since(startTime)
	success := err == nil

	// Update metrics
	w.queue.updateMetrics(func(m *QueueMetrics) {
		if success {
			m.SuccessfulJobs++
		} else {
			m.FailedJobs++
		}
		if job.Retries > 0 {
			m.RetryCount += int64(job.Retries)
		}
	})

	// Send result
	jobResult := JobResult{
		JobID:    job.ID,
		Success:  success,
		Result:   result,
		Error:    err,
		Duration: duration,
	}

	select {
	case w.queue.results <- jobResult:
		log.Debug().
			Int("workerId", w.id).
			Str("jobId", job.ID).
			Bool("success", success).
			Dur("duration", duration).
			Msg("Job completed")
	case <-time.After(5 * time.Second):
		log.Error().
			Int("workerId", w.id).
			Str("jobId", job.ID).
			Msg("Failed to send job result - results channel full")
	}
}

func (w *Worker) executeJob(ctx context.Context, job Job) (interface{}, error) {
	// Job execution logic based on job type
	switch job.Type {
	case JobTypeCreateIssue:
		return w.executeCreateIssue(ctx, job)
	case JobTypeUpdateIssue:
		return w.executeUpdateIssue(ctx, job)
	case JobTypeTransition:
		return w.executeTransition(ctx, job)
	case JobTypeBulkUpdate:
		return w.executeBulkUpdate(ctx, job)
	case JobTypeSprintMove:
		return w.executeSprintMove(ctx, job)
	case JobTypeWorkflowChange:
		return w.executeWorkflowChange(ctx, job)
	default:
		return nil, fmt.Errorf("unknown job type: %s", job.Type)
	}
}

func (w *Worker) executeCreateIssue(ctx context.Context, job Job) (interface{}, error) {
	// Placeholder for create issue logic
	// In real implementation, this would call the Jira client
	log.Info().
		Str("jobId", job.ID).
		Msg("Executing create issue job")

	// Simulate processing
	select {
	case <-time.After(100 * time.Millisecond):
		return map[string]string{"issueKey": "PROJ-123"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *Worker) executeUpdateIssue(ctx context.Context, job Job) (interface{}, error) {
	// Placeholder for update issue logic
	log.Info().
		Str("jobId", job.ID).
		Msg("Executing update issue job")

	// Simulate processing
	select {
	case <-time.After(100 * time.Millisecond):
		return map[string]bool{"updated": true}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *Worker) executeTransition(ctx context.Context, job Job) (interface{}, error) {
	// Placeholder for transition logic
	log.Info().
		Str("jobId", job.ID).
		Msg("Executing transition job")

	// Simulate processing
	select {
	case <-time.After(150 * time.Millisecond):
		return map[string]string{"status": "Done"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *Worker) executeBulkUpdate(ctx context.Context, job Job) (interface{}, error) {
	// Placeholder for bulk update logic
	log.Info().
		Str("jobId", job.ID).
		Msg("Executing bulk update job")

	// Simulate processing
	select {
	case <-time.After(500 * time.Millisecond):
		return map[string]int{"updated": 10}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *Worker) executeSprintMove(ctx context.Context, job Job) (interface{}, error) {
	// Placeholder for sprint move logic
	log.Info().
		Str("jobId", job.ID).
		Msg("Executing sprint move job")

	// Simulate processing
	select {
	case <-time.After(200 * time.Millisecond):
		return map[string]int{"moved": 5}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *Worker) executeWorkflowChange(ctx context.Context, job Job) (interface{}, error) {
	// Placeholder for workflow change logic
	log.Info().
		Str("jobId", job.ID).
		Msg("Executing workflow change job")

	// Simulate processing
	select {
	case <-time.After(300 * time.Millisecond):
		return map[string]string{"workflow": "Updated"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}