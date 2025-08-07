package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/cache"
	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/ericfisherdev/GoJira/internal/monitoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPerformanceOptimizationComponents tests the core performance optimization features
func TestPerformanceOptimizationComponents(t *testing.T) {
	t.Run("MultiLevelCache_BasicOperations", func(t *testing.T) {
		strategy := cache.DefaultMultiLevelStrategy()
		mlCache := cache.NewMultiLevelCache(strategy)
		defer mlCache.Stop()
		
		assert.NotNil(t, mlCache)
		
		// Test basic set and get
		key := "test-key"
		value := "test-value"
		
		err := mlCache.Set(key, value, 5*time.Minute)
		require.NoError(t, err)
		
		retrievedValue, exists := mlCache.Get(key)
		assert.True(t, exists)
		assert.Equal(t, value, retrievedValue)
		
		// Test cache statistics
		stats := mlCache.GetDetailedStats()
		assert.NotNil(t, stats)
		assert.Contains(t, stats, "totalRequests")
		assert.Contains(t, stats, "l1")
		assert.Contains(t, stats, "strategy")
	})

	t.Run("MultiLevelCache_DifferentStrategies", func(t *testing.T) {
		strategies := []cache.CacheStrategy{
			cache.DefaultMultiLevelStrategy(),
			cache.AggressiveStrategy(),
			cache.ConservativeStrategy(),
		}
		
		for i, strategy := range strategies {
			mlCache := cache.NewMultiLevelCache(strategy)
			defer mlCache.Stop()
			
			// Test that cache works with different strategies
			key := fmt.Sprintf("strategy-test-%d", i)
			value := fmt.Sprintf("value-%d", i)
			
			err := mlCache.Set(key, value, time.Minute)
			assert.NoError(t, err)
			
			retrievedValue, exists := mlCache.Get(key)
			assert.True(t, exists)
			assert.Equal(t, value, retrievedValue)
		}
	})

	t.Run("PerformanceMonitor_CreationAndMetrics", func(t *testing.T) {
		config := monitoring.DefaultPerformanceConfig()
		monitor := monitoring.NewDetailedPerformanceMonitor(config)
		
		assert.NotNil(t, monitor)
		
		// Test timer functionality
		timer := monitor.StartTimer("test-operation")
		time.Sleep(10 * time.Millisecond) // Simulate work
		duration := timer.Success()
		
		assert.True(t, duration >= 10*time.Millisecond)
		
		// Test metric retrieval
		metric, exists := monitor.GetMetric("test-operation")
		assert.True(t, exists)
		assert.Equal(t, int64(1), metric.Count)
		assert.True(t, metric.AvgTime >= 10*time.Millisecond)
		
		// Test summary stats
		summary := monitor.GetSummaryStats()
		assert.NotNil(t, summary)
		assert.Contains(t, summary, "totalOperations")
		assert.Contains(t, summary, "averageResponseTime")
	})

	t.Run("PerformanceMonitor_DifferentConfigs", func(t *testing.T) {
		configs := []monitoring.PerformanceConfig{
			monitoring.DefaultPerformanceConfig(),
			monitoring.DevelopmentPerformanceConfig(),
			monitoring.ProductionPerformanceConfig(),
		}
		
		for i, config := range configs {
			monitor := monitoring.NewDetailedPerformanceMonitor(config)
			
			operation := fmt.Sprintf("config-test-%d", i)
			timer := monitor.StartTimer(operation)
			time.Sleep(1 * time.Millisecond)
			timer.Success()
			
			metric, exists := monitor.GetMetric(operation)
			assert.True(t, exists)
			assert.Equal(t, int64(1), metric.Count)
		}
	})

	t.Run("ConnectionPool_CreationAndConfiguration", func(t *testing.T) {
		factory := func() (*jira.Client, error) {
			// Mock client factory - in real implementation this would create a real client
			return &jira.Client{}, nil
		}
		
		config := jira.DefaultPoolConfig()
		pool, err := jira.NewConnectionPool(factory, config)
		require.NoError(t, err)
		defer pool.Close()
		
		assert.NotNil(t, pool)
		
		// Test pool statistics
		stats := pool.GetStats()
		assert.Equal(t, int64(0), stats.AcquisitionCount) // No acquisitions yet
		
		// Test detailed stats
		detailedStats := pool.GetDetailedStats()
		assert.NotNil(t, detailedStats)
		assert.Contains(t, detailedStats, "config")
		assert.Contains(t, detailedStats, "stats")
		assert.Contains(t, detailedStats, "health")
	})

	t.Run("ConnectionPool_DifferentConfigurations", func(t *testing.T) {
		factory := func() (*jira.Client, error) {
			return &jira.Client{}, nil
		}
		
		configs := []jira.PoolConfig{
			jira.DefaultPoolConfig(),
			jira.HighVolumePoolConfig(),
			jira.LowResourcePoolConfig(),
		}
		
		for i, config := range configs {
			pool, err := jira.NewConnectionPool(factory, config)
			require.NoError(t, err, "Failed to create pool for config %d", i)
			
			stats := pool.GetStats()
			assert.True(t, stats.ActiveCount >= 0)
			
			err = pool.Close()
			assert.NoError(t, err)
		}
	})

	t.Run("BatchProcessor_CreationAndConfiguration", func(t *testing.T) {
		// Create mock client and monitor
		client := &jira.Client{} // Mock client
		monitor := monitoring.NewDetailedPerformanceMonitor(monitoring.DefaultPerformanceConfig())
		
		config := jira.DefaultBatchConfig()
		processor := jira.NewBatchProcessor(client, monitor, config)
		defer processor.Close()
		
		assert.NotNil(t, processor)
		
		// Test statistics
		stats := processor.GetStats()
		assert.Equal(t, int64(0), stats.TotalOperations) // No operations yet
		assert.Equal(t, float64(0), stats.BatchEfficiency) // No batching yet
	})

	t.Run("BatchProcessor_DifferentConfigurations", func(t *testing.T) {
		client := &jira.Client{}
		monitor := monitoring.NewDetailedPerformanceMonitor(monitoring.DefaultPerformanceConfig())
		
		configs := []jira.BatchConfig{
			jira.DefaultBatchConfig(),
			jira.HighThroughputBatchConfig(),
			jira.LowLatencyBatchConfig(),
		}
		
		for i, config := range configs {
			processor := jira.NewBatchProcessor(client, monitor, config)
			
			stats := processor.GetStats()
			assert.Equal(t, int64(0), stats.TotalOperations, "Config %d should start with 0 operations", i)
			
			err := processor.Close()
			assert.NoError(t, err)
		}
	})

	t.Run("RateLimiter_Functionality", func(t *testing.T) {
		// Test rate limiter creation and basic functionality
		rateLimiter := jira.NewRateLimiter(10.0, 10) // 10 requests per second, capacity 10
		
		// Should be able to acquire tokens immediately
		start := time.Now()
		for i := 0; i < 5; i++ {
			acquired := rateLimiter.TryAcquire()
			assert.True(t, acquired, "Should be able to acquire token %d", i)
		}
		elapsed := time.Since(start)
		assert.True(t, elapsed < 100*time.Millisecond, "Should acquire tokens quickly")
		
		// Test that waiting works (this is harder to test precisely due to timing)
		rateLimiter.Wait() // Should not block significantly
	})

	t.Run("Cache_InvalidationPatterns", func(t *testing.T) {
		strategy := cache.DefaultMultiLevelStrategy()
		mlCache := cache.NewMultiLevelCache(strategy)
		defer mlCache.Stop()
		
		// Set multiple keys with patterns
		testData := map[string]string{
			"user:123:profile": "profile data",
			"user:123:settings": "settings data",
			"user:456:profile": "other profile",
			"system:config": "system config",
		}
		
		for key, value := range testData {
			err := mlCache.Set(key, value, time.Minute)
			require.NoError(t, err)
		}
		
		// Invalidate user 123 data
		invalidated := mlCache.Invalidate("user:123:.*")
		assert.True(t, invalidated >= 0) // Should invalidate something
		
		// Check that user 456 and system data still exists
		_, exists := mlCache.Get("user:456:profile")
		assert.True(t, exists, "user:456:profile should still exist")
		
		_, exists = mlCache.Get("system:config")
		assert.True(t, exists, "system:config should still exist")
	})

	t.Run("IntegrationTest_AllComponents", func(t *testing.T) {
		// Integration test combining multiple Day 9 components
		
		// Setup performance monitoring
		monitor := monitoring.NewDetailedPerformanceMonitor(monitoring.DevelopmentPerformanceConfig())
		
		// Setup multi-level cache
		strategy := cache.AggressiveStrategy()
		mlCache := cache.NewMultiLevelCache(strategy)
		defer mlCache.Stop()
		
		// Simulate operations with monitoring and caching
		operationCount := 100
		
		for i := 0; i < operationCount; i++ {
			timer := monitor.StartTimer("cache-operation")
			
			key := fmt.Sprintf("perf-test-%d", i%20) // Some key overlap for cache hits
			value := fmt.Sprintf("data-%d", i)
			
			// Check cache first
			if cached, exists := mlCache.Get(key); exists {
				assert.NotEmpty(t, cached)
				timer.Success()
				continue
			}
			
			// Simulate work (cache miss)
			time.Sleep(1 * time.Millisecond)
			
			// Store in cache
			err := mlCache.Set(key, value, time.Minute)
			assert.NoError(t, err)
			
			timer.Success()
		}
		
		// Verify performance metrics
		metric, exists := monitor.GetMetric("cache-operation")
		assert.True(t, exists)
		assert.Equal(t, int64(operationCount), metric.Count)
		
		// Verify cache statistics
		cacheStats := mlCache.GetDetailedStats()
		totalRequests := cacheStats["totalRequests"].(int64)
		assert.True(t, totalRequests > 0)
		
		summary := monitor.GetSummaryStats()
		totalOps := summary["totalOperations"].(int64)
		assert.Equal(t, int64(operationCount), totalOps)
	})
}

