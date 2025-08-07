package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/ericfisherdev/GoJira/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflowService tests the workflow service functionality
func TestWorkflowService(t *testing.T) {
	mockClient := NewMockWorkflowServiceClient()
	service := services.NewWorkflowService(mockClient)
	ctx := context.Background()

	// Setup test data
	testWorkflow := createTestWorkflow()
	mockClient.workflows["Test Workflow"] = testWorkflow

	t.Run("Get Workflow with Cache", func(t *testing.T) {
		// First call - should fetch from client
		workflow1, err := service.GetWorkflowWithCache(ctx, "Test Workflow")
		require.NoError(t, err)
		assert.Equal(t, "Test Workflow", workflow1.Name)

		// Second call - should use cache
		workflow2, err := service.GetWorkflowWithCache(ctx, "Test Workflow")
		require.NoError(t, err)
		assert.Equal(t, workflow1, workflow2)
	})

	t.Run("Get State Machine", func(t *testing.T) {
		stateMachine, err := service.GetStateMachine(ctx, "Test Workflow")
		require.NoError(t, err)
		assert.NotNil(t, stateMachine)
		assert.Equal(t, "Test Workflow", stateMachine.WorkflowName)
		assert.Equal(t, "1", stateMachine.InitialState)
		assert.Contains(t, stateMachine.FinalStates, "3")
	})

	t.Run("Validate Transition - Valid", func(t *testing.T) {
		// Setup mock transitions
		mockClient.transitions["TEST-123"] = []jira.Transition{
			{ID: "11", Name: "Start Progress"},
			{ID: "21", Name: "Done"},
		}

		result, err := service.ValidateTransition(ctx, "TEST-123", "11")
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("Validate Transition - Invalid", func(t *testing.T) {
		result, err := service.ValidateTransition(ctx, "TEST-123", "99")
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "TRANSITION_NOT_AVAILABLE", result.Errors[0].Type)
	})

	t.Run("Execute Transition", func(t *testing.T) {
		req := &services.TransitionRequest{
			IssueKey:     "TEST-123",
			TransitionID: "11",
			Comment:      "Starting work",
		}

		result, err := service.ExecuteTransition(ctx, req)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, "TEST-123", result.IssueKey)
		assert.Equal(t, "11", result.TransitionID)
	})

	t.Run("Transition with Validation Only", func(t *testing.T) {
		// Reset execution count
		mockClient.executionCount = 0
		
		req := &services.TransitionRequest{
			IssueKey:     "TEST-123",
			TransitionID: "11",
			ValidateOnly: true,
		}

		result, err := service.ExecuteTransition(ctx, req)
		require.NoError(t, err)
		assert.True(t, result.Success)
		// Should not actually execute
		assert.Equal(t, 0, mockClient.executionCount)
	})

	t.Run("Get Transition Metrics", func(t *testing.T) {
		// Execute some transitions to generate metrics
		for i := 0; i < 5; i++ {
			req := &services.TransitionRequest{
				IssueKey:     fmt.Sprintf("TEST-%d", i),
				TransitionID: "11",
			}
			service.ExecuteTransition(ctx, req)
		}

		metrics := service.GetTransitionMetrics()
		assert.Greater(t, metrics.TotalTransitions, int64(0))
		assert.Greater(t, metrics.SuccessCount, int64(0))
	})
}

// TestWorkflowAnalytics tests workflow analytics functionality
func TestWorkflowAnalytics(t *testing.T) {
	mockClient := NewMockWorkflowServiceClient()
	service := services.NewWorkflowService(mockClient)
	ctx := context.Background()

	// Setup complex workflow for analytics
	complexWorkflow := createComplexWorkflow()
	mockClient.workflows["Complex Workflow"] = complexWorkflow

	t.Run("Generate Analytics", func(t *testing.T) {
		analytics, err := service.GetWorkflowAnalytics(ctx, "Complex Workflow", 30)
		require.NoError(t, err)
		assert.Equal(t, "Complex Workflow", analytics.WorkflowName)
		assert.Equal(t, 5, analytics.TotalStates)
		assert.Equal(t, 7, analytics.TotalTransitions)
		assert.Greater(t, analytics.Complexity, 1.0)
	})

	t.Run("Detect Bottlenecks", func(t *testing.T) {
		analytics, err := service.GetWorkflowAnalytics(ctx, "Complex Workflow", 30)
		require.NoError(t, err)
		// In Review state should be a bottleneck (multiple incoming transitions)
		assert.Contains(t, analytics.Bottlenecks, "3")
	})
}

