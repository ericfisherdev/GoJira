package handlers

import (
	"net/http"

	"github.com/ericfisherdev/GoJira/internal/monitoring"
	"github.com/go-chi/render"
)

// GetMetrics returns current performance metrics
func GetMetrics(w http.ResponseWriter, r *http.Request) {
	stats := monitoring.GlobalMetrics.GetStats()
	
	response := &IssueResponse{
		Success: true,
		Data:    stats,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// ResetMetrics resets all metrics counters (useful for testing)
func ResetMetrics(w http.ResponseWriter, r *http.Request) {
	monitoring.GlobalMetrics.Reset()
	
	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"message": "Metrics reset successfully",
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetHealth returns enhanced health check with metrics
func GetHealthWithMetrics(w http.ResponseWriter, r *http.Request) {
	stats := monitoring.GlobalMetrics.GetStats()
	
	// Determine health status based on metrics
	healthy := true
	issues := []string{}

	// Check error rates
	if requests, ok := stats["requests"].(map[string]interface{}); ok {
		if errorRate, ok := requests["errorRate"].(float64); ok && errorRate > 0.1 {
			healthy = false
			issues = append(issues, "High error rate detected")
		}
	}

	// Check Jira API health
	if jiraAPI, ok := stats["jiraAPI"].(map[string]interface{}); ok {
		if errorRate, ok := jiraAPI["errorRate"].(float64); ok && errorRate > 0.2 {
			healthy = false
			issues = append(issues, "High Jira API error rate")
		}
	}

	status := "healthy"
	if !healthy {
		status = "degraded"
	}

	response := &IssueResponse{
		Success: true,
		Data: map[string]interface{}{
			"status":  status,
			"healthy": healthy,
			"issues":  issues,
			"metrics": stats,
		},
	}

	if healthy {
		render.Status(r, http.StatusOK)
	} else {
		render.Status(r, http.StatusServiceUnavailable)
	}
	
	render.Render(w, r, response)
}