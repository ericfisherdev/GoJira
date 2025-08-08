package jira

import (
	"context"
	"time"
)

// ClientInterface defines the interface for Jira client operations
type ClientInterface interface {
	// Sprint operations
	GetSprints(boardID int) (*SprintList, error)
	GetSprint(sprintID int) (*Sprint, error)
	CreateSprint(req *CreateSprintRequest) (*Sprint, error)
	UpdateSprint(sprintID int, req *UpdateSprintRequest) (*Sprint, error)
	StartSprint(sprintID int, startDate, endDate time.Time) error
	CloseSprint(sprintID int) error
	GetSprintIssues(sprintID int) (*SprintIssueList, error)
	MoveIssuesToSprint(sprintID int, issueKeys []string) error
	GetSprintReport(sprintID int) (*SprintReport, error)
	
	// Board operations
	GetBoards() (*BoardList, error)
	GetBoard(boardID int) (*Board, error)
	GetBoardConfiguration(boardID int) (*BoardConfiguration, error)
	GetBoardIssues(boardID int) (*BoardIssueList, error)
	GetBoardBacklog(boardID int) (*BoardIssueList, error)
	GetBoardSprints(boardID int) (*SprintList, error)
	MoveIssuesToBacklog(issueKeys []string) error
	MoveIssuesToBoard(boardID int, issueKeys []string, position string) error

	// Issue operations
	GetIssue(ctx context.Context, issueKey string, expand []string) (*Issue, error)
	GetTransitions(issueKey string) ([]Transition, error)
	TransitionIssueAdvanced(issueKey string, transitionID string, fields map[string]interface{}, comment string) error

	// Workflow operations
	GetWorkflows() (*WorkflowList, error)
	GetWorkflow(workflowName string) (*Workflow, error)
	GetWorkflowSchemes() ([]WorkflowScheme, error)
	GetProjectWorkflowScheme(projectKey string) (*WorkflowScheme, error)
	GetIssueWorkflow(issueKey string) (*Workflow, error)
	BuildWorkflowStateMachine(workflow *Workflow) (*WorkflowStateMachine, error)
	ValidateTransition(issueKey, transitionID string) (*WorkflowExecutionResult, error)
	ExecuteTransition(ctx *WorkflowExecutionContext) (*WorkflowExecutionResult, error)
}