// TestTransitionHooks tests transition hook functionality
func TestTransitionHooks(t *testing.T) {
	mockClient := NewMockWorkflowServiceClient()
	service := services.NewWorkflowService(mockClient)
	ctx := context.Background()

	// Setup test workflow
	mockClient.workflows["Test Workflow"] = createTestWorkflow()
	mockClient.transitions["TEST-456"] = []jira.Transition{
		{ID: "11", Name: "Start Progress"},
	}

	t.Run("Pre-Transition Hook", func(t *testing.T) {
		hookCalled := false
		hook := &TestTransitionHook{
			preHook: func(ctx context.Context, req *services.TransitionRequest) error {
				hookCalled = true
				assert.Equal(t, "TEST-456", req.IssueKey)
				return nil
			},
		}

		service.AddTransitionHook(hook)

		req := &services.TransitionRequest{
			IssueKey:     "TEST-456",
			TransitionID: "11",
		}

		result, err := service.ExecuteTransition(ctx, req)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.True(t, hookCalled)
	})

	t.Run("Hook Prevents Transition", func(t *testing.T) {
		hook := &TestTransitionHook{
			preHook: func(ctx context.Context, req *services.TransitionRequest) error {
				return fmt.Errorf("transition blocked by hook")
			},
		}

		service2 := services.NewWorkflowService(mockClient)
		service2.AddTransitionHook(hook)

		req := &services.TransitionRequest{
			IssueKey:     "TEST-789",
			TransitionID: "11",
		}

		result, err := service2.ExecuteTransition(ctx, req)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Equal(t, "PRE_HOOK_ERROR", result.Errors[0].Type)
	})
}

// TestWorkflowCaching tests caching functionality
func TestWorkflowCaching(t *testing.T) {
	mockClient := NewMockWorkflowServiceClient()
	service := services.NewWorkflowService(mockClient)
	ctx := context.Background()

	mockClient.workflows["Cached Workflow"] = createTestWorkflow()

	t.Run("Cache Hit", func(t *testing.T) {
		// First call
		_, err := service.GetWorkflowWithCache(ctx, "Cached Workflow")
		require.NoError(t, err)
		firstCallCount := mockClient.getWorkflowCallCount

		// Second call should use cache
		_, err = service.GetWorkflowWithCache(ctx, "Cached Workflow")
		require.NoError(t, err)
		assert.Equal(t, firstCallCount, mockClient.getWorkflowCallCount)
	})

	t.Run("State Machine Caching", func(t *testing.T) {
		// First call
		_, err := service.GetStateMachine(ctx, "Cached Workflow")
		require.NoError(t, err)
		firstBuildCount := mockClient.buildStateMachineCallCount

		// Second call should use cache
		_, err = service.GetStateMachine(ctx, "Cached Workflow")
		require.NoError(t, err)
		assert.Equal(t, firstBuildCount, mockClient.buildStateMachineCallCount)
	})
}

// Helper functions to create test data

func createTestWorkflow() *jira.Workflow {
	return &jira.Workflow{
		ID:   "1",
		Name: "Test Workflow",
		Statuses: []jira.WorkflowStatus{
			{
				ID:   "1",
				Name: "To Do",
				StatusCategory: jira.WorkflowStatusCategory{
					Key: "new",
				},
			},
			{
				ID:   "2",
				Name: "In Progress",
				StatusCategory: jira.WorkflowStatusCategory{
					Key: "indeterminate",
				},
			},
			{
				ID:   "3",
				Name: "Done",
				StatusCategory: jira.WorkflowStatusCategory{
					Key: "done",
				},
			},
		},
		Transitions: []jira.WorkflowTransition{
			{
				ID:   "11",
				Name: "Start Progress",
				From: []jira.WorkflowTransitionEndpoint{
					{ID: "1", Name: "To Do"},
				},
				To: jira.WorkflowTransitionEndpoint{
					ID: "2", Name: "In Progress",
				},
			},
			{
				ID:   "21",
				Name: "Done",
				From: []jira.WorkflowTransitionEndpoint{
					{ID: "2", Name: "In Progress"},
				},
				To: jira.WorkflowTransitionEndpoint{
					ID: "3", Name: "Done",
				},
			},
		},
	}
}

