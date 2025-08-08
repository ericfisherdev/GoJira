package cache

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// DiskCache implements a persistent disk-based cache
type DiskCache struct {
	baseDir    string
	maxSize    int64
	ttl        time.Duration
	compress   bool
	mu         sync.RWMutex
	stats      diskCacheStats
	stopCh     chan struct{}
	cleanup    *time.Ticker
	sizeCache  map[string]int64
}

// diskCacheEntry represents metadata for a cached file
type diskCacheEntry struct {
	Key        string    `gob:"key"`
	Expiration time.Time `gob:"expiration"`
	Size       int64     `gob:"size"`
	Compressed bool      `gob:"compressed"`
}

// diskCacheStats tracks disk cache performance
type diskCacheStats struct {
	hits        int64
	misses      int64
	size        int64
	fileCount   int
	diskUsed    int64
	cleanups    int64
}

// NewDiskCache creates a new disk-based cache
func NewDiskCache(maxSize int64, ttl time.Duration) *DiskCache {
	baseDir := filepath.Join(os.TempDir(), "gojira-cache")
	
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		log.Error().Err(err).Str("dir", baseDir).Msg("Failed to create cache directory")
		return nil
	}

	dc := &DiskCache{
		baseDir:   baseDir,
		maxSize:   maxSize,
		ttl:       ttl,
		compress:  true,
		stopCh:    make(chan struct{}),
		cleanup:   time.NewTicker(10 * time.Minute),
		sizeCache: make(map[string]int64),
	}

	// Initialize size tracking
	dc.calculateDiskUsage()

	// Start cleanup goroutine
	go dc.cleanupExpired()

	return dc
}

// Get retrieves a value from the disk cache
func (dc *DiskCache) Get(key string) (interface{}, error) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	filePath := dc.getFilePath(key)
	
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		dc.stats.misses++
		return nil, fmt.Errorf("cache miss: key not found")
	}

	// Read metadata
	entry, err := dc.readMetadata(filePath)
	if err != nil {
		dc.stats.misses++
		return nil, fmt.Errorf("failed to read metadata: %v", err)
	}

	// Check if expired
	if time.Now().After(entry.Expiration) {
		dc.deleteFile(filePath)
		dc.stats.misses++
		return nil, fmt.Errorf("cache miss: entry expired")
	}

	// Read data
	value, err := dc.readData(filePath, entry.Compressed)
	if err != nil {
		dc.stats.misses++
		return nil, fmt.Errorf("failed to read data: %v", err)
	}

	dc.stats.hits++
	return value, nil
}

// Set stores a value in the disk cache
func (dc *DiskCache) Set(key string, value interface{}, ttl time.Duration) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if ttl == 0 {
		ttl = dc.ttl
	}

	filePath := dc.getFilePath(key)
	expiration := time.Now().Add(ttl)

	// Serialize data
	data, err := dc.serialize(value)
	if err != nil {
		return fmt.Errorf("serialization failed: %v", err)
	}

	// Compress if enabled
	compressed := false
	if dc.compress && len(data) > 1024 { // Only compress if > 1KB
		if compressedData, err := dc.compressData(data); err == nil && len(compressedData) < len(data) {
			data = compressedData
			compressed = true
		}
	}

	// Check if we need to make space
	dataSize := int64(len(data))
	if dc.stats.diskUsed+dataSize > dc.maxSize {
		if err := dc.evictOldest(dataSize); err != nil {
			return fmt.Errorf("failed to make space: %v", err)
		}
	}

	// Create entry metadata
	entry := diskCacheEntry{
		Key:        key,
		Expiration: expiration,
		Size:       dataSize,
		Compressed: compressed,
	}

	// Write to disk
	if err := dc.writeFile(filePath, entry, data); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	// Update stats
	if oldSize, exists := dc.sizeCache[key]; exists {
		dc.stats.diskUsed -= oldSize
	} else {
		dc.stats.fileCount++
	}
	dc.stats.diskUsed += dataSize
	dc.sizeCache[key] = dataSize

	log.Debug().
		Str("key", key).
		Int64("size", dataSize).
		Bool("compressed", compressed).
		Msg("Stored in disk cache")

	return nil
}

// Delete removes a key from the disk cache
func (dc *DiskCache) Delete(key string) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	filePath := dc.getFilePath(key)
	return dc.deleteFile(filePath)
}

// Clear removes all entries from the disk cache
func (dc *DiskCache) Clear() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Remove all files in cache directory
	entries, err := os.ReadDir(dc.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(dc.baseDir, entry.Name())
			if err := os.Remove(filePath); err != nil {
				log.Warn().Err(err).Str("file", filePath).Msg("Failed to remove cache file")
			}
		}
	}

	// Reset stats
	dc.stats = diskCacheStats{}
	dc.sizeCache = make(map[string]int64)

	return nil
}

// InvalidatePattern removes all entries matching the regex pattern
func (dc *DiskCache) InvalidatePattern(pattern string) int {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	regex, err := regexp.Compile(pattern)
	if err != nil {
		log.Error().Err(err).Str("pattern", pattern).Msg("Invalid cache invalidation pattern")
		return 0
	}

	count := 0
	for key := range dc.sizeCache {
		if regex.MatchString(key) {
			filePath := dc.getFilePath(key)
			if err := dc.deleteFile(filePath); err == nil {
				count++
			}
		}
	}

	return count
}

// Stop shuts down the disk cache
func (dc *DiskCache) Stop() {
	close(dc.stopCh)
	dc.cleanup.Stop()
}