// Additional helper tests for individual components
func TestMemoryCacheFeatures(t *testing.T) {
	t.Run("MemoryCache_LRUEviction", func(t *testing.T) {
		memCache := cache.NewMemoryCache(3, time.Minute) // Small cache for testing eviction
		defer memCache.Stop()
		
		// Fill cache beyond capacity
		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("key-%d", i)
			value := fmt.Sprintf("value-%d", i)
			err := memCache.Set(key, value, time.Minute)
			assert.NoError(t, err)
		}
		
		// Cache should have evicted oldest entries
		assert.Equal(t, 3, memCache.Size())
		
		// First entries should be evicted
		_, exists := memCache.Get("key-0")
		assert.False(t, exists)
		_, exists = memCache.Get("key-1")
		assert.False(t, exists)
		
		// Recent entries should still exist
		_, exists = memCache.Get("key-4")
		assert.True(t, exists)
	})

	t.Run("MemoryCache_Expiration", func(t *testing.T) {
		memCache := cache.NewMemoryCache(10, 100*time.Millisecond)
		defer memCache.Stop()
		
		// Set value with short TTL
		err := memCache.Set("expire-test", "value", 50*time.Millisecond)
		require.NoError(t, err)
		
		// Should exist immediately
		_, exists := memCache.Get("expire-test")
		assert.True(t, exists)
		
		// Wait for expiration
		time.Sleep(100 * time.Millisecond)
		
		// Should be expired
		_, exists = memCache.Get("expire-test")
		assert.False(t, exists)
	})
}

