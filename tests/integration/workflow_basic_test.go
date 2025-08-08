package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflowStateMachineBuilding tests the workflow state machine building functionality
func TestWorkflowStateMachineBuilding(t *testing.T) {
	// Create a mock workflow
	workflow := &jira.Workflow{
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

	// Create a mock client
	mockClient := &MockWorkflowClient{
		workflows: map[string]*jira.Workflow{
			"Test Workflow": workflow,
		},
	}

	// Build state machine
	stateMachine, err := mockClient.BuildWorkflowStateMachine(workflow)

	require.NoError(t, err)
	assert.NotNil(t, stateMachine)
	assert.Equal(t, workflow.ID, stateMachine.WorkflowID)
	assert.Equal(t, workflow.Name, stateMachine.WorkflowName)
	assert.Equal(t, "1", stateMachine.InitialState)
	assert.Contains(t, stateMachine.FinalStates, "3")
	assert.Len(t, stateMachine.States, 3)
	assert.Len(t, stateMachine.Transitions, 2)

	// Verify states
	todoState := stateMachine.States["1"]
	require.NotNil(t, todoState)
	assert.Equal(t, "initial", todoState.Type)
	assert.Equal(t, "new", todoState.Category)

	doneState := stateMachine.States["3"]
	require.NotNil(t, doneState)
	assert.Equal(t, "final", doneState.Type)
	assert.Equal(t, "done", doneState.Category)
}

// TestWorkflowValidation tests basic workflow validation
func TestWorkflowValidation(t *testing.T) {
	mockClient := &MockWorkflowClient{
		transitions: map[string][]jira.Transition{
			"TEST-123": {
				{ID: "11", Name: "Start Progress"},
				{ID: "21", Name: "Done"},
			},
		},
	}

	// Valid transition
	result, err := mockClient.ValidateTransition("TEST-123", "11")
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "TEST-123", result.IssueKey)
	assert.Equal(t, "11", result.TransitionID)

	// Invalid transition
	result, err = mockClient.ValidateTransition("TEST-123", "99")
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "TRANSITION_NOT_AVAILABLE", result.Errors[0].Type)
}

// MockWorkflowClient implements jira.ClientInterface for testing
type MockWorkflowClient struct {
	workflows   map[string]*jira.Workflow
	transitions map[string][]jira.Transition
}

func (m *MockWorkflowClient) BuildWorkflowStateMachine(workflow *jira.Workflow) (*jira.WorkflowStateMachine, error) {
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
		smTransition := &jira.WorkflowTransitionSM{
			ID:      transition.ID,
			Name:    transition.Name,
			ToState: transition.To.ID,
		}

		// Handle from states
		for _, from := range transition.From {
			smTransition.FromStates = append(smTransition.FromStates, from.ID)
		}

		sm.Transitions[transition.ID] = smTransition
	}

	return sm, nil
}

func (m *MockWorkflowClient) ValidateTransition(issueKey, transitionID string) (*jira.WorkflowExecutionResult, error) {
	result := &jira.WorkflowExecutionResult{
		IssueKey:     issueKey,
		TransitionID: transitionID,
		ExecutedAt:   time.Now(),
	}

	// Check if transition is available
	transitions, exists := m.transitions[issueKey]
	if !exists {
		result.Success = false
		result.Errors = []jira.WorkflowError{
			{
				Type:      "ISSUE_NOT_FOUND",
				Message:   fmt.Sprintf("Issue '%s' not found", issueKey),
				Timestamp: time.Now(),
			},
		}
		return result, nil
	}

	for _, t := range transitions {
		if t.ID == transitionID {
			result.Success = true
			return result, nil
		}
	}

	// Transition not found
	result.Success = false
	result.Errors = []jira.WorkflowError{
		{
			Type:      "TRANSITION_NOT_AVAILABLE",
			Message:   fmt.Sprintf("Transition '%s' is not available for issue '%s'", transitionID, issueKey),
			Timestamp: time.Now(),
		},
	}

	return result, nil
}

