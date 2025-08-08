package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ericfisherdev/GoJira/internal/auth"
	"github.com/ericfisherdev/GoJira/internal/monitoring"
	"github.com/go-resty/resty/v2"
)

// Client represents a Jira API client
type Client struct {
	baseURL       string
	authenticator auth.Authenticator
	httpClient    *resty.Client
}

// ClientOptions contains options for creating a new client
type ClientOptions struct {
	Timeout     time.Duration
	RetryCount  int
	RetryWait   time.Duration
	RetryMaxWait time.Duration
}

// NewClient creates a new Jira client
func NewClient(baseURL string, authenticator auth.Authenticator, opts *ClientOptions) *Client {
	if opts == nil {
		opts = &ClientOptions{
			Timeout:      30 * time.Second,
			RetryCount:   3,
			RetryWait:    1 * time.Second,
			RetryMaxWait: 5 * time.Second,
		}
	}

	client := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(opts.Timeout).
		SetRetryCount(opts.RetryCount).
		SetRetryWaitTime(opts.RetryWait).
		SetRetryMaxWaitTime(opts.RetryMaxWait).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		AddRetryCondition(func(r *resty.Response, err error) bool {
			// Retry on network errors or 5xx status codes
			return err != nil || r.StatusCode() >= 500
		})

	return &Client{
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		authenticator: authenticator,
		httpClient:    client,
	}
}

// doRequest executes an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*resty.Response, error) {
	// Track API call
	monitoring.GlobalMetrics.IncrementJiraAPICalls()
	
	req := c.httpClient.R().SetContext(ctx)

	// Add authentication headers
	if c.authenticator != nil {
		for k, v := range c.authenticator.GetHeaders() {
			req.SetHeader(k, v)
		}
	}

	// Set request body if provided
	if body != nil {
		req.SetBody(body)
	}

	// Execute request
	var resp *resty.Response
	var err error
	
	switch strings.ToUpper(method) {
	case "GET":
		resp, err = req.Get(endpoint)
	case "POST":
		resp, err = req.Post(endpoint)
	case "PUT":
		resp, err = req.Put(endpoint)
	case "DELETE":
		resp, err = req.Delete(endpoint)
	default:
		err = fmt.Errorf("unsupported HTTP method: %s", method)
	}
	
	// Track API call errors
	if err != nil || (resp != nil && resp.StatusCode() >= 400) {
		monitoring.GlobalMetrics.IncrementJiraAPIErrors()
	}
	
	return resp, err
}

// handleErrorResponse handles Jira API error responses
func (c *Client) handleErrorResponse(resp *resty.Response) error {
	if resp.IsSuccess() {
		return nil
	}

	var errorResp ErrorResponse
	if err := json.Unmarshal(resp.Body(), &errorResp); err == nil {
		if len(errorResp.ErrorMessages) > 0 {
			return fmt.Errorf("jira API error: %s", strings.Join(errorResp.ErrorMessages, "; "))
		}
		if len(errorResp.Errors) > 0 {
			var errorMsgs []string
			for field, msg := range errorResp.Errors {
				errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", field, msg))
			}
			return fmt.Errorf("jira API errors: %s", strings.Join(errorMsgs, "; "))
		}
	}

	return fmt.Errorf("jira API error: %d %s", resp.StatusCode(), resp.Status())
}

// GetServerInfo gets server information
func (c *Client) GetServerInfo(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.doRequest(ctx, "GET", "/rest/api/2/serverInfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var serverInfo map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &serverInfo); err != nil {
		return nil, fmt.Errorf("failed to parse server info: %w", err)
	}

	return serverInfo, nil
}

// GetMyself gets information about the current user
func (c *Client) GetMyself(ctx context.Context) (*User, error) {
	resp, err := c.doRequest(ctx, "GET", "/rest/api/2/myself", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var user User
	if err := json.Unmarshal(resp.Body(), &user); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &user, nil
}

// CreateIssue creates a new issue
func (c *Client) CreateIssue(ctx context.Context, issue *CreateIssueRequest) (*Issue, error) {
	resp, err := c.doRequest(ctx, "POST", "/rest/api/2/issue", issue)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var created Issue
	if err := json.Unmarshal(resp.Body(), &created); err != nil {
		return nil, fmt.Errorf("failed to parse created issue: %w", err)
	}

	return &created, nil
}

// GetIssue retrieves an issue by key or ID
func (c *Client) GetIssue(ctx context.Context, issueKey string, expand []string) (*Issue, error) {
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s", issueKey)
	
	// Add expand parameter if provided
	if len(expand) > 0 {
		endpoint += "?expand=" + strings.Join(expand, ",")
	}

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue %s: %w", issueKey, err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(resp.Body(), &issue); err != nil {
		return nil, fmt.Errorf("failed to parse issue: %w", err)
	}

	return &issue, nil
}

