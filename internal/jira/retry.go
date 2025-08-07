package jira

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries      int
	BaseDelay       time.Duration
	MaxDelay        time.Duration
	BackoffMultiplier float64
	Jitter          bool
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:        3,
		BaseDelay:         1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:           true,
	}
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	Err       error
	Retryable bool
	Delay     time.Duration
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

// IsRetryable determines if an error is retryable
func IsRetryable(err error, statusCode int) bool {
	if err == nil {
		return false
	}

	// Network errors are usually retryable
	if isNetworkError(err) {
		return true
	}

	// HTTP status codes that are retryable
	switch statusCode {
	case http.StatusRequestTimeout,          // 408
		http.StatusTooManyRequests,          // 429
		http.StatusInternalServerError,      // 500
		http.StatusBadGateway,               // 502
		http.StatusServiceUnavailable,       // 503
		http.StatusGatewayTimeout:           // 504
		return true
	}

	// Jira-specific errors that might be retryable
	errMsg := strings.ToLower(err.Error())
	retryableMessages := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"rate limit",
		"server overloaded",
	}

	for _, msg := range retryableMessages {
		if strings.Contains(errMsg, msg) {
			return true
		}
	}

	return false
}

// isNetworkError checks if an error is a network-related error
func isNetworkError(err error) bool {
	errMsg := strings.ToLower(err.Error())
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"no such host",
		"network unreachable",
		"temporary failure in name resolution",
	}

	for _, netErr := range networkErrors {
		if strings.Contains(errMsg, netErr) {
			return true
		}
	}

	return false
}

// WithRetry executes a function with retry logic
func WithRetry(ctx context.Context, config *RetryConfig, operation func() (interface{}, int, error)) (interface{}, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	var lastStatusCode int

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Execute the operation
		result, statusCode, err := operation()
		lastErr = err
		lastStatusCode = statusCode

		// Success case
		if err == nil {
			if attempt > 0 {
				log.Info().
					Int("attempt", attempt+1).
					Int("total_attempts", config.MaxRetries+1).
					Msg("Operation succeeded after retry")
			}
			return result, nil
		}

		// Check if error is retryable
		if !IsRetryable(err, statusCode) {
			log.Debug().
				Err(err).
				Int("status_code", statusCode).
				Msg("Error is not retryable")
			return nil, err
		}

		// Don't retry on the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate delay for next attempt
		delay := calculateDelay(attempt, config)

		// Log retry attempt
		log.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_attempts", config.MaxRetries+1).
			Int("status_code", statusCode).
			Dur("retry_delay", delay).
			Msg("Operation failed, retrying")

		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	// All retries exhausted
	log.Error().
		Err(lastErr).
		Int("status_code", lastStatusCode).
		Int("total_attempts", config.MaxRetries+1).
		Msg("Operation failed after all retry attempts")

	return nil, fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(attempt int, config *RetryConfig) time.Duration {
	// Exponential backoff
	delay := float64(config.BaseDelay) * math.Pow(config.BackoffMultiplier, float64(attempt))
	
	// Apply maximum delay limit
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	duration := time.Duration(delay)

	// Add jitter if enabled
	if config.Jitter {
		jitter := time.Duration(float64(duration) * 0.1 * (2.0*rand.Float64() - 1.0))
		duration += jitter
		
		// Ensure delay doesn't go below base delay
		if duration < config.BaseDelay {
			duration = config.BaseDelay
		}
	}

	return duration
}

// RetryableHTTPOperation creates a retryable HTTP operation
func RetryableHTTPOperation(ctx context.Context, client *Client, method, endpoint string, body interface{}) (interface{}, error) {
	config := DefaultRetryConfig()
	
	operation := func() (interface{}, int, error) {
		resp, err := client.doRequest(ctx, method, endpoint, body)
		if err != nil {
			return nil, 0, err
		}

		statusCode := resp.StatusCode()
		
		if err := client.handleErrorResponse(resp); err != nil {
			return nil, statusCode, err
		}

		return resp, statusCode, nil
	}

	return WithRetry(ctx, config, operation)
}

// RetryableOperation creates a generic retryable operation
func RetryableOperation[T any](ctx context.Context, config *RetryConfig, operation func() (T, error)) (T, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	wrappedOp := func() (interface{}, int, error) {
		result, err := operation()
		return result, 0, err // No status code for generic operations
	}

	result, err := WithRetry(ctx, config, wrappedOp)
	if err != nil {
		var zero T
		return zero, err
	}

	if result == nil {
		var zero T
		return zero, fmt.Errorf("operation returned nil result")
	}

	return result.(T), nil
}