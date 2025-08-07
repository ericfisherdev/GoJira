package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/rs/zerolog/log"
)

// WorkflowService provides business logic for workflow operations
type WorkflowService struct {
	jiraClient       jira.ClientInterface
	cache            *WorkflowCache
	transitionEngine *TransitionEngine
	validator        *WorkflowValidator
}

// WorkflowCache provides in-memory caching for workflow data
type WorkflowCache struct {
	mu              sync.RWMutex
	workflows       map[string]*jira.Workflow
	stateMachines   map[string]*jira.WorkflowStateMachine
	schemes         map[string]*jira.WorkflowScheme
	lastUpdate      map[string]time.Time
	ttl             time.Duration
}

// NewWorkflowService creates a new workflow service
func NewWorkflowService(jiraClient jira.ClientInterface) *WorkflowService {
	cache := &WorkflowCache{
		workflows:     make(map[string]*jira.Workflow),
		stateMachines: make(map[string]*jira.WorkflowStateMachine),
		schemes:       make(map[string]*jira.WorkflowScheme),
		lastUpdate:    make(map[string]time.Time),
		ttl:           10 * time.Minute,
	}

	return &WorkflowService{
		jiraClient:       jiraClient,
		cache:            cache,
		transitionEngine: NewTransitionEngine(jiraClient),
		validator:        NewWorkflowValidator(),
	}
}

// TransitionEngine handles workflow transition execution
type TransitionEngine struct {
	jiraClient jira.ClientInterface
	hooks      []TransitionHook
	metrics    *TransitionMetrics
}

// TransitionHook allows custom logic during transitions
type TransitionHook interface {
	PreTransition(ctx context.Context, req *TransitionRequest) error
	PostTransition(ctx context.Context, req *TransitionRequest, result *jira.WorkflowExecutionResult) error
}

// TransitionRequest represents a transition request with metadata
type TransitionRequest struct {
	IssueKey     string
	TransitionID string
	Fields       map[string]interface{}
	Comment      string
	UserKey      string
	Reason       string
	ValidateOnly bool
}

// TransitionMetrics tracks transition performance
type TransitionMetrics struct {
	mu               sync.RWMutex
	TotalTransitions int64
	SuccessCount     int64
	FailureCount     int64
	AvgDuration      time.Duration
	TransitionCounts map[string]int64
}

// NewTransitionEngine creates a new transition engine
func NewTransitionEngine(jiraClient jira.ClientInterface) *TransitionEngine {
	return &TransitionEngine{
		jiraClient: jiraClient,
		hooks:      []TransitionHook{},
		metrics: &TransitionMetrics{
			TransitionCounts: make(map[string]int64),
		},
	}
}

// WorkflowValidator validates workflow operations
type WorkflowValidator struct {
	rules []ValidationRule
}

// ValidationRule represents a validation rule for workflows
type ValidationRule interface {
	Validate(ctx context.Context, workflow *jira.Workflow) error
}

// NewWorkflowValidator creates a new workflow validator
func NewWorkflowValidator() *WorkflowValidator {
	return &WorkflowValidator{
		rules: []ValidationRule{
			&StateValidationRule{},
			&TransitionValidationRule{},
			&CycleDetectionRule{},
		},
	}
}

