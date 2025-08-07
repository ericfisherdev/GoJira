package jira

import (
	"encoding/json"
	"fmt"
	"time"
	
	"github.com/go-resty/resty/v2"
)

// Sprint represents a Jira sprint
type Sprint struct {
	ID            int        `json:"id"`
	Self          string     `json:"self,omitempty"`
	State         string     `json:"state"`
	Name          string     `json:"name"`
	StartDate     *time.Time `json:"startDate,omitempty"`
	EndDate       *time.Time `json:"endDate,omitempty"`
	CompleteDate  *time.Time `json:"completeDate,omitempty"`
	OriginBoardID int        `json:"originBoardId,omitempty"`
	Goal          string     `json:"goal,omitempty"`
}

// SprintList represents a list of sprints
type SprintList struct {
	MaxResults int      `json:"maxResults"`
	StartAt    int      `json:"startAt"`
	IsLast     bool     `json:"isLast"`
	Values     []Sprint `json:"values"`
}

// SprintIssue represents an issue in a sprint
type SprintIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  Status `json:"status"`
	} `json:"fields"`
}

// SprintIssueList represents issues in a sprint
type SprintIssueList struct {
	Issues []SprintIssue `json:"issues"`
	Total  int           `json:"total"`
}

// CreateSprintRequest represents a request to create a sprint
type CreateSprintRequest struct {
	Name          string     `json:"name"`
	Goal          string     `json:"goal,omitempty"`
	StartDate     *time.Time `json:"startDate,omitempty"`
	EndDate       *time.Time `json:"endDate,omitempty"`
	OriginBoardID int        `json:"originBoardId"`
}

// UpdateSprintRequest represents a request to update a sprint
type UpdateSprintRequest struct {
	Name      string     `json:"name,omitempty"`
	Goal      string     `json:"goal,omitempty"`
	StartDate *time.Time `json:"startDate,omitempty"`
	EndDate   *time.Time `json:"endDate,omitempty"`
	State     string     `json:"state,omitempty"`
}

// MoveIssuesToSprintRequest represents a request to move issues to a sprint
type MoveIssuesToSprintRequest struct {
	Issues []string `json:"issues"`
}

// SprintVelocity represents sprint velocity metrics
type SprintVelocity struct {
	SprintID         int     `json:"sprintId"`
	Completed        float64 `json:"completed"`
	Committed        float64 `json:"committed"`
	CompletedIssues  int     `json:"completedIssues"`
	CommittedIssues  int     `json:"committedIssues"`
	IncompleteIssues int     `json:"incompleteIssues"`
}

// SprintReport represents a sprint report with burndown data
type SprintReport struct {
	Sprint     Sprint                    `json:"sprint"`
	Velocity   SprintVelocity            `json:"velocity"`
	Burndown   []BurndownDataPoint       `json:"burndown"`
	Issues     map[string][]SprintIssue `json:"issues"`
	StartDate  time.Time                 `json:"startDate"`
	EndDate    time.Time                 `json:"endDate"`
	IsComplete bool                      `json:"isComplete"`
}

// BurndownDataPoint represents a point in the burndown chart
type BurndownDataPoint struct {
	Date          time.Time `json:"date"`
	StoryPoints   float64   `json:"storyPoints"`
	IssueCount    int       `json:"issueCount"`
	IdealProgress float64   `json:"idealProgress"`
}

// Helper method to create authenticated request
func (c *Client) newRequest() *resty.Request {
	req := c.httpClient.R()
	
	// Add authentication headers
	if c.authenticator != nil {
		for k, v := range c.authenticator.GetHeaders() {
			req.SetHeader(k, v)
		}
	}
	
	return req
}

// GetSprints retrieves all sprints for a board
func (c *Client) GetSprints(boardID int) (*SprintList, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/sprint", c.baseURL, boardID)
	
	resp, err := c.newRequest().
		SetHeader("Accept", "application/json").
		Get(url)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get sprints: %w", err)
	}
	
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp)
	}
	
	var sprintList SprintList
	if err := json.Unmarshal(resp.Body(), &sprintList); err != nil {
		return nil, fmt.Errorf("failed to parse sprint list: %w", err)
	}
	
	return &sprintList, nil
}

// GetSprint retrieves a specific sprint by ID
func (c *Client) GetSprint(sprintID int) (*Sprint, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d", c.baseURL, sprintID)
	
	resp, err := c.newRequest().
		SetHeader("Accept", "application/json").
		Get(url)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get sprint: %w", err)
	}
	
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp)
	}
	
	var sprint Sprint
	if err := json.Unmarshal(resp.Body(), &sprint); err != nil {
		return nil, fmt.Errorf("failed to parse sprint: %w", err)
	}
	
	return &sprint, nil
}

// CreateSprint creates a new sprint
func (c *Client) CreateSprint(req *CreateSprintRequest) (*Sprint, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint", c.baseURL)
	
	resp, err := c.newRequest().
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		Post(url)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create sprint: %w", err)
	}
	
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp)
	}
	
	var sprint Sprint
	if err := json.Unmarshal(resp.Body(), &sprint); err != nil {
		return nil, fmt.Errorf("failed to parse created sprint: %w", err)
	}
	
	return &sprint, nil
}

