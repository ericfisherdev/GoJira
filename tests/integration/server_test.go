package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ericfisherdev/GoJira/internal/api/handlers"
	"github.com/ericfisherdev/GoJira/internal/api/routes"
	"github.com/ericfisherdev/GoJira/internal/auth"
	"github.com/ericfisherdev/GoJira/internal/server"
)

func setupTestServer(t *testing.T) *server.Server {
	config := &server.Config{
		Port:        "8080",
		Mode:        "test",
		EnableCORS:  true,
		LogRequests: false,
	}

	srv := server.New(config)
	
	// Initialize auth manager
	authManager := auth.NewManager(nil)
	handlers.SetAuthManager(authManager)
	
	routes.SetupRoutes(srv.Router())
	return srv
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Create test request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Execute request
	srv.Router().ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if len(body) == 0 {
		t.Error("Expected response body, got empty")
	}

	// Parse JSON response
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}
}

func TestReadinessEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Create test request
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	// Execute request
	srv.Router().ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCORSHeaders(t *testing.T) {
	// Create test server with CORS enabled
	config := &server.Config{
		Port:        "8080",
		Mode:        "test",
		EnableCORS:  true,
		LogRequests: false,
	}

	srv := server.New(config)
	routes.SetupRoutes(srv.Router())

	// Create OPTIONS request (CORS preflight)
	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()

	// Execute request
	srv.Router().ServeHTTP(w, req)

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("Expected CORS Allow-Origin header, got none")
	}
}

func TestAPIEndpoints(t *testing.T) {
	config := &server.Config{
		Port:        "8080",
		Mode:        "test",
		EnableCORS:  true,
		LogRequests: false,
	}

	srv := server.New(config)
	routes.SetupRoutes(srv.Router())

	tests := []struct {
		method   string
		path     string
		expected int
	}{
		{"GET", "/api/v1/auth/status", http.StatusOK},
		{"POST", "/api/v1/auth/connect", http.StatusBadRequest}, // Should fail without body
		{"GET", "/api/v1/issues/TEST-1", http.StatusUnauthorized}, // Should fail without auth
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			if w.Code != test.expected {
				t.Errorf("Expected status %d, got %d", test.expected, w.Code)
			}
		})
	}
}

func TestMiddlewareChain(t *testing.T) {
	config := &server.Config{
		Port:        "8080",
		Mode:        "test",
		EnableCORS:  true,
		LogRequests: true, // Enable request logging for this test
	}

	srv := server.New(config)

	// Test that middleware is applied
	// This is a basic test - in a real scenario we'd test each middleware individually
	router := srv.Router()
	
	// Check that router is properly configured
	if router == nil {
		t.Error("Expected router to be configured, got nil")
	}

	// Check that we can walk the middleware chain
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Add routes
	routes.SetupRoutes(router)

	// This should not panic and should apply all middleware
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 after middleware chain, got %d", w.Code)
	}
}

func TestAuthStatusEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Test auth status without authentication
	req := httptest.NewRequest("GET", "/api/v1/auth/status", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	body, _ := io.ReadAll(w.Body)
	if err := json.Unmarshal(body, &response); err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if response["connected"] != false {
		t.Errorf("Expected connected to be false, got %v", response["connected"])
	}
}

func TestConnectEndpointValidation(t *testing.T) {
	srv := setupTestServer(t)

	tests := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
	}{
		{
			name: "missing type",
			payload: map[string]interface{}{
				"credentials": map[string]string{"token": "test"},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing credentials",
			payload: map[string]interface{}{
				"type": "api_token",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid auth type",
			payload: map[string]interface{}{
				"type":        "invalid_type",
				"credentials": map[string]string{"token": "test"},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "valid structure but will fail auth",
			payload: map[string]interface{}{
				"type": "api_token",
				"credentials": map[string]interface{}{
					"email": "test@example.com",
					"token": "invalid-token",
				},
				"jira_url": "https://invalid-domain.atlassian.net",
			},
			expectedStatus: http.StatusUnauthorized, // Will fail actual auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/connect", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				t.Logf("Response body: %s", w.Body.String())
			}
		})
	}
}

func TestIssueEndpointsWithoutAuth(t *testing.T) {
	srv := setupTestServer(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/issues/TEST-1"},
		{"POST", "/api/v1/issues"},
		{"PUT", "/api/v1/issues/TEST-1"},
		{"DELETE", "/api/v1/issues/TEST-1"},
		{"GET", "/api/v1/search?jql=project=TEST"},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.method+" "+endpoint.path, func(t *testing.T) {
			var req *http.Request
			if endpoint.method == "POST" || endpoint.method == "PUT" {
				payload := `{"project":"TEST","summary":"Test","issueType":"Bug"}`
				req = httptest.NewRequest(endpoint.method, endpoint.path, bytes.NewBufferString(payload))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(endpoint.method, endpoint.path, nil)
			}
			
			w := httptest.NewRecorder()
			srv.Router().ServeHTTP(w, req)

			// Should return 401 Unauthorized since no auth is set up
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401 for unauthorized request, got %d", w.Code)
			}
		})
	}
}