package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
)

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version,omitempty"`
	Uptime    string    `json:"uptime,omitempty"`
}

func (hr *HealthResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

var startTime = time.Now()

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(startTime).String()

	response := &HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Uptime:    uptime,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

func ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	// TODO: Add checks for dependencies (database, external services)
	// For now, just return OK
	response := &HealthResponse{
		Status:    "ready",
		Timestamp: time.Now(),
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}