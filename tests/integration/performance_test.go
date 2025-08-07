package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/api/routes"
	"github.com/ericfisherdev/GoJira/internal/monitoring"
	"github.com/go-chi/chi/v5"
)

func BenchmarkHealthEndpoint(b *testing.B) {
	router := chi.NewRouter()
	routes.SetupRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Get(server.URL + "/health")
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

func BenchmarkSearchValidation(b *testing.B) {
	router := chi.NewRouter()
	routes.SetupRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Get(server.URL + "/api/v1/search/validate?jql=project=TEST")
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

func BenchmarkClaudeCommandProcessing(b *testing.B) {
	router := chi.NewRouter()
	routes.SetupRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()

	commandReq := map[string]interface{}{
		"command": "show issue TEST-1",
	}
	body, _ := json.Marshal(commandReq)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Post(server.URL+"/api/v1/claude/command", "application/json", bytes.NewReader(body))
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

func TestLoadTesting(t *testing.T) {
	router := chi.NewRouter()
	
	// Add metrics middleware
	router.Use(monitoring.RequestMetricsMiddleware())
	
	routes.SetupRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()

	// Reset metrics for clean test
	monitoring.GlobalMetrics.Reset()

	testCases := []struct {
		name        string
		concurrency int
		duration    time.Duration
		endpoint    string
		method      string
		body        []byte
	}{
		{
			name:        "Health Check Load",
			concurrency: 50,
			duration:    10 * time.Second,
			endpoint:    "/health",
			method:      "GET",
		},
		{
			name:        "Search Validation Load",
			concurrency: 20,
			duration:    10 * time.Second,
			endpoint:    "/api/v1/search/validate?jql=project=TEST",
			method:      "GET",
		},
		{
			name:        "Command Processing Load",
			concurrency: 10,
			duration:    15 * time.Second,
			endpoint:    "/api/v1/claude/command",
			method:      "POST",
			body:        []byte(`{"command": "show issue TEST-1"}`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runLoadTest(t, server.URL, tc.concurrency, tc.duration, tc.endpoint, tc.method, tc.body)
		})
	}

	// Check final metrics
	stats := monitoring.GlobalMetrics.GetStats()
	t.Logf("Final metrics: %+v", stats)

	// Verify performance thresholds
	requests := stats["requests"].(map[string]interface{})
	errorRate := requests["errorRate"].(float64)
	if errorRate > 0.05 { // 5% error rate threshold
		t.Errorf("Error rate too high: %.2f%%", errorRate*100)
	}

	avgResponseTime, _ := time.ParseDuration(requests["avgResponseTime"].(string))
	if avgResponseTime > 2*time.Second {
		t.Errorf("Average response time too slow: %v", avgResponseTime)
	}
}

func runLoadTest(t *testing.T, baseURL string, concurrency int, duration time.Duration, endpoint, method string, body []byte) {
	start := time.Now()
	endTime := start.Add(duration)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var totalRequests int64
	var totalErrors int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			client := &http.Client{
				Timeout: 10 * time.Second,
			}

			for time.Now().Before(endTime) {
				var req *http.Request
				var err error

				if method == "POST" && body != nil {
					req, err = http.NewRequest(method, baseURL+endpoint, bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req, err = http.NewRequest(method, baseURL+endpoint, nil)
				}

				if err != nil {
					mu.Lock()
					totalErrors++
					mu.Unlock()
					continue
				}

				resp, err := client.Do(req)
				
				mu.Lock()
				totalRequests++
				if err != nil || resp.StatusCode >= 400 {
					totalErrors++
				}
				mu.Unlock()

				if resp != nil {
					resp.Body.Close()
				}

				// Small delay to prevent overwhelming
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	actualDuration := time.Since(start)

	throughput := float64(totalRequests) / actualDuration.Seconds()
	errorRate := float64(totalErrors) / float64(totalRequests)

	t.Logf("Load test results for %s:", endpoint)
	t.Logf("  Duration: %v", actualDuration)
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Total errors: %d", totalErrors)
	t.Logf("  Throughput: %.2f req/sec", throughput)
	t.Logf("  Error rate: %.2f%%", errorRate*100)

	// Performance assertions
	if errorRate > 0.1 { // 10% error rate threshold for load tests
		t.Errorf("Error rate too high: %.2f%%", errorRate*100)
	}

	if throughput < 10 { // Minimum 10 req/sec
		t.Errorf("Throughput too low: %.2f req/sec", throughput)
	}
}

func TestMemoryUsageUnderLoad(t *testing.T) {
	router := chi.NewRouter()
	router.Use(monitoring.RequestMetricsMiddleware())
	routes.SetupRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()

	// Start performance monitor
	monitor := monitoring.NewPerformanceMonitor(1 * time.Second)
	monitor.Start()
	defer monitor.Stop()

	// Run sustained load
	concurrency := 20
	duration := 30 * time.Second

	runLoadTest(t, server.URL, concurrency, duration, "/health", "GET", nil)

	// Allow monitoring to capture final state
	time.Sleep(2 * time.Second)

	stats := monitoring.GlobalMetrics.GetStats()
	t.Logf("Performance stats after sustained load: %+v", stats)

	// Check if any alerts would have been triggered
	requests := stats["requests"].(map[string]interface{})
	errorRate := requests["errorRate"].(float64)
	
	if errorRate > 0.05 {
		t.Logf("Warning: Error rate exceeded 5%% threshold: %.2f%%", errorRate*100)
	}

	avgResponseTime, _ := time.ParseDuration(requests["avgResponseTime"].(string))
	if avgResponseTime > time.Second {
		t.Logf("Warning: Average response time exceeded 1s: %v", avgResponseTime)
	}
}

func TestConcurrentDifferentEndpoints(t *testing.T) {
	router := chi.NewRouter()
	router.Use(monitoring.RequestMetricsMiddleware())
	routes.SetupRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()

	endpoints := []struct {
		path   string
		method string
		body   []byte
	}{
		{"/health", "GET", nil},
		{"/ready", "GET", nil},
		{"/api/v1/search/validate?jql=project=TEST", "GET", nil},
		{"/api/v1/claude/suggestions?input=create", "GET", nil},
		{"/api/v1/claude/jql", "POST", []byte(`{"query": "find open issues"}`)},
	}

	concurrency := 5
	requestsPerEndpoint := 50

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]int)

	for _, endpoint := range endpoints {
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(ep struct {
				path   string
				method string
				body   []byte
			}, worker int) {
				defer wg.Done()

				client := &http.Client{Timeout: 5 * time.Second}
				successes := 0

				for j := 0; j < requestsPerEndpoint; j++ {
					var req *http.Request
					var err error

					if ep.method == "POST" && ep.body != nil {
						req, err = http.NewRequest(ep.method, server.URL+ep.path, bytes.NewReader(ep.body))
						req.Header.Set("Content-Type", "application/json")
					} else {
						req, err = http.NewRequest(ep.method, server.URL+ep.path, nil)
					}

					if err != nil {
						continue
					}

					resp, err := client.Do(req)
					if err == nil && resp.StatusCode < 400 {
						successes++
					}
					if resp != nil {
						resp.Body.Close()
					}
				}

				mu.Lock()
				results[fmt.Sprintf("%s %s", ep.method, ep.path)] = successes
				mu.Unlock()
			}(endpoint, i)
		}
	}

	wg.Wait()

	t.Log("Concurrent endpoint test results:")
	for endpoint, successes := range results {
		expected := concurrency * requestsPerEndpoint
		successRate := float64(successes) / float64(expected)
		t.Logf("  %s: %d/%d (%.1f%%)", endpoint, successes, expected, successRate*100)
		
		if successRate < 0.9 { // 90% success rate threshold
			t.Errorf("Success rate too low for %s: %.1f%%", endpoint, successRate*100)
		}
	}

	stats := monitoring.GlobalMetrics.GetStats()
	t.Logf("Final concurrent test metrics: %+v", stats)
}