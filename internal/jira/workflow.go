package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Workflow represents a Jira workflow configuration
type Workflow struct {
	ID          string             `json:"id,omitempty"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	IsDefault   bool               `json:"isDefault"`
	Statuses    []WorkflowStatus   `json:"statuses"`
	Transitions []WorkflowTransition `json:"transitions"`
	Draft       bool               `json:"draft"`
	ProjectKey  string             `json:"projectKey,omitempty"`
	IssueTypes  []string           `json:"issueTypes,omitempty"`
	CreatedDate *time.Time         `json:"created,omitempty"`
	UpdatedDate *time.Time         `json:"updated,omitempty"`
}

// WorkflowList represents a list of workflows
type WorkflowList struct {
	Workflows []Workflow `json:"values"`
	Size      int        `json:"size"`
	Start     int        `json:"start"`
	Total     int        `json:"total"`
	IsLast    bool       `json:"isLast"`
}

// WorkflowStatus represents a status within a workflow
type WorkflowStatus struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Description    string                    `json:"description,omitempty"`
	StatusCategory WorkflowStatusCategory    `json:"statusCategory"`
	Properties     []WorkflowStatusProperty  `json:"properties,omitempty"`
	Scope          *WorkflowScope           `json:"scope,omitempty"`
}

// WorkflowStatusCategory represents status category in workflow context
type WorkflowStatusCategory struct {
	ID    int    `json:"id"`
	Key   string `json:"key"`   // new, indeterminate, done
	Name  string `json:"name"`
	Color string `json:"colorName,omitempty"`
}

// WorkflowStatusProperty represents custom properties for workflow statuses
type WorkflowStatusProperty struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// WorkflowScope defines where the workflow status is used
type WorkflowScope struct {
	Type    string                `json:"type"`    // PROJECT or GLOBAL
	Project *WorkflowScopeProject `json:"project,omitempty"`
}

// WorkflowScopeProject represents project scope for workflow status
type WorkflowScopeProject struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// WorkflowTransition represents a transition within a workflow
type WorkflowTransition struct {
	ID          string                       `json:"id"`
	Name        string                       `json:"name"`
	Description string                       `json:"description,omitempty"`
	From        []WorkflowTransitionEndpoint `json:"from"`
	To          WorkflowTransitionEndpoint   `json:"to"`
	Type        string                       `json:"type"` // global, initial, common, directed
	Screen      *WorkflowScreen             `json:"screen,omitempty"`
	Rules       *WorkflowTransitionRules    `json:"rules,omitempty"`
	Properties  []WorkflowTransitionProperty `json:"properties,omitempty"`
}

// WorkflowTransitionEndpoint represents the from/to endpoints of a transition
type WorkflowTransitionEndpoint struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// WorkflowScreen represents screen configuration for transitions
type WorkflowScreen struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// WorkflowTransitionRules represents rules applied to transitions
type WorkflowTransitionRules struct {
	Conditions   []WorkflowCondition   `json:"conditions,omitempty"`
	Validators   []WorkflowValidator   `json:"validators,omitempty"`
	PostFunctions []WorkflowPostFunction `json:"postFunctions,omitempty"`
}

// WorkflowCondition represents a condition that must be met for a transition
type WorkflowCondition struct {
	Type        string                 `json:"type"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

// WorkflowValidator represents a validator for a transition
type WorkflowValidator struct {
	Type        string                 `json:"type"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

// WorkflowPostFunction represents a post-function executed during transition
type WorkflowPostFunction struct {
	Type        string                 `json:"type"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

// WorkflowTransitionProperty represents custom properties for transitions
type WorkflowTransitionProperty struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// WorkflowScheme represents a workflow scheme
type WorkflowScheme struct {
	ID               string                      `json:"id,omitempty"`
	Name             string                      `json:"name"`
	Description      string                      `json:"description,omitempty"`
	DefaultWorkflow  string                      `json:"defaultWorkflow"`
	IssueTypeMappings []WorkflowSchemeMapping    `json:"issueTypeMappings,omitempty"`
	Draft            bool                        `json:"draft"`
	Projects         []WorkflowSchemeProject     `json:"projects,omitempty"`
}

// WorkflowSchemeMapping represents mapping between issue type and workflow
type WorkflowSchemeMapping struct {
	IssueType string `json:"issueType"`
	Workflow  string `json:"workflow"`
}

// WorkflowSchemeProject represents projects using the workflow scheme
type WorkflowSchemeProject struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// WorkflowStateMachine represents the state machine for a workflow
type WorkflowStateMachine struct {
	WorkflowID   string                           `json:"workflowId"`
	WorkflowName string                           `json:"workflowName"`
	States       map[string]*WorkflowState        `json:"states"`
	InitialState string                           `json:"initialState"`
	FinalStates  []string                         `json:"finalStates"`
	Transitions  map[string]*WorkflowTransitionSM `json:"transitions"`
}

// WorkflowState represents a state in the workflow state machine
type WorkflowState struct {
	ID             string                        `json:"id"`
	Name           string                        `json:"name"`
	Category       string                        `json:"category"` // new, indeterminate, done
	Type           string                        `json:"type"`     // initial, intermediate, final
	Transitions    []string                      `json:"transitions"`
	EntryActions   []WorkflowAction              `json:"entryActions,omitempty"`
	ExitActions    []WorkflowAction              `json:"exitActions,omitempty"`
	Properties     map[string]interface{}        `json:"properties,omitempty"`
}

// WorkflowTransitionSM represents a transition in the state machine
type WorkflowTransitionSM struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	FromStates   []string               `json:"fromStates"`
	ToState      string                 `json:"toState"`
	Conditions   []WorkflowCondition    `json:"conditions,omitempty"`
	Guards       []WorkflowGuard        `json:"guards,omitempty"`
	Actions      []WorkflowAction       `json:"actions,omitempty"`
	Priority     int                    `json:"priority"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
}

// WorkflowAction represents an action executed during workflow transitions
type WorkflowAction struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Order       int                    `json:"order"`
}

// WorkflowGuard represents a guard condition for workflow transitions
type WorkflowGuard struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Condition   string                 `json:"condition,omitempty"`
}

// WorkflowExecutionContext represents context for workflow execution
type WorkflowExecutionContext struct {
	IssueKey       string                 `json:"issueKey"`
	TransitionID   string                 `json:"transitionId"`
	FromStatusID   string                 `json:"fromStatusId"`
	ToStatusID     string                 `json:"toStatusId"`
	User           *User                  `json:"user,omitempty"`
	Fields         map[string]interface{} `json:"fields,omitempty"`
	Comment        string                 `json:"comment,omitempty"`
	ExecutedAt     time.Time              `json:"executedAt"`
	ExecutionID    string                 `json:"executionId"`
}

// WorkflowExecutionResult represents the result of workflow execution
type WorkflowExecutionResult struct {
	Success        bool                   `json:"success"`
	ExecutionID    string                 `json:"executionId"`
	IssueKey       string                 `json:"issueKey"`
	TransitionID   string                 `json:"transitionId"`
	FromStatusID   string                 `json:"fromStatusId"`
	ToStatusID     string                 `json:"toStatusId"`
	ExecutedAt     time.Time              `json:"executedAt"`
	Duration       time.Duration          `json:"duration"`
	ActionsExecuted []WorkflowActionResult `json:"actionsExecuted,omitempty"`
	Errors         []WorkflowError        `json:"errors,omitempty"`
	Warnings       []WorkflowWarning      `json:"warnings,omitempty"`
}

// WorkflowActionResult represents the result of executing a workflow action
type WorkflowActionResult struct {
	Action     WorkflowAction `json:"action"`
	Success    bool           `json:"success"`
	Message    string         `json:"message,omitempty"`
	Duration   time.Duration  `json:"duration"`
	ExecutedAt time.Time      `json:"executedAt"`
}

// WorkflowError represents an error during workflow execution
type WorkflowError struct {
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	Field       string    `json:"field,omitempty"`
	Code        string    `json:"code,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// WorkflowWarning represents a warning during workflow execution
type WorkflowWarning struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Field     string    `json:"field,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Client methods for Workflow operations

// GetWorkflows retrieves all workflows
func (c *Client) GetWorkflows() (*WorkflowList, error) {
	endpoint := "/rest/api/2/workflow"
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflows: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get workflows, status: %d", resp.StatusCode())
	}

	var result WorkflowList
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode workflows: %w", err)
	}

	return &result, nil
}

// GetWorkflow retrieves a specific workflow by name
func (c *Client) GetWorkflow(workflowName string) (*Workflow, error) {
	endpoint := fmt.Sprintf("/rest/api/2/workflow/%s", workflowName)
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("workflow '%s' not found", workflowName)
	}
	
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get workflow, status: %d", resp.StatusCode())
	}

	var workflow Workflow
	if err := json.Unmarshal(resp.Body(), &workflow); err != nil {
		return nil, fmt.Errorf("failed to decode workflow: %w", err)
	}

	return &workflow, nil
}

// GetWorkflowSchemes retrieves all workflow schemes
func (c *Client) GetWorkflowSchemes() ([]WorkflowScheme, error) {
	endpoint := "/rest/api/2/workflowscheme"
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow schemes: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get workflow schemes, status: %d", resp.StatusCode())
	}

	var result struct {
		Values []WorkflowScheme `json:"values"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode workflow schemes: %w", err)
	}

	return result.Values, nil
}

