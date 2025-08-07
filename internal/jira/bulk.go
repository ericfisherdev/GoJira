package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// BulkOperation represents a bulk operation request
type BulkOperation struct {
	IssueKeys []string               `json:"issueKeys"`
	Operation string                 `json:"operation"` // update, delete, transition
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Transition string                `json:"transition,omitempty"`
}

// BulkOperationResult represents the result of a bulk operation
type BulkOperationResult struct {
	Successful []string              `json:"successful"`
	Failed     map[string]string     `json:"failed"` // key -> error message
	TotalTime  time.Duration         `json:"totalTime"`
}

// BulkCreateRequest represents a request to create multiple issues
type BulkCreateRequest struct {
	IssueUpdates []IssueCreate `json:"issueUpdates"`
}

// IssueCreate represents a single issue creation in bulk
type IssueCreate struct {
	Fields map[string]interface{} `json:"fields"`
}

// BulkCreateResponse represents the response from bulk create
type BulkCreateResponse struct {
	Issues []Issue              `json:"issues"`
	Errors []map[string]string  `json:"errors"`
}

// BulkUpdateIssues updates multiple issues with the same fields
func (c *Client) BulkUpdateIssues(issueKeys []string, fields map[string]interface{}) (*BulkOperationResult, error) {
	start := time.Now()
	result := &BulkOperationResult{
		Successful: []string{},
		Failed:     make(map[string]string),
	}

	// Use a worker pool for concurrent updates
	const maxWorkers = 5
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, key := range issueKeys {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		
		go func(issueKey string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			updateRequest := &UpdateIssueRequest{
				Fields: fields,
			}

			err := c.UpdateIssue(context.Background(), issueKey, updateRequest)
			
			mu.Lock()
			defer mu.Unlock()
			
			if err != nil {
				result.Failed[issueKey] = err.Error()
			} else {
				result.Successful = append(result.Successful, issueKey)
			}
		}(key)
	}

	wg.Wait()
	result.TotalTime = time.Since(start)

	return result, nil
}

// BulkTransitionIssues transitions multiple issues to the same state
func (c *Client) BulkTransitionIssues(issueKeys []string, transitionName string, comment string) (*BulkOperationResult, error) {
	start := time.Now()
	result := &BulkOperationResult{
		Successful: []string{},
		Failed:     make(map[string]string),
	}

	// Use a worker pool for concurrent transitions
	const maxWorkers = 5
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, key := range issueKeys {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		
		go func(issueKey string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			err := c.TransitionIssueByName(issueKey, transitionName, nil, comment)
			
			mu.Lock()
			defer mu.Unlock()
			
			if err != nil {
				result.Failed[issueKey] = err.Error()
			} else {
				result.Successful = append(result.Successful, issueKey)
			}
		}(key)
	}

	wg.Wait()
	result.TotalTime = time.Since(start)

	return result, nil
}

// BulkDeleteIssues deletes multiple issues
func (c *Client) BulkDeleteIssues(issueKeys []string, deleteSubtasks bool) (*BulkOperationResult, error) {
	start := time.Now()
	result := &BulkOperationResult{
		Successful: []string{},
		Failed:     make(map[string]string),
	}

	// Use a worker pool for concurrent deletes
	const maxWorkers = 3 // Lower concurrency for deletes
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, key := range issueKeys {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		
		go func(issueKey string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			endpoint := fmt.Sprintf("/rest/api/2/issue/%s", issueKey)
			if deleteSubtasks {
				endpoint += "?deleteSubtasks=true"
			}

			resp, err := c.doRequest(context.Background(), "DELETE", endpoint, nil)
			if err != nil {
				mu.Lock()
				result.Failed[issueKey] = err.Error()
				mu.Unlock()
				return
			}
			mu.Lock()
			defer mu.Unlock()
			
			if resp.StatusCode() == http.StatusNoContent || resp.StatusCode() == http.StatusOK {
				result.Successful = append(result.Successful, issueKey)
			} else {
				result.Failed[issueKey] = fmt.Sprintf("deletion failed with status: %d", resp.StatusCode())
			}
		}(key)
	}

	wg.Wait()
	result.TotalTime = time.Since(start)

	return result, nil
}

// BulkCreateIssues creates multiple issues in a single request
func (c *Client) BulkCreateIssues(issues []map[string]interface{}) (*BulkCreateResponse, error) {
	endpoint := "/rest/api/2/issue/bulk"
	
	issueUpdates := make([]IssueCreate, len(issues))
	for i, fields := range issues {
		issueUpdates[i] = IssueCreate{Fields: fields}
	}

	request := BulkCreateRequest{
		IssueUpdates: issueUpdates,
	}

	resp, err := c.doRequest(context.Background(), "POST", endpoint, request)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk create issues: %w", err)
	}
	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("bulk create failed with status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var response BulkCreateResponse
	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return nil, fmt.Errorf("failed to decode bulk create response: %w", err)
	}

	return &response, nil
}

// BulkAssignIssues assigns multiple issues to a user
func (c *Client) BulkAssignIssues(issueKeys []string, assignee string) (*BulkOperationResult, error) {
	fields := map[string]interface{}{
		"assignee": map[string]string{
			"name": assignee,
		},
	}

	return c.BulkUpdateIssues(issueKeys, fields)
}

// BulkAddLabels adds labels to multiple issues
func (c *Client) BulkAddLabels(issueKeys []string, labels []string) (*BulkOperationResult, error) {
	start := time.Now()
	result := &BulkOperationResult{
		Successful: []string{},
		Failed:     make(map[string]string),
	}

	// Use a worker pool for concurrent updates
	const maxWorkers = 5
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, key := range issueKeys {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		
		go func(issueKey string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			// Get current labels
			issue, err := c.GetIssue(context.Background(), issueKey, []string{})
			if err != nil {
				mu.Lock()
				result.Failed[issueKey] = fmt.Sprintf("failed to get issue: %v", err)
				mu.Unlock()
				return
			}

			// Extract current labels
			currentLabels := issue.Fields.Labels

			// Merge labels (avoid duplicates)
			labelMap := make(map[string]bool)
			for _, label := range currentLabels {
				labelMap[label] = true
			}
			for _, label := range labels {
				labelMap[label] = true
			}

			// Convert back to slice
			allLabels := []string{}
			for label := range labelMap {
				allLabels = append(allLabels, label)
			}

			// Update issue with new labels
			updateRequest := &UpdateIssueRequest{
				Fields: map[string]interface{}{
					"labels": allLabels,
				},
			}

			err = c.UpdateIssue(context.Background(), issueKey, updateRequest)
			
			mu.Lock()
			defer mu.Unlock()
			
			if err != nil {
				result.Failed[issueKey] = err.Error()
			} else {
				result.Successful = append(result.Successful, issueKey)
			}
		}(key)
	}

	wg.Wait()
	result.TotalTime = time.Since(start)

	return result, nil
}