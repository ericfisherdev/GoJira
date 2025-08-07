package middleware

import (
	"net/http"
	"runtime/debug"

	appErrors "github.com/ericfisherdev/GoJira/pkg/errors"
	"github.com/go-chi/render"
)

// ErrorResponse represents an HTTP error response
type ErrorResponse struct {
	Success   bool                   `json:"success"`
	Error     map[string]interface{} `json:"error"`
	Timestamp string                 `json:"timestamp"`
	RequestID string                 `json:"request_id,omitempty"`
}

func (e *ErrorResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// ErrorHandler handles panics and errors in HTTP handlers
func ErrorHandler() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger := RequestLogger(r)
					
					// Log the panic with stack trace
					logger.Error().
						Interface("panic", err).
						Bytes("stack", debug.Stack()).
						Msg("Panic recovered in HTTP handler")

					// Return internal server error
					errorResp := &ErrorResponse{
						Success: false,
						Error: map[string]interface{}{
							"code":    appErrors.ErrCodeInternal,
							"message": "An internal server error occurred",
						},
					}

					render.Status(r, http.StatusInternalServerError)
					render.Render(w, r, errorResp)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RenderError renders an error response
func RenderError(w http.ResponseWriter, r *http.Request, err error) {
	logger := RequestLogger(r)
	
	// Log the error
	if appErr, ok := appErrors.IsAppError(err); ok {
		if appErr.HTTPStatus >= 500 {
			logger.Error().Err(err).
				Str("code", string(appErr.Code)).
				Interface("details", appErr.Details).
				Msg("Application error occurred")
		} else {
			logger.Warn().Err(err).
				Str("code", string(appErr.Code)).
				Interface("details", appErr.Details).
				Msg("Client error occurred")
		}
	} else {
		logger.Error().Err(err).Msg("Unexpected error occurred")
	}

	// Get HTTP status code
	statusCode := appErrors.GetHTTPStatus(err)
	
	// Create error response
	errorResp := &ErrorResponse{
		Success: false,
		Error:   appErrors.ToMap(err),
	}

	// Add request ID if available
	if requestID := GetRequestID(r); requestID != "" {
		errorResp.RequestID = requestID
	}

	render.Status(r, statusCode)
	render.Render(w, r, errorResp)
}

// GetRequestID extracts request ID from context
func GetRequestID(r *http.Request) string {
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		return requestID
	}
	
	// Try to get from middleware
	if ctx := r.Context(); ctx != nil {
		if requestID := ctx.Value("requestID"); requestID != nil {
			if id, ok := requestID.(string); ok {
				return id
			}
		}
	}
	
	return ""
}