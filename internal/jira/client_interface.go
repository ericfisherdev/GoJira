package jira

import "time"

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
}