// GetWorkflowWithCache retrieves a workflow with caching
func (s *WorkflowService) GetWorkflowWithCache(ctx context.Context, workflowName string) (*jira.Workflow, error) {
	// Check cache first
	s.cache.mu.RLock()
	if cached, exists := s.cache.workflows[workflowName]; exists {
		if time.Since(s.cache.lastUpdate[workflowName]) < s.cache.ttl {
			s.cache.mu.RUnlock()
			log.Debug().Str("workflow", workflowName).Msg("Returning cached workflow")
			return cached, nil
		}
	}
	s.cache.mu.RUnlock()

	// Fetch from Jira
	workflow, err := s.jiraClient.GetWorkflow(workflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Update cache
	s.cache.mu.Lock()
	s.cache.workflows[workflowName] = workflow
	s.cache.lastUpdate[workflowName] = time.Now()
	s.cache.mu.Unlock()

	return workflow, nil
}

// GetStateMachine builds or retrieves cached state machine
func (s *WorkflowService) GetStateMachine(ctx context.Context, workflowName string) (*jira.WorkflowStateMachine, error) {
	// Check cache
	s.cache.mu.RLock()
	if cached, exists := s.cache.stateMachines[workflowName]; exists {
		if time.Since(s.cache.lastUpdate["sm_"+workflowName]) < s.cache.ttl {
			s.cache.mu.RUnlock()
			return cached, nil
		}
	}
	s.cache.mu.RUnlock()

	// Get workflow
	workflow, err := s.GetWorkflowWithCache(ctx, workflowName)
	if err != nil {
		return nil, err
	}

	// Build state machine
	stateMachine, err := s.jiraClient.BuildWorkflowStateMachine(workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to build state machine: %w", err)
	}

	// Cache it
	s.cache.mu.Lock()
	s.cache.stateMachines[workflowName] = stateMachine
	s.cache.lastUpdate["sm_"+workflowName] = time.Now()
	s.cache.mu.Unlock()

	return stateMachine, nil
}

// ExecuteTransition executes a workflow transition with full validation
func (s *WorkflowService) ExecuteTransition(ctx context.Context, req *TransitionRequest) (*jira.WorkflowExecutionResult, error) {
	startTime := time.Now()

	// Pre-transition hooks
	for _, hook := range s.transitionEngine.hooks {
		if err := hook.PreTransition(ctx, req); err != nil {
			return &jira.WorkflowExecutionResult{
				Success:      false,
				IssueKey:     req.IssueKey,
				TransitionID: req.TransitionID,
				ExecutedAt:   startTime,
				Errors: []jira.WorkflowError{
					{
						Type:      "PRE_HOOK_ERROR",
						Message:   err.Error(),
						Timestamp: time.Now(),
					},
				},
			}, nil
		}
	}

	// Validate transition
	validation, err := s.ValidateTransition(ctx, req.IssueKey, req.TransitionID)
	if err != nil {
		return nil, err
	}

	if !validation.Success {
		return validation, nil
	}

	// If validate only, return here
	if req.ValidateOnly {
		return validation, nil
	}

	// Execute transition
	result := s.transitionEngine.Execute(ctx, req)

	// Update metrics
	s.transitionEngine.updateMetrics(result, time.Since(startTime))

	// Post-transition hooks
	for _, hook := range s.transitionEngine.hooks {
		if err := hook.PostTransition(ctx, req, result); err != nil {
			log.Warn().Err(err).Msg("Post-transition hook failed")
		}
	}

	return result, nil
}

// ValidateTransition validates if a transition is allowed
func (s *WorkflowService) ValidateTransition(ctx context.Context, issueKey, transitionID string) (*jira.WorkflowExecutionResult, error) {
	// Get issue details
	issue, err := s.jiraClient.GetIssue(ctx, issueKey, []string{"transitions"})
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	// Get available transitions
	transitions, err := s.jiraClient.GetTransitions(issueKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get transitions: %w", err)
	}

	// Check if transition is available
	var transition *jira.Transition
	for _, t := range transitions {
		if t.ID == transitionID {
			transition = &t
			break
		}
	}

	if transition == nil {
		return &jira.WorkflowExecutionResult{
			Success:      false,
			IssueKey:     issueKey,
			TransitionID: transitionID,
			ExecutedAt:   time.Now(),
			Errors: []jira.WorkflowError{
				{
					Type:    "TRANSITION_NOT_AVAILABLE",
					Message: fmt.Sprintf("Transition '%s' is not available for issue '%s' in current state", transitionID, issueKey),
					Code:    "TRANS_001",
					Timestamp: time.Now(),
				},
			},
		}, nil
	}

	// Get workflow for additional validation
	workflow, err := s.jiraClient.GetIssueWorkflow(issueKey)
	if err != nil {
		log.Warn().Err(err).Str("issueKey", issueKey).Msg("Failed to get issue workflow for validation")
	} else {
		// Validate against workflow rules
		if err := s.validator.ValidateTransition(ctx, workflow, issue, transition); err != nil {
			return &jira.WorkflowExecutionResult{
				Success:      false,
				IssueKey:     issueKey,
				TransitionID: transitionID,
				ExecutedAt:   time.Now(),
				Errors: []jira.WorkflowError{
					{
						Type:      "VALIDATION_ERROR",
						Message:   err.Error(),
						Code:      "VAL_001",
						Timestamp: time.Now(),
					},
				},
			}, nil
		}
	}

	return &jira.WorkflowExecutionResult{
		Success:      true,
		IssueKey:     issueKey,
		TransitionID: transitionID,
		ExecutedAt:   time.Now(),
	}, nil
}

// Execute performs the actual transition
func (e *TransitionEngine) Execute(ctx context.Context, req *TransitionRequest) *jira.WorkflowExecutionResult {
	executionCtx := &jira.WorkflowExecutionContext{
		IssueKey:     req.IssueKey,
		TransitionID: req.TransitionID,
		Fields:       req.Fields,
		Comment:      req.Comment,
		ExecutedAt:   time.Now(),
		ExecutionID:  generateExecutionID(),
	}

	result, err := e.jiraClient.ExecuteTransition(executionCtx)
	if err != nil {
		return &jira.WorkflowExecutionResult{
			Success:      false,
			ExecutionID:  executionCtx.ExecutionID,
			IssueKey:     req.IssueKey,
			TransitionID: req.TransitionID,
			ExecutedAt:   executionCtx.ExecutedAt,
			Errors: []jira.WorkflowError{
				{
					Type:      "EXECUTION_ERROR",
					Message:   err.Error(),
					Timestamp: time.Now(),
				},
			},
		}
	}

	return result
}

// updateMetrics updates transition metrics
func (e *TransitionEngine) updateMetrics(result *jira.WorkflowExecutionResult, duration time.Duration) {
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()

	e.metrics.TotalTransitions++
	
	if result.Success {
		e.metrics.SuccessCount++
	} else {
		e.metrics.FailureCount++
	}

	// Update average duration
	if e.metrics.AvgDuration == 0 {
		e.metrics.AvgDuration = duration
	} else {
		e.metrics.AvgDuration = (e.metrics.AvgDuration + duration) / 2
	}

	// Track transition counts
	e.metrics.TransitionCounts[result.TransitionID]++
}

// GetTransitionMetrics returns current transition metrics
func (s *WorkflowService) GetTransitionMetrics() TransitionMetrics {
	s.transitionEngine.metrics.mu.RLock()
	defer s.transitionEngine.metrics.mu.RUnlock()

	// Return a copy
	return TransitionMetrics{
		TotalTransitions: s.transitionEngine.metrics.TotalTransitions,
		SuccessCount:     s.transitionEngine.metrics.SuccessCount,
		FailureCount:     s.transitionEngine.metrics.FailureCount,
		AvgDuration:      s.transitionEngine.metrics.AvgDuration,
		TransitionCounts: copyMap(s.transitionEngine.metrics.TransitionCounts),
	}
}

// AddTransitionHook adds a transition hook
func (s *WorkflowService) AddTransitionHook(hook TransitionHook) {
	s.transitionEngine.hooks = append(s.transitionEngine.hooks, hook)
}

// GetWorkflowAnalytics generates analytics for a workflow
func (s *WorkflowService) GetWorkflowAnalytics(ctx context.Context, workflowName string, days int) (*WorkflowAnalytics, error) {
	workflow, err := s.GetWorkflowWithCache(ctx, workflowName)
	if err != nil {
		return nil, err
	}

	stateMachine, err := s.GetStateMachine(ctx, workflowName)
	if err != nil {
		return nil, err
	}

	analytics := &WorkflowAnalytics{
		WorkflowName:     workflowName,
		TotalStates:      len(workflow.Statuses),
		TotalTransitions: len(workflow.Transitions),
		PeriodDays:       days,
		GeneratedAt:      time.Now(),
	}

	// Analyze state machine
	analytics.InitialState = stateMachine.InitialState
	analytics.FinalStates = stateMachine.FinalStates

	// Calculate complexity metrics
	analytics.Complexity = s.calculateComplexity(stateMachine)

	// Find bottlenecks (states with many incoming transitions)
	analytics.Bottlenecks = s.findBottlenecks(stateMachine)

	// Get transition metrics
	metrics := s.GetTransitionMetrics()
	analytics.TransitionMetrics = map[string]interface{}{
		"total":       metrics.TotalTransitions,
		"success":     metrics.SuccessCount,
		"failure":     metrics.FailureCount,
		"successRate": float64(metrics.SuccessCount) / float64(metrics.TotalTransitions) * 100,
		"avgDuration": metrics.AvgDuration.String(),
	}

	// Most used transitions
	analytics.MostUsedTransitions = s.getTopTransitions(metrics.TransitionCounts, 5)

	return analytics, nil
}

// WorkflowAnalytics contains workflow usage analytics
type WorkflowAnalytics struct {
	WorkflowName        string                 `json:"workflowName"`
	TotalStates         int                    `json:"totalStates"`
	TotalTransitions    int                    `json:"totalTransitions"`
	InitialState        string                 `json:"initialState"`
	FinalStates         []string               `json:"finalStates"`
	Complexity          float64                `json:"complexity"`
	Bottlenecks         []string               `json:"bottlenecks"`
	TransitionMetrics   map[string]interface{} `json:"transitionMetrics"`
	MostUsedTransitions []TransitionUsage      `json:"mostUsedTransitions"`
	PeriodDays          int                    `json:"periodDays"`
	GeneratedAt         time.Time              `json:"generatedAt"`
}

// TransitionUsage represents transition usage statistics
type TransitionUsage struct {
	TransitionID string `json:"transitionId"`
	Name         string `json:"name"`
	Count        int64  `json:"count"`
	Percentage   float64 `json:"percentage"`
}

// calculateComplexity calculates workflow complexity
func (s *WorkflowService) calculateComplexity(sm *jira.WorkflowStateMachine) float64 {
	if len(sm.States) == 0 {
		return 0
	}

	// Complexity = (transitions / states) * (1 + cycles/10)
	complexity := float64(len(sm.Transitions)) / float64(len(sm.States))
	
	// Detect cycles
	cycles := s.detectCycles(sm)
	complexity *= (1 + float64(len(cycles))/10)

	return complexity
}

// findBottlenecks identifies states with many incoming transitions
func (s *WorkflowService) findBottlenecks(sm *jira.WorkflowStateMachine) []string {
	incomingCount := make(map[string]int)

	for _, transition := range sm.Transitions {
		incomingCount[transition.ToState]++
	}

	var bottlenecks []string
	threshold := 2 // States with 2+ incoming transitions are bottlenecks (lowered threshold)

	for state, count := range incomingCount {
		if count >= threshold {
			bottlenecks = append(bottlenecks, state)
		}
	}

	return bottlenecks
}

// detectCycles detects cycles in the workflow
func (s *WorkflowService) detectCycles(sm *jira.WorkflowStateMachine) [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}

	for stateID := range sm.States {
		if !visited[stateID] {
			s.dfsDetectCycles(sm, stateID, visited, recStack, path, &cycles)
		}
	}

	return cycles
}

