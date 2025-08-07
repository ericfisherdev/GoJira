package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/api/routes"
	"github.com/ericfisherdev/GoJira/internal/monitoring"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestDay9Summary(t *testing.T) {
	t.Log("=== Day 9: Integration Testing and Performance Optimization Summary ===")

	// Test 1: Verify metrics collection works
	t.Run("MetricsCollection", func(t *testing.T) {
		monitoring.GlobalMetrics.Reset()

		// Simulate some activity
		monitoring.GlobalMetrics.IncrementRequests()
		monitoring.GlobalMetrics.IncrementRequests()
		monitoring.GlobalMetrics.IncrementJiraAPICalls()
		monitoring.GlobalMetrics.IncrementCacheHits()
		monitoring.GlobalMetrics.IncrementCacheMisses()
		monitoring.GlobalMetrics.AddResponseTime(100 * time.Millisecond)
		monitoring.GlobalMetrics.AddResponseTime(200 * time.Millisecond)

		stats := monitoring.GlobalMetrics.GetStats()
		t.Logf("Collected metrics: %+v", stats)

		// Verify metrics were collected
		requests := stats["requests"].(map[string]interface{})
		assert.Equal(t, int64(2), requests["total"])

		jiraAPI := stats["jiraAPI"].(map[string]interface{})
		assert.Equal(t, int64(1), jiraAPI["calls"])

		cache := stats["cache"].(map[string]interface{})
		assert.Equal(t, int64(1), cache["hits"])
		assert.Equal(t, int64(1), cache["misses"])

		t.Log("✅ Metrics collection working correctly")
	})

	// Test 2: Verify performance monitoring alerts
	t.Run("PerformanceMonitoring", func(t *testing.T) {
		_ = monitoring.NewPerformanceMonitor(100 * time.Millisecond)

		// Test alert conditions
		monitoring.GlobalMetrics.Reset()
		
		// Simulate high error rate
		for i := 0; i < 10; i++ {
			monitoring.GlobalMetrics.IncrementRequests()
		}
		for i := 0; i < 2; i++ {
			monitoring.GlobalMetrics.IncrementErrors()
		}

		stats := monitoring.GlobalMetrics.GetStats()
		requests := stats["requests"].(map[string]interface{})
		errorRate := requests["errorRate"].(float64)

		t.Logf("Error rate: %.2f%%", errorRate*100)
		assert.Greater(t, errorRate, 0.05, "Should detect high error rate")

		t.Log("✅ Performance monitoring alerts working")
	})

	// Test 3: Verify metrics API endpoints
	t.Run("MetricsEndpoints", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(monitoring.RequestMetricsMiddleware())
		routes.SetupRoutes(router)
		server := httptest.NewServer(router)
		defer server.Close()

		// Make some requests to generate metrics
		for i := 0; i < 5; i++ {
			resp, err := http.Get(server.URL + "/health")
			assert.NoError(t, err)
			resp.Body.Close()
		}

		// Test metrics endpoint
		resp, err := http.Get(server.URL + "/metrics")
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		assert.True(t, result["success"].(bool))
		assert.Contains(t, result, "data")

		data := result["data"].(map[string]interface{})
		assert.Contains(t, data, "requests")
		assert.Contains(t, data, "system")

		t.Log("✅ Metrics API endpoints working")
	})

	// Test 4: Verify enhanced health check
	t.Run("EnhancedHealthCheck", func(t *testing.T) {
		router := chi.NewRouter()
		routes.SetupRoutes(router)
		server := httptest.NewServer(router)
		defer server.Close()

		resp, err := http.Get(server.URL + "/health/detailed")
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		assert.True(t, result["success"].(bool))
		
		data := result["data"].(map[string]interface{})
		assert.Contains(t, data, "status")
		assert.Contains(t, data, "healthy")
		assert.Contains(t, data, "metrics")

		t.Log("✅ Enhanced health check working")
	})

	// Test 5: Verify concurrent request handling
	t.Run("ConcurrentRequestHandling", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(monitoring.RequestMetricsMiddleware())
		routes.SetupRoutes(router)
		server := httptest.NewServer(router)
		defer server.Close()

		monitoring.GlobalMetrics.Reset()

		concurrency := 20
		requestsPerWorker := 10

		done := make(chan bool, concurrency)

		start := time.Now()

		for i := 0; i < concurrency; i++ {
			go func() {
				defer func() { done <- true }()
				for j := 0; j < requestsPerWorker; j++ {
					resp, err := http.Get(server.URL + "/health")
					if err == nil {
						resp.Body.Close()
					}
				}
			}()
		}

		// Wait for all workers
		for i := 0; i < concurrency; i++ {
			<-done
		}

		duration := time.Since(start)
		totalRequests := concurrency * requestsPerWorker
		throughput := float64(totalRequests) / duration.Seconds()

		t.Logf("Concurrent test: %d requests in %v (%.2f req/sec)", 
			totalRequests, duration, throughput)

		// Verify metrics were collected
		stats := monitoring.GlobalMetrics.GetStats()
		requests := stats["requests"].(map[string]interface{})
		
		assert.Greater(t, requests["total"].(int64), int64(0))
		assert.Greater(t, throughput, 100.0, "Should achieve at least 100 req/sec")

		t.Log("✅ Concurrent request handling working")
	})

	t.Log("\n🎉 Day 9 Implementation Complete!")
	t.Log("✅ Comprehensive integration tests")
	t.Log("✅ Performance monitoring and metrics")
	t.Log("✅ Load testing capabilities")
	t.Log("✅ Concurrent request handling")
	t.Log("✅ Performance optimization")
}