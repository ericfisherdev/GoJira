package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger returns a request logging middleware
func Logger() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer that captures status code and response size
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Get or create request ID
			requestID := middleware.GetReqID(r.Context())
			if requestID == "" {
				requestID = generateRequestID()
			}

			// Add request context to logger
			logger := log.With().
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("query", r.URL.RawQuery).
				Str("user_agent", r.UserAgent()).
				Str("remote_addr", r.RemoteAddr).
				Logger()

			// Add logger to request context
			r = r.WithContext(logger.WithContext(r.Context()))

			// Log request start
			logger.Info().Msg("Request started")

			// Process request
			next.ServeHTTP(ww, r)

			// Calculate duration
			duration := time.Since(start)

			// Log request completion
			logEvent := logger.Info().
				Int("status", ww.Status()).
				Int64("bytes", int64(ww.BytesWritten())).
				Dur("duration", duration).
				Dur("duration_ms", duration/time.Millisecond)

			// Add error information if request failed
			if ww.Status() >= 400 {
				logEvent = logEvent.Str("level", "warn")
			}

			logEvent.Msg("Request completed")
		})
	}
}

// RequestLogger returns a logger with request context
func RequestLogger(r *http.Request) zerolog.Logger {
	return zerolog.Ctx(r.Context()).With().Logger()
}

// generateRequestID generates a simple request ID
func generateRequestID() string {
	return fmt.Sprintf("%d", middleware.NextRequestID())
}