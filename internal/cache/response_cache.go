package cache

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ResponseCache provides caching for formatted responses
type ResponseCache struct {
	entries map[string]*CacheEntry
	ttl     time.Duration
	mu      sync.RWMutex
	maxSize int
	stats   CacheStats
	cleanup *time.Ticker
	stopCh  chan struct{}
}

// CacheEntry represents a cached response
type CacheEntry struct {
	Data       interface{} `json:"data"`
	Format     string      `json:"format"`
	Metadata   CacheMetadata `json:"metadata"`
	Timestamp  time.Time   `json:"timestamp"`
	TTL        time.Duration `json:"ttl"`
	AccessCount int         `json:"accessCount"`
	LastAccess time.Time   `json:"lastAccess"`
	Size       int         `json:"size"`
}

// CacheMetadata holds additional information about the cached entry
type CacheMetadata struct {
	Source      string            `json:"source"`
	Version     string            `json:"version"`
	Tags        []string          `json:"tags"`
	Context     map[string]string `json:"context,omitempty"`
	Compression bool              `json:"compression"`
}

// CacheStats tracks cache performance
type CacheStats struct {
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	Evictions  int64   `json:"evictions"`
	Size       int     `json:"size"`
	HitRate    float64 `json:"hitRate"`
	MemoryUsed int64   `json:"memoryUsed"`
}

// CacheKey represents the components that make up a cache key
type CacheKey struct {
	Operation  string            `json:"operation"`
	Parameters map[string]string `json:"parameters"`
	UserID     string            `json:"userId,omitempty"`
	Format     string            `json:"format"`
	Timestamp  time.Time         `json:"timestamp,omitempty"`
}

// NewResponseCache creates a new response cache
func NewResponseCache(maxSize int, defaultTTL time.Duration) *ResponseCache {
	rc := &ResponseCache{
		entries: make(map[string]*CacheEntry),
		ttl:     defaultTTL,
		maxSize: maxSize,
		stats:   CacheStats{},
		stopCh:  make(chan struct{}),
	}
	
	// Start cleanup routine
	rc.cleanup = time.NewTicker(defaultTTL / 4) // Clean up 4 times per TTL period
	go rc.cleanupRoutine()
	
	log.Info().
		Int("maxSize", maxSize).
		Dur("ttl", defaultTTL).
		Msg("Response cache initialized")
	
	return rc
}

// Get retrieves a cached response
func (rc *ResponseCache) Get(key string) (interface{}, bool) {
	rc.mu.RLock()
	entry, exists := rc.entries[key]
	rc.mu.RUnlock()
	
	if !exists {
		rc.recordMiss()
		return nil, false
	}
	
	// Check if expired
	if rc.isExpired(entry) {
		rc.mu.Lock()
		delete(rc.entries, key)
		rc.stats.Evictions++
		rc.mu.Unlock()
		rc.recordMiss()
		
		log.Debug().
			Str("key", key).
			Msg("Cache entry expired and removed")
		
		return nil, false
	}
	
	// Update access statistics
	rc.mu.Lock()
	entry.AccessCount++
	entry.LastAccess = time.Now()
	rc.mu.Unlock()
	
	rc.recordHit()
	
	log.Debug().
		Str("key", key).
		Int("accessCount", entry.AccessCount).
		Msg("Cache hit")
	
	return entry.Data, true
}

// Set stores a response in the cache
func (rc *ResponseCache) Set(key string, data interface{}, options ...CacheOption) error {
	if rc.isFull() {
		rc.evictLRU()
	}
	
	entry := &CacheEntry{
		Data:        data,
		Timestamp:   time.Now(),
		TTL:         rc.ttl,
		AccessCount: 0,
		LastAccess:  time.Now(),
		Metadata: CacheMetadata{
			Source:  "gojira",
			Version: "1.0",
		},
	}
	
	// Apply options
	for _, option := range options {
		option(entry)
	}
	
	// Calculate size
	entry.Size = rc.calculateSize(data)
	
	rc.mu.Lock()
	rc.entries[key] = entry
	rc.stats.Size++
	rc.stats.MemoryUsed += int64(entry.Size)
	rc.mu.Unlock()
	
	log.Debug().
		Str("key", key).
		Str("format", entry.Format).
		Dur("ttl", entry.TTL).
		Int("size", entry.Size).
		Msg("Cache entry stored")
	
	return nil
}

// Delete removes a specific entry from the cache
func (rc *ResponseCache) Delete(key string) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	if entry, exists := rc.entries[key]; exists {
		delete(rc.entries, key)
		rc.stats.Size--
		rc.stats.MemoryUsed -= int64(entry.Size)
		rc.stats.Evictions++
		
		log.Debug().
			Str("key", key).
			Msg("Cache entry deleted")
		
		return true
	}
	
	return false
}

// InvalidatePattern removes all entries matching a regex pattern
func (rc *ResponseCache) InvalidatePattern(pattern string) int {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		log.Error().
			Err(err).
			Str("pattern", pattern).
			Msg("Invalid regex pattern for cache invalidation")
		return 0
	}
	
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	count := 0
	for key, entry := range rc.entries {
		if regex.MatchString(key) {
			delete(rc.entries, key)
			rc.stats.Size--
			rc.stats.MemoryUsed -= int64(entry.Size)
			rc.stats.Evictions++
			count++
		}
	}
	
	log.Info().
		Str("pattern", pattern).
		Int("invalidated", count).
		Msg("Cache pattern invalidation completed")
	
	return count
}

