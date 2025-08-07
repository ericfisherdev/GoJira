package claude

import (
	"fmt"
	"strings"
	"time"

	"github.com/ericfisherdev/GoJira/internal/nlp"
	"github.com/rs/zerolog/log"
)

// WorkflowEngine manages multi-step command workflows for Claude Code
type WorkflowEngine struct {
	sessionManager *SessionManager
	patternManager *PatternManager
	templates      map[string]*WorkflowTemplate
}

// WorkflowTemplate defines a multi-step workflow
type WorkflowTemplate struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Steps       []WorkflowStep          `json:"steps"`
	Triggers    []string                `json:"triggers"`
	Category    string                  `json:"category"`
	Metadata    map[string]interface{}  `json:"metadata,omitempty"`
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Type         WorkflowStepType       `json:"type"`
	Required     bool                   `json:"required"`
	Prompt       string                 `json:"prompt,omitempty"`
	Validation   *StepValidation        `json:"validation,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Action       WorkflowAction         `json:"action,omitempty"`
	Timeout      time.Duration          `json:"timeout,omitempty"`
	Retry        *RetryConfig           `json:"retry,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowStepType defines the type of workflow step
type WorkflowStepType string

const (
	StepTypeInput      WorkflowStepType = "input"
	StepTypeValidation WorkflowStepType = "validation"
	StepTypeAction     WorkflowStepType = "action"
	StepTypeCondition  WorkflowStepType = "condition"
	StepTypeParallel   WorkflowStepType = "parallel"
	StepTypeWait       WorkflowStepType = "wait"
)

// StepValidation defines validation rules for a step
type StepValidation struct {
	EntityType   nlp.EntityType `json:"entityType,omitempty"`
	Pattern      string         `json:"pattern,omitempty"`
	Required     bool           `json:"required"`
	MinLength    int            `json:"minLength,omitempty"`
	MaxLength    int            `json:"maxLength,omitempty"`
	Options      []string       `json:"options,omitempty"`
	CustomRules  []string       `json:"customRules,omitempty"`
}

// WorkflowAction defines what action to perform in a step
type WorkflowAction struct {
	Type       string                 `json:"type"`
	Handler    string                 `json:"handler"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Async      bool                   `json:"async"`
}

// RetryConfig defines retry behavior for failed steps
type RetryConfig struct {
	MaxAttempts int           `json:"maxAttempts"`
	Delay       time.Duration `json:"delay"`
	Backoff     string        `json:"backoff"` // "linear", "exponential"
}

// WorkflowExecution tracks the execution state of a workflow
type WorkflowExecution struct {
	ID            string                 `json:"id"`
	SessionID     string                 `json:"sessionId"`
	TemplateID    string                 `json:"templateId"`
	CurrentStep   int                    `json:"currentStep"`
	Status        ExecutionStatus        `json:"status"`
	StartTime     time.Time              `json:"startTime"`
	EndTime       time.Time              `json:"endTime,omitempty"`
	Data          map[string]interface{} `json:"data"`
	Results       map[int]*StepResult    `json:"results"`
	ErrorMessage  string                 `json:"errorMessage,omitempty"`
	Context       *CommandContext        `json:"context,omitempty"`
}

// ExecutionStatus represents workflow execution status
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusWaiting   ExecutionStatus = "waiting"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
	StatusCancelled ExecutionStatus = "cancelled"
)

// StepResult captures the result of a workflow step
type StepResult struct {
	StepID      string                 `json:"stepId"`
	Status      ExecutionStatus        `json:"status"`
	StartTime   time.Time              `json:"startTime"`
	EndTime     time.Time              `json:"endTime,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Attempts    int                    `json:"attempts"`
}

// NewWorkflowEngine creates a new workflow engine
func NewWorkflowEngine(sessionManager *SessionManager, patternManager *PatternManager) *WorkflowEngine {
	we := &WorkflowEngine{
		sessionManager: sessionManager,
		patternManager: patternManager,
		templates:      make(map[string]*WorkflowTemplate),
	}

	we.initializeBuiltinWorkflows()
	return we
}

