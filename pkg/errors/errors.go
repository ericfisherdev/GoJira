package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode represents different types of errors
type ErrorCode string

const (
	// General errors
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeConflict     ErrorCode = "CONFLICT"
	ErrCodeRateLimited  ErrorCode = "RATE_LIMITED"

	// Authentication errors
	ErrCodeAuthFailed        ErrorCode = "AUTH_FAILED"
	ErrCodeAuthExpired       ErrorCode = "AUTH_EXPIRED"
	ErrCodeAuthInvalid       ErrorCode = "AUTH_INVALID"
	ErrCodeAuthUnsupported   ErrorCode = "AUTH_UNSUPPORTED"

	// Jira API errors
	ErrCodeJiraAPI          ErrorCode = "JIRA_API_ERROR"
	ErrCodeJiraConnection   ErrorCode = "JIRA_CONNECTION_ERROR"
	ErrCodeJiraPermission   ErrorCode = "JIRA_PERMISSION_ERROR"
	ErrCodeJiraNotFound     ErrorCode = "JIRA_NOT_FOUND"
	ErrCodeJiraValidation   ErrorCode = "JIRA_VALIDATION_ERROR"

	// Configuration errors
	ErrCodeConfigInvalid    ErrorCode = "CONFIG_INVALID"
	ErrCodeConfigMissing    ErrorCode = "CONFIG_MISSING"
)

// AppError represents an application error with structured information
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Cause      error                  `json:"-"`
	HTTPStatus int                    `json:"-"`
	Internal   bool                   `json:"-"` // Whether to hide details from client
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap implements the unwrap interface for error chains
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithCause adds a cause to the error
func (e *AppError) WithCause(cause error) *AppError {
	return &AppError{
		Code:       e.Code,
		Message:    e.Message,
		Details:    e.Details,
		Cause:      cause,
		HTTPStatus: e.HTTPStatus,
		Internal:   e.Internal,
	}
}

// WithDetails adds details to the error
func (e *AppError) WithDetails(details map[string]interface{}) *AppError {
	return &AppError{
		Code:       e.Code,
		Message:    e.Message,
		Details:    details,
		Cause:      e.Cause,
		HTTPStatus: e.HTTPStatus,
		Internal:   e.Internal,
	}
}

// WithDetail adds a single detail to the error
func (e *AppError) WithDetail(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// New creates a new application error
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: getHTTPStatusForCode(code),
	}
}

// Newf creates a new application error with formatted message
func Newf(code ErrorCode, format string, args ...interface{}) *AppError {
	return New(code, fmt.Sprintf(format, args...))
}

// Wrap wraps an existing error with application error information
func Wrap(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Cause:      err,
		HTTPStatus: getHTTPStatusForCode(code),
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *AppError {
	return Wrap(err, code, fmt.Sprintf(format, args...))
}

// Internal creates an internal error that hides details from clients
func Internal(err error, message string) *AppError {
	return &AppError{
		Code:       ErrCodeInternal,
		Message:    message,
		Cause:      err,
		HTTPStatus: http.StatusInternalServerError,
		Internal:   true,
	}
}

// InvalidInput creates an invalid input error
func InvalidInput(message string) *AppError {
	return New(ErrCodeInvalidInput, message)
}

// NotFound creates a not found error
func NotFound(resource string) *AppError {
	return New(ErrCodeNotFound, fmt.Sprintf("%s not found", resource))
}

// Unauthorized creates an unauthorized error
func Unauthorized(message string) *AppError {
	return New(ErrCodeUnauthorized, message)
}

// Forbidden creates a forbidden error
func Forbidden(message string) *AppError {
	return New(ErrCodeForbidden, message)
}

// JiraAPI creates a Jira API error
func JiraAPI(err error, message string) *AppError {
	return Wrap(err, ErrCodeJiraAPI, message)
}

// AuthFailed creates an authentication failed error
func AuthFailed(message string) *AppError {
	return New(ErrCodeAuthFailed, message)
}

// getHTTPStatusForCode returns the appropriate HTTP status code for an error code
func getHTTPStatusForCode(code ErrorCode) int {
	switch code {
	case ErrCodeInvalidInput, ErrCodeJiraValidation:
		return http.StatusBadRequest
	case ErrCodeUnauthorized, ErrCodeAuthFailed, ErrCodeAuthExpired, ErrCodeAuthInvalid:
		return http.StatusUnauthorized
	case ErrCodeForbidden, ErrCodeJiraPermission:
		return http.StatusForbidden
	case ErrCodeNotFound, ErrCodeJiraNotFound:
		return http.StatusNotFound
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeRateLimited:
		return http.StatusTooManyRequests
	case ErrCodeJiraConnection:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// GetHTTPStatus returns the HTTP status code for an error
func GetHTTPStatus(err error) int {
	if appErr, ok := IsAppError(err); ok {
		return appErr.HTTPStatus
	}
	return http.StatusInternalServerError
}

// ShouldHideDetails returns whether error details should be hidden from client
func ShouldHideDetails(err error) bool {
	if appErr, ok := IsAppError(err); ok {
		return appErr.Internal
	}
	return true // Hide details for non-AppErrors by default
}

// ToMap converts an error to a map for JSON serialization
func ToMap(err error) map[string]interface{} {
	if appErr, ok := IsAppError(err); ok {
		result := map[string]interface{}{
			"code":    appErr.Code,
			"message": appErr.Message,
		}
		
		if !appErr.Internal && appErr.Details != nil {
			result["details"] = appErr.Details
		}
		
		return result
	}

	// For non-AppErrors, return generic error
	return map[string]interface{}{
		"code":    ErrCodeInternal,
		"message": "An internal error occurred",
	}
}