// InvalidateTag removes all entries with a specific tag
func (rc *ResponseCache) InvalidateTag(tag string) int {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	count := 0
	for key, entry := range rc.entries {
		for _, entryTag := range entry.Metadata.Tags {
			if entryTag == tag {
				delete(rc.entries, key)
				rc.stats.Size--
				rc.stats.MemoryUsed -= int64(entry.Size)
				rc.stats.Evictions++
				count++
				break
			}
		}
	}
	
	log.Info().
		Str("tag", tag).
		Int("invalidated", count).
		Msg("Cache tag invalidation completed")
	
	return count
}

// Clear removes all entries from the cache
func (rc *ResponseCache) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	count := len(rc.entries)
	rc.entries = make(map[string]*CacheEntry)
	rc.stats.Size = 0
	rc.stats.MemoryUsed = 0
	rc.stats.Evictions += int64(count)
	
	log.Info().
		Int("cleared", count).
		Msg("Cache cleared")
}

// GetStats returns current cache statistics
func (rc *ResponseCache) GetStats() CacheStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	
	stats := rc.stats
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}
	
	return stats
}

// Stop shuts down the cache and cleanup routines
func (rc *ResponseCache) Stop() {
	close(rc.stopCh)
	if rc.cleanup != nil {
		rc.cleanup.Stop()
	}
	
	log.Info().Msg("Response cache stopped")
}

// Helper methods

func (rc *ResponseCache) isExpired(entry *CacheEntry) bool {
	return time.Since(entry.Timestamp) > entry.TTL
}

func (rc *ResponseCache) isFull() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.entries) >= rc.maxSize
}

func (rc *ResponseCache) evictLRU() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	if len(rc.entries) == 0 {
		return
	}
	
	// Find entry with oldest LastAccess
	var oldestKey string
	var oldestTime time.Time
	first := true
	
	for key, entry := range rc.entries {
		if first || entry.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccess
			first = false
		}
	}
	
	// Remove oldest entry
	if oldestKey != "" {
		entry := rc.entries[oldestKey]
		delete(rc.entries, oldestKey)
		rc.stats.Size--
		rc.stats.MemoryUsed -= int64(entry.Size)
		rc.stats.Evictions++
		
		log.Debug().
			Str("key", oldestKey).
			Time("lastAccess", oldestTime).
			Msg("LRU cache eviction")
	}
}

func (rc *ResponseCache) cleanupRoutine() {
	for {
		select {
		case <-rc.cleanup.C:
			rc.performCleanup()
		case <-rc.stopCh:
			return
		}
	}
}

func (rc *ResponseCache) performCleanup() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	expired := 0
	for key, entry := range rc.entries {
		if rc.isExpired(entry) {
			delete(rc.entries, key)
			rc.stats.Size--
			rc.stats.MemoryUsed -= int64(entry.Size)
			rc.stats.Evictions++
			expired++
		}
	}
	
	if expired > 0 {
		log.Debug().
			Int("expired", expired).
			Msg("Cache cleanup completed")
	}
}

func (rc *ResponseCache) recordHit() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.stats.Hits++
}

func (rc *ResponseCache) recordMiss() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.stats.Misses++
}

func (rc *ResponseCache) calculateSize(data interface{}) int {
	// Simple size estimation - in production might use reflection or serialization size
	if jsonData, err := json.Marshal(data); err == nil {
		return len(jsonData)
	}
	return 1000 // Default estimate
}

// Utility functions

// GenerateKey creates a consistent cache key from components
func GenerateKey(operation string, params map[string]string, format string) string {
	key := CacheKey{
		Operation:  operation,
		Parameters: params,
		Format:     format,
	}
	
	jsonKey, _ := json.Marshal(key)
	hash := md5.Sum(jsonKey)
	return fmt.Sprintf("%x", hash)
}

// GenerateUserKey creates a user-specific cache key
func GenerateUserKey(userID, operation string, params map[string]string, format string) string {
	key := CacheKey{
		Operation:  operation,
		Parameters: params,
		UserID:     userID,
		Format:     format,
	}
	
	jsonKey, _ := json.Marshal(key)
	hash := md5.Sum(jsonKey)
	return fmt.Sprintf("user:%s:%x", userID, hash)
}

// GenerateTimestampKey creates a time-sensitive cache key
func GenerateTimestampKey(operation string, params map[string]string, format string, granularity time.Duration) string {
	// Round timestamp to granularity (e.g., to nearest hour)
	timestamp := time.Now().Truncate(granularity)
	
	key := CacheKey{
		Operation:  operation,
		Parameters: params,
		Format:     format,
		Timestamp:  timestamp,
	}
	
	jsonKey, _ := json.Marshal(key)
	hash := md5.Sum(jsonKey)
	return fmt.Sprintf("time:%d:%x", timestamp.Unix(), hash)
}

// CacheOption provides functional options for cache entries
type CacheOption func(*CacheEntry)

// WithTTL sets a custom TTL for the cache entry
func WithTTL(ttl time.Duration) CacheOption {
	return func(entry *CacheEntry) {
		entry.TTL = ttl
	}
}

// WithFormat sets the format for the cache entry
func WithFormat(format string) CacheOption {
	return func(entry *CacheEntry) {
		entry.Format = format
	}
}

// WithTags adds tags to the cache entry
func WithTags(tags ...string) CacheOption {
	return func(entry *CacheEntry) {
		entry.Metadata.Tags = append(entry.Metadata.Tags, tags...)
	}
}

// WithContext adds context metadata to the cache entry
func WithContext(context map[string]string) CacheOption {
	return func(entry *CacheEntry) {
		entry.Metadata.Context = context
	}
}

// WithSource sets the source identifier for the cache entry
func WithSource(source string) CacheOption {
	return func(entry *CacheEntry) {
		entry.Metadata.Source = source
	}
}