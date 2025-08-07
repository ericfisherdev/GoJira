package cache

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/ericfisherdev/GoJira/internal/monitoring"
)

// SearchCacheEntry represents a cached search result
type SearchCacheEntry struct {
	Result    *jira.ExtendedSearchResult `json:"result"`
	Timestamp time.Time                  `json:"timestamp"`
	JQL       string                     `json:"jql"`
	Params    map[string]string          `json:"params"`
}

// SearchCache manages cached search results
type SearchCache struct {
	entries map[string]*SearchCacheEntry
	mutex   sync.RWMutex
	ttl     time.Duration
	maxSize int
}

// NewSearchCache creates a new search cache with specified TTL and max size
func NewSearchCache(ttl time.Duration, maxSize int) *SearchCache {
	cache := &SearchCache{
		entries: make(map[string]*SearchCacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// generateKey creates a unique key for cache entries
func (sc *SearchCache) generateKey(jql string, params map[string]string) string {
	paramsJSON, _ := json.Marshal(params)
	data := fmt.Sprintf("%s:%s", jql, string(paramsJSON))
	return fmt.Sprintf("%x", md5.Sum([]byte(data)))
}

// Get retrieves a cached search result if it exists and is not expired
func (sc *SearchCache) Get(jql string, params map[string]string) (*jira.ExtendedSearchResult, bool) {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	key := sc.generateKey(jql, params)
	entry, exists := sc.entries[key]

	if !exists || time.Since(entry.Timestamp) > sc.ttl {
		monitoring.GlobalMetrics.IncrementCacheMisses()
		return nil, false
	}

	monitoring.GlobalMetrics.IncrementCacheHits()
	return entry.Result, true
}

// Set stores a search result in the cache
func (sc *SearchCache) Set(jql string, params map[string]string, result *jira.ExtendedSearchResult) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	// If cache is full, remove oldest entry
	if len(sc.entries) >= sc.maxSize {
		sc.evictOldest()
	}

	key := sc.generateKey(jql, params)
	sc.entries[key] = &SearchCacheEntry{
		Result:    result,
		Timestamp: time.Now(),
		JQL:       jql,
		Params:    params,
	}
}

// evictOldest removes the oldest cache entry
func (sc *SearchCache) evictOldest() {
	oldestKey := ""
	oldestTime := time.Now()

	for key, entry := range sc.entries {
		if entry.Timestamp.Before(oldestTime) {
			oldestTime = entry.Timestamp
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(sc.entries, oldestKey)
	}
}

// Clear removes all entries from the cache
func (sc *SearchCache) Clear() {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	sc.entries = make(map[string]*SearchCacheEntry)
}

// Size returns the current number of cached entries
func (sc *SearchCache) Size() int {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	return len(sc.entries)
}

// GetStats returns cache statistics
func (sc *SearchCache) GetStats() map[string]interface{} {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	stats := map[string]interface{}{
		"size":    len(sc.entries),
		"maxSize": sc.maxSize,
		"ttl":     sc.ttl.String(),
	}

	return stats
}

// cleanup runs periodically to remove expired entries
func (sc *SearchCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sc.mutex.Lock()

		now := time.Now()
		for key, entry := range sc.entries {
			if now.Sub(entry.Timestamp) > sc.ttl {
				delete(sc.entries, key)
			}
		}

		sc.mutex.Unlock()
	}
}