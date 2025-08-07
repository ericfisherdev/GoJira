package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

// GetWorkflows retrieves all workflows
func GetWorkflows(w http.ResponseWriter, r *http.Request) {
	client, exists := r.Context().Value("jira_client").(*jira.Client)
	if !exists {
		log.Error().Msg("Jira client not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not available")))
		return
	}

	workflows, err := client.GetWorkflows()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get workflows")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, workflows)
}

// GetWorkflow retrieves a specific workflow by name
func GetWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowName := chi.URLParam(r, "name")
	if workflowName == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("workflow name is required")))
		return
	}

	client, exists := r.Context().Value("jira_client").(*jira.Client)
	if !exists {
		log.Error().Msg("Jira client not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not available")))
		return
	}

	workflow, err := client.GetWorkflow(workflowName)
	if err != nil {
		log.Error().Err(err).Str("workflowName", workflowName).Msg("Failed to get workflow")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, workflow)
}

// GetWorkflowSchemes retrieves all workflow schemes
func GetWorkflowSchemes(w http.ResponseWriter, r *http.Request) {
	client, exists := r.Context().Value("jira_client").(*jira.Client)
	if !exists {
		log.Error().Msg("Jira client not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not available")))
		return
	}

	schemes, err := client.GetWorkflowSchemes()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get workflow schemes")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"schemes": schemes,
		"total":   len(schemes),
	})
}

// GetProjectWorkflowScheme retrieves workflow scheme for a project
func GetProjectWorkflowScheme(w http.ResponseWriter, r *http.Request) {
	projectKey := chi.URLParam(r, "projectKey")
	if projectKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("project key is required")))
		return
	}

	client, exists := r.Context().Value("jira_client").(*jira.Client)
	if !exists {
		log.Error().Msg("Jira client not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not available")))
		return
	}

	scheme, err := client.GetProjectWorkflowScheme(projectKey)
	if err != nil {
		log.Error().Err(err).Str("projectKey", projectKey).Msg("Failed to get project workflow scheme")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, scheme)
}

// GetIssueWorkflow retrieves workflow information for a specific issue
func GetIssueWorkflow(w http.ResponseWriter, r *http.Request) {
	issueKey := chi.URLParam(r, "issueKey")
	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("issue key is required")))
		return
	}

	client, exists := r.Context().Value("jira_client").(*jira.Client)
	if !exists {
		log.Error().Msg("Jira client not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not available")))
		return
	}

	workflow, err := client.GetIssueWorkflow(issueKey)
	if err != nil {
		log.Error().Err(err).Str("issueKey", issueKey).Msg("Failed to get issue workflow")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, workflow)
}

// GetWorkflowStateMachine builds and returns a state machine for a workflow
func GetWorkflowStateMachine(w http.ResponseWriter, r *http.Request) {
	workflowName := chi.URLParam(r, "name")
	if workflowName == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("workflow name is required")))
		return
	}

	client, exists := r.Context().Value("jira_client").(*jira.Client)
	if !exists {
		log.Error().Msg("Jira client not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not available")))
		return
	}

	// Get the workflow first
	workflow, err := client.GetWorkflow(workflowName)
	if err != nil {
		log.Error().Err(err).Str("workflowName", workflowName).Msg("Failed to get workflow")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Build state machine
	stateMachine, err := client.BuildWorkflowStateMachine(workflow)
	if err != nil {
		log.Error().Err(err).Str("workflowName", workflowName).Msg("Failed to build state machine")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, stateMachine)
}

// ValidateTransition validates if a transition is allowed for an issue
func ValidateTransition(w http.ResponseWriter, r *http.Request) {
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

	client, exists := r.Context().Value("jira_client").(*jira.Client)
	if !exists {
		log.Error().Msg("Jira client not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not available")))
		return
	}

	result, err := client.ValidateTransition(issueKey, transitionID)
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

