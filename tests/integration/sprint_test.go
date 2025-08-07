package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/api/routes"
	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/ericfisherdev/GoJira/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockJiraClient provides a mock implementation for testing
type MockJiraClient struct {
	sprints      map[int]*jira.Sprint
	boards       map[int]*jira.Board
	sprintIssues map[int][]jira.SprintIssue
	nextID       int
}

func NewMockJiraClient() *MockJiraClient {
	return &MockJiraClient{
		sprints:      make(map[int]*jira.Sprint),
		boards:       make(map[int]*jira.Board),
		sprintIssues: make(map[int][]jira.SprintIssue),
		nextID:       1000,
	}
}

func (m *MockJiraClient) GetSprints(boardID int) (*jira.SprintList, error) {
	var sprints []jira.Sprint
	for _, sprint := range m.sprints {
		if sprint.OriginBoardID == boardID {
			sprints = append(sprints, *sprint)
		}
	}
	
	return &jira.SprintList{
		Values:     sprints,
		MaxResults: len(sprints),
		IsLast:     true,
	}, nil
}

func (m *MockJiraClient) GetSprint(sprintID int) (*jira.Sprint, error) {
	sprint, exists := m.sprints[sprintID]
	if !exists {
		return nil, fmt.Errorf("sprint %d not found", sprintID)
	}
	return sprint, nil
}

func (m *MockJiraClient) CreateSprint(req *jira.CreateSprintRequest) (*jira.Sprint, error) {
	m.nextID++
	sprint := &jira.Sprint{
		ID:            m.nextID,
		Name:          req.Name,
		State:         "future",
		Goal:          req.Goal,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		OriginBoardID: req.OriginBoardID,
	}
	m.sprints[sprint.ID] = sprint
	return sprint, nil
}

func (m *MockJiraClient) UpdateSprint(sprintID int, req *jira.UpdateSprintRequest) (*jira.Sprint, error) {
	sprint, exists := m.sprints[sprintID]
	if !exists {
		return nil, fmt.Errorf("sprint %d not found", sprintID)
	}
	
	if req.Name != "" {
		sprint.Name = req.Name
	}
	if req.Goal != "" {
		sprint.Goal = req.Goal
	}
	if req.State != "" {
		sprint.State = req.State
	}
	if req.StartDate != nil {
		sprint.StartDate = req.StartDate
	}
	if req.EndDate != nil {
		sprint.EndDate = req.EndDate
	}
	
	return sprint, nil
}

func (m *MockJiraClient) StartSprint(sprintID int, startDate, endDate time.Time) error {
	sprint, exists := m.sprints[sprintID]
	if !exists {
		return fmt.Errorf("sprint %d not found", sprintID)
	}
	
	sprint.State = "active"
	sprint.StartDate = &startDate
	sprint.EndDate = &endDate
	return nil
}

func (m *MockJiraClient) CloseSprint(sprintID int) error {
	sprint, exists := m.sprints[sprintID]
	if !exists {
		return fmt.Errorf("sprint %d not found", sprintID)
	}
	
	sprint.State = "closed"
	now := time.Now()
	sprint.CompleteDate = &now
	return nil
}

func (m *MockJiraClient) GetSprintIssues(sprintID int) (*jira.SprintIssueList, error) {
	issues, exists := m.sprintIssues[sprintID]
	if !exists {
		issues = []jira.SprintIssue{}
	}
	
	return &jira.SprintIssueList{
		Issues: issues,
		Total:  len(issues),
	}, nil
}

func (m *MockJiraClient) MoveIssuesToSprint(sprintID int, issueKeys []string) error {
	if _, exists := m.sprints[sprintID]; !exists {
		return fmt.Errorf("sprint %d not found", sprintID)
	}
	
	// Add mock issues to sprint
	for _, key := range issueKeys {
		issue := jira.SprintIssue{
			ID:  key,
			Key: key,
		}
		issue.Fields.Summary = fmt.Sprintf("Issue %s", key)
		issue.Fields.Status = jira.Status{
			Name: "To Do",
			StatusCategory: jira.StatusCategory{
				Key: "new",
			},
		}
		m.sprintIssues[sprintID] = append(m.sprintIssues[sprintID], issue)
	}
	
	return nil
}

