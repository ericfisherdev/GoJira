package errors

import (
	"fmt"
	"net/http"
	"testing"
)

func TestAppError(t *testing.T) {
	// Test New
	err := New(ErrCodeInvalidInput, "test message")
	if err.Code != ErrCodeInvalidInput {
		t.Errorf("Expected code %s, got %s", ErrCodeInvalidInput, err.Code)
	}
	if err.Message != "test message" {
		t.Errorf("Expected message 'test message', got %s", err.Message)
	}
	if err.HTTPStatus != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, err.HTTPStatus)
	}

	// Test Error() method
	expectedMsg := "[INVALID_INPUT] test message"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %s, got %s", expectedMsg, err.Error())
	}

	// Test WithCause
	cause := fmt.Errorf("underlying error")
	errWithCause := err.WithCause(cause)
	if errWithCause.Cause != cause {
		t.Error("WithCause() did not set cause correctly")
	}

	expectedMsg = "[INVALID_INPUT] test message: underlying error"
	if errWithCause.Error() != expectedMsg {
		t.Errorf("Expected error with cause %s, got %s", expectedMsg, errWithCause.Error())
	}

	// Test WithDetails
	details := map[string]interface{}{
		"field": "username",
		"value": "invalid",
	}
	errWithDetails := err.WithDetails(details)
	if errWithDetails.Details == nil {
		t.Error("WithDetails() did not set details")
	}
	if errWithDetails.Details["field"] != "username" {
		t.Error("WithDetails() did not set details correctly")
	}

	// Test WithDetail
	errWithDetail := err.WithDetail("key", "value")
	if errWithDetail.Details["key"] != "value" {
		t.Error("WithDetail() did not set detail correctly")
	}
}

func TestNewf(t *testing.T) {
	err := Newf(ErrCodeNotFound, "resource %s not found", "user")
	expectedMsg := "resource user not found"
	if err.Message != expectedMsg {
		t.Errorf("Expected message %s, got %s", expectedMsg, err.Message)
	}
}

func TestWrap(t *testing.T) {
	cause := fmt.Errorf("original error")
	err := Wrap(cause, ErrCodeInternal, "wrapped message")
	
	if err.Cause != cause {
		t.Error("Wrap() did not set cause correctly")
	}
	if err.Message != "wrapped message" {
		t.Error("Wrap() did not set message correctly")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	// Test InvalidInput
	err := InvalidInput("invalid data")
	if err.Code != ErrCodeInvalidInput {
		t.Error("InvalidInput() did not set correct code")
	}

	// Test NotFound
	err = NotFound("user")
	if err.Code != ErrCodeNotFound {
		t.Error("NotFound() did not set correct code")
	}
	if err.Message != "user not found" {
		t.Error("NotFound() did not set correct message")
	}

	// Test Unauthorized
	err = Unauthorized("invalid token")
	if err.Code != ErrCodeUnauthorized {
		t.Error("Unauthorized() did not set correct code")
	}

	// Test AuthFailed
	err = AuthFailed("invalid credentials")
	if err.Code != ErrCodeAuthFailed {
		t.Error("AuthFailed() did not set correct code")
	}
}

func TestHTTPStatusMapping(t *testing.T) {
	tests := []struct {
		code   ErrorCode
		status int
	}{
		{ErrCodeInvalidInput, http.StatusBadRequest},
		{ErrCodeUnauthorized, http.StatusUnauthorized},
		{ErrCodeForbidden, http.StatusForbidden},
		{ErrCodeNotFound, http.StatusNotFound},
		{ErrCodeConflict, http.StatusConflict},
		{ErrCodeRateLimited, http.StatusTooManyRequests},
		{ErrCodeJiraConnection, http.StatusBadGateway},
		{ErrCodeInternal, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			status := getHTTPStatusForCode(tt.code)
			if status != tt.status {
				t.Errorf("Expected status %d for code %s, got %d", tt.status, tt.code, status)
			}
		})
	}
}

func TestIsAppError(t *testing.T) {
	// Test with AppError
	appErr := New(ErrCodeInvalidInput, "test")
	detectedErr, ok := IsAppError(appErr)
	if !ok {
		t.Error("IsAppError() should have detected AppError")
	}
	if detectedErr != appErr {
		t.Error("IsAppError() returned wrong error")
	}

	// Test with regular error
	regularErr := fmt.Errorf("regular error")
	_, ok = IsAppError(regularErr)
	if ok {
		t.Error("IsAppError() should not have detected regular error as AppError")
	}
}

func TestGetHTTPStatus(t *testing.T) {
	// Test with AppError
	appErr := New(ErrCodeNotFound, "test")
	status := GetHTTPStatus(appErr)
	if status != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, status)
	}

	// Test with regular error
	regularErr := fmt.Errorf("regular error")
	status = GetHTTPStatus(regularErr)
	if status != http.StatusInternalServerError {
		t.Errorf("Expected status %d for regular error, got %d", http.StatusInternalServerError, status)
	}
}

func TestToMap(t *testing.T) {
	// Test with AppError
	appErr := New(ErrCodeInvalidInput, "test message").WithDetails(map[string]interface{}{
		"field": "username",
	})
	
	result := ToMap(appErr)
	if result["code"] != ErrCodeInvalidInput {
		t.Error("ToMap() did not include correct code")
	}
	if result["message"] != "test message" {
		t.Error("ToMap() did not include correct message")
	}
	if result["details"] == nil {
		t.Error("ToMap() did not include details")
	}

	// Test with internal error (should hide details)
	internalErr := Internal(fmt.Errorf("sensitive error"), "generic message")
	result = ToMap(internalErr)
	if result["details"] != nil {
		t.Error("ToMap() should hide details for internal errors")
	}

	// Test with regular error
	regularErr := fmt.Errorf("regular error")
	result = ToMap(regularErr)
	if result["code"] != ErrCodeInternal {
		t.Error("ToMap() should return internal error code for regular errors")
	}
}