// ExecuteTransitionRequest represents a request to execute a workflow transition
type ExecuteTransitionRequest struct {
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Comment   string                 `json:"comment,omitempty"`
	UserKey   string                 `json:"userKey,omitempty"`
}

// ExecuteTransition executes a workflow transition with full context
func ExecuteTransition(w http.ResponseWriter, r *http.Request) {
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

	var req ExecuteTransitionRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	client, exists := r.Context().Value("jira_client").(*jira.Client)
	if !exists {
		log.Error().Msg("Jira client not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not available")))
		return
	}

	// Create execution context
	ctx := &jira.WorkflowExecutionContext{
		IssueKey:     issueKey,
		TransitionID: transitionID,
		Fields:       req.Fields,
		Comment:      req.Comment,
		ExecutedAt:   time.Now(),
		ExecutionID:  generateExecutionID(),
	}

	// Execute transition
	result, err := client.ExecuteTransition(ctx)
	if err != nil {
		log.Error().Err(err).
			Str("issueKey", issueKey).
			Str("transitionId", transitionID).
			Msg("Failed to execute transition")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Return appropriate status code based on result
	if result.Success {
		render.Status(r, http.StatusOK)
	} else {
		render.Status(r, http.StatusBadRequest)
	}

	render.JSON(w, r, result)
}

// GetAvailableTransitions gets available transitions for an issue
func GetAvailableTransitions(w http.ResponseWriter, r *http.Request) {
	issueKey := chi.URLParam(r, "issueKey")
	if issueKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("issue key is required")))
		return
	}

	client, exists := r.Context().Value("jira_client").(*jira.Client)
	if !exists {
		log.Error().Msg("Jira client not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not available")))
		return
	}

	transitions, err := client.GetTransitions(issueKey)
	if err != nil {
		log.Error().Err(err).Str("issueKey", issueKey).Msg("Failed to get transitions")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"issueKey":    issueKey,
		"transitions": transitions,
		"total":       len(transitions),
		"retrievedAt": time.Now(),
	})
}

// GetWorkflowAnalytics provides analytics about workflow usage
func GetWorkflowAnalytics(w http.ResponseWriter, r *http.Request) {
	workflowName := chi.URLParam(r, "name")
	
	// Optional query parameters for analytics filters
	projectKey := r.URL.Query().Get("project")
	days := r.URL.Query().Get("days")
	
	daysInt := 30 // Default to 30 days
	if days != "" {
		if parsed, err := strconv.Atoi(days); err == nil && parsed > 0 {
			daysInt = parsed
		}
	}

	client, exists := r.Context().Value("jira_client").(*jira.Client)
	if !exists {
		log.Error().Msg("Jira client not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("jira client not available")))
		return
	}

	// Get workflow information
	var workflow *jira.Workflow
	var err error
	
	if workflowName != "" {
		workflow, err = client.GetWorkflow(workflowName)
		if err != nil {
			log.Error().Err(err).Str("workflowName", workflowName).Msg("Failed to get workflow")
			render.Render(w, r, ErrInternalServer(err))
			return
		}
	}

	// Build analytics response
	analytics := map[string]interface{}{
		"workflowName": workflowName,
		"projectKey":   projectKey,
		"periodDays":   daysInt,
		"generatedAt":  time.Now(),
	}

	if workflow != nil {
		analytics["workflow"] = workflow
		analytics["totalStatuses"] = len(workflow.Statuses)
		analytics["totalTransitions"] = len(workflow.Transitions)
		
		// Build state machine for analysis
		stateMachine, err := client.BuildWorkflowStateMachine(workflow)
		if err == nil {
			analytics["stateMachine"] = map[string]interface{}{
				"initialState": stateMachine.InitialState,
				"finalStates":  stateMachine.FinalStates,
				"stateCount":   len(stateMachine.States),
			}
		}
	}

	render.JSON(w, r, analytics)
}

// generateExecutionID generates a unique execution ID for workflow transitions
func generateExecutionID() string {
	return fmt.Sprintf("exec_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
}