func (m *MockJiraClient) GetBoards() (*jira.BoardList, error) {
	var boards []jira.Board
	for _, board := range m.boards {
		boards = append(boards, *board)
	}
	
	return &jira.BoardList{
		Values: boards,
		IsLast: true,
	}, nil
}

func (m *MockJiraClient) GetBoard(boardID int) (*jira.Board, error) {
	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %d not found", boardID)
	}
	return board, nil
}

func (m *MockJiraClient) GetBoardConfiguration(boardID int) (*jira.BoardConfiguration, error) {
	return &jira.BoardConfiguration{ID: boardID}, nil
}

func (m *MockJiraClient) GetBoardIssues(boardID int) (*jira.BoardIssueList, error) {
	return &jira.BoardIssueList{}, nil
}

func (m *MockJiraClient) GetBoardBacklog(boardID int) (*jira.BoardIssueList, error) {
	return &jira.BoardIssueList{}, nil
}

func (m *MockJiraClient) GetBoardSprints(boardID int) (*jira.SprintList, error) {
	return m.GetSprints(boardID)
}

func (m *MockJiraClient) MoveIssuesToBoard(boardID int, issueKeys []string, position string) error {
	return nil
}

func (m *MockJiraClient) MoveIssuesToBacklog(issueKeys []string) error {
	// Mock implementation
	return nil
}

// Add missing workflow-related methods for interface compliance
func (m *MockJiraClient) GetIssue(ctx context.Context, issueKey string, expand []string) (*jira.Issue, error) {
	return &jira.Issue{Key: issueKey}, nil
}

func (m *MockJiraClient) GetTransitions(issueKey string) ([]jira.Transition, error) {
	return []jira.Transition{{ID: "1", Name: "Test Transition"}}, nil
}

func (m *MockJiraClient) TransitionIssueAdvanced(issueKey string, transitionID string, fields map[string]interface{}, comment string) error {
	return nil
}

func (m *MockJiraClient) GetWorkflows() (*jira.WorkflowList, error) {
	return &jira.WorkflowList{}, nil
}

func (m *MockJiraClient) GetWorkflow(workflowName string) (*jira.Workflow, error) {
	return &jira.Workflow{Name: workflowName}, nil
}

func (m *MockJiraClient) GetWorkflowSchemes() ([]jira.WorkflowScheme, error) {
	return []jira.WorkflowScheme{}, nil
}

func (m *MockJiraClient) GetProjectWorkflowScheme(projectKey string) (*jira.WorkflowScheme, error) {
	return &jira.WorkflowScheme{}, nil
}

func (m *MockJiraClient) GetIssueWorkflow(issueKey string) (*jira.Workflow, error) {
	return &jira.Workflow{}, nil
}

func (m *MockJiraClient) BuildWorkflowStateMachine(workflow *jira.Workflow) (*jira.WorkflowStateMachine, error) {
	if workflow == nil {
		return nil, fmt.Errorf("workflow cannot be nil")
	}
	return &jira.WorkflowStateMachine{
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		States:       make(map[string]*jira.WorkflowState),
		Transitions:  make(map[string]*jira.WorkflowTransitionSM),
	}, nil
}

func (m *MockJiraClient) ValidateTransition(issueKey, transitionID string) (*jira.WorkflowExecutionResult, error) {
	return &jira.WorkflowExecutionResult{
		Success:      true,
		IssueKey:     issueKey,
		TransitionID: transitionID,
		ExecutedAt:   time.Now(),
	}, nil
}

func (m *MockJiraClient) ExecuteTransition(ctx *jira.WorkflowExecutionContext) (*jira.WorkflowExecutionResult, error) {
	return &jira.WorkflowExecutionResult{
		Success:      true,
		IssueKey:     ctx.IssueKey,
		TransitionID: ctx.TransitionID,
		ExecutedAt:   time.Now(),
	}, nil
}

func (m *MockJiraClient) GetSprintReport(sprintID int) (*jira.SprintReport, error) {
	return nil, fmt.Errorf("sprint report not implemented in mock")
}

