package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// CustomField represents a custom field in Jira
type CustomField struct {
	ID            string        `json:"id"`
	Key           string        `json:"key"`
	Name          string        `json:"name"`
	Description   string        `json:"description,omitempty"`
	Type          string        `json:"custom,omitempty"`
	Schema        FieldSchema   `json:"schema,omitempty"`
	Required      bool          `json:"required"`
	AllowedValues []interface{} `json:"allowedValues,omitempty"`
}

// FieldConfiguration represents field configuration for a project
type FieldConfiguration struct {
	Fields map[string]FieldConfig `json:"fields"`
}

// FieldConfig represents the configuration of a single field
type FieldConfig struct {
	Required     bool          `json:"required"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	Type         string        `json:"type,omitempty"`
	AllowedValues []interface{} `json:"allowedValues,omitempty"`
}

// GetCustomFields retrieves all custom fields for a project
func (c *Client) GetCustomFields(projectKey string) ([]CustomField, error) {
	endpoint := "/rest/api/2/field"
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get fields: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get fields, status: %d", resp.StatusCode())
	}

	var fields []CustomField
	if err := json.Unmarshal(resp.Body(), &fields); err != nil {
		return nil, fmt.Errorf("failed to decode fields: %w", err)
	}

	// Filter for custom fields only
	var customFields []CustomField
	for _, field := range fields {
		if strings.HasPrefix(field.ID, "customfield_") {
			customFields = append(customFields, field)
		}
	}

	return customFields, nil
}

// GetFieldConfiguration gets the field configuration for creating issues in a project
func (c *Client) GetFieldConfiguration(projectKey string, issueType string) (*FieldConfiguration, error) {
	endpoint := fmt.Sprintf("/rest/api/2/issue/createmeta/%s/issuetypes/%s", projectKey, issueType)
	
	resp, err := c.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		// Try the older endpoint format
		endpoint = fmt.Sprintf("/rest/api/2/issue/createmeta?projectKeys=%s&issuetypeNames=%s&expand=projects.issuetypes.fields", projectKey, issueType)
		resp, err = c.doRequest(context.Background(), "GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get field configuration: %w", err)
		}
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get field configuration, status: %d", resp.StatusCode())
	}

	var result FieldConfiguration
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		// Try parsing the older format
		var oldFormat struct {
			Projects []struct {
				IssueTypes []struct {
					Fields map[string]FieldConfig `json:"fields"`
				} `json:"issuetypes"`
			} `json:"projects"`
		}
		
		// Reset the response body
		resp, err = c.doRequest(context.Background(), "GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to retry request: %w", err)
		}
		
		if err := json.Unmarshal(resp.Body(), &oldFormat); err != nil {
			return nil, fmt.Errorf("failed to decode field configuration: %w", err)
		}
		
		if len(oldFormat.Projects) > 0 && len(oldFormat.Projects[0].IssueTypes) > 0 {
			result.Fields = oldFormat.Projects[0].IssueTypes[0].Fields
		}
	}

	return &result, nil
}

// ValidateCustomFields validates custom field values before submission
func (c *Client) ValidateCustomFields(projectKey string, issueType string, customFields map[string]interface{}) error {
	config, err := c.GetFieldConfiguration(projectKey, issueType)
	if err != nil {
		return err
	}

	for fieldID, value := range customFields {
		if fieldConfig, exists := config.Fields[fieldID]; exists {
			if fieldConfig.Required && value == nil {
				return fmt.Errorf("custom field '%s' (%s) is required", fieldConfig.Name, fieldID)
			}
			
			// Additional validation can be added here based on field type
			if len(fieldConfig.AllowedValues) > 0 {
				// Check if value is in allowed values
				valid := false
				for _, allowed := range fieldConfig.AllowedValues {
					if value == allowed {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("value '%v' is not allowed for field '%s'", value, fieldConfig.Name)
				}
			}
		}
	}

	// Check for required fields that are missing
	for fieldID, fieldConfig := range config.Fields {
		if fieldConfig.Required && strings.HasPrefix(fieldID, "customfield_") {
			if _, exists := customFields[fieldID]; !exists {
				return fmt.Errorf("required custom field '%s' (%s) is missing", fieldConfig.Name, fieldID)
			}
		}
	}

	return nil
}

// GetCustomFieldByName finds a custom field by its name
func (c *Client) GetCustomFieldByName(projectKey string, fieldName string) (*CustomField, error) {
	fields, err := c.GetCustomFields(projectKey)
	if err != nil {
		return nil, err
	}

	for _, field := range fields {
		if field.Name == fieldName {
			return &field, nil
		}
	}

	return nil, fmt.Errorf("custom field '%s' not found", fieldName)
}

// UpdateIssueCustomFields updates only the custom fields of an issue
func (c *Client) UpdateIssueCustomFields(issueKey string, customFields map[string]interface{}) error {
	fields := make(map[string]interface{})
	
	// Only include custom fields
	for key, value := range customFields {
		if strings.HasPrefix(key, "customfield_") {
			fields[key] = value
		}
	}

	if len(fields) == 0 {
		return fmt.Errorf("no custom fields provided")
	}

	updateRequest := &UpdateIssueRequest{
		Fields: fields,
	}

	return c.UpdateIssue(context.Background(), issueKey, updateRequest)
}