func createComplexWorkflow() *jira.Workflow {
	return &jira.Workflow{
		ID:   "2",
		Name: "Complex Workflow",
		Statuses: []jira.WorkflowStatus{
			{ID: "1", Name: "To Do", StatusCategory: jira.WorkflowStatusCategory{Key: "new"}},
			{ID: "2", Name: "In Progress", StatusCategory: jira.WorkflowStatusCategory{Key: "indeterminate"}},
			{ID: "3", Name: "In Review", StatusCategory: jira.WorkflowStatusCategory{Key: "indeterminate"}},
			{ID: "4", Name: "Testing", StatusCategory: jira.WorkflowStatusCategory{Key: "indeterminate"}},
			{ID: "5", Name: "Done", StatusCategory: jira.WorkflowStatusCategory{Key: "done"}},
		},
		Transitions: []jira.WorkflowTransition{
			// From To Do
			{ID: "11", Name: "Start", From: []jira.WorkflowTransitionEndpoint{{ID: "1"}}, To: jira.WorkflowTransitionEndpoint{ID: "2"}},
			// From In Progress
			{ID: "21", Name: "Send to Review", From: []jira.WorkflowTransitionEndpoint{{ID: "2"}}, To: jira.WorkflowTransitionEndpoint{ID: "3"}},
			{ID: "22", Name: "Back to Todo", From: []jira.WorkflowTransitionEndpoint{{ID: "2"}}, To: jira.WorkflowTransitionEndpoint{ID: "1"}},
			// From In Review (bottleneck - multiple incoming)
			{ID: "31", Name: "Approve", From: []jira.WorkflowTransitionEndpoint{{ID: "3"}}, To: jira.WorkflowTransitionEndpoint{ID: "4"}},
			{ID: "32", Name: "Reject", From: []jira.WorkflowTransitionEndpoint{{ID: "3"}}, To: jira.WorkflowTransitionEndpoint{ID: "2"}},
			// From Testing
			{ID: "41", Name: "Pass", From: []jira.WorkflowTransitionEndpoint{{ID: "4"}}, To: jira.WorkflowTransitionEndpoint{ID: "5"}},
			{ID: "42", Name: "Fail", From: []jira.WorkflowTransitionEndpoint{{ID: "4"}}, To: jira.WorkflowTransitionEndpoint{ID: "3"}},
		},
	}
}

// Test helpers

// TestTransitionHook is a test implementation of TransitionHook
type TestTransitionHook struct {
	preHook  func(ctx context.Context, req *services.TransitionRequest) error
	postHook func(ctx context.Context, req *services.TransitionRequest, result *jira.WorkflowExecutionResult) error
}

func (h *TestTransitionHook) PreTransition(ctx context.Context, req *services.TransitionRequest) error {
	if h.preHook != nil {
		return h.preHook(ctx, req)
	}
	return nil
}

func (h *TestTransitionHook) PostTransition(ctx context.Context, req *services.TransitionRequest, result *jira.WorkflowExecutionResult) error {
	if h.postHook != nil {
		return h.postHook(ctx, req, result)
	}
	return nil
}

// NewMockWorkflowServiceClient creates a mock workflow client for testing
func NewMockWorkflowServiceClient() *MockWorkflowServiceClient {
	return &MockWorkflowServiceClient{
		workflows:                  make(map[string]*jira.Workflow),
		transitions:                make(map[string][]jira.Transition),
		issues:                     make(map[string]*jira.Issue),
		getWorkflowCallCount:       0,
		buildStateMachineCallCount: 0,
		executionCount:             0,
	}
}

// MockWorkflowServiceClient implements jira.ClientInterface for workflow testing
type MockWorkflowServiceClient struct {
	workflows                  map[string]*jira.Workflow
	transitions                map[string][]jira.Transition
	issues                     map[string]*jira.Issue
	getWorkflowCallCount       int
	buildStateMachineCallCount int
	executionCount             int
}

func (m *MockWorkflowServiceClient) GetWorkflow(workflowName string) (*jira.Workflow, error) {
	m.getWorkflowCallCount++
	if workflow, exists := m.workflows[workflowName]; exists {
		return workflow, nil
	}
	return nil, fmt.Errorf("workflow not found")
}

func (m *MockWorkflowServiceClient) GetIssue(ctx context.Context, issueKey string, expand []string) (*jira.Issue, error) {
	if issue, exists := m.issues[issueKey]; exists {
		return issue, nil
	}
	// Return a default issue
	return &jira.Issue{
		Key: issueKey,
		Fields: jira.IssueFields{
			Summary: "Test Issue",
		},
	}, nil
}

func (m *MockWorkflowServiceClient) GetTransitions(issueKey string) ([]jira.Transition, error) {
	if transitions, exists := m.transitions[issueKey]; exists {
		return transitions, nil
	}
	return []jira.Transition{}, nil
}

func (m *MockWorkflowServiceClient) GetIssueWorkflow(issueKey string) (*jira.Workflow, error) {
	// Return the first workflow for simplicity
	for _, workflow := range m.workflows {
		return workflow, nil
	}
	return nil, fmt.Errorf("no workflow found")
}