// UpdateIssue updates an existing issue
func (c *Client) UpdateIssue(ctx context.Context, issueKey string, update *UpdateIssueRequest) error {
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s", issueKey)

	resp, err := c.doRequest(ctx, "PUT", endpoint, update)
	if err != nil {
		return fmt.Errorf("failed to update issue %s: %w", issueKey, err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return err
	}

	return nil
}

// DeleteIssue deletes an issue
func (c *Client) DeleteIssue(ctx context.Context, issueKey string, deleteSubtasks bool) error {
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s", issueKey)
	
	if deleteSubtasks {
		endpoint += "?deleteSubtasks=true"
	}

	resp, err := c.doRequest(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to delete issue %s: %w", issueKey, err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return err
	}

	return nil
}

// SearchIssues searches for issues using JQL
func (c *Client) SearchIssues(ctx context.Context, jql string, startAt, maxResults int, expand []string) (*SearchResult, error) {
	params := url.Values{}
	params.Add("jql", jql)
	params.Add("startAt", strconv.Itoa(startAt))
	params.Add("maxResults", strconv.Itoa(maxResults))
	
	if len(expand) > 0 {
		params.Add("expand", strings.Join(expand, ","))
	}

	endpoint := "/rest/api/2/search?" + params.Encode()

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var result SearchResult
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	return &result, nil
}

// GetIssueTransitions gets available transitions for an issue
func (c *Client) GetIssueTransitions(ctx context.Context, issueKey string) (*TransitionsResult, error) {
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s/transitions", issueKey)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get transitions for issue %s: %w", issueKey, err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var result TransitionsResult
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse transitions: %w", err)
	}

	return &result, nil
}

// TransitionIssue transitions an issue to a new status
func (c *Client) TransitionIssue(ctx context.Context, issueKey string, transition *TransitionRequest) error {
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s/transitions", issueKey)

	resp, err := c.doRequest(ctx, "POST", endpoint, transition)
	if err != nil {
		return fmt.Errorf("failed to transition issue %s: %w", issueKey, err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return err
	}

	return nil
}

// AddComment adds a comment to an issue
func (c *Client) AddComment(ctx context.Context, issueKey string, comment *CreateCommentRequest) (*Comment, error) {
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s/comment", issueKey)

	resp, err := c.doRequest(ctx, "POST", endpoint, comment)
	if err != nil {
		return nil, fmt.Errorf("failed to add comment to issue %s: %w", issueKey, err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var result Comment
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse comment: %w", err)
	}

	return &result, nil
}

// GetComments gets comments for an issue
func (c *Client) GetComments(ctx context.Context, issueKey string, startAt, maxResults int) (*CommentResult, error) {
	params := url.Values{}
	params.Add("startAt", strconv.Itoa(startAt))
	params.Add("maxResults", strconv.Itoa(maxResults))

	endpoint := fmt.Sprintf("/rest/api/2/issue/%s/comment?%s", issueKey, params.Encode())

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for issue %s: %w", issueKey, err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var result CommentResult
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse comments: %w", err)
	}

	return &result, nil
}

// GetProjects gets all visible projects
func (c *Client) GetProjects(ctx context.Context) ([]Project, error) {
	resp, err := c.doRequest(ctx, "GET", "/rest/api/2/project", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var projects []Project
	if err := json.Unmarshal(resp.Body(), &projects); err != nil {
		return nil, fmt.Errorf("failed to parse projects: %w", err)
	}

	return projects, nil
}

// GetProject gets a project by key
func (c *Client) GetProject(ctx context.Context, projectKey string) (*Project, error) {
	endpoint := fmt.Sprintf("/rest/api/2/project/%s", projectKey)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectKey, err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var project Project
	if err := json.Unmarshal(resp.Body(), &project); err != nil {
		return nil, fmt.Errorf("failed to parse project: %w", err)
	}

	return &project, nil
}

// HealthCheck performs a health check on the client connection
func (c *Client) HealthCheck(ctx context.Context) error {
	// Use a lightweight endpoint to check connectivity
	resp, err := c.doRequest(ctx, "GET", "/rest/api/2/serverInfo", nil)
	if err != nil {
		return fmt.Errorf("health check failed - request error: %w", err)
	}

	if err := c.handleErrorResponse(resp); err != nil {
		return fmt.Errorf("health check failed - server error: %w", err)
	}

	// Additional validation - ensure response has expected data
	var serverInfo map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &serverInfo); err != nil {
		return fmt.Errorf("health check failed - invalid response: %w", err)
	}

	// Check that we got some expected server info fields
	if _, hasVersion := serverInfo["version"]; !hasVersion {
		return fmt.Errorf("health check failed - missing version info")
	}

	return nil
}