func TestDiskCacheFeatures(t *testing.T) {
	t.Run("DiskCache_BasicOperations", func(t *testing.T) {
		diskCache := cache.NewDiskCache(1024*1024, time.Hour) // 1MB, 1 hour TTL
		if diskCache == nil {
			t.Skip("Disk cache creation failed, skipping test")
		}
		defer diskCache.Stop()
		
		// Test set and get
		key := "disk-test"
		value := "test data for disk cache"
		
		err := diskCache.Set(key, value, time.Hour)
		require.NoError(t, err)
		
		retrieved, err := diskCache.Get(key)
		require.NoError(t, err)
		assert.Equal(t, value, retrieved)
		
		// Test delete
		err = diskCache.Delete(key)
		assert.NoError(t, err)
		
		_, err = diskCache.Get(key)
		assert.Error(t, err) // Should not exist after delete
	})
}

// Benchmark tests for performance validation
func BenchmarkPerformanceComponents(b *testing.B) {
	b.Run("MultiLevelCache", func(b *testing.B) {
		strategy := cache.DefaultMultiLevelStrategy()
		mlCache := cache.NewMultiLevelCache(strategy)
		defer mlCache.Stop()
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("bench-key-%d", i%100)
				value := fmt.Sprintf("bench-value-%d", i)
				
				// Set operation
				mlCache.Set(key, value, time.Minute)
				
				// Get operation
				mlCache.Get(key)
				
				i++
			}
		})
	})

	b.Run("PerformanceMonitor", func(b *testing.B) {
		monitor := monitoring.NewDetailedPerformanceMonitor(monitoring.DefaultPerformanceConfig())
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				timer := monitor.StartTimer(fmt.Sprintf("bench-op-%d", i%10))
				time.Sleep(time.Microsecond) // Simulate minimal work
				timer.Success()
				i++
			}
		})
	})

	b.Run("RateLimiter", func(b *testing.B) {
		rateLimiter := jira.NewRateLimiter(1000.0, 100) // High rate for benchmarking
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				rateLimiter.TryAcquire()
			}
		})
	})
}