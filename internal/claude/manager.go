package claude

import (
	"sync"
	"time"

	"github.com/ericfisherdev/GoJira/internal/cache"
)

// ClaudeManager handles Claude-specific components and their lifecycle
type ClaudeManager struct {
	formatter     *ResponseFormatter
	processor     *CommandProcessor
	searchCache   *cache.SearchCache
	mutex         sync.RWMutex
	initialized   bool
}

var (
	manager     *ClaudeManager
	managerOnce sync.Once
)

// GetManager returns the singleton ClaudeManager instance
func GetManager() *ClaudeManager {
	managerOnce.Do(func() {
		manager = &ClaudeManager{}
	})
	return manager
}

// Initialize sets up all Claude components
func (cm *ClaudeManager) Initialize() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.initialized {
		return nil
	}

	// Initialize response formatter
	config := FormatterConfig{
		IncludeMetadata:      true,
		UseMarkdown:          true,
		SummarizeResults:     true,
		MaxDescriptionLength: 200,
	}
	cm.formatter = NewResponseFormatter(config)

	// Initialize command processor
	cm.processor = NewCommandProcessor()

	// Initialize search cache (5 minute TTL, max 1000 entries)
	cm.searchCache = cache.NewSearchCache(5*time.Minute, 1000)

	cm.initialized = true
	return nil
}

// GetFormatter returns the response formatter, initializing if necessary
func (cm *ClaudeManager) GetFormatter() *ResponseFormatter {
	if !cm.initialized {
		cm.Initialize()
	}
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.formatter
}

// GetProcessor returns the command processor, initializing if necessary
func (cm *ClaudeManager) GetProcessor() *CommandProcessor {
	if !cm.initialized {
		cm.Initialize()
	}
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.processor
}

// GetSearchCache returns the search cache, initializing if necessary
func (cm *ClaudeManager) GetSearchCache() *cache.SearchCache {
	if !cm.initialized {
		cm.Initialize()
	}
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.searchCache
}

// IsInitialized returns whether the manager has been initialized
func (cm *ClaudeManager) IsInitialized() bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.initialized
}

// Shutdown gracefully shuts down the Claude components
func (cm *ClaudeManager) Shutdown() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.searchCache != nil {
		cm.searchCache.Clear()
	}

	cm.initialized = false
	return nil
}

// GetStats returns statistics about Claude components
func (cm *ClaudeManager) GetStats() map[string]interface{} {
	if !cm.initialized {
		return map[string]interface{}{
			"initialized": false,
		}
	}

	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	stats := map[string]interface{}{
		"initialized": cm.initialized,
		"formatter":   cm.formatter != nil,
		"processor":   cm.processor != nil,
	}

	if cm.searchCache != nil {
		stats["searchCache"] = cm.searchCache.GetStats()
	}

	return stats
}

// UpdateFormatterConfig allows runtime configuration updates
func (cm *ClaudeManager) UpdateFormatterConfig(config FormatterConfig) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if config.MaxDescriptionLength == 0 {
		config.MaxDescriptionLength = 200
	}

	cm.formatter = NewResponseFormatter(config)
}