// GetStats returns disk cache performance statistics
func (dc *DiskCache) GetStats() diskCacheStats {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.stats
}

// Helper methods

func (dc *DiskCache) getFilePath(key string) string {
	// Use MD5 hash to create safe filename
	hash := md5.Sum([]byte(key))
	filename := fmt.Sprintf("%x.cache", hash)
	return filepath.Join(dc.baseDir, filename)
}

func (dc *DiskCache) serialize(value interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (dc *DiskCache) deserialize(data []byte) (interface{}, error) {
	buf := bytes.NewReader(data)
	decoder := gob.NewDecoder(buf)
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func (dc *DiskCache) compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (dc *DiskCache) decompressData(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func (dc *DiskCache) writeFile(filePath string, entry diskCacheEntry, data []byte) error {
	// Create temporary file first
	tempPath := filePath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write metadata header
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(entry); err != nil {
		os.Remove(tempPath)
		return err
	}

	// Write data
	if _, err := file.Write(data); err != nil {
		os.Remove(tempPath)
		return err
	}

	if err := file.Sync(); err != nil {
		os.Remove(tempPath)
		return err
	}

	// Atomically move to final location
	return os.Rename(tempPath, filePath)
}

func (dc *DiskCache) readMetadata(filePath string) (*diskCacheEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entry diskCacheEntry
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

func (dc *DiskCache) readData(filePath string, compressed bool) (interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Skip metadata header
	var entry diskCacheEntry
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&entry); err != nil {
		return nil, err
	}

	// Read remaining data
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Decompress if needed
	if compressed {
		if data, err = dc.decompressData(data); err != nil {
			return nil, err
		}
	}

	// Deserialize
	return dc.deserialize(data)
}

func (dc *DiskCache) deleteFile(filePath string) error {
	// Get file info for size tracking
	if info, err := os.Stat(filePath); err == nil {
		dc.stats.diskUsed -= info.Size()
		dc.stats.fileCount--
		
		// Extract key from path for size cache cleanup
		for key, size := range dc.sizeCache {
			if dc.getFilePath(key) == filePath {
				delete(dc.sizeCache, key)
				dc.stats.diskUsed += size // Adjust for actual vs tracked size
				break
			}
		}
	}

	return os.Remove(filePath)
}

func (dc *DiskCache) calculateDiskUsage() {
	entries, err := os.ReadDir(dc.baseDir)
	if err != nil {
		return
	}

	dc.stats.diskUsed = 0
	dc.stats.fileCount = 0

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".cache" {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			dc.stats.diskUsed += info.Size()
			dc.stats.fileCount++
		}
	}
}

func (dc *DiskCache) evictOldest(neededSpace int64) error {
	// Get all cache files with their modification times
	type fileInfo struct {
		path    string
		modTime time.Time
		size    int64
	}

	var files []fileInfo
	entries, err := os.ReadDir(dc.baseDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".cache" {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fileInfo{
				path:    filepath.Join(dc.baseDir, entry.Name()),
				modTime: info.ModTime(),
				size:    info.Size(),
			})
		}
	}

	// Sort by modification time (oldest first)
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.After(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	// Remove oldest files until we have enough space
	freedSpace := int64(0)
	for _, file := range files {
		if freedSpace >= neededSpace {
			break
		}
		if err := dc.deleteFile(file.path); err != nil {
			log.Warn().Err(err).Str("file", file.path).Msg("Failed to evict cache file")
		} else {
			freedSpace += file.size
		}
	}

	return nil
}

func (dc *DiskCache) cleanupExpired() {
	for {
		select {
		case <-dc.cleanup.C:
			dc.mu.Lock()
			now := time.Now()
			cleaned := 0
			
			entries, err := os.ReadDir(dc.baseDir)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to read cache directory for cleanup")
				dc.mu.Unlock()
				continue
			}

			for _, entry := range entries {
				if entry.IsDir() || filepath.Ext(entry.Name()) != ".cache" {
					continue
				}

				filePath := filepath.Join(dc.baseDir, entry.Name())
				if metadata, err := dc.readMetadata(filePath); err == nil {
					if now.After(metadata.Expiration) {
						if err := dc.deleteFile(filePath); err == nil {
							cleaned++
						}
					}
				}
			}

			if cleaned > 0 {
				log.Debug().
					Int("cleaned", cleaned).
					Msg("Cleaned up expired disk cache entries")
			}
			
			dc.stats.cleanups++
			dc.mu.Unlock()

		case <-dc.stopCh:
			return
		}
	}
}

// RedisCache is a placeholder for Redis-based caching (L2 cache)
// This would be implemented if Redis support is needed
type RedisCache struct {
	// Implementation would depend on Redis client library
}

func (rc *RedisCache) Get(key string) (interface{}, error) {
	// Redis implementation
	return nil, fmt.Errorf("Redis cache not implemented")
}

func (rc *RedisCache) Set(key string, value interface{}, ttl time.Duration) error {
	// Redis implementation
	return fmt.Errorf("Redis cache not implemented")
}

func (rc *RedisCache) Delete(key string) error {
	// Redis implementation
	return fmt.Errorf("Redis cache not implemented")
}

func (rc *RedisCache) Clear() error {
	// Redis implementation
	return fmt.Errorf("Redis cache not implemented")
}

func (rc *RedisCache) InvalidatePattern(pattern string) int {
	// Redis implementation
	return 0
}

func (rc *RedisCache) Stop() {
	// Redis implementation
}