// dfsDetectCycles performs DFS to detect cycles
func (s *WorkflowService) dfsDetectCycles(sm *jira.WorkflowStateMachine, stateID string, visited, recStack map[string]bool, path []string, cycles *[][]string) {
	visited[stateID] = true
	recStack[stateID] = true
	path = append(path, stateID)

	// Find outgoing transitions
	for _, transition := range sm.Transitions {
		for _, fromState := range transition.FromStates {
			if fromState == stateID {
				nextState := transition.ToState
				
				if !visited[nextState] {
					s.dfsDetectCycles(sm, nextState, visited, recStack, path, cycles)
				} else if recStack[nextState] {
					// Found a cycle
					cycleStart := 0
					for i, state := range path {
						if state == nextState {
							cycleStart = i
							break
						}
					}
					cycle := append([]string{}, path[cycleStart:]...)
					cycle = append(cycle, nextState)
					*cycles = append(*cycles, cycle)
				}
			}
		}
	}

	recStack[stateID] = false
}

// getTopTransitions returns the most used transitions
func (s *WorkflowService) getTopTransitions(counts map[string]int64, limit int) []TransitionUsage {
	var usage []TransitionUsage
	total := int64(0)

	for _, count := range counts {
		total += count
	}

	for id, count := range counts {
		usage = append(usage, TransitionUsage{
			TransitionID: id,
			Count:        count,
			Percentage:   float64(count) / float64(total) * 100,
		})
	}

	// Sort by count
	sort.Slice(usage, func(i, j int) bool {
		return usage[i].Count > usage[j].Count
	})

	if len(usage) > limit {
		usage = usage[:limit]
	}

	return usage
}