// Placeholder implementations for interface compliance
func (m *MockWorkflowClient) GetSprints(boardID int) (*jira.SprintList, error) { return nil, nil }
func (m *MockWorkflowClient) GetSprint(sprintID int) (*jira.Sprint, error)     { return nil, nil }
func (m *MockWorkflowClient) CreateSprint(req *jira.CreateSprintRequest) (*jira.Sprint, error) {
	return nil, nil
}
func (m *MockWorkflowClient) UpdateSprint(sprintID int, req *jira.UpdateSprintRequest) (*jira.Sprint, error) {
	return nil, nil
}
func (m *MockWorkflowClient) StartSprint(sprintID int, startDate, endDate time.Time) error {
	return nil
}
func (m *MockWorkflowClient) CloseSprint(sprintID int) error                   { return nil }
func (m *MockWorkflowClient) GetSprintIssues(sprintID int) (*jira.SprintIssueList, error) {
	return nil, nil
}
func (m *MockWorkflowClient) MoveIssuesToSprint(sprintID int, issueKeys []string) error {
	return nil
}
func (m *MockWorkflowClient) GetSprintReport(sprintID int) (*jira.SprintReport, error) {
	return nil, nil
}
func (m *MockWorkflowClient) GetBoards() (*jira.BoardList, error)   { return nil, nil }
func (m *MockWorkflowClient) GetBoard(boardID int) (*jira.Board, error) { return nil, nil }
func (m *MockWorkflowClient) GetBoardConfiguration(boardID int) (*jira.BoardConfiguration, error) {
	return nil, nil
}
func (m *MockWorkflowClient) GetBoardIssues(boardID int) (*jira.BoardIssueList, error) {
	return nil, nil
}
func (m *MockWorkflowClient) GetBoardBacklog(boardID int) (*jira.BoardIssueList, error) {
	return nil, nil
}
func (m *MockWorkflowClient) GetBoardSprints(boardID int) (*jira.SprintList, error) { return nil, nil }
func (m *MockWorkflowClient) MoveIssuesToBacklog(issueKeys []string) error           { return nil }
func (m *MockWorkflowClient) MoveIssuesToBoard(boardID int, issueKeys []string, position string) error {
	return nil
}
func (m *MockWorkflowClient) GetIssue(ctx context.Context, issueKey string, expand []string) (*jira.Issue, error) {
	return nil, nil
}
func (m *MockWorkflowClient) GetTransitions(issueKey string) ([]jira.Transition, error) {
	if transitions, exists := m.transitions[issueKey]; exists {
		return transitions, nil
	}
	return nil, fmt.Errorf("issue not found")
}
func (m *MockWorkflowClient) TransitionIssueAdvanced(issueKey string, transitionID string, fields map[string]interface{}, comment string) error {
	return nil
}
func (m *MockWorkflowClient) GetWorkflows() (*jira.WorkflowList, error) { return nil, nil }
func (m *MockWorkflowClient) GetWorkflow(workflowName string) (*jira.Workflow, error) {
	if workflow, exists := m.workflows[workflowName]; exists {
		return workflow, nil
	}
	return nil, fmt.Errorf("workflow not found")
}
func (m *MockWorkflowClient) GetWorkflowSchemes() ([]jira.WorkflowScheme, error) { return nil, nil }
func (m *MockWorkflowClient) GetProjectWorkflowScheme(projectKey string) (*jira.WorkflowScheme, error) {
	return nil, nil
}
func (m *MockWorkflowClient) GetIssueWorkflow(issueKey string) (*jira.Workflow, error) {
	return nil, nil
}
func (m *MockWorkflowClient) ExecuteTransition(ctx *jira.WorkflowExecutionContext) (*jira.WorkflowExecutionResult, error) {
	return nil, nil
}