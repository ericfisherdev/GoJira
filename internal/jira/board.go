package jira

import (
	"encoding/json"
	"fmt"
)

// Board represents a Jira board (Scrum or Kanban)
type Board struct {
	ID       int    `json:"id"`
	Self     string `json:"self"`
	Name     string `json:"name"`
	Type     string `json:"type"` // scrum or kanban
	Location struct {
		ProjectID      int    `json:"projectId"`
		DisplayName    string `json:"displayName"`
		ProjectName    string `json:"projectName"`
		ProjectKey     string `json:"projectKey"`
		ProjectTypeKey string `json:"projectTypeKey"`
		AvatarURI      string `json:"avatarURI"`
		Name           string `json:"name"`
	} `json:"location"`
}

// BoardList represents a list of boards
type BoardList struct {
	MaxResults int     `json:"maxResults"`
	StartAt    int     `json:"startAt"`
	Total      int     `json:"total"`
	IsLast     bool    `json:"isLast"`
	Values     []Board `json:"values"`
}

// BoardConfiguration represents board configuration
type BoardConfiguration struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Self            string `json:"self"`
	Location        string `json:"location"`
	Filter          Filter `json:"filter"`
	SubQuery        string `json:"subQuery,omitempty"`
	ColumnConfig    struct {
		Columns []struct {
			Name     string   `json:"name"`
			Statuses []Status `json:"statuses"`
			Min      int      `json:"min,omitempty"`
			Max      int      `json:"max,omitempty"`
		} `json:"columns"`
		ConstraintType string `json:"constraintType"`
	} `json:"columnConfig"`
	EstimationConfig struct {
		Type  string `json:"type"`
		Field struct {
			FieldID     string `json:"fieldId"`
			DisplayName string `json:"displayName"`
		} `json:"field"`
	} `json:"estimationConfig,omitempty"`
	Ranking struct {
		RankCustomFieldID int `json:"rankCustomFieldId"`
	} `json:"ranking"`
}

// Filter represents a Jira filter
type Filter struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Query       string `json:"query"`
	Owner       User   `json:"owner"`
	JQL         string `json:"jql"`
	ViewURL     string `json:"viewUrl"`
	SearchURL   string `json:"searchUrl"`
	Favourite   bool   `json:"favourite"`
	SharePermissions []struct {
		ID   int    `json:"id"`
		Type string `json:"type"`
	} `json:"sharePermissions"`
}

// BoardIssue represents an issue on a board
type BoardIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Self   string `json:"self"`
	Fields struct {
		Summary  string   `json:"summary"`
		Status   Status   `json:"status"`
		Priority Priority `json:"priority"`
		Assignee *User    `json:"assignee,omitempty"`
		Sprint   *Sprint  `json:"sprint,omitempty"`
	} `json:"fields"`
}

// BoardIssueList represents a list of issues on a board
type BoardIssueList struct {
	MaxResults int          `json:"maxResults"`
	StartAt    int          `json:"startAt"`
	Total      int          `json:"total"`
	Issues     []BoardIssue `json:"issues"`
}

// GetBoards retrieves all boards accessible to the user
func (c *Client) GetBoards() (*BoardList, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board", c.baseURL)
	
	resp, err := c.newRequest().
		SetHeader("Accept", "application/json").
		Get(url)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get boards: %w", err)
	}
	
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp)
	}
	
	var boardList BoardList
	if err := json.Unmarshal(resp.Body(), &boardList); err != nil {
		return nil, fmt.Errorf("failed to parse board list: %w", err)
	}
	
	return &boardList, nil
}

// GetBoard retrieves a specific board by ID
func (c *Client) GetBoard(boardID int) (*Board, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d", c.baseURL, boardID)
	
	resp, err := c.newRequest().
		SetHeader("Accept", "application/json").
		Get(url)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get board: %w", err)
	}
	
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp)
	}
	
	var board Board
	if err := json.Unmarshal(resp.Body(), &board); err != nil {
		return nil, fmt.Errorf("failed to parse board: %w", err)
	}
	
	return &board, nil
}

// GetBoardConfiguration retrieves the configuration of a board
func (c *Client) GetBoardConfiguration(boardID int) (*BoardConfiguration, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/configuration", c.baseURL, boardID)
	
	resp, err := c.newRequest().
		SetHeader("Accept", "application/json").
		Get(url)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get board configuration: %w", err)
	}
	
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp)
	}
	
	var config BoardConfiguration
	if err := json.Unmarshal(resp.Body(), &config); err != nil {
		return nil, fmt.Errorf("failed to parse board configuration: %w", err)
	}
	
	return &config, nil
}

// GetBoardIssues retrieves issues on a board
func (c *Client) GetBoardIssues(boardID int) (*BoardIssueList, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/issue", c.baseURL, boardID)
	
	resp, err := c.newRequest().
		SetHeader("Accept", "application/json").
		Get(url)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get board issues: %w", err)
	}
	
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp)
	}
	
	var issueList BoardIssueList
	if err := json.Unmarshal(resp.Body(), &issueList); err != nil {
		return nil, fmt.Errorf("failed to parse board issues: %w", err)
	}
	
	return &issueList, nil
}

// GetBoardBacklog retrieves issues in the backlog of a board
func (c *Client) GetBoardBacklog(boardID int) (*BoardIssueList, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/backlog", c.baseURL, boardID)
	
	resp, err := c.newRequest().
		SetHeader("Accept", "application/json").
		Get(url)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get board backlog: %w", err)
	}
	
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp)
	}
	
	var issueList BoardIssueList
	if err := json.Unmarshal(resp.Body(), &issueList); err != nil {
		return nil, fmt.Errorf("failed to parse board backlog: %w", err)
	}
	
	return &issueList, nil
}

// GetBoardSprints retrieves all sprints for a board
func (c *Client) GetBoardSprints(boardID int) (*SprintList, error) {
	return c.GetSprints(boardID)
}

// MoveIssuesToBacklog moves issues to the backlog
func (c *Client) MoveIssuesToBacklog(issueKeys []string) error {
	url := fmt.Sprintf("%s/rest/agile/1.0/backlog/issue", c.baseURL)
	
	req := struct {
		Issues []string `json:"issues"`
	}{
		Issues: issueKeys,
	}
	
	resp, err := c.newRequest().
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		Post(url)
	
	if err != nil {
		return fmt.Errorf("failed to move issues to backlog: %w", err)
	}
	
	if resp.IsError() {
		return c.handleErrorResponse(resp)
	}
	
	return nil
}

// MoveIssuesToBoard moves issues to a specific position on a board
func (c *Client) MoveIssuesToBoard(boardID int, issueKeys []string, position string) error {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/issue", c.baseURL, boardID)
	
	req := struct {
		Issues           []string `json:"issues"`
		RankBeforeIssue  string   `json:"rankBeforeIssue,omitempty"`
		RankAfterIssue   string   `json:"rankAfterIssue,omitempty"`
		RankCustomFieldID int     `json:"rankCustomFieldId,omitempty"`
	}{
		Issues: issueKeys,
	}
	
	// Position can be an issue key to rank before/after
	if position != "" {
		req.RankAfterIssue = position
	}
	
	resp, err := c.newRequest().
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		Post(url)
	
	if err != nil {
		return fmt.Errorf("failed to move issues on board: %w", err)
	}
	
	if resp.IsError() {
		return c.handleErrorResponse(resp)
	}
	
	return nil
}