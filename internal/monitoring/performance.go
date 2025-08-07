package monitoring

import (
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// DetailedPerformanceMonitor tracks and analyzes application performance metrics
type DetailedPerformanceMonitor struct {
	metrics   map[string]*Metric
	mu        sync.RWMutex
	reporter  Reporter
	config    PerformanceConfig
}

// Metric represents performance data for a specific operation
type Metric struct {
	Name         string        `json:"name"`
	Count        int64         `json:"count"`
	TotalTime    time.Duration `json:"totalTime"`
	MinTime      time.Duration `json:"minTime"`
	MaxTime      time.Duration `json:"maxTime"`
	AvgTime      time.Duration `json:"avgTime"`
	P95Time      time.Duration `json:"p95Time"`
	P99Time      time.Duration `json:"p99Time"`
	samples      []time.Duration
	lastUpdated  time.Time
	Errors       int64         `json:"errors"`
	Throughput   float64       `json:"throughput"` // operations per second
}

// PerformanceConfig configures monitoring behavior
type PerformanceConfig struct {
	SampleSize        int           `json:"sampleSize"`        // Max samples to keep for percentile calculation
	ReportInterval    time.Duration `json:"reportInterval"`    // How often to report metrics
	SlowThreshold     time.Duration `json:"slowThreshold"`     // Threshold for slow operations
	EnablePercentiles bool          `json:"enablePercentiles"` // Whether to calculate percentiles
	EnableReporting   bool          `json:"enableReporting"`   // Whether to enable automated reporting
}

// Reporter defines the interface for metric reporting
type Reporter interface {
	Report(metrics map[string]*Metric) error
}

// OperationTimer provides timing functionality for operations
type OperationTimer struct {
	operation string
	monitor   *DetailedPerformanceMonitor
	startTime time.Time
	success   bool
}

// NewDetailedPerformanceMonitor creates a new detailed performance monitoring instance
func NewDetailedPerformanceMonitor(config PerformanceConfig) *DetailedPerformanceMonitor {
	if config.SampleSize == 0 {
		config.SampleSize = 1000
	}
	if config.ReportInterval == 0 {
		config.ReportInterval = time.Minute
	}
	if config.SlowThreshold == 0 {
		config.SlowThreshold = time.Second
	}

	pm := &DetailedPerformanceMonitor{
		metrics: make(map[string]*Metric),
		config:  config,
	}

	// Start reporting goroutine if enabled
	if config.EnableReporting && config.ReportInterval > 0 {
		go pm.reportingLoop()
	}

	return pm
}

// StartTimer begins timing an operation
func (pm *DetailedPerformanceMonitor) StartTimer(operation string) *OperationTimer {
	return &OperationTimer{
		operation: operation,
		monitor:   pm,
		startTime: time.Now(),
		success:   true, // Default to success, call SetError to mark as error
	}
}

// RecordMetric records a timing metric for an operation
func (pm *DetailedPerformanceMonitor) RecordMetric(operation string, duration time.Duration) {
	pm.recordMetricWithStatus(operation, duration, true)
}

// RecordError records a failed operation timing
func (pm *DetailedPerformanceMonitor) RecordError(operation string, duration time.Duration) {
	pm.recordMetricWithStatus(operation, duration, false)
}

// recordMetricWithStatus internal method to record metrics with success/failure status
func (pm *DetailedPerformanceMonitor) recordMetricWithStatus(operation string, duration time.Duration, success bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	metric, exists := pm.metrics[operation]
	if !exists {
		metric = &Metric{
			Name:        operation,
			MinTime:     duration,
			MaxTime:     duration,
			samples:     make([]time.Duration, 0, pm.config.SampleSize),
			lastUpdated: time.Now(),
		}
		pm.metrics[operation] = metric
	}

	// Update basic metrics
	metric.Count++
	metric.TotalTime += duration
	metric.AvgTime = metric.TotalTime / time.Duration(metric.Count)
	metric.lastUpdated = time.Now()

	if !success {
		metric.Errors++
	}

	// Update min/max
	if duration < metric.MinTime {
		metric.MinTime = duration
	}
	if duration > metric.MaxTime {
		metric.MaxTime = duration
	}

	// Calculate throughput (operations per second)
	if metric.Count > 1 {
		totalSeconds := time.Since(time.Now().Add(-metric.TotalTime)).Seconds()
		if totalSeconds > 0 {
			metric.Throughput = float64(metric.Count) / totalSeconds
		}
	}

	// Maintain samples for percentile calculation
	if pm.config.EnablePercentiles {
		metric.samples = append(metric.samples, duration)
		
		// Keep only the most recent samples
		if len(metric.samples) > pm.config.SampleSize {
			metric.samples = metric.samples[len(metric.samples)-pm.config.SampleSize:]
		}

		// Calculate percentiles periodically or when we have enough samples
		if len(metric.samples) >= 10 && (metric.Count%100 == 0 || len(metric.samples) == pm.config.SampleSize) {
			pm.updatePercentiles(metric)
		}
	}

	// Log slow operations
	if duration > pm.config.SlowThreshold {
		log.Warn().
			Str("operation", operation).
			Dur("duration", duration).
			Dur("threshold", pm.config.SlowThreshold).
			Msg("Slow operation detected")
	}
}

// updatePercentiles calculates P95 and P99 percentiles for a metric
func (pm *DetailedPerformanceMonitor) updatePercentiles(metric *Metric) {
	if len(metric.samples) == 0 {
		return
	}

	// Make a copy and sort
	samples := make([]time.Duration, len(metric.samples))
	copy(samples, metric.samples)
	sort.Slice(samples, func(i, j int) bool {
		return samples[i] < samples[j]
	})

	// Calculate percentiles
	n := len(samples)
	if n > 0 {
		p95Index := int(float64(n) * 0.95)
		if p95Index >= n {
			p95Index = n - 1
		}
		metric.P95Time = samples[p95Index]

		p99Index := int(float64(n) * 0.99)
		if p99Index >= n {
			p99Index = n - 1
		}
		metric.P99Time = samples[p99Index]
	}
}

// GetMetric returns the metric for a specific operation
func (pm *DetailedPerformanceMonitor) GetMetric(operation string) (*Metric, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	metric, exists := pm.metrics[operation]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	metricCopy := *metric
	return &metricCopy, true
}

// GetAllMetrics returns a copy of all current metrics
func (pm *DetailedPerformanceMonitor) GetAllMetrics() map[string]*Metric {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]*Metric, len(pm.metrics))
	for name, metric := range pm.metrics {
		metricCopy := *metric
		// Don't expose the samples array
		metricCopy.samples = nil
		result[name] = &metricCopy
	}

	return result
}

