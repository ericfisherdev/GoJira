package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// MultiLevelCache implements a multi-tier caching strategy
type MultiLevelCache struct {
	l1Cache  *MemoryCache      // In-memory cache (fastest)
	l2Cache  *RedisCache       // Redis cache (shared) - optional
	l3Cache  *DiskCache        // Disk cache (persistent) - optional
	strategy CacheStrategy
	stats    MultiLevelStats
	mu       sync.RWMutex
}

// CacheStrategy defines caching behavior across levels
type CacheStrategy struct {
	L1TTL      time.Duration `json:"l1TTL"`
	L2TTL      time.Duration `json:"l2TTL"`
	L3TTL      time.Duration `json:"l3TTL"`
	MaxL1Size  int           `json:"maxL1Size"`
	MaxL3Size  int64         `json:"maxL3Size"`
	Compress   bool          `json:"compress"`
	PromoteToL1 bool         `json:"promoteToL1"`
	WriteThrough bool        `json:"writeThrough"`
}

// MultiLevelStats tracks performance across cache levels
type MultiLevelStats struct {
	L1Hits   int64 `json:"l1Hits"`
	L2Hits   int64 `json:"l2Hits"`
	L3Hits   int64 `json:"l3Hits"`
	Misses   int64 `json:"misses"`
	L1Evictions int64 `json:"l1Evictions"`
	L2Errors int64 `json:"l2Errors"`
	L3Errors int64 `json:"l3Errors"`
	TotalRequests int64 `json:"totalRequests"`
}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache(strategy CacheStrategy) *MultiLevelCache {
	if strategy.L1TTL == 0 {
		strategy.L1TTL = 5 * time.Minute
	}
	if strategy.L2TTL == 0 {
		strategy.L2TTL = 15 * time.Minute
	}
	if strategy.L3TTL == 0 {
		strategy.L3TTL = time.Hour
	}
	if strategy.MaxL1Size == 0 {
		strategy.MaxL1Size = 1000
	}

	l1Cache := NewMemoryCache(strategy.MaxL1Size, strategy.L1TTL)
	
	mc := &MultiLevelCache{
		l1Cache:  l1Cache,
		strategy: strategy,
		stats:    MultiLevelStats{},
	}

	// Initialize optional cache levels
	if strategy.MaxL3Size > 0 {
		mc.l3Cache = NewDiskCache(strategy.MaxL3Size, strategy.L3TTL)
	}

	return mc
}

// Get retrieves a value from the multi-level cache
func (mc *MultiLevelCache) Get(key string) (interface{}, bool) {
	mc.mu.Lock()
	mc.stats.TotalRequests++
	mc.mu.Unlock()

	// Check L1 (memory) first
	if val, exists := mc.l1Cache.Get(key); exists {
		mc.mu.Lock()
		mc.stats.L1Hits++
		mc.mu.Unlock()
		
		log.Debug().
			Str("key", key).
			Str("level", "L1").
			Msg("Cache hit")
		return val, true
	}

	// Check L2 (Redis) if available
	if mc.l2Cache != nil {
		if val, err := mc.l2Cache.Get(key); err == nil {
			mc.mu.Lock()
			mc.stats.L2Hits++
			mc.mu.Unlock()
			
			// Promote to L1 if enabled
			if mc.strategy.PromoteToL1 {
				mc.l1Cache.Set(key, val, mc.strategy.L1TTL)
			}
			
			log.Debug().
				Str("key", key).
				Str("level", "L2").
				Msg("Cache hit")
			return val, true
		} else {
			mc.mu.Lock()
			mc.stats.L2Errors++
			mc.mu.Unlock()
			log.Debug().Err(err).Str("key", key).Msg("L2 cache error")
		}
	}

	// Check L3 (disk) if available
	if mc.l3Cache != nil {
		if val, err := mc.l3Cache.Get(key); err == nil {
			mc.mu.Lock()
			mc.stats.L3Hits++
			mc.mu.Unlock()
			
			// Promote to L1 and L2 if enabled
			if mc.strategy.PromoteToL1 {
				mc.l1Cache.Set(key, val, mc.strategy.L1TTL)
			}
			if mc.l2Cache != nil {
				if err := mc.l2Cache.Set(key, val, mc.strategy.L2TTL); err != nil {
					log.Debug().Err(err).Str("key", key).Msg("Failed to promote to L2")
				}
			}
			
			log.Debug().
				Str("key", key).
				Str("level", "L3").
				Msg("Cache hit")
			return val, true
		} else {
			mc.mu.Lock()
			mc.stats.L3Errors++
			mc.mu.Unlock()
			log.Debug().Err(err).Str("key", key).Msg("L3 cache error")
		}
	}

	// Cache miss
	mc.mu.Lock()
	mc.stats.Misses++
	mc.mu.Unlock()
	
	log.Debug().
		Str("key", key).
		Msg("Cache miss")
	return nil, false
}

