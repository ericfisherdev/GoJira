package integration

import (
	"context"
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/ericfisherdev/GoJira/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test basic Sprint validation
func TestSprintValidation(t *testing.T) {
	service := services.NewSprintService(nil)
	
	tests := []struct {
		name      string
		request   jira.CreateSprintRequest
		wantError bool
		errorMsg  string
	}{
		{
			name: "Valid sprint",
			request: jira.CreateSprintRequest{
				Name:          "Sprint 1",
				Goal:          "Complete features",
				OriginBoardID: 1,
			},
			wantError: false,
		},
		{
			name: "Missing name",
			request: jira.CreateSprintRequest{
				OriginBoardID: 1,
			},
			wantError: true,
			errorMsg:  "name is required",
		},
		{
			name: "Missing board ID",
			request: jira.CreateSprintRequest{
				Name: "Sprint 1",
			},
			wantError: true,
			errorMsg:  "board ID is required",
		},
		{
			name: "Name too long",
			request: jira.CreateSprintRequest{
				Name:          string(make([]byte, 300)),
				OriginBoardID: 1,
			},
			wantError: true,
			errorMsg:  "exceeds maximum length",
		},
		{
			name: "Invalid date range",
			request: jira.CreateSprintRequest{
				Name:          "Sprint 1",
				OriginBoardID: 1,
				StartDate:     timePtr(time.Now().AddDate(0, 0, 7)),
				EndDate:       timePtr(time.Now()),
			},
			wantError: true,
			errorMsg:  "start date must be before end date",
		},
		{
			name: "Sprint too short",
			request: jira.CreateSprintRequest{
				Name:          "Sprint 1",
				OriginBoardID: 1,
				StartDate:     timePtr(time.Now()),
				EndDate:       timePtr(time.Now().AddDate(0, 0, 3)),
			},
			wantError: true,
			errorMsg:  "sprint duration must be at least",
		},
		{
			name: "Sprint too long",
			request: jira.CreateSprintRequest{
				Name:          "Sprint 1",
				OriginBoardID: 1,
				StartDate:     timePtr(time.Now()),
				EndDate:       timePtr(time.Now().AddDate(0, 2, 0)),
			},
			wantError: true,
			errorMsg:  "sprint duration cannot exceed",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateSprint(&tt.request)
			
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Test Sprint metrics calculation
func TestSprintMetricsCalculation(t *testing.T) {
	// This is a simple unit test for metrics logic
	mockClient := NewMockJiraClient()
	service := services.NewSprintService(mockClient)
	
	// Create test sprint
	sprint := &jira.Sprint{
		ID:            1,
		Name:          "Test Sprint",
		State:         "active",
		OriginBoardID: 1,
	}
	startDate := time.Now().AddDate(0, 0, -7)
	sprint.StartDate = &startDate
	mockClient.sprints[1] = sprint
	
	// Add issues with different statuses
	mockClient.sprintIssues[1] = []jira.SprintIssue{
		{
			ID:  "DONE-1",
			Key: "DONE-1",
			Fields: struct {
				Summary string      `json:"summary"`
				Status  jira.Status `json:"status"`
			}{
				Summary: "Completed task",
				Status: jira.Status{
					Name: "Done",
					StatusCategory: jira.StatusCategory{
						Key: "done",
					},
				},
			},
		},
		{
			ID:  "PROG-1",
			Key: "PROG-1",
			Fields: struct {
				Summary string      `json:"summary"`
				Status  jira.Status `json:"status"`
			}{
				Summary: "In progress task",
				Status: jira.Status{
					Name: "In Progress",
					StatusCategory: jira.StatusCategory{
						Key: "indeterminate",
					},
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
				Summary: "Todo task",
				Status: jira.Status{
					Name: "To Do",
					StatusCategory: jira.StatusCategory{
						Key: "new",
					},
				},
			},
		},
	}
	
	ctx := context.Background()
	metrics, err := service.GetSprintMetrics(ctx, 1)
	
	require.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Equal(t, 3, metrics.TotalIssues)
	assert.Equal(t, 1, metrics.CompletedIssues)
	assert.Equal(t, 1, metrics.InProgressIssues)
	assert.Equal(t, 1, metrics.TodoIssues)
	assert.InDelta(t, 33.33, metrics.CompletionPercentage, 0.1)
	
	// Check burndown rate calculation
	assert.Greater(t, metrics.BurndownRate, 0.0)
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}