// GetSummaryStats returns overall performance statistics
func (pm *DetailedPerformanceMonitor) GetSummaryStats() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	totalOps := int64(0)
	totalErrors := int64(0)
	totalTime := time.Duration(0)
	slowOps := 0
	operationCount := len(pm.metrics)

	var slowestOp string
	var slowestTime time.Duration

	for _, metric := range pm.metrics {
		totalOps += metric.Count
		totalErrors += metric.Errors
		totalTime += metric.TotalTime

		if metric.MaxTime > pm.config.SlowThreshold {
			slowOps++
		}

		if metric.MaxTime > slowestTime {
			slowestTime = metric.MaxTime
			slowestOp = metric.Name
		}
	}

	errorRate := float64(0)
	if totalOps > 0 {
		errorRate = float64(totalErrors) / float64(totalOps) * 100
	}

	avgResponseTime := time.Duration(0)
	if totalOps > 0 {
		avgResponseTime = totalTime / time.Duration(totalOps)
	}

	return map[string]interface{}{
		"totalOperations":    totalOps,
		"totalErrors":        totalErrors,
		"errorRate":          errorRate,
		"operationTypes":     operationCount,
		"slowOperations":     slowOps,
		"averageResponseTime": avgResponseTime,
		"slowestOperation":   slowestOp,
		"slowestTime":        slowestTime,
		"slowThreshold":      pm.config.SlowThreshold,
	}
}

// ResetMetrics clears all collected metrics
func (pm *DetailedPerformanceMonitor) ResetMetrics() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.metrics = make(map[string]*Metric)
	log.Info().Msg("Performance metrics reset")
}