// Set stores a value in the multi-level cache
func (mc *MultiLevelCache) Set(key string, value interface{}, ttl time.Duration) error {
	var errs []error

	// Always store in L1
	if err := mc.l1Cache.Set(key, value, mc.strategy.L1TTL); err != nil {
		errs = append(errs, fmt.Errorf("L1 set failed: %v", err))
	}

	// Store in L2 if available and write-through enabled
	if mc.strategy.WriteThrough && mc.l2Cache != nil {
		if err := mc.l2Cache.Set(key, value, mc.strategy.L2TTL); err != nil {
			errs = append(errs, fmt.Errorf("L2 set failed: %v", err))
			log.Debug().Err(err).Str("key", key).Msg("L2 cache set error")
		}
	}

	// Store in L3 if available and write-through enabled
	if mc.strategy.WriteThrough && mc.l3Cache != nil {
		if err := mc.l3Cache.Set(key, value, mc.strategy.L3TTL); err != nil {
			errs = append(errs, fmt.Errorf("L3 set failed: %v", err))
			log.Debug().Err(err).Str("key", key).Msg("L3 cache set error")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cache set errors: %v", errs)
	}

	log.Debug().
		Str("key", key).
		Msg("Multi-level cache set")
	return nil
}

// Delete removes a value from all cache levels
func (mc *MultiLevelCache) Delete(key string) error {
	var errs []error

	// Delete from all levels
	if err := mc.l1Cache.Delete(key); err != nil {
		errs = append(errs, fmt.Errorf("L1 delete failed: %v", err))
	}

	if mc.l2Cache != nil {
		if err := mc.l2Cache.Delete(key); err != nil {
			errs = append(errs, fmt.Errorf("L2 delete failed: %v", err))
			log.Debug().Err(err).Str("key", key).Msg("L2 cache delete error")
		}
	}

	if mc.l3Cache != nil {
		if err := mc.l3Cache.Delete(key); err != nil {
			errs = append(errs, fmt.Errorf("L3 delete failed: %v", err))
			log.Debug().Err(err).Str("key", key).Msg("L3 cache delete error")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cache delete errors: %v", errs)
	}

	return nil
}

// Clear removes all entries from all cache levels
func (mc *MultiLevelCache) Clear() error {
	var errs []error

	if err := mc.l1Cache.Clear(); err != nil {
		errs = append(errs, fmt.Errorf("L1 clear failed: %v", err))
	}

	if mc.l2Cache != nil {
		if err := mc.l2Cache.Clear(); err != nil {
			errs = append(errs, fmt.Errorf("L2 clear failed: %v", err))
		}
	}

	if mc.l3Cache != nil {
		if err := mc.l3Cache.Clear(); err != nil {
			errs = append(errs, fmt.Errorf("L3 clear failed: %v", err))
		}
	}

	// Reset stats
	mc.mu.Lock()
	mc.stats = MultiLevelStats{}
	mc.mu.Unlock()

	if len(errs) > 0 {
		return fmt.Errorf("cache clear errors: %v", errs)
	}

	return nil
}

// GetStats returns multi-level cache statistics
func (mc *MultiLevelCache) GetStats() MultiLevelStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.stats
}

// GetDetailedStats returns comprehensive cache statistics
func (mc *MultiLevelCache) GetDetailedStats() map[string]interface{} {
	stats := mc.GetStats()
	
	hitRate := float64(0)
	if stats.TotalRequests > 0 {
		hitRate = float64(stats.L1Hits+stats.L2Hits+stats.L3Hits) / float64(stats.TotalRequests)
	}

	l1HitRate := float64(0)
	if stats.TotalRequests > 0 {
		l1HitRate = float64(stats.L1Hits) / float64(stats.TotalRequests)
	}

	return map[string]interface{}{
		"totalRequests": stats.TotalRequests,
		"totalHits":     stats.L1Hits + stats.L2Hits + stats.L3Hits,
		"totalMisses":   stats.Misses,
		"hitRate":       hitRate,
		"l1": map[string]interface{}{
			"hits":     stats.L1Hits,
			"hitRate":  l1HitRate,
			"evictions": stats.L1Evictions,
			"size":     mc.l1Cache.Size(),
		},
		"l2": map[string]interface{}{
			"hits":   stats.L2Hits,
			"errors": stats.L2Errors,
			"enabled": mc.l2Cache != nil,
		},
		"l3": map[string]interface{}{
			"hits":   stats.L3Hits,
			"errors": stats.L3Errors,
			"enabled": mc.l3Cache != nil,
		},
		"strategy": mc.strategy,
	}
}

// Invalidate removes entries matching a pattern from all levels
func (mc *MultiLevelCache) Invalidate(pattern string) int {
	count := 0
	
	if invalidated := mc.l1Cache.InvalidatePattern(pattern); invalidated > 0 {
		count += invalidated
	}

	if mc.l2Cache != nil {
		if invalidated := mc.l2Cache.InvalidatePattern(pattern); invalidated > 0 {
			count += invalidated
		}
	}

	if mc.l3Cache != nil {
		if invalidated := mc.l3Cache.InvalidatePattern(pattern); invalidated > 0 {
			count += invalidated
		}
	}

	return count
}

// Stop gracefully shuts down the multi-level cache
func (mc *MultiLevelCache) Stop() {
	log.Info().Msg("Shutting down multi-level cache")
	
	if mc.l1Cache != nil {
		mc.l1Cache.Stop()
	}
	
	if mc.l2Cache != nil {
		mc.l2Cache.Stop()
	}
	
	if mc.l3Cache != nil {
		mc.l3Cache.Stop()
	}
}

// SetL2Cache sets the Redis cache (L2) implementation
func (mc *MultiLevelCache) SetL2Cache(cache *RedisCache) {
	mc.l2Cache = cache
}

// SetL3Cache sets the disk cache (L3) implementation
func (mc *MultiLevelCache) SetL3Cache(cache *DiskCache) {
	mc.l3Cache = cache
}

// Warm pre-loads frequently accessed data into cache
func (mc *MultiLevelCache) Warm(keys []string, loader func(key string) (interface{}, error)) error {
	for _, key := range keys {
		// Check if already cached
		if _, exists := mc.Get(key); !exists {
			// Load and cache the data
			if value, err := loader(key); err == nil {
				if err := mc.Set(key, value, mc.strategy.L1TTL); err != nil {
					log.Warn().
						Err(err).
						Str("key", key).
						Msg("Failed to warm cache")
				}
			} else {
				log.Debug().
					Err(err).
					Str("key", key).
					Msg("Failed to load data for cache warming")
			}
		}
	}
	return nil
}

// DefaultMultiLevelStrategy returns a sensible default caching strategy
func DefaultMultiLevelStrategy() CacheStrategy {
	return CacheStrategy{
		L1TTL:       5 * time.Minute,
		L2TTL:       15 * time.Minute,
		L3TTL:       time.Hour,
		MaxL1Size:   1000,
		MaxL3Size:   100 * 1024 * 1024, // 100MB
		Compress:    true,
		PromoteToL1: true,
		WriteThrough: false, // Write-behind is typically better for performance
	}
}

// AggressiveStrategy returns a strategy optimized for high performance
func AggressiveStrategy() CacheStrategy {
	return CacheStrategy{
		L1TTL:       10 * time.Minute,
		L2TTL:       30 * time.Minute,
		L3TTL:       2 * time.Hour,
		MaxL1Size:   2000,
		MaxL3Size:   500 * 1024 * 1024, // 500MB
		Compress:    true,
		PromoteToL1: true,
		WriteThrough: true,
	}
}

// ConservativeStrategy returns a strategy optimized for memory efficiency
func ConservativeStrategy() CacheStrategy {
	return CacheStrategy{
		L1TTL:       2 * time.Minute,
		L2TTL:       10 * time.Minute,
		L3TTL:       30 * time.Minute,
		MaxL1Size:   500,
		MaxL3Size:   50 * 1024 * 1024, // 50MB
		Compress:    true,
		PromoteToL1: false,
		WriteThrough: false,
	}
}