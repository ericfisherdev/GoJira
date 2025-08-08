package handlers

import (
	"context"
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
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	workflows, err := jiraClient.GetWorkflows()
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

	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	workflow, err := jiraClient.GetWorkflow(workflowName)
	if err != nil {
		log.Error().Err(err).Str("workflowName", workflowName).Msg("Failed to get workflow")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, workflow)
}

// GetWorkflowSchemes retrieves all workflow schemes
func GetWorkflowSchemes(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	schemes, err := jiraClient.GetWorkflowSchemes()
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

	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	scheme, err := jiraClient.GetProjectWorkflowScheme(projectKey)
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

	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	workflow, err := jiraClient.GetIssueWorkflow(issueKey)
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

	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	// Get the workflow first
	workflow, err := jiraClient.GetWorkflow(workflowName)
	if err != nil {
		log.Error().Err(err).Str("workflowName", workflowName).Msg("Failed to get workflow")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Build state machine
	stateMachine, err := jiraClient.BuildWorkflowStateMachine(workflow)
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

	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	result, err := jiraClient.ValidateTransition(issueKey, transitionID)
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

	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
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
	result, err := jiraClient.ExecuteTransition(ctx)
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

	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	transitions, err := jiraClient.GetTransitions(issueKey)
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

	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	// Get workflow information
	var workflow *jira.Workflow
	var err error
	
	if workflowName != "" {
		workflow, err = jiraClient.GetWorkflow(workflowName)
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
		stateMachine, err := jiraClient.BuildWorkflowStateMachine(workflow)
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

// ProjectTransitionsRequest represents request for project transitions lookup
type ProjectTransitionsRequest struct {
	ProjectKey string `json:"projectKey,omitempty"`
	BoardID    *int   `json:"boardId,omitempty"`
	IssueType  string `json:"issueType,omitempty"`
}

func (ptr *ProjectTransitionsRequest) Bind(r *http.Request) error {
	// Either projectKey or boardId is required
	if ptr.ProjectKey == "" && ptr.BoardID == nil {
		return fmt.Errorf("either projectKey or boardId is required")
	}
	return nil
}

// TransitionInfo represents simplified transition information
type TransitionInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	FromStatus  string `json:"fromStatus,omitempty"`
	ToStatus    string `json:"toStatus"`
	ToStatusID  string `json:"toStatusId"`
}

// ProjectTransitionsResponse represents the response containing transitions for a project/board
type ProjectTransitionsResponse struct {
	Success      bool             `json:"success"`
	ProjectKey   string           `json:"projectKey,omitempty"`
	BoardID      *int             `json:"boardId,omitempty"`
	IssueType    string           `json:"issueType,omitempty"`
	WorkflowName string           `json:"workflowName,omitempty"`
	Transitions  []TransitionInfo `json:"transitions"`
	Total        int              `json:"total"`
	RetrievedAt  string           `json:"retrievedAt"`
}

func (ptr *ProjectTransitionsResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// GetProjectTransitions returns all available transitions for a project or board
func GetProjectTransitions(w http.ResponseWriter, r *http.Request) {
	if jiraClient == nil || !authManager.IsAuthenticated() {
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("not connected to Jira")))
		return
	}

	// Get project key from URL param or request body
	projectKey := chi.URLParam(r, "projectKey")
	
	var req ProjectTransitionsRequest
	if r.Method == "POST" {
		if err := render.Bind(r, &req); err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}
		if projectKey == "" {
			projectKey = req.ProjectKey
		}
	}

	// Handle board ID from query param if provided
	var boardID *int
	if boardIDStr := r.URL.Query().Get("boardId"); boardIDStr != "" {
		if parsed, err := strconv.Atoi(boardIDStr); err == nil {
			boardID = &parsed
		}
	}
	if req.BoardID != nil {
		boardID = req.BoardID
	}

	// Get issue type filter
	issueType := r.URL.Query().Get("issueType")
	if req.IssueType != "" {
		issueType = req.IssueType
	}

	// If boardID is provided, get the project key from the board
	if boardID != nil && projectKey == "" {
		board, err := jiraClient.GetBoard(*boardID)
		if err != nil {
			render.Render(w, r, ErrInternalServer(fmt.Errorf("failed to get board: %w", err)))
			return
		}
		if board.Location.ProjectKey != "" {
			projectKey = board.Location.ProjectKey
		}
	}

	if projectKey == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("project key is required")))
		return
	}

	// SIMPLIFIED APPROACH: Search for issues in the project and aggregate their transitions
	// This is more reliable than workflow scheme APIs which may not be available in Jira Cloud
	
	// Search for recent issues in the project (limit to get variety of statuses)
	searchJQL := fmt.Sprintf("project = %s ORDER BY updated DESC", projectKey)
	
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	
	searchResults, err := jiraClient.SearchIssues(ctx, searchJQL, 0, 10, nil) // Get 10 recent issues
	if err != nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("failed to search project issues: %w", err)))
		return
	}

	// Collect unique transitions from all issues
	transitionMap := make(map[string]TransitionInfo)
	var sampleIssueKey string

	for _, issue := range searchResults.Issues {
		// Filter by issue type if specified
		if issueType != "" && issue.Fields.IssueType.Name != issueType {
			continue
		}

		sampleIssueKey = issue.Key
		
		// Get transitions for this issue
		transitions, err := jiraClient.GetTransitions(issue.Key)
		if err != nil {
			log.Error().Err(err).Str("issueKey", issue.Key).Msg("Failed to get transitions for issue")
			continue // Skip this issue, try others
		}

		// Add transitions to our map (deduplicating)
		for _, transition := range transitions {
			transitionMap[transition.ID] = TransitionInfo{
				ID:         transition.ID,
				Name:       transition.Name,
				ToStatus:   "Unknown", // Basic transition info doesn't include destination status
				ToStatusID: "",        // Not available in basic transition response
			}
		}
	}

	// Convert map to slice
	var transitions []TransitionInfo
	for _, transition := range transitionMap {
		transitions = append(transitions, transition)
	}

	response := &ProjectTransitionsResponse{
		Success:     true,
		ProjectKey:  projectKey,
		BoardID:     boardID,
		IssueType:   issueType,
		Transitions: transitions,
		Total:       len(transitions),
		RetrievedAt: time.Now().Format(time.RFC3339),
	}

	// Add helpful note if we found transitions
	if len(transitions) > 0 && sampleIssueKey != "" {
		// Add metadata to help users understand the data source
		response.WorkflowName = fmt.Sprintf("Aggregated from project issues (sample: %s)", sampleIssueKey)
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}