// initializeBuiltinWorkflows sets up predefined workflows for common Claude Code tasks
func (we *WorkflowEngine) initializeBuiltinWorkflows() {
	workflows := []*WorkflowTemplate{
		{
			ID:          "create_issue_guided",
			Name:        "Guided Issue Creation",
			Description: "Step-by-step issue creation with intelligent suggestions",
			Category:    "creation",
			Triggers:    []string{"create issue", "new issue", "guided create"},
			Steps: []WorkflowStep{
				{
					ID:          "select_project",
					Name:        "Select Project",
					Description: "Choose which project to create the issue in",
					Type:        StepTypeInput,
					Required:    true,
					Prompt:      "Which project would you like to create the issue in?",
					Validation: &StepValidation{
						EntityType: nlp.EntityProject,
						Required:   true,
					},
				},
				{
					ID:          "select_issue_type",
					Name:        "Issue Type",
					Description: "Select the type of issue to create",
					Type:        StepTypeInput,
					Required:    true,
					Prompt:      "What type of issue would you like to create?",
					Validation: &StepValidation{
						EntityType: nlp.EntityIssueType,
						Required:   true,
						Options:    []string{"Bug", "Task", "Story", "Epic", "Improvement"},
					},
				},
				{
					ID:          "set_summary",
					Name:        "Issue Summary",
					Description: "Provide a brief summary for the issue",
					Type:        StepTypeInput,
					Required:    true,
					Prompt:      "Please provide a brief summary for the issue:",
					Validation: &StepValidation{
						Required:  true,
						MinLength: 5,
						MaxLength: 200,
					},
				},
				{
					ID:          "set_priority",
					Name:        "Set Priority",
					Description: "Set the priority level for the issue",
					Type:        StepTypeInput,
					Required:    false,
					Prompt:      "What priority should this issue have? (default: Medium)",
					Validation: &StepValidation{
						EntityType: nlp.EntityPriority,
						Options:    []string{"Highest", "High", "Medium", "Low", "Lowest"},
					},
				},
				{
					ID:          "assign_issue",
					Name:        "Assign Issue",
					Description: "Assign the issue to someone (optional)",
					Type:        StepTypeInput,
					Required:    false,
					Prompt:      "Who should this issue be assigned to? (leave blank for unassigned)",
					Validation: &StepValidation{
						EntityType: nlp.EntityAssignee,
					},
				},
				{
					ID:          "create_action",
					Name:        "Create Issue",
					Description: "Execute the issue creation",
					Type:        StepTypeAction,
					Required:    true,
					Action: WorkflowAction{
						Type:    "create_issue",
						Handler: "handleCreateIssue",
						Async:   false,
					},
					Retry: &RetryConfig{
						MaxAttempts: 3,
						Delay:       time.Second * 2,
						Backoff:     "exponential",
					},
				},
			},
		},
		{
			ID:          "batch_transition",
			Name:        "Batch Issue Transition",
			Description: "Transition multiple issues to a new status",
			Category:    "batch",
			Triggers:    []string{"move all", "transition all", "batch transition"},
			Steps: []WorkflowStep{
				{
					ID:          "define_criteria",
					Name:        "Search Criteria",
					Description: "Define which issues to transition",
					Type:        StepTypeInput,
					Required:    true,
					Prompt:      "What criteria should be used to find the issues? (e.g., 'all bugs in PROJ assigned to me')",
				},
				{
					ID:          "confirm_search",
					Name:        "Confirm Search Results",
					Description: "Review and confirm the issues found",
					Type:        StepTypeAction,
					Action: WorkflowAction{
						Type:    "search_issues",
						Handler: "handleSearchIssues",
					},
				},
				{
					ID:          "select_target_status",
					Name:        "Target Status",
					Description: "Select the status to transition issues to",
					Type:        StepTypeInput,
					Required:    true,
					Prompt:      "What status should these issues be moved to?",
					Validation: &StepValidation{
						EntityType: nlp.EntityStatus,
						Required:   true,
					},
				},
				{
					ID:          "confirm_transition",
					Name:        "Confirm Transition",
					Description: "Confirm the batch transition operation",
					Type:        StepTypeCondition,
					Prompt:      "Are you sure you want to transition these issues?",
				},
				{
					ID:          "execute_transitions",
					Name:        "Execute Transitions",
					Description: "Perform the batch transition",
					Type:        StepTypeAction,
					Action: WorkflowAction{
						Type:    "batch_transition",
						Handler: "handleBatchTransition",
						Async:   true,
					},
				},
			},
		},
		{
			ID:          "sprint_planning",
			Name:        "Sprint Planning Assistant",
			Description: "Guided sprint planning and issue organization",
			Category:    "sprint",
			Triggers:    []string{"plan sprint", "sprint planning", "organize sprint"},
			Steps: []WorkflowStep{
				{
					ID:          "select_sprint",
					Name:        "Select Sprint",
					Description: "Choose which sprint to plan",
					Type:        StepTypeInput,
					Required:    true,
					Prompt:      "Which sprint would you like to plan?",
					Validation: &StepValidation{
						EntityType: nlp.EntitySprint,
						Required:   true,
					},
				},
				{
					ID:          "review_backlog",
					Name:        "Review Backlog",
					Description: "Review available backlog items",
					Type:        StepTypeAction,
					Action: WorkflowAction{
						Type:    "get_backlog",
						Handler: "handleGetBacklog",
					},
				},
				{
					ID:          "select_issues",
					Name:        "Select Issues",
					Description: "Choose issues for the sprint",
					Type:        StepTypeInput,
					Prompt:      "Which issues should be added to this sprint?",
				},
				{
					ID:          "estimate_capacity",
					Name:        "Estimate Capacity",
					Description: "Review sprint capacity and estimates",
					Type:        StepTypeAction,
					Action: WorkflowAction{
						Type:    "estimate_capacity",
						Handler: "handleEstimateCapacity",
					},
				},
				{
					ID:          "finalize_sprint",
					Name:        "Finalize Sprint",
					Description: "Complete sprint setup",
					Type:        StepTypeAction,
					Action: WorkflowAction{
						Type:    "finalize_sprint",
						Handler: "handleFinalizeSprint",
					},
				},
			},
		},
	}

	for _, workflow := range workflows {
		we.templates[workflow.ID] = workflow
	}

	log.Info().
		Int("workflows", len(workflows)).
		Msg("Initialized builtin Claude Code workflows")
}