// Validation Rules

// StateValidationRule validates workflow states
type StateValidationRule struct{}

func (r *StateValidationRule) Validate(ctx context.Context, workflow *jira.Workflow) error {
	if len(workflow.Statuses) == 0 {
		return fmt.Errorf("workflow must have at least one status")
	}

	// Check for initial state
	hasInitial := false
	for _, status := range workflow.Statuses {
		if status.StatusCategory.Key == "new" {
			hasInitial = true
			break
		}
	}

	if !hasInitial {
		return fmt.Errorf("workflow must have at least one initial state (category: new)")
	}

	return nil
}

// TransitionValidationRule validates workflow transitions
type TransitionValidationRule struct{}

func (r *TransitionValidationRule) Validate(ctx context.Context, workflow *jira.Workflow) error {
	if len(workflow.Transitions) == 0 {
		return fmt.Errorf("workflow must have at least one transition")
	}

	// Check for orphaned states
	connectedStates := make(map[string]bool)
	for _, transition := range workflow.Transitions {
		for _, from := range transition.From {
			connectedStates[from.ID] = true
		}
		connectedStates[transition.To.ID] = true
	}

	for _, status := range workflow.Statuses {
		if !connectedStates[status.ID] {
			log.Warn().Str("status", status.Name).Msg("Orphaned status found in workflow")
		}
	}

	return nil
}