// UpdateSprint updates an existing sprint
func (c *Client) UpdateSprint(sprintID int, req *UpdateSprintRequest) (*Sprint, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d", c.baseURL, sprintID)
	
	resp, err := c.newRequest().
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		Post(url)
	
	if err != nil {
		return nil, fmt.Errorf("failed to update sprint: %w", err)
	}
	
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp)
	}
	
	var sprint Sprint
	if err := json.Unmarshal(resp.Body(), &sprint); err != nil {
		return nil, fmt.Errorf("failed to parse updated sprint: %w", err)
	}
	
	return &sprint, nil
}

// StartSprint starts a sprint
func (c *Client) StartSprint(sprintID int, startDate, endDate time.Time) error {
	req := &UpdateSprintRequest{
		State:     "active",
		StartDate: &startDate,
		EndDate:   &endDate,
	}
	
	_, err := c.UpdateSprint(sprintID, req)
	return err
}

// CloseSprint closes a sprint
func (c *Client) CloseSprint(sprintID int) error {
	req := &UpdateSprintRequest{
		State: "closed",
	}
	
	_, err := c.UpdateSprint(sprintID, req)
	return err
}

// GetSprintIssues retrieves issues in a sprint
func (c *Client) GetSprintIssues(sprintID int) (*SprintIssueList, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue", c.baseURL, sprintID)
	
	resp, err := c.newRequest().
		SetHeader("Accept", "application/json").
		Get(url)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get sprint issues: %w", err)
	}
	
	if resp.IsError() {
		return nil, c.handleErrorResponse(resp)
	}
	
	var issueList SprintIssueList
	if err := json.Unmarshal(resp.Body(), &issueList); err != nil {
		return nil, fmt.Errorf("failed to parse sprint issues: %w", err)
	}
	
	return &issueList, nil
}

// MoveIssuesToSprint moves issues to a sprint
func (c *Client) MoveIssuesToSprint(sprintID int, issueKeys []string) error {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue", c.baseURL, sprintID)
	
	req := &MoveIssuesToSprintRequest{
		Issues: issueKeys,
	}
	
	resp, err := c.newRequest().
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		Post(url)
	
	if err != nil {
		return fmt.Errorf("failed to move issues to sprint: %w", err)
	}
	
	if resp.IsError() {
		return c.handleErrorResponse(resp)
	}
	
	return nil
}

// GetSprintReport generates a sprint report with velocity and burndown
func (c *Client) GetSprintReport(sprintID int) (*SprintReport, error) {
	sprint, err := c.GetSprint(sprintID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sprint: %w", err)
	}
	
	issues, err := c.GetSprintIssues(sprintID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sprint issues: %w", err)
	}
	
	// Calculate velocity
	velocity := c.calculateVelocity(issues)
	
	// Generate burndown data (simplified)
	burndown := c.generateBurndown(sprint, issues)
	
	// Categorize issues
	categorizedIssues := c.categorizeIssues(issues)
	
	report := &SprintReport{
		Sprint:     *sprint,
		Velocity:   velocity,
		Burndown:   burndown,
		Issues:     categorizedIssues,
		IsComplete: sprint.State == "closed",
	}
	
	if sprint.StartDate != nil {
		report.StartDate = *sprint.StartDate
	}
	if sprint.EndDate != nil {
		report.EndDate = *sprint.EndDate
	}
	
	return report, nil
}

// Helper functions for sprint report
func (c *Client) calculateVelocity(issues *SprintIssueList) SprintVelocity {
	velocity := SprintVelocity{}
	
	for _, issue := range issues.Issues {
		velocity.CommittedIssues++
		// In a real implementation, we'd extract story points from custom fields
		velocity.Committed += 1.0 // Simplified: count each issue as 1 point
		
		if issue.Fields.Status.StatusCategory.Key == "done" {
			velocity.CompletedIssues++
			velocity.Completed += 1.0
		} else {
			velocity.IncompleteIssues++
		}
	}
	
	return velocity
}

func (c *Client) generateBurndown(sprint *Sprint, issues *SprintIssueList) []BurndownDataPoint {
	if sprint.StartDate == nil || sprint.EndDate == nil {
		return []BurndownDataPoint{}
	}
	
	// Simplified burndown generation
	totalPoints := float64(len(issues.Issues))
	days := int(sprint.EndDate.Sub(*sprint.StartDate).Hours() / 24)
	burndown := make([]BurndownDataPoint, 0, days)
	
	for i := 0; i <= days; i++ {
		date := sprint.StartDate.AddDate(0, 0, i)
		idealProgress := totalPoints * (1.0 - float64(i)/float64(days))
		
		burndown = append(burndown, BurndownDataPoint{
			Date:          date,
			StoryPoints:   totalPoints - (float64(i) * totalPoints / float64(days)),
			IssueCount:    len(issues.Issues),
			IdealProgress: idealProgress,
		})
	}
	
	return burndown
}

func (c *Client) categorizeIssues(issues *SprintIssueList) map[string][]SprintIssue {
	categories := make(map[string][]SprintIssue)
	
	for _, issue := range issues.Issues {
		status := issue.Fields.Status.Name
		categories[status] = append(categories[status], issue)
	}
	
	return categories
}