// StartWorkflow initiates a workflow based on command input
func (we *WorkflowEngine) StartWorkflow(ctx *CommandContext, templateID string) (*WorkflowExecution, error) {
	template, exists := we.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("workflow template not found: %s", templateID)
	}

	execution := &WorkflowExecution{
		ID:          generateExecutionID(),
		SessionID:   ctx.Session.ID,
		TemplateID:  templateID,
		CurrentStep: 0,
		Status:      StatusPending,
		StartTime:   time.Now(),
		Data:        make(map[string]interface{}),
		Results:     make(map[int]*StepResult),
		Context:     ctx,
	}

	// Start workflow in session manager
	err := we.sessionManager.StartWorkflow(ctx.Session.ID, templateID, len(template.Steps))
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow in session: %w", err)
	}

	execution.Status = StatusRunning

	log.Info().
		Str("executionId", execution.ID).
		Str("sessionId", ctx.Session.ID).
		Str("templateId", templateID).
		Int("steps", len(template.Steps)).
		Msg("Started workflow execution")

	return execution, nil
}

// ProcessWorkflowStep processes a single step in the workflow
func (we *WorkflowEngine) ProcessWorkflowStep(execution *WorkflowExecution, input string) (*StepResult, error) {
	template := we.templates[execution.TemplateID]
	if template == nil {
		return nil, fmt.Errorf("workflow template not found: %s", execution.TemplateID)
	}

	if execution.CurrentStep >= len(template.Steps) {
		return nil, fmt.Errorf("workflow already completed")
	}

	step := template.Steps[execution.CurrentStep]
	result := &StepResult{
		StepID:    step.ID,
		Status:    StatusRunning,
		StartTime: time.Now(),
		Data:      make(map[string]interface{}),
		Attempts:  1,
	}

	log.Debug().
		Str("executionId", execution.ID).
		Str("stepId", step.ID).
		Int("stepNumber", execution.CurrentStep+1).
		Str("input", input).
		Msg("Processing workflow step")

	// Validate input if required
	if step.Validation != nil {
		if err := we.validateStepInput(step, input); err != nil {
			result.Status = StatusFailed
			result.Error = err.Error()
			result.EndTime = time.Now()
			return result, err
		}
	}

	// Execute step based on type
	var err error
	switch step.Type {
	case StepTypeInput:
		err = we.processInputStep(step, input, result, execution)
	case StepTypeValidation:
		err = we.processValidationStep(step, input, result, execution)
	case StepTypeAction:
		err = we.processActionStep(step, input, result, execution)
	case StepTypeCondition:
		err = we.processConditionStep(step, input, result, execution)
	default:
		err = fmt.Errorf("unsupported step type: %s", step.Type)
	}

	result.EndTime = time.Now()

	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()

		// Handle retry logic
		if step.Retry != nil && result.Attempts < step.Retry.MaxAttempts {
			log.Debug().
				Str("stepId", step.ID).
				Int("attempt", result.Attempts).
				Int("maxAttempts", step.Retry.MaxAttempts).
				Msg("Retrying failed step")
			
			// Schedule retry (simplified - in production would use proper scheduler)
			time.Sleep(step.Retry.Delay)
			result.Attempts++
			return we.ProcessWorkflowStep(execution, input)
		}

		execution.Status = StatusFailed
		execution.ErrorMessage = err.Error()
		return result, err
	}

	result.Status = StatusCompleted

	// Store result and advance workflow
	execution.Results[execution.CurrentStep] = result
	execution.CurrentStep++

	// Update session
	stepData := map[string]interface{}{
		"stepId":     step.ID,
		"stepName":   step.Name,
		"input":      input,
		"result":     result.Data,
		"completed":  true,
	}
	we.sessionManager.CompleteWorkflowStep(execution.SessionID, stepData)

	// Check if workflow is complete
	if execution.CurrentStep >= len(template.Steps) {
		execution.Status = StatusCompleted
		execution.EndTime = time.Now()

		log.Info().
			Str("executionId", execution.ID).
			Dur("duration", time.Since(execution.StartTime)).
			Msg("Workflow execution completed")
	}

	return result, nil
}

