package monitoring

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Metrics struct {
	mutex              sync.RWMutex
	requestCount       int64
	errorCount         int64
	totalResponseTime  time.Duration
	jiraAPICallCount   int64
	jiraAPIErrorCount  int64
	cacheHitCount      int64
	cacheMissCount     int64
	startTime          time.Time
}

var GlobalMetrics = &Metrics{
	startTime: time.Now(),
}

func (m *Metrics) IncrementRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.requestCount++
}

func (m *Metrics) IncrementErrors() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.errorCount++
}

func (m *Metrics) AddResponseTime(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.totalResponseTime += duration
}

func (m *Metrics) IncrementJiraAPICalls() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.jiraAPICallCount++
}

func (m *Metrics) IncrementJiraAPIErrors() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.jiraAPIErrorCount++
}

func (m *Metrics) IncrementCacheHits() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cacheHitCount++
}

func (m *Metrics) IncrementCacheMisses() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cacheMissCount++
}

func (m *Metrics) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	avgResponseTime := time.Duration(0)
	if m.requestCount > 0 {
		avgResponseTime = m.totalResponseTime / time.Duration(m.requestCount)
	}

	cacheHitRatio := float64(0)
	totalCacheRequests := m.cacheHitCount + m.cacheMissCount
	if totalCacheRequests > 0 {
		cacheHitRatio = float64(m.cacheHitCount) / float64(totalCacheRequests)
	}

	jiraAPIErrorRate := float64(0)
	if m.jiraAPICallCount > 0 {
		jiraAPIErrorRate = float64(m.jiraAPIErrorCount) / float64(m.jiraAPICallCount)
	}

	uptime := time.Since(m.startTime)

	return map[string]interface{}{
		"requests": map[string]interface{}{
			"total":           m.requestCount,
			"errors":          m.errorCount,
			"avgResponseTime": avgResponseTime.String(),
			"errorRate":       float64(m.errorCount) / float64(m.requestCount),
		},
		"jiraAPI": map[string]interface{}{
			"calls":     m.jiraAPICallCount,
			"errors":    m.jiraAPIErrorCount,
			"errorRate": jiraAPIErrorRate,
		},
		"cache": map[string]interface{}{
			"hits":     m.cacheHitCount,
			"misses":   m.cacheMissCount,
			"hitRatio": cacheHitRatio,
		},
		"system": map[string]interface{}{
			"uptime":    uptime.String(),
			"startTime": m.startTime.Format(time.RFC3339),
		},
	}
}

func (m *Metrics) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.requestCount = 0
	m.errorCount = 0
	m.totalResponseTime = 0
	m.jiraAPICallCount = 0
	m.jiraAPIErrorCount = 0
	m.cacheHitCount = 0
	m.cacheMissCount = 0
	m.startTime = time.Now()
}

// RequestTimer helps track request duration
type RequestTimer struct {
	start time.Time
}

func NewRequestTimer() *RequestTimer {
	return &RequestTimer{start: time.Now()}
}

func (rt *RequestTimer) Stop() {
	duration := time.Since(rt.start)
	GlobalMetrics.AddResponseTime(duration)
}

// Middleware for automatic request tracking
func RequestMetricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			timer := NewRequestTimer()
			GlobalMetrics.IncrementRequests()

			// Create a custom response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: 200}

			defer func() {
				timer.Stop()
				if rw.statusCode >= 400 {
					GlobalMetrics.IncrementErrors()
				}
			}()

			next.ServeHTTP(rw, r)
		})
	}
}

// Custom response writer to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Performance monitor that runs periodic checks
type PerformanceMonitor struct {
	interval time.Duration
	stopCh   chan struct{}
	alerts   []AlertRule
}

type AlertRule struct {
	Name      string
	Check     func(*Metrics) bool
	Message   string
	Triggered bool
}

func NewPerformanceMonitor(interval time.Duration) *PerformanceMonitor {
	return &PerformanceMonitor{
		interval: interval,
		stopCh:   make(chan struct{}),
		alerts: []AlertRule{
			{
				Name: "HighErrorRate",
				Check: func(m *Metrics) bool {
					stats := m.GetStats()
					requests := stats["requests"].(map[string]interface{})
					errorRate := requests["errorRate"].(float64)
					return errorRate > 0.05 // 5% error rate
				},
				Message: "Error rate exceeded 5%",
			},
			{
				Name: "SlowResponseTime",
				Check: func(m *Metrics) bool {
					stats := m.GetStats()
					requests := stats["requests"].(map[string]interface{})
					avgTime, _ := time.ParseDuration(requests["avgResponseTime"].(string))
					return avgTime > 2*time.Second
				},
				Message: "Average response time exceeded 2 seconds",
			},
			{
				Name: "HighJiraAPIErrorRate",
				Check: func(m *Metrics) bool {
					stats := m.GetStats()
					jiraAPI := stats["jiraAPI"].(map[string]interface{})
					errorRate := jiraAPI["errorRate"].(float64)
					return errorRate > 0.1 // 10% error rate
				},
				Message: "Jira API error rate exceeded 10%",
			},
		},
	}
}

func (pm *PerformanceMonitor) Start() {
	go func() {
		ticker := time.NewTicker(pm.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pm.checkAlerts()
			case <-pm.stopCh:
				return
			}
		}
	}()
}

func (pm *PerformanceMonitor) Stop() {
	close(pm.stopCh)
}

func (pm *PerformanceMonitor) checkAlerts() {
	for i := range pm.alerts {
		alert := &pm.alerts[i]
		triggered := alert.Check(GlobalMetrics)

		if triggered && !alert.Triggered {
			// Alert just triggered
			alert.Triggered = true
			pm.handleAlert(alert)
		} else if !triggered && alert.Triggered {
			// Alert resolved
			alert.Triggered = false
		}
	}
}

func (pm *PerformanceMonitor) handleAlert(alert *AlertRule) {
	// Log alert - in a real system you might send notifications
	stats := GlobalMetrics.GetStats()
	println(fmt.Sprintf("ALERT: %s - %s", alert.Name, alert.Message))
	println(fmt.Sprintf("Current metrics: %+v", stats))
}