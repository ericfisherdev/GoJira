package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// TransitionDetail represents a detailed workflow transition in Jira
type TransitionDetail struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	To        Status              `json:"to"`
	Fields    map[string]FieldMeta `json:"fields,omitempty"`
	HasScreen bool                 `json:"hasScreen"`
}

// FieldMeta represents field metadata for transitions
type FieldMeta struct {
	Required     bool        `json:"required"`
	Name         string      `json:"name"`
	Schema       FieldSchema `json:"schema"`
	AllowedValues []interface{} `json:"allowedValues,omitempty"`
}

// FieldSchema represents the schema of a field
type FieldSchema struct {
	Type   string `json:"type"`
	System string `json:"system,omitempty"`
}

// TransitionIssueRequest represents a request to transition an issue with extended options
type TransitionIssueRequest struct {
	Transition Transition            `json:"transition"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	Update     map[string]interface{} `json:"update,omitempty"`
	Comment    *Comment              `json:"comment,omitempty"`
}

// GetTransitions retrieves available transitions for an issue
func (c *Client) GetTransitions(issueKey string) ([]Transition, error) {
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s/transitions", issueKey)
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get transitions: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get transitions, status: %d", resp.StatusCode())
	}

	var result struct {
		Transitions []Transition `json:"transitions"`
	}
	
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode transitions: %w", err)
	}

	return result.Transitions, nil
}

// TransitionIssueAdvanced transitions an issue to a new status with extended options
func (c *Client) TransitionIssueAdvanced(issueKey string, transitionID string, fields map[string]interface{}, comment string) error {
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s/transitions", issueKey)
	
	req := map[string]interface{}{
		"transition": map[string]string{
			"id": transitionID,
		},
	}

	if len(fields) > 0 {
		req["fields"] = fields
	}

	if comment != "" {
		req["update"] = map[string]interface{}{
			"comment": []map[string]interface{}{
				{
					"add": map[string]string{
						"body": comment,
					},
				},
			},
		}
	}
	
	resp, err := c.doRequest(context.Background(), "POST", endpoint, req)
	if err != nil {
		return fmt.Errorf("failed to transition issue: %w", err)
	}

	if resp.StatusCode() != http.StatusNoContent && resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("transition failed with status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// GetTransitionByName finds a transition by its name
func (c *Client) GetTransitionByName(issueKey string, transitionName string) (*Transition, error) {
	transitions, err := c.GetTransitions(issueKey)
	if err != nil {
		return nil, err
	}

	for _, t := range transitions {
		if t.Name == transitionName {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("transition '%s' not found for issue %s", transitionName, issueKey)
}

// TransitionIssueByName transitions an issue using the transition name
func (c *Client) TransitionIssueByName(issueKey string, transitionName string, fields map[string]interface{}, comment string) error {
	transition, err := c.GetTransitionByName(issueKey, transitionName)
	if err != nil {
		return err
	}

	return c.TransitionIssueAdvanced(issueKey, transition.ID, fields, comment)
}