// DetectWorkflowTrigger checks if input should trigger a workflow
func (we *WorkflowEngine) DetectWorkflowTrigger(input string) string {
	lowerInput := strings.ToLower(input)

	for _, template := range we.templates {
		for _, trigger := range template.Triggers {
			if strings.Contains(lowerInput, strings.ToLower(trigger)) {
				log.Debug().
					Str("input", input).
					Str("templateId", template.ID).
					Str("trigger", trigger).
					Msg("Detected workflow trigger")
				return template.ID
			}
		}
	}

	return ""
}

// GetWorkflowStatus returns the current status of a workflow execution
func (we *WorkflowEngine) GetWorkflowStatus(sessionID string) (*WorkflowExecution, error) {
	session, exists := we.sessionManager.GetSession(sessionID)
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	if !session.IsWorkflowActive() {
		return nil, fmt.Errorf("no active workflow in session")
	}

	// Build execution status from session workflow state
	workflowState := session.WorkflowState
	_ = we.templates[workflowState.Type]
	
	execution := &WorkflowExecution{
		ID:          workflowState.ID,
		SessionID:   sessionID,
		TemplateID:  workflowState.Type,
		CurrentStep: workflowState.CurrentStep,
		Data:        workflowState.StepData,
		Results:     make(map[int]*StepResult),
	}

	if workflowState.IsCompleted {
		execution.Status = StatusCompleted
		execution.EndTime = workflowState.CompletedAt
	} else {
		execution.Status = StatusRunning
	}

	return execution, nil
}

