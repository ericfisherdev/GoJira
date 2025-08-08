package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/ericfisherdev/GoJira/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

// Global workflow service instance
var workflowService *services.WorkflowService

// InitWorkflowService initializes the workflow service
func InitWorkflowService(client jira.ClientInterface) {
	workflowService = services.NewWorkflowService(client)
}

// GetWorkflowService returns the workflow service instance
func GetWorkflowService() *services.WorkflowService {
	if workflowService == nil {
		log.Warn().Msg("Workflow service not initialized, creating with nil client")
		workflowService = services.NewWorkflowService(nil)
	}
	return workflowService
}

// ExecuteWorkflowTransition executes a workflow transition using the service
func ExecuteWorkflowTransition(w http.ResponseWriter, r *http.Request) {
	issueKey := chi.URLParam(r, "issueKey")
	transitionID := chi.URLParam(r, "transitionId")

	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("issue key is required")))
		return
	}
	if transitionID == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("transition ID is required")))
		return
	}

	var requestBody struct {
		Fields       map[string]interface{} `json:"fields"`
		Comment      string                 `json:"comment"`
		ValidateOnly bool                   `json:"validateOnly"`
		Reason       string                 `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		// Allow empty body
		log.Debug().Err(err).Msg("No request body provided for transition")
	}

	service := GetWorkflowService()
	if service == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("workflow service not available")))
		return
	}

	req := &services.TransitionRequest{
		IssueKey:     issueKey,
		TransitionID: transitionID,
		Fields:       requestBody.Fields,
		Comment:      requestBody.Comment,
		ValidateOnly: requestBody.ValidateOnly,
		Reason:       requestBody.Reason,
	}

	result, err := service.ExecuteTransition(r.Context(), req)
	if err != nil {
		log.Error().Err(err).
			Str("issueKey", issueKey).
			Str("transitionId", transitionID).
			Msg("Failed to execute transition")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	if result.Success {
		render.Status(r, http.StatusOK)
	} else {
		render.Status(r, http.StatusBadRequest)
	}

	render.JSON(w, r, result)
}

// ValidateWorkflowTransition validates a workflow transition
func ValidateWorkflowTransition(w http.ResponseWriter, r *http.Request) {
	issueKey := chi.URLParam(r, "issueKey")
	transitionID := chi.URLParam(r, "transitionId")

	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("issue key is required")))
		return
	}
	if transitionID == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("transition ID is required")))
		return
	}

	service := GetWorkflowService()
	if service == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("workflow service not available")))
		return
	}

	result, err := service.ValidateTransition(r.Context(), issueKey, transitionID)
	if err != nil {
		log.Error().Err(err).
			Str("issueKey", issueKey).
			Str("transitionId", transitionID).
			Msg("Failed to validate transition")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, result)
}

// GetWorkflowTransitionMetrics returns workflow transition metrics
func GetWorkflowTransitionMetrics(w http.ResponseWriter, r *http.Request) {
	service := GetWorkflowService()
	if service == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("workflow service not available")))
		return
	}

	metrics := service.GetTransitionMetrics()
	
	// Convert to JSON-friendly format
	response := map[string]interface{}{
		"totalTransitions": metrics.TotalTransitions,
		"successCount":     metrics.SuccessCount,
		"failureCount":     metrics.FailureCount,
		"successRate":      float64(metrics.SuccessCount) / float64(metrics.TotalTransitions) * 100,
		"avgDuration":      metrics.AvgDuration.String(),
		"topTransitions":   metrics.TransitionCounts,
	}

	render.JSON(w, r, response)
}

// GetWorkflowAnalyticsAdvanced provides advanced workflow analytics
func GetWorkflowAnalyticsAdvanced(w http.ResponseWriter, r *http.Request) {
	workflowName := chi.URLParam(r, "name")
	if workflowName == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("workflow name is required")))
		return
	}

	// Get query parameters
	daysStr := r.URL.Query().Get("days")
	days := 30 // default
	if daysStr != "" {
		if parsed, err := strconv.Atoi(daysStr); err == nil && parsed > 0 {
			days = parsed
		}
	}

	service := GetWorkflowService()
	if service == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("workflow service not available")))
		return
	}

	analytics, err := service.GetWorkflowAnalytics(r.Context(), workflowName, days)
	if err != nil {
		log.Error().Err(err).
			Str("workflowName", workflowName).
			Msg("Failed to generate workflow analytics")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, analytics)
}

// GetCachedWorkflow retrieves a workflow using cache
func GetCachedWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowName := chi.URLParam(r, "name")
	if workflowName == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("workflow name is required")))
		return
	}

	service := GetWorkflowService()
	if service == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("workflow service not available")))
		return
	}

	workflow, err := service.GetWorkflowWithCache(r.Context(), workflowName)
	if err != nil {
		log.Error().Err(err).
			Str("workflowName", workflowName).
			Msg("Failed to get workflow")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, workflow)
}

// GetWorkflowStateMachineAdvanced retrieves the state machine for a workflow
func GetWorkflowStateMachineAdvanced(w http.ResponseWriter, r *http.Request) {
	workflowName := chi.URLParam(r, "name")
	if workflowName == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("workflow name is required")))
		return
	}

	service := GetWorkflowService()
	if service == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("workflow service not available")))
		return
	}

	stateMachine, err := service.GetStateMachine(r.Context(), workflowName)
	if err != nil {
		log.Error().Err(err).
			Str("workflowName", workflowName).
			Msg("Failed to get state machine")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Add visualization hints
	response := map[string]interface{}{
		"stateMachine": stateMachine,
		"visualization": map[string]interface{}{
			"initialState": stateMachine.InitialState,
			"finalStates":  stateMachine.FinalStates,
			"stateCount":   len(stateMachine.States),
			"transitionCount": len(stateMachine.Transitions),
		},
	}

	render.JSON(w, r, response)
}

// BatchValidateTransitions validates multiple transitions at once
func BatchValidateTransitions(w http.ResponseWriter, r *http.Request) {
	var request struct {
		IssueKey    string   `json:"issueKey"`
		Transitions []string `json:"transitions"`
	}

	if err := render.DecodeJSON(r.Body, &request); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	if request.IssueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("issue key is required")))
		return
	}

	if len(request.Transitions) == 0 {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("at least one transition is required")))
		return
	}

	service := GetWorkflowService()
	if service == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("workflow service not available")))
		return
	}

	results := make(map[string]*jira.WorkflowExecutionResult)
	
	for _, transitionID := range request.Transitions {
		result, err := service.ValidateTransition(r.Context(), request.IssueKey, transitionID)
		if err != nil {
			log.Error().Err(err).
				Str("issueKey", request.IssueKey).
				Str("transitionId", transitionID).
				Msg("Failed to validate transition")
			results[transitionID] = &jira.WorkflowExecutionResult{
				Success:      false,
				IssueKey:     request.IssueKey,
				TransitionID: transitionID,
				Errors: []jira.WorkflowError{
					{
						Type:    "VALIDATION_ERROR",
						Message: err.Error(),
					},
				},
			}
		} else {
			results[transitionID] = result
		}
	}

	render.JSON(w, r, results)
}

// SimulateWorkflowTransition simulates a workflow transition without executing
func SimulateWorkflowTransition(w http.ResponseWriter, r *http.Request) {
	issueKey := chi.URLParam(r, "issueKey")
	transitionID := chi.URLParam(r, "transitionId")

	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("issue key is required")))
		return
	}
	if transitionID == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("transition ID is required")))
		return
	}

	var requestBody struct {
		Fields  map[string]interface{} `json:"fields"`
		Comment string                 `json:"comment"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		// Allow empty body
		log.Debug().Err(err).Msg("No request body provided for simulation")
	}

	service := GetWorkflowService()
	if service == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("workflow service not available")))
		return
	}

	// Execute with validateOnly flag
	req := &services.TransitionRequest{
		IssueKey:     issueKey,
		TransitionID: transitionID,
		Fields:       requestBody.Fields,
		Comment:      requestBody.Comment,
		ValidateOnly: true,
	}

	result, err := service.ExecuteTransition(r.Context(), req)
	if err != nil {
		log.Error().Err(err).
			Str("issueKey", issueKey).
			Str("transitionId", transitionID).
			Msg("Failed to simulate transition")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Add simulation metadata
	response := map[string]interface{}{
		"simulation": true,
		"result":     result,
		"message":    "This was a simulation. No changes were made.",
	}

	render.JSON(w, r, response)
}