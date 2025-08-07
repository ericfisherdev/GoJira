package queue

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"time"

	"github.com/rs/zerolog/log"
)

type RetryManager struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

func NewRetryManager(maxRetries int, baseDelay, maxDelay time.Duration) *RetryManager {
	return &RetryManager{
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   maxDelay,
	}
}

func (rm *RetryManager) ExecuteWithRetry(fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= rm.maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			log.Debug().
				Err(err).
				Int("attempt", attempt).
				Msg("Error is not retryable")
			return err
		}

		if attempt < rm.maxRetries {
			delay := rm.calculateDelay(attempt)
			log.Warn().
				Err(err).
				Int("attempt", attempt+1).
				Int("maxRetries", rm.maxRetries).
				Dur("delay", delay).
				Msg("Retrying after error")
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", rm.maxRetries, lastErr)
}

func (rm *RetryManager) ExecuteWithRetryContext(fn func() error, shouldRetry func(error) bool) error {
	var lastErr error

	for attempt := 0; attempt <= rm.maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Use custom retry logic if provided
		if shouldRetry != nil && !shouldRetry(err) {
			return err
		} else if shouldRetry == nil && !isRetryableError(err) {
			return err
		}

		if attempt < rm.maxRetries {
			delay := rm.calculateDelay(attempt)
			log.Warn().
				Err(err).
				Int("attempt", attempt+1).
				Int("maxRetries", rm.maxRetries).
				Dur("delay", delay).
				Msg("Retrying with custom logic")
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", rm.maxRetries, lastErr)
}

func (rm *RetryManager) calculateDelay(attempt int) time.Duration {
	// Exponential backoff with jitter
	delay := rm.baseDelay * time.Duration(math.Pow(2, float64(attempt)))

	// Add jitter (±25%)
	jitterRange := float64(delay) * 0.5
	jitter := time.Duration((rand.Float64() - 0.5) * jitterRange)
	delay += jitter

	// Cap at max delay
	if delay > rm.maxDelay {
		delay = rm.maxDelay
	}

	return delay
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for retryable HTTP status codes
	if httpErr, ok := err.(*HTTPError); ok {
		switch httpErr.StatusCode {
		case 429: // Too Many Requests
			return true
		case 502, 503, 504: // Server errors
			return true
		case 408: // Request Timeout
			return true
		default:
			return false
		}
	}

	// Check for network errors
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary() || netErr.Timeout()
	}

	// Check for specific error messages
	errMsg := err.Error()
	retryableMessages := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"too many requests",
		"service unavailable",
	}

	for _, msg := range retryableMessages {
		if contains(errMsg, msg) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}

type RetryPolicy struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	JitterPercent float64
}

func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		Multiplier:    2.0,
		JitterPercent: 0.25,
	}
}

func (rp *RetryPolicy) Execute(fn func() error) error {
	rm := NewRetryManager(rp.MaxAttempts, rp.InitialDelay, rp.MaxDelay)
	return rm.ExecuteWithRetry(fn)
}

type RetryableFunc func() (interface{}, error)

func RetryWithResult(fn RetryableFunc, policy *RetryPolicy) (interface{}, error) {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	var result interface{}

	rm := NewRetryManager(policy.MaxAttempts, policy.InitialDelay, policy.MaxDelay)

	err := rm.ExecuteWithRetry(func() error {
		res, err := fn()
		if err != nil {
			return err
		}
		result = res
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}