// SetReporter sets the reporter for automated metric reporting
func (pm *DetailedPerformanceMonitor) SetReporter(reporter Reporter) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.reporter = reporter
}

// reportingLoop runs automated reporting
func (pm *DetailedPerformanceMonitor) reportingLoop() {
	ticker := time.NewTicker(pm.config.ReportInterval)
	defer ticker.Stop()

	for range ticker.C {
		if pm.reporter != nil {
			metrics := pm.GetAllMetrics()
			if len(metrics) > 0 {
				if err := pm.reporter.Report(metrics); err != nil {
					log.Error().Err(err).Msg("Failed to report performance metrics")
				}
			}
		}
	}
}

// OperationTimer methods

// Success marks the operation as successful and records the timing
func (ot *OperationTimer) Success() time.Duration {
	duration := time.Since(ot.startTime)
	ot.monitor.recordMetricWithStatus(ot.operation, duration, true)
	return duration
}

// Error marks the operation as failed and records the timing
func (ot *OperationTimer) Error() time.Duration {
	duration := time.Since(ot.startTime)
	ot.monitor.recordMetricWithStatus(ot.operation, duration, false)
	return duration
}

// Duration returns the elapsed time without recording the metric
func (ot *OperationTimer) Duration() time.Duration {
	return time.Since(ot.startTime)
}

// Helper functions for common patterns

// TimedFunction wraps a function with timing
func (pm *DetailedPerformanceMonitor) TimedFunction(operation string, fn func() error) error {
	timer := pm.StartTimer(operation)
	
	err := fn()
	if err != nil {
		timer.Error()
		return err
	}
	
	timer.Success()
	return nil
}

// TimedFunctionWithResult wraps a function with timing and returns both result and error
func TimedFunctionWithResult[T any](pm *DetailedPerformanceMonitor, operation string, fn func() (T, error)) (T, error) {
	timer := pm.StartTimer(operation)
	
	result, err := fn()
	if err != nil {
		timer.Error()
		return result, err
	}
	
	timer.Success()
	return result, nil
}

// ConsoleReporter implements Reporter for console output
type ConsoleReporter struct{}

// Report outputs metrics to console
func (cr *ConsoleReporter) Report(metrics map[string]*Metric) error {
	log.Info().Msg("=== Performance Metrics Report ===")
	
	for name, metric := range metrics {
		errorRate := float64(0)
		if metric.Count > 0 {
			errorRate = float64(metric.Errors) / float64(metric.Count) * 100
		}

		log.Info().
			Str("operation", name).
			Int64("count", metric.Count).
			Dur("avg", metric.AvgTime).
			Dur("min", metric.MinTime).
			Dur("max", metric.MaxTime).
			Dur("p95", metric.P95Time).
			Dur("p99", metric.P99Time).
			Float64("errorRate", errorRate).
			Float64("throughput", metric.Throughput).
			Msg("Metric")
	}
	
	return nil
}

// DefaultPerformanceConfig returns a sensible default configuration
func DefaultPerformanceConfig() PerformanceConfig {
	return PerformanceConfig{
		SampleSize:        1000,
		ReportInterval:    5 * time.Minute,
		SlowThreshold:     time.Second,
		EnablePercentiles: true,
		EnableReporting:   false, // Disabled by default to avoid spam
	}
}

// DevelopmentPerformanceConfig returns configuration optimized for development
func DevelopmentPerformanceConfig() PerformanceConfig {
	return PerformanceConfig{
		SampleSize:        500,
		ReportInterval:    time.Minute,
		SlowThreshold:     500 * time.Millisecond,
		EnablePercentiles: true,
		EnableReporting:   true,
	}
}

// ProductionPerformanceConfig returns configuration optimized for production
func ProductionPerformanceConfig() PerformanceConfig {
	return PerformanceConfig{
		SampleSize:        2000,
		ReportInterval:    10 * time.Minute,
		SlowThreshold:     2 * time.Second,
		EnablePercentiles: true,
		EnableReporting:   false, // Use external monitoring instead
	}
}