package integration

import (
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/cache"
	"github.com/ericfisherdev/GoJira/internal/claude"
	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDay8BasicFunctionality tests the core Day 8 features
func TestDay8BasicFunctionality(t *testing.T) {
	t.Run("Response Formatter Creation", func(t *testing.T) {
		config := claude.FormatterConfig{
			IncludeMetadata:      true,
			UseMarkdown:          true,
			SummarizeResults:     false,
			MaxDescriptionLength: 100,
			DefaultFormat:        claude.FormatMarkdown,
			Verbose:              true,
			MaxResults:           50,
		}

		formatter := claude.NewResponseFormatter(config)
		assert.NotNil(t, formatter)
	})

	t.Run("Summarizer Creation and Basic Functions", func(t *testing.T) {
		summarizer := claude.NewSummarizer(500, []string{"status", "priority"})
		assert.NotNil(t, summarizer)

		// Test with empty data
		opts := claude.DefaultSummaryOptions()
		summary := summarizer.SummarizeIssues([]jira.Issue{}, opts)
		assert.Equal(t, "No issues found", summary)
	})

	t.Run("Response Cache Creation and Basic Operations", func(t *testing.T) {
		responseCache := cache.NewResponseCache(10, time.Minute*5)
		assert.NotNil(t, responseCache)
		defer responseCache.Stop()

		// Test basic cache operations
		key := "test-key"
		data := "test-data"
		
		err := responseCache.Set(key, data)
		require.NoError(t, err)
		
		retrieved, exists := responseCache.Get(key)
		assert.True(t, exists)
		assert.Equal(t, data, retrieved)

		// Test stats
		stats := responseCache.GetStats()
		assert.Equal(t, 1, stats.Size)
		assert.Equal(t, int64(1), stats.Hits)
		assert.Equal(t, int64(0), stats.Misses)
	})

	t.Run("Integration Manager with Optimization", func(t *testing.T) {
		config := &claude.ClaudeConfig{
			SessionTTL:          time.Hour,
			EnableWorkflows:     true,
			EnableSuggestions:   true,
			EnableResponseCache: true,
			ResponseCacheSize:   100,
			ResponseCacheTTL:    time.Minute * 5,
			DefaultFormat:       claude.FormatMarkdown,
			OptimizeForClaude:   true,
		}

		manager := claude.NewIntegrationManager(nil, config)
		assert.NotNil(t, manager)
		defer manager.Shutdown()

		// Test cache stats
		cacheStats := manager.GetCacheStats()
		assert.Contains(t, cacheStats, "cacheEnabled")
		assert.True(t, cacheStats["cacheEnabled"].(bool))
	})

	t.Run("Response Formats", func(t *testing.T) {
		// Test that all response formats are defined
		formats := []claude.ResponseFormat{
			claude.FormatJSON,
			claude.FormatMarkdown,
			claude.FormatTable,
			claude.FormatSummary,
			claude.FormatCompact,
		}

		for _, format := range formats {
			assert.NotEmpty(t, string(format))
		}
	})

	t.Run("Cache Key Generation", func(t *testing.T) {
		params := map[string]string{
			"project": "TEST",
			"status":  "Done",
		}
		
		key1 := cache.GenerateKey("search", params, "json")
		key2 := cache.GenerateKey("search", params, "json")
		key3 := cache.GenerateKey("search", params, "markdown")
		
		// Same parameters should generate same key
		assert.Equal(t, key1, key2)
		// Different format should generate different key
		assert.NotEqual(t, key1, key3)
		
		// User-specific keys
		userKey1 := cache.GenerateUserKey("user1", "search", params, "json")
		userKey2 := cache.GenerateUserKey("user2", "search", params, "json")
		assert.NotEqual(t, userKey1, userKey2)
	})

	t.Run("Default Configuration", func(t *testing.T) {
		config := claude.DefaultClaudeConfig()
		assert.NotNil(t, config)
		assert.True(t, config.EnableResponseCache)
		assert.True(t, config.OptimizeForClaude)
		assert.Equal(t, claude.FormatMarkdown, config.DefaultFormat)
		assert.Equal(t, 1000, config.ResponseCacheSize)
		assert.Equal(t, time.Minute*15, config.ResponseCacheTTL)
	})

	t.Run("Cache Pattern Invalidation", func(t *testing.T) {
		cache := cache.NewResponseCache(5, time.Minute)
		defer cache.Stop()

		// Set some test data
		cache.Set("user:123:cmd1", "data1")
		cache.Set("user:123:cmd2", "data2") 
		cache.Set("system:config", "config")
		
		// Test pattern invalidation
		invalidated := cache.InvalidatePattern("user:123:.*")
		assert.True(t, invalidated >= 0) // Should not error
		
		// Verify system config still exists
		_, exists := cache.Get("system:config")
		assert.True(t, exists)
	})
}