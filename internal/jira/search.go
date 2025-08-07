package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// SearchRequest represents a request to search for issues
type SearchRequest struct {
	JQL        string   `json:"jql" binding:"required"`
	StartAt    int      `json:"startAt,omitempty"`
	MaxResults int      `json:"maxResults,omitempty"`
	Fields     []string `json:"fields,omitempty"`
	Expand     []string `json:"expand,omitempty"`
	Properties []string `json:"properties,omitempty"`
}

func (s *SearchRequest) Bind(r *http.Request) error {
	if s.JQL == "" {
		return fmt.Errorf("jql is required")
	}
	return nil
}

// ExtendedSearchResult represents enhanced search results with additional metadata
type ExtendedSearchResult struct {
	StartAt         int      `json:"startAt"`
	MaxResults      int      `json:"maxResults"`
	Total           int      `json:"total"`
	Issues          []Issue  `json:"issues"`
	WarningMessages []string `json:"warningMessages,omitempty"`
}

// JQLSuggestion represents a JQL autocomplete suggestion
type JQLSuggestion struct {
	Value       string `json:"value"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
}

// SearchFilter represents a saved Jira filter
type SearchFilter struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Description      string             `json:"description,omitempty"`
	Owner            *User              `json:"owner"`
	JQL              string             `json:"jql"`
	ViewURL          string             `json:"viewUrl"`
	SearchURL        string             `json:"searchUrl"`
	Favourite        bool               `json:"favourite"`
	SharePermissions []SharePermission  `json:"sharePermissions,omitempty"`
}

// SharePermission represents filter sharing permissions
type SharePermission struct {
	ID    int    `json:"id"`
	Type  string `json:"type"` // global, project, group, user
	Value string `json:"value,omitempty"`
}

// SearchIssuesAdvanced performs an advanced search using POST method with full request body
func (c *Client) SearchIssuesAdvanced(req SearchRequest) (*ExtendedSearchResult, error) {
	endpoint := "/rest/api/2/search"
	
	resp, err := c.doRequest(context.Background(), "POST", endpoint, req)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result ExtendedSearchResult
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode search result: %w", err)
	}

	return &result, nil
}

// SearchIssuesGET performs a search using GET method with query parameters
func (c *Client) SearchIssuesGET(jql string, params map[string]string) (*ExtendedSearchResult, error) {
	endpoint := "/rest/api/2/search"
	
	// Build query parameters
	values := url.Values{}
	values.Add("jql", jql)
	
	for k, v := range params {
		values.Add(k, v)
	}
	
	fullURL := endpoint + "?" + values.Encode()
	
	resp, err := c.doRequest(context.Background(), "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result ExtendedSearchResult
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode search result: %w", err)
	}

	return &result, nil
}

// GetJQLSuggestions retrieves autocomplete suggestions for JQL fields
func (c *Client) GetJQLSuggestions(fieldName, fieldValue string) ([]JQLSuggestion, error) {
	endpoint := "/rest/api/2/jql/autocompletedata/suggestions"
	
	params := url.Values{}
	params.Add("fieldName", fieldName)
	if fieldValue != "" {
		params.Add("fieldValue", fieldValue)
	}
	
	fullURL := endpoint + "?" + params.Encode()
	
	resp, err := c.doRequest(context.Background(), "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get JQL suggestions: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get suggestions, status: %d", resp.StatusCode())
	}

	var suggestions struct {
		Results []JQLSuggestion `json:"results"`
	}
	
	if err := json.Unmarshal(resp.Body(), &suggestions); err != nil {
		return nil, fmt.Errorf("failed to decode suggestions: %w", err)
	}

	return suggestions.Results, nil
}

// ValidateJQL validates a JQL query and returns any errors
func (c *Client) ValidateJQL(jql string) (bool, []string, error) {
	endpoint := "/rest/api/2/jql/parse"
	
	req := map[string]string{"jql": jql}
	
	resp, err := c.doRequest(context.Background(), "POST", endpoint, req)
	if err != nil {
		return false, nil, fmt.Errorf("failed to validate JQL: %w", err)
	}

	var result struct {
		IsValid bool     `json:"isValid"`
		Errors  []string `json:"errors"`
	}
	
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return false, nil, fmt.Errorf("failed to decode validation result: %w", err)
	}

	return result.IsValid, result.Errors, nil
}

// GetAllFilters retrieves all filters accessible to the current user
func (c *Client) GetAllFilters() ([]SearchFilter, error) {
	endpoint := "/rest/api/2/filter/my"
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get filters: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get filters, status: %d", resp.StatusCode())
	}

	var filters []SearchFilter
	if err := json.Unmarshal(resp.Body(), &filters); err != nil {
		return nil, fmt.Errorf("failed to decode filters: %w", err)
	}

	return filters, nil
}

// GetFilter retrieves a specific filter by ID
func (c *Client) GetFilter(filterID string) (*SearchFilter, error) {
	endpoint := fmt.Sprintf("/rest/api/2/filter/%s", filterID)
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get filter: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		if resp.StatusCode() == http.StatusNotFound {
			return nil, fmt.Errorf("filter not found")
		}
		return nil, fmt.Errorf("failed to get filter, status: %d", resp.StatusCode())
	}

	var filter SearchFilter
	if err := json.Unmarshal(resp.Body(), &filter); err != nil {
		return nil, fmt.Errorf("failed to decode filter: %w", err)
	}

	return &filter, nil
}

// SearchWithFilter searches using a saved filter
func (c *Client) SearchWithFilter(filterID string, startAt, maxResults int) (*ExtendedSearchResult, error) {
	filter, err := c.GetFilter(filterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get filter: %w", err)
	}

	req := SearchRequest{
		JQL:        filter.JQL,
		StartAt:    startAt,
		MaxResults: maxResults,
	}

	return c.SearchIssuesAdvanced(req)
}

// GetJQLFields retrieves all available JQL fields for autocomplete
func (c *Client) GetJQLFields() ([]string, error) {
	endpoint := "/rest/api/2/jql/autocompletedata"
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get JQL fields: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get JQL fields, status: %d", resp.StatusCode())
	}

	var result struct {
		VisibleFieldNames []struct {
			Value       string `json:"value"`
			DisplayName string `json:"displayName"`
		} `json:"visibleFieldNames"`
	}
	
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode JQL fields: %w", err)
	}

	var fields []string
	for _, field := range result.VisibleFieldNames {
		fields = append(fields, field.Value)
	}

	return fields, nil
}

// GetJQLFunctions retrieves all available JQL functions
func (c *Client) GetJQLFunctions() ([]string, error) {
	endpoint := "/rest/api/2/jql/autocompletedata"
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get JQL functions: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get JQL functions, status: %d", resp.StatusCode())
	}

	var result struct {
		VisibleFunctionNames []struct {
			Value       string `json:"value"`
			DisplayName string `json:"displayName"`
		} `json:"visibleFunctionNames"`
	}
	
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode JQL functions: %w", err)
	}

	var functions []string
	for _, function := range result.VisibleFunctionNames {
		functions = append(functions, function.Value)
	}

	return functions, nil
}