// Step processing methods

func (we *WorkflowEngine) processInputStep(step WorkflowStep, input string, result *StepResult, execution *WorkflowExecution) error {
	// Parse input and extract relevant data
	entities := we.extractStepEntities(input, step)
	
	result.Data["input"] = input
	result.Data["entities"] = entities

	// Store data for later steps
	execution.Data[step.ID] = result.Data

	return nil
}

func (we *WorkflowEngine) processValidationStep(step WorkflowStep, input string, result *StepResult, execution *WorkflowExecution) error {
	if step.Validation == nil {
		return nil
	}

	return we.validateStepInput(step, input)
}

func (we *WorkflowEngine) processActionStep(step WorkflowStep, input string, result *StepResult, execution *WorkflowExecution) error {
	if step.Action.Handler == "" {
		return fmt.Errorf("no handler specified for action step")
	}

	// Simulate action execution (in real implementation, would call actual handlers)
	result.Data["action"] = step.Action.Type
	result.Data["handler"] = step.Action.Handler
	result.Data["async"] = step.Action.Async

	if step.Action.Async {
		// For async actions, we would start the action and continue
		result.Data["status"] = "started"
	} else {
		result.Data["status"] = "completed"
	}

	return nil
}

func (we *WorkflowEngine) processConditionStep(step WorkflowStep, input string, result *StepResult, execution *WorkflowExecution) error {
	// Simple confirmation logic
	lowerInput := strings.ToLower(strings.TrimSpace(input))
	
	confirmed := false
	switch lowerInput {
	case "yes", "y", "ok", "confirm", "proceed", "continue":
		confirmed = true
	case "no", "n", "cancel", "abort", "stop":
		confirmed = false
	default:
		return fmt.Errorf("please respond with 'yes' or 'no'")
	}

	result.Data["confirmed"] = confirmed
	
	if !confirmed {
		execution.Status = StatusCancelled
		return fmt.Errorf("workflow cancelled by user")
	}

	return nil
}

// Helper methods

func (we *WorkflowEngine) validateStepInput(step WorkflowStep, input string) error {
	validation := step.Validation
	
	if validation.Required && strings.TrimSpace(input) == "" {
		return fmt.Errorf("input is required for step: %s", step.Name)
	}

	if validation.MinLength > 0 && len(input) < validation.MinLength {
		return fmt.Errorf("input too short (minimum %d characters)", validation.MinLength)
	}

	if validation.MaxLength > 0 && len(input) > validation.MaxLength {
		return fmt.Errorf("input too long (maximum %d characters)", validation.MaxLength)
	}

	if len(validation.Options) > 0 {
		lowerInput := strings.ToLower(input)
		valid := false
		for _, option := range validation.Options {
			if strings.ToLower(option) == lowerInput {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid option. Valid options: %s", strings.Join(validation.Options, ", "))
		}
	}

	return nil
}

func (we *WorkflowEngine) extractStepEntities(input string, step WorkflowStep) map[string]interface{} {
	entities := make(map[string]interface{})

	if step.Validation != nil && step.Validation.EntityType != "" {
		// Use pattern manager or NLP parser to extract entities
		// This is a simplified implementation
		entities[string(step.Validation.EntityType)] = input
	}

	return entities
}

func generateExecutionID() string {
	return fmt.Sprintf("exec_%d", time.Now().UnixNano())
}

// GetAvailableWorkflows returns all available workflow templates
func (we *WorkflowEngine) GetAvailableWorkflows() []*WorkflowTemplate {
	workflows := make([]*WorkflowTemplate, 0, len(we.templates))
	for _, template := range we.templates {
		workflows = append(workflows, template)
	}
	return workflows
}

// GetWorkflowByCategory returns workflows in a specific category
func (we *WorkflowEngine) GetWorkflowByCategory(category string) []*WorkflowTemplate {
	workflows := make([]*WorkflowTemplate, 0)
	for _, template := range we.templates {
		if template.Category == category {
			workflows = append(workflows, template)
		}
	}
	return workflows
}