// Test Sprint CRUD operations - DISABLED pending context setup
func TestSprintCRUD_DISABLED(t *testing.T) {
	t.Skip("Skipping until context middleware is implemented")
	// Setup
	mockClient := NewMockJiraClient()
	// handlers.SetJiraClient(mockClient) // This method doesn't exist yet
	
	router := chi.NewRouter()
	routes.SetupRoutes(router)
	
	// Create a board first
	mockClient.boards[1] = &jira.Board{
		ID:   1,
		Name: "Test Board",
		Type: "scrum",
	}
	
	t.Run("Create Sprint", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":          "Sprint 1",
			"goal":          "Complete user stories",
			"originBoardId": 1,
		}
		
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/sprints", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusCreated, w.Code)
		
		var response jira.Sprint
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Sprint 1", response.Name)
		assert.Equal(t, "future", response.State)
	})
	
	t.Run("Get Sprint", func(t *testing.T) {
		// Create a sprint first
		sprint := &jira.Sprint{
			ID:            100,
			Name:          "Test Sprint",
			State:         "active",
			OriginBoardID: 1,
		}
		mockClient.sprints[100] = sprint
		
		req := httptest.NewRequest("GET", "/api/v1/sprints/100", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response jira.Sprint
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Test Sprint", response.Name)
		assert.Equal(t, "active", response.State)
	})
	
	t.Run("Update Sprint", func(t *testing.T) {
		// Create a sprint first
		sprint := &jira.Sprint{
			ID:            101,
			Name:          "Original Sprint",
			State:         "future",
			OriginBoardID: 1,
		}
		mockClient.sprints[101] = sprint
		
		payload := map[string]interface{}{
			"name": "Updated Sprint",
			"goal": "New goal",
		}
		
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PUT", "/api/v1/sprints/101", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response jira.Sprint
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Updated Sprint", response.Name)
		assert.Equal(t, "New goal", response.Goal)
	})
	
	t.Run("Start Sprint", func(t *testing.T) {
		// Create a future sprint
		sprint := &jira.Sprint{
			ID:            102,
			Name:          "Future Sprint",
			State:         "future",
			OriginBoardID: 1,
		}
		mockClient.sprints[102] = sprint
		
		startDate := time.Now()
		endDate := startDate.AddDate(0, 0, 14)
		
		payload := map[string]interface{}{
			"startDate": startDate.Format(time.RFC3339),
			"endDate":   endDate.Format(time.RFC3339),
		}
		
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/sprints/102/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		// Verify sprint state changed
		updatedSprint := mockClient.sprints[102]
		assert.Equal(t, "active", updatedSprint.State)
		assert.NotNil(t, updatedSprint.StartDate)
		assert.NotNil(t, updatedSprint.EndDate)
	})
	
	t.Run("Close Sprint", func(t *testing.T) {
		// Create an active sprint
		sprint := &jira.Sprint{
			ID:            103,
			Name:          "Active Sprint",
			State:         "active",
			OriginBoardID: 1,
		}
		mockClient.sprints[103] = sprint
		
		req := httptest.NewRequest("POST", "/api/v1/sprints/103/close", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		// Verify sprint state changed
		updatedSprint := mockClient.sprints[103]
		assert.Equal(t, "closed", updatedSprint.State)
		assert.NotNil(t, updatedSprint.CompleteDate)
	})
}

// Test Sprint Service Layer
func TestSprintService(t *testing.T) {
	mockClient := NewMockJiraClient()
	service := services.NewSprintService(mockClient)
	ctx := context.Background()
	
	// Setup test data
	board := &jira.Board{
		ID:   1,
		Name: "Test Board",
		Type: "scrum",
	}
	mockClient.boards[1] = board
	
	t.Run("Validate Sprint", func(t *testing.T) {
		// Valid sprint
		validReq := &jira.CreateSprintRequest{
			Name:          "Valid Sprint",
			Goal:          "Complete features",
			OriginBoardID: 1,
		}
		err := service.ValidateSprint(validReq)
		assert.NoError(t, err)
		
		// Invalid: no name
		invalidReq := &jira.CreateSprintRequest{
			OriginBoardID: 1,
		}
		err = service.ValidateSprint(invalidReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
		
		// Invalid: dates wrong order
		startDate := time.Now().AddDate(0, 0, 10)
		endDate := time.Now()
		invalidReq = &jira.CreateSprintRequest{
			Name:          "Sprint",
			OriginBoardID: 1,
			StartDate:     &startDate,
			EndDate:       &endDate,
		}
		err = service.ValidateSprint(invalidReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "start date must be before end date")
	})
	
	t.Run("Get Active Sprints", func(t *testing.T) {
		// Create test sprints
		activeSprint := &jira.Sprint{
			ID:            200,
			Name:          "Active Sprint",
			State:         "active",
			OriginBoardID: 1,
		}
		futureSprint := &jira.Sprint{
			ID:            201,
			Name:          "Future Sprint",
			State:         "future",
			OriginBoardID: 1,
		}
		closedSprint := &jira.Sprint{
			ID:            202,
			Name:          "Closed Sprint",
			State:         "closed",
			OriginBoardID: 1,
		}
		
		mockClient.sprints[200] = activeSprint
		mockClient.sprints[201] = futureSprint
		mockClient.sprints[202] = closedSprint
		
		activeSprints, err := service.GetActiveSprints(ctx)
		require.NoError(t, err)
		assert.Len(t, activeSprints, 1)
		assert.Equal(t, "Active Sprint", activeSprints[0].Name)
	})
	
	t.Run("Auto Start Sprint", func(t *testing.T) {
		// Create a future sprint
		futureSprint := &jira.Sprint{
			ID:            300,
			Name:          "Future Sprint",
			State:         "future",
			OriginBoardID: 1,
		}
		mockClient.sprints[300] = futureSprint
		
		err := service.AutoStartSprint(ctx, 300)
		require.NoError(t, err)
		
		// Verify sprint started
		updatedSprint := mockClient.sprints[300]
		assert.Equal(t, "active", updatedSprint.State)
		assert.NotNil(t, updatedSprint.StartDate)
		assert.NotNil(t, updatedSprint.EndDate)
	})
	
	t.Run("Sprint Metrics", func(t *testing.T) {
		// Create sprint with issues
		sprintID := 400
		sprint := &jira.Sprint{
			ID:            sprintID,
			Name:          "Metrics Sprint",
			State:         "active",
			OriginBoardID: 1,
		}
		startDate := time.Now().AddDate(0, 0, -7)
		sprint.StartDate = &startDate
		mockClient.sprints[sprintID] = sprint
		
		// Add test issues
		mockClient.sprintIssues[sprintID] = []jira.SprintIssue{
			{
				ID:  "ISSUE-1",
				Key: "ISSUE-1",
				Fields: struct {
					Summary string      `json:"summary"`
					Status  jira.Status `json:"status"`
				}{
					Summary: "Completed Issue",
					Status: jira.Status{
						Name: "Done",
						StatusCategory: jira.StatusCategory{
							Key: "done",
						},
					},
				},
			},
			{
				ID:  "ISSUE-2",
				Key: "ISSUE-2",
				Fields: struct {
					Summary string      `json:"summary"`
					Status  jira.Status `json:"status"`
				}{
					Summary: "In Progress Issue",
					Status: jira.Status{
						Name: "In Progress",
						StatusCategory: jira.StatusCategory{
							Key: "indeterminate",
						},
					},
				},
			},
			{
				ID:  "ISSUE-3",
				Key: "ISSUE-3",
				Fields: struct {
					Summary string      `json:"summary"`
					Status  jira.Status `json:"status"`
				}{
					Summary: "Todo Issue",
					Status: jira.Status{
						Name: "To Do",
						StatusCategory: jira.StatusCategory{
							Key: "new",
						},
					},
				},
			},
		}
		
		metrics, err := service.GetSprintMetrics(ctx, sprintID)
		require.NoError(t, err)
		
		assert.Equal(t, 3, metrics.TotalIssues)
		assert.Equal(t, 1, metrics.CompletedIssues)
		assert.Equal(t, 1, metrics.InProgressIssues)
		assert.Equal(t, 1, metrics.TodoIssues)
		assert.InDelta(t, 33.33, metrics.CompletionPercentage, 0.1)
	})
	
	t.Run("Complete Sprint With Report", func(t *testing.T) {
		// Create active sprint with issues
		sprintID := 500
		sprint := &jira.Sprint{
			ID:            sprintID,
			Name:          "Completion Sprint",
			State:         "active",
			OriginBoardID: 1,
		}
		startDate := time.Now().AddDate(0, 0, -14)
		endDate := time.Now()
		sprint.StartDate = &startDate
		sprint.EndDate = &endDate
		mockClient.sprints[sprintID] = sprint
		
		// Add mixed issues
		mockClient.sprintIssues[sprintID] = []jira.SprintIssue{
			{
				ID:  "DONE-1",
				Key: "DONE-1",
				Fields: struct {
					Summary string      `json:"summary"`
					Status  jira.Status `json:"status"`
				}{
					Status: jira.Status{
						StatusCategory: jira.StatusCategory{Key: "done"},
					},
				},
			},
			{
				ID:  "TODO-1",
				Key: "TODO-1",
				Fields: struct {
					Summary string      `json:"summary"`
					Status  jira.Status `json:"status"`
				}{
					Status: jira.Status{
						StatusCategory: jira.StatusCategory{Key: "new"},
					},
				},
			},
		}
		
		report, err := service.CompleteSprintWithReport(ctx, sprintID, "backlog")
		require.NoError(t, err)
		
		assert.Equal(t, 2, report.TotalIssues)
		assert.Len(t, report.CompletedIssues, 1)
		assert.Len(t, report.IncompleteIssues, 1)
		assert.Equal(t, 50.0, report.CompletionRate)
		
		// Verify sprint closed
		updatedSprint := mockClient.sprints[sprintID]
		assert.Equal(t, "closed", updatedSprint.State)
	})
}

// Test Sprint Predictions
func TestSprintPrediction(t *testing.T) {
	mockClient := NewMockJiraClient()
	service := services.NewSprintService(mockClient)
	ctx := context.Background()
	
	t.Run("Predict Sprint Success", func(t *testing.T) {
		sprintID := 600
		sprint := &jira.Sprint{
			ID:            sprintID,
			Name:          "Prediction Sprint",
			State:         "active",
			OriginBoardID: 1,
		}
		
		// Sprint started 7 days ago, ends in 7 days
		startDate := time.Now().AddDate(0, 0, -7)
		endDate := time.Now().AddDate(0, 0, 7)
		sprint.StartDate = &startDate
		sprint.EndDate = &endDate
		mockClient.sprints[sprintID] = sprint
		
		// Half completed
		mockClient.sprintIssues[sprintID] = []jira.SprintIssue{
			{Fields: struct {
				Summary string      `json:"summary"`
				Status  jira.Status `json:"status"`
			}{Status: jira.Status{StatusCategory: jira.StatusCategory{Key: "done"}}}},
			{Fields: struct {
				Summary string      `json:"summary"`
				Status  jira.Status `json:"status"`
			}{Status: jira.Status{StatusCategory: jira.StatusCategory{Key: "done"}}}},
			{Fields: struct {
				Summary string      `json:"summary"`
				Status  jira.Status `json:"status"`
			}{Status: jira.Status{StatusCategory: jira.StatusCategory{Key: "new"}}}},
			{Fields: struct {
				Summary string      `json:"summary"`
				Status  jira.Status `json:"status"`
			}{Status: jira.Status{StatusCategory: jira.StatusCategory{Key: "new"}}}},
		}
		
		prediction, err := service.PredictSprintSuccess(ctx, sprintID)
		require.NoError(t, err)
		
		assert.Equal(t, sprintID, prediction.SprintID)
		assert.NotEmpty(t, prediction.RiskLevel)
		assert.Greater(t, prediction.SuccessProbability, 0.0)
		
		// Should be on track (50% done at 50% time)
		assert.Equal(t, "Low", prediction.RiskLevel)
	})
}