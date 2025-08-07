package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/ericfisherdev/GoJira/internal/queue"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Helper functions for response handling
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
	}
}

func RespondWithError(w http.ResponseWriter, code int, message string) {
	RespondWithJSON(w, code, map[string]string{"error": message})
}

type QueueHandler struct {
	jobQueue      *queue.JobQueue
	priorityQueue *queue.PriorityQueue
	rateLimiter   *queue.RateLimiter
}

func NewQueueHandler() *QueueHandler {
	config := queue.QueueConfig{
		MaxWorkers:   10,
		MaxQueueSize: 100,
		MaxRetries:   3,
		RetryDelay:   1 * time.Second,
		RateLimit:    20,
	}

	jobQueue := queue.NewJobQueue(config)
	jobQueue.Start()

	return &QueueHandler{
		jobQueue:      jobQueue,
		priorityQueue: queue.NewPriorityQueue(),
		rateLimiter:   queue.NewRateLimiter(20, 40),
	}
}

type JobRequest struct {
	Type     string      `json:"type"`
	Priority int         `json:"priority"`
	Payload  interface{} `json:"payload"`
}

type JobResponse struct {
	JobID    string    `json:"jobId"`
	Status   string    `json:"status"`
	Created  time.Time `json:"created"`
	Priority int       `json:"priority"`
}

type QueueStatusResponse struct {
	QueueSize        int                     `json:"queueSize"`
	ResultsSize      int                     `json:"resultsSize"`
	Workers          int                     `json:"workers"`
	Metrics          queue.QueueMetrics      `json:"metrics"`
	RateLimiterStats queue.RateLimiterStats  `json:"rateLimiterStats"`
}

func (h *QueueHandler) SubmitJob(w http.ResponseWriter, r *http.Request) {
	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate job type
	jobType := queue.JobType(req.Type)
	switch jobType {
	case queue.JobTypeCreateIssue,
		queue.JobTypeUpdateIssue,
		queue.JobTypeTransition,
		queue.JobTypeBulkUpdate,
		queue.JobTypeSprintMove,
		queue.JobTypeWorkflowChange:
		// Valid job type
	default:
		RespondWithError(w, http.StatusBadRequest, "Invalid job type")
		return
	}

	// Create job
	job := queue.Job{
		Type:     jobType,
		Priority: req.Priority,
		Payload:  req.Payload,
		Created:  time.Now(),
	}

	// Submit to queue
	if err := h.jobQueue.Submit(job); err != nil {
		log.Error().Err(err).Msg("Failed to submit job")
		RespondWithError(w, http.StatusServiceUnavailable, "Queue is full")
		return
	}

	// Return response
	response := JobResponse{
		JobID:    job.ID,
		Status:   "queued",
		Created:  job.Created,
		Priority: job.Priority,
	}

	RespondWithJSON(w, http.StatusAccepted, response)
}

func (h *QueueHandler) GetJobResult(w http.ResponseWriter, r *http.Request) {
	// Get timeout parameter
	timeoutStr := r.URL.Query().Get("timeout")
	timeout := 5 * time.Second
	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(t) * time.Second
		}
	}

	// Get result
	result, err := h.jobQueue.GetResult(timeout)
	if err != nil {
		RespondWithError(w, http.StatusRequestTimeout, "No result available")
		return
	}

	RespondWithJSON(w, http.StatusOK, result)
}

func (h *QueueHandler) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	status := QueueStatusResponse{
		QueueSize:        h.jobQueue.QueueSize(),
		ResultsSize:      h.jobQueue.ResultsSize(),
		Workers:          10, // From config
		Metrics:          h.jobQueue.GetMetrics(),
		RateLimiterStats: h.rateLimiter.GetStats(),
	}

	RespondWithJSON(w, http.StatusOK, status)
}

func (h *QueueHandler) GetQueueMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.jobQueue.GetMetrics()
	RespondWithJSON(w, http.StatusOK, metrics)
}

func (h *QueueHandler) ClearQueue(w http.ResponseWriter, r *http.Request) {
	// Clear priority queue
	h.priorityQueue.Clear()

	response := map[string]string{
		"status": "Queue cleared",
	}

	RespondWithJSON(w, http.StatusOK, response)
}

func (h *QueueHandler) GetRateLimiterStats(w http.ResponseWriter, r *http.Request) {
	stats := h.rateLimiter.GetStats()
	
	response := map[string]interface{}{
		"stats":            stats,
		"availableTokens": h.rateLimiter.AvailableTokens(),
	}

	RespondWithJSON(w, http.StatusOK, response)
}

func (h *QueueHandler) ResetRateLimiter(w http.ResponseWriter, r *http.Request) {
	h.rateLimiter.ResetStats()

	response := map[string]string{
		"status": "Rate limiter stats reset",
	}

	RespondWithJSON(w, http.StatusOK, response)
}

type BatchJobRequest struct {
	Jobs []JobRequest `json:"jobs"`
}

type BatchJobResponse struct {
	Submitted   int      `json:"submitted"`
	Failed      int      `json:"failed"`
	JobIDs      []string `json:"jobIds"`
	FailedJobs  []string `json:"failedJobs,omitempty"`
}

func (h *QueueHandler) SubmitBatchJobs(w http.ResponseWriter, r *http.Request) {
	var req BatchJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Jobs) == 0 {
		RespondWithError(w, http.StatusBadRequest, "No jobs provided")
		return
	}

	if len(req.Jobs) > 100 {
		RespondWithError(w, http.StatusBadRequest, "Too many jobs (max 100)")
		return
	}

	response := BatchJobResponse{
		JobIDs:     make([]string, 0),
		FailedJobs: make([]string, 0),
	}

	for i, jobReq := range req.Jobs {
		// Validate job type
		jobType := queue.JobType(jobReq.Type)
		
		// Create job
		job := queue.Job{
			Type:     jobType,
			Priority: jobReq.Priority,
			Payload:  jobReq.Payload,
			Created:  time.Now(),
		}

		// Submit to queue
		if err := h.jobQueue.Submit(job); err != nil {
			response.Failed++
			response.FailedJobs = append(response.FailedJobs, strconv.Itoa(i))
			log.Error().Err(err).Int("index", i).Msg("Failed to submit batch job")
		} else {
			response.Submitted++
			response.JobIDs = append(response.JobIDs, job.ID)
		}
	}

	status := http.StatusAccepted
	if response.Failed > 0 && response.Submitted == 0 {
		status = http.StatusServiceUnavailable
	}

	RespondWithJSON(w, status, response)
}

func (h *QueueHandler) GetPriorityQueueStatus(w http.ResponseWriter, r *http.Request) {
	jobs := h.priorityQueue.GetAll()
	
	response := map[string]interface{}{
		"queueSize": h.priorityQueue.Len(),
		"isEmpty":   h.priorityQueue.IsEmpty(),
		"jobs":      jobs,
	}

	RespondWithJSON(w, http.StatusOK, response)
}

func (h *QueueHandler) RemoveJobFromQueue(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		RespondWithError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	removed := h.priorityQueue.Remove(jobID)
	
	response := map[string]interface{}{
		"jobId":   jobID,
		"removed": removed,
	}

	status := http.StatusOK
	if !removed {
		status = http.StatusNotFound
	}

	RespondWithJSON(w, status, response)
}

func (h *QueueHandler) Shutdown() {
	log.Info().Msg("Shutting down queue handler")
	h.jobQueue.Stop()
	h.rateLimiter.Stop()
}