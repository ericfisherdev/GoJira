package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ericfisherdev/GoJira/internal/api/routes"
	"github.com/ericfisherdev/GoJira/internal/server"
)

func TestHealthEndpoint(t *testing.T) {
	// Create test server
	config := &server.Config{
		Port:        "8080",
		Mode:        "test",
		EnableCORS:  true,
		LogRequests: false,
	}

	srv := server.New(config)
	routes.SetupRoutes(srv.Router())

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
}

func TestReadinessEndpoint(t *testing.T) {
	// Create test server
	config := &server.Config{
		Port:        "8080",
		Mode:        "test",
		EnableCORS:  true,
		LogRequests: false,
	}

	srv := server.New(config)
	routes.SetupRoutes(srv.Router())

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
		{"GET", "/api/v1/issues/TEST-1", http.StatusOK},
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