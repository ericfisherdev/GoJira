package cache

import (
	"regexp"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// MemoryCache implements an in-memory LRU cache
type MemoryCache struct {
	entries   map[string]*memoryCacheEntry
	evictList *memoryEntryList
	maxSize   int
	ttl       time.Duration
	mu        sync.RWMutex
	stats     memoryCacheStats
	stopCh    chan struct{}
	cleanup   *time.Ticker
}

// memoryCacheEntry represents a single cache entry
type memoryCacheEntry struct {
	key        string
	value      interface{}
	expiration time.Time
	next       *memoryCacheEntry
	prev       *memoryCacheEntry
}

// memoryEntryList implements a doubly linked list for LRU tracking
type memoryEntryList struct {
	head *memoryCacheEntry
	tail *memoryCacheEntry
	size int
}

// memoryCacheStats tracks cache performance metrics
type memoryCacheStats struct {
	hits        int64
	misses      int64
	evictions   int64
	size        int
	memoryUsed  int64
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxSize int, ttl time.Duration) *MemoryCache {
	mc := &MemoryCache{
		entries:   make(map[string]*memoryCacheEntry),
		evictList: &memoryEntryList{},
		maxSize:   maxSize,
		ttl:       ttl,
		stopCh:    make(chan struct{}),
		cleanup:   time.NewTicker(time.Minute),
	}

	// Start cleanup goroutine
	go mc.cleanupExpired()

	return mc
}

// Get retrieves a value from the cache
func (mc *MemoryCache) Get(key string) (interface{}, bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	entry, exists := mc.entries[key]
	if !exists {
		mc.stats.misses++
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.expiration) {
		mc.removeEntry(entry)
		mc.stats.misses++
		return nil, false
	}

	// Move to front (most recently used)
	mc.evictList.moveToFront(entry)
	mc.stats.hits++
	
	return entry.value, true
}

// Set stores a value in the cache
func (mc *MemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Use instance TTL if none specified
	if ttl == 0 {
		ttl = mc.ttl
	}

	expiration := time.Now().Add(ttl)

	// Check if key already exists
	if entry, exists := mc.entries[key]; exists {
		// Update existing entry
		entry.value = value
		entry.expiration = expiration
		mc.evictList.moveToFront(entry)
		return nil
	}

	// Create new entry
	entry := &memoryCacheEntry{
		key:        key,
		value:      value,
		expiration: expiration,
	}

	mc.entries[key] = entry
	mc.evictList.addToFront(entry)
	mc.stats.size++

	// Evict if over capacity
	if mc.stats.size > mc.maxSize {
		mc.evictOldest()
	}

	return nil
}

// Delete removes a key from the cache
func (mc *MemoryCache) Delete(key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if entry, exists := mc.entries[key]; exists {
		mc.removeEntry(entry)
	}

	return nil
}

// Clear removes all entries from the cache
func (mc *MemoryCache) Clear() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.entries = make(map[string]*memoryCacheEntry)
	mc.evictList = &memoryEntryList{}
	mc.stats.size = 0
	mc.stats.memoryUsed = 0

	return nil
}

// Size returns the current number of entries
func (mc *MemoryCache) Size() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.stats.size
}

// InvalidatePattern removes all entries matching the regex pattern
func (mc *MemoryCache) InvalidatePattern(pattern string) int {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	regex, err := regexp.Compile(pattern)
	if err != nil {
		log.Error().Err(err).Str("pattern", pattern).Msg("Invalid cache invalidation pattern")
		return 0
	}

	var toDelete []*memoryCacheEntry
	for _, entry := range mc.entries {
		if regex.MatchString(entry.key) {
			toDelete = append(toDelete, entry)
		}
	}

	for _, entry := range toDelete {
		mc.removeEntry(entry)
	}

	return len(toDelete)
}

// Stop shuts down the cache and cleanup goroutine
func (mc *MemoryCache) Stop() {
	close(mc.stopCh)
	mc.cleanup.Stop()
}

// GetStats returns cache performance statistics
func (mc *MemoryCache) GetStats() memoryCacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.stats
}

// removeEntry removes an entry from both the map and eviction list
func (mc *MemoryCache) removeEntry(entry *memoryCacheEntry) {
	delete(mc.entries, entry.key)
	mc.evictList.remove(entry)
	mc.stats.size--
}

// evictOldest removes the least recently used entry
func (mc *MemoryCache) evictOldest() {
	if mc.evictList.tail != nil {
		mc.removeEntry(mc.evictList.tail)
		mc.stats.evictions++
	}
}

// cleanupExpired runs periodically to remove expired entries
func (mc *MemoryCache) cleanupExpired() {
	for {
		select {
		case <-mc.cleanup.C:
			mc.mu.Lock()
			now := time.Now()
			var expired []*memoryCacheEntry
			
			for _, entry := range mc.entries {
				if now.After(entry.expiration) {
					expired = append(expired, entry)
				}
			}
			
			for _, entry := range expired {
				mc.removeEntry(entry)
			}
			
			if len(expired) > 0 {
				log.Debug().
					Int("expired", len(expired)).
					Int("remaining", mc.stats.size).
					Msg("Cleaned up expired cache entries")
			}
			mc.mu.Unlock()
			
		case <-mc.stopCh:
			return
		}
	}
}

// memoryEntryList methods

func (list *memoryEntryList) addToFront(entry *memoryCacheEntry) {
	if list.head == nil {
		list.head = entry
		list.tail = entry
	} else {
		entry.next = list.head
		list.head.prev = entry
		list.head = entry
	}
	list.size++
}

func (list *memoryEntryList) moveToFront(entry *memoryCacheEntry) {
	if entry == list.head {
		return
	}

	// Remove from current position
	if entry.prev != nil {
		entry.prev.next = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	}
	if entry == list.tail {
		list.tail = entry.prev
	}

	// Add to front
	entry.prev = nil
	entry.next = list.head
	if list.head != nil {
		list.head.prev = entry
	}
	list.head = entry
}

func (list *memoryEntryList) remove(entry *memoryCacheEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		list.head = entry.next
	}

	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		list.tail = entry.prev
	}

	entry.prev = nil
	entry.next = nil
	list.size--
}