// CycleDetectionRule detects problematic cycles
type CycleDetectionRule struct{}

func (r *CycleDetectionRule) Validate(ctx context.Context, workflow *jira.Workflow) error {
	// This is informational only - cycles are allowed in workflows
	return nil
}

// ValidateTransition validates a specific transition
func (v *WorkflowValidator) ValidateTransition(ctx context.Context, workflow *jira.Workflow, issue *jira.Issue, transition *jira.Transition) error {
	// Add custom validation logic here
	// For example, check user permissions, field requirements, etc.
	
	// Check if transition name contains restricted keywords
	restrictedKeywords := []string{"DELETE", "REMOVE", "DESTROY"}
	transitionName := strings.ToUpper(transition.Name)
	
	for _, keyword := range restrictedKeywords {
		if strings.Contains(transitionName, keyword) {
			// Additional validation for destructive transitions
			log.Warn().
				Str("transition", transition.Name).
				Str("issue", issue.Key).
				Msg("Destructive transition requested")
		}
	}

	return nil
}

// Helper functions

func generateExecutionID() string {
	return fmt.Sprintf("exec_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
}

func copyMap(m map[string]int64) map[string]int64 {
	copy := make(map[string]int64)
	for k, v := range m {
		copy[k] = v
	}
	return copy
}