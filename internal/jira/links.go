package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// LinkTypeDetail represents the detailed type of link between issues
type LinkTypeDetail struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
	Self    string `json:"self,omitempty"`
}

// CreateIssueLinkRequest represents a request to create an issue link
type CreateIssueLinkRequest struct {
	Type         IssueLinkType  `json:"type"`
	InwardIssue  IssueRef       `json:"inwardIssue"`
	OutwardIssue IssueRef       `json:"outwardIssue"`
	Comment      *Comment       `json:"comment,omitempty"`
}

// GetIssueLinkTypes retrieves all available issue link types
func (c *Client) GetIssueLinkTypes() ([]IssueLinkType, error) {
	endpoint := "/rest/api/2/issueLinkType"
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get link types: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get link types, status: %d", resp.StatusCode())
	}

	var result struct {
		IssueLinkTypes []IssueLinkType `json:"issueLinkTypes"`
	}
	
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode link types: %w", err)
	}

	return result.IssueLinkTypes, nil
}

// CreateIssueLink creates a link between two issues
func (c *Client) CreateIssueLink(inwardIssueKey, outwardIssueKey, linkTypeName string, comment string) error {
	// First get the link type ID
	linkTypes, err := c.GetIssueLinkTypes()
	if err != nil {
		return err
	}

	var linkType *IssueLinkType
	for _, lt := range linkTypes {
		if lt.Name == linkTypeName {
			linkType = &lt
			break
		}
	}

	if linkType == nil {
		return fmt.Errorf("link type '%s' not found", linkTypeName)
	}

	endpoint := "/rest/api/2/issueLink"
	
	req := map[string]interface{}{
		"type": map[string]string{
			"name": linkType.Name,
		},
		"inwardIssue": map[string]string{
			"key": inwardIssueKey,
		},
		"outwardIssue": map[string]string{
			"key": outwardIssueKey,
		},
	}

	if comment != "" {
		req["comment"] = map[string]string{
			"body": comment,
		}
	}
	
	resp, err := c.doRequest(context.Background(), "POST", endpoint, req)
	if err != nil {
		return fmt.Errorf("failed to create link: %w", err)
	}
	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("link creation failed with status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// DeleteIssueLink deletes an issue link by ID
func (c *Client) DeleteIssueLink(linkID string) error {
	endpoint := fmt.Sprintf("/rest/api/2/issueLink/%s", linkID)
	
	resp, err := c.doRequest(context.Background(), "DELETE", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to delete link: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return fmt.Errorf("link deletion failed with status: %d", resp.StatusCode())
	}

	return nil
}

// GetIssueLinks retrieves all links for an issue
func (c *Client) GetIssueLinks(issueKey string) ([]IssueLink, error) {
	issue, err := c.GetIssue(context.Background(), issueKey, []string{})
	if err != nil {
		return nil, err
	}

	// Return the issue links from the fields
	return issue.Fields.IssueLinks, nil
}