// GetProjectWorkflowScheme retrieves workflow scheme for a project
func (c *Client) GetProjectWorkflowScheme(projectKey string) (*WorkflowScheme, error) {
	endpoint := fmt.Sprintf("/rest/api/2/project/%s/workflowscheme", projectKey)
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get project workflow scheme: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("workflow scheme for project '%s' not found", projectKey)
	}
	
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get project workflow scheme, status: %d", resp.StatusCode())
	}

	var scheme WorkflowScheme
	if err := json.Unmarshal(resp.Body(), &scheme); err != nil {
		return nil, fmt.Errorf("failed to decode workflow scheme: %w", err)
	}

	return &scheme, nil
}

// GetIssueWorkflow retrieves workflow information for a specific issue
func (c *Client) GetIssueWorkflow(issueKey string) (*Workflow, error) {
	// First get the issue to determine project and issue type
	issue, err := c.GetIssue(context.Background(), issueKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	// Get project workflow scheme
	scheme, err := c.GetProjectWorkflowScheme(issue.Fields.Project.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow scheme: %w", err)
	}

	// Find appropriate workflow for issue type
	workflowName := scheme.DefaultWorkflow
	for _, mapping := range scheme.IssueTypeMappings {
		if mapping.IssueType == issue.Fields.IssueType.ID {
			workflowName = mapping.Workflow
			break
		}
	}

	return c.GetWorkflow(workflowName)
}

// BuildWorkflowStateMachine builds a state machine from workflow definition
func (c *Client) BuildWorkflowStateMachine(workflow *Workflow) (*WorkflowStateMachine, error) {
	if workflow == nil {
		return nil, fmt.Errorf("workflow cannot be nil")
	}

	sm := &WorkflowStateMachine{
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		States:       make(map[string]*WorkflowState),
		Transitions:  make(map[string]*WorkflowTransitionSM),
	}

	// Build states
	for _, status := range workflow.Statuses {
		state := &WorkflowState{
			ID:       status.ID,
			Name:     status.Name,
			Category: status.StatusCategory.Key,
			Type:     "intermediate",
		}

		// Determine state type
		if status.StatusCategory.Key == "new" {
			state.Type = "initial"
			if sm.InitialState == "" {
				sm.InitialState = status.ID
			}
		} else if status.StatusCategory.Key == "done" {
			state.Type = "final"
			sm.FinalStates = append(sm.FinalStates, status.ID)
		}

		sm.States[status.ID] = state
	}

	// Build transitions
	for _, transition := range workflow.Transitions {
		smTransition := &WorkflowTransitionSM{
			ID:       transition.ID,
			Name:     transition.Name,
			ToState:  transition.To.ID,
		}

		// Handle from states
		for _, from := range transition.From {
			smTransition.FromStates = append(smTransition.FromStates, from.ID)
			
			// Add transition to state
			if state, exists := sm.States[from.ID]; exists {
				state.Transitions = append(state.Transitions, transition.ID)
			}
		}

		// Convert rules to state machine constructs
		if transition.Rules != nil {
			smTransition.Conditions = transition.Rules.Conditions
			
			// Convert validators to guards
			for _, validator := range transition.Rules.Validators {
				guard := WorkflowGuard{
					Type:          validator.Type,
					Configuration: validator.Configuration,
				}
				smTransition.Guards = append(smTransition.Guards, guard)
			}

			// Convert post-functions to actions
			for i, postFunc := range transition.Rules.PostFunctions {
				action := WorkflowAction{
					Type:          postFunc.Type,
					Configuration: postFunc.Configuration,
					Order:         i,
				}
				smTransition.Actions = append(smTransition.Actions, action)
			}
		}

		sm.Transitions[transition.ID] = smTransition
	}

	return sm, nil
}

// ValidateTransition validates if a transition is allowed for an issue
func (c *Client) ValidateTransition(issueKey, transitionID string) (*WorkflowExecutionResult, error) {
	// Get available transitions for the issue
	transitions, err := c.GetTransitions(issueKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get transitions: %w", err)
	}

	// Check if transition is available
	var transition *Transition
	for _, t := range transitions {
		if t.ID == transitionID {
			transition = &t
			break
		}
	}

	if transition == nil {
		return &WorkflowExecutionResult{
			Success:     false,
			IssueKey:    issueKey,
			TransitionID: transitionID,
			ExecutedAt:  time.Now(),
			Errors: []WorkflowError{
				{
					Type:      "TRANSITION_NOT_AVAILABLE",
					Message:   fmt.Sprintf("Transition '%s' is not available for issue '%s'", transitionID, issueKey),
					Timestamp: time.Now(),
				},
			},
		}, nil
	}

	return &WorkflowExecutionResult{
		Success:      true,
		IssueKey:     issueKey,
		TransitionID: transitionID,
		ExecutedAt:   time.Now(),
	}, nil
}

// ExecuteTransition executes a workflow transition with full context
func (c *Client) ExecuteTransition(ctx *WorkflowExecutionContext) (*WorkflowExecutionResult, error) {
	startTime := time.Now()
	
	result := &WorkflowExecutionResult{
		IssueKey:     ctx.IssueKey,
		TransitionID: ctx.TransitionID,
		ExecutionID:  ctx.ExecutionID,
		ExecutedAt:   startTime,
	}

	// Validate transition first
	validation, err := c.ValidateTransition(ctx.IssueKey, ctx.TransitionID)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, WorkflowError{
			Type:      "VALIDATION_ERROR",
			Message:   err.Error(),
			Timestamp: time.Now(),
		})
		return result, nil
	}

	if !validation.Success {
		result.Success = false
		result.Errors = validation.Errors
		return result, nil
	}

	// Execute the transition
	err = c.TransitionIssueAdvanced(ctx.IssueKey, ctx.TransitionID, ctx.Fields, ctx.Comment)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, WorkflowError{
			Type:      "EXECUTION_ERROR",
			Message:   err.Error(),
			Timestamp: time.Now(),
		})
		return result, nil
	}

	result.Success = true
	result.Duration = time.Since(startTime)
	
	return result, nil
}