func (m *MockWorkflowServiceClient) BuildWorkflowStateMachine(workflow *jira.Workflow) (*jira.WorkflowStateMachine, error) {
	m.buildStateMachineCallCount++
	if workflow == nil {
		return nil, fmt.Errorf("workflow cannot be nil")
	}

	sm := &jira.WorkflowStateMachine{
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		States:       make(map[string]*jira.WorkflowState),
		Transitions:  make(map[string]*jira.WorkflowTransitionSM),
	}

	// Build states
	for _, status := range workflow.Statuses {
		state := &jira.WorkflowState{
			ID:       status.ID,
			Name:     status.Name,
			Category: status.StatusCategory.Key,
			Type:     "intermediate",
		}

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
		smTransition := &jira.WorkflowTransitionSM{
			ID:      transition.ID,
			Name:    transition.Name,
			ToState: transition.To.ID,
		}

		for _, from := range transition.From {
			smTransition.FromStates = append(smTransition.FromStates, from.ID)
		}

		sm.Transitions[transition.ID] = smTransition
	}

	return sm, nil
}

func (m *MockWorkflowServiceClient) ValidateTransition(issueKey, transitionID string) (*jira.WorkflowExecutionResult, error) {
	transitions, _ := m.GetTransitions(issueKey)
	
	for _, t := range transitions {
		if t.ID == transitionID {
			return &jira.WorkflowExecutionResult{
				Success:      true,
				IssueKey:     issueKey,
				TransitionID: transitionID,
				ExecutedAt:   time.Now(),
			}, nil
		}
	}

	return &jira.WorkflowExecutionResult{
		Success:      false,
		IssueKey:     issueKey,
		TransitionID: transitionID,
		ExecutedAt:   time.Now(),
		Errors: []jira.WorkflowError{
			{
				Type:      "TRANSITION_NOT_AVAILABLE",
				Message:   "Transition not found",
				Timestamp: time.Now(),
			},
		},
	}, nil
}

func (m *MockWorkflowServiceClient) ExecuteTransition(ctx *jira.WorkflowExecutionContext) (*jira.WorkflowExecutionResult, error) {
	m.executionCount++
	return &jira.WorkflowExecutionResult{
		Success:      true,
		ExecutionID:  ctx.ExecutionID,
		IssueKey:     ctx.IssueKey,
		TransitionID: ctx.TransitionID,
		ExecutedAt:   ctx.ExecutedAt,
	}, nil
}

// Implement remaining interface methods with minimal implementations
func (m *MockWorkflowServiceClient) GetSprints(boardID int) (*jira.SprintList, error) { return nil, nil }
func (m *MockWorkflowServiceClient) GetSprint(sprintID int) (*jira.Sprint, error) { return nil, nil }
func (m *MockWorkflowServiceClient) CreateSprint(req *jira.CreateSprintRequest) (*jira.Sprint, error) {
	return nil, nil
}
func (m *MockWorkflowServiceClient) UpdateSprint(sprintID int, req *jira.UpdateSprintRequest) (*jira.Sprint, error) {
	return nil, nil
}
func (m *MockWorkflowServiceClient) StartSprint(sprintID int, startDate, endDate time.Time) error { return nil }
func (m *MockWorkflowServiceClient) CloseSprint(sprintID int) error { return nil }
func (m *MockWorkflowServiceClient) GetSprintIssues(sprintID int) (*jira.SprintIssueList, error) { return nil, nil }
func (m *MockWorkflowServiceClient) MoveIssuesToSprint(sprintID int, issueKeys []string) error { return nil }
func (m *MockWorkflowServiceClient) GetSprintReport(sprintID int) (*jira.SprintReport, error) { return nil, nil }
func (m *MockWorkflowServiceClient) GetBoards() (*jira.BoardList, error) { return nil, nil }
func (m *MockWorkflowServiceClient) GetBoard(boardID int) (*jira.Board, error) { return nil, nil }
func (m *MockWorkflowServiceClient) GetBoardConfiguration(boardID int) (*jira.BoardConfiguration, error) {
	return nil, nil
}
func (m *MockWorkflowServiceClient) GetBoardIssues(boardID int) (*jira.BoardIssueList, error) { return nil, nil }
func (m *MockWorkflowServiceClient) GetBoardBacklog(boardID int) (*jira.BoardIssueList, error) { return nil, nil }
func (m *MockWorkflowServiceClient) GetBoardSprints(boardID int) (*jira.SprintList, error) { return nil, nil }
func (m *MockWorkflowServiceClient) MoveIssuesToBacklog(issueKeys []string) error { return nil }
func (m *MockWorkflowServiceClient) MoveIssuesToBoard(boardID int, issueKeys []string, position string) error {
	return nil
}
func (m *MockWorkflowServiceClient) TransitionIssueAdvanced(issueKey string, transitionID string, fields map[string]interface{}, comment string) error {
	return nil
}
func (m *MockWorkflowServiceClient) GetWorkflows() (*jira.WorkflowList, error) { return nil, nil }
func (m *MockWorkflowServiceClient) GetWorkflowSchemes() ([]jira.WorkflowScheme, error) { return nil, nil }
func (m *MockWorkflowServiceClient) GetProjectWorkflowScheme(projectKey string) (*jira.WorkflowScheme, error) {
	return nil, nil
}