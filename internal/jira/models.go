package jira

import (
	"encoding/json"
	"strings"
	"time"
)

// JiraTime wraps time.Time to handle Jira's timestamp format
type JiraTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler interface for Jira timestamps
func (jt *JiraTime) UnmarshalJSON(data []byte) error {
	// Remove quotes
	s := strings.Trim(string(data), "\"")
	if s == "null" || s == "" {
		return nil
	}
	
	// Try parsing with different formats that Jira uses
	formats := []string{
		"2006-01-02T15:04:05.000-0700",  // Jira format with milliseconds and timezone
		"2006-01-02T15:04:05-0700",      // Jira format with timezone
		"2006-01-02T15:04:05.000Z",      // ISO format with milliseconds
		"2006-01-02T15:04:05Z",          // ISO format
		time.RFC3339Nano,                // Standard RFC3339 with nanoseconds
		time.RFC3339,                    // Standard RFC3339
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			jt.Time = t
			return nil
		}
	}
	
	return &time.ParseError{Layout: "Jira timestamp", Value: s}
}

// MarshalJSON implements json.Marshaler interface
func (jt JiraTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(jt.Time)
}

// Issue represents a Jira issue
type Issue struct {
	ID       string                 `json:"id,omitempty"`
	Key      string                 `json:"key,omitempty"`
	Self     string                 `json:"self,omitempty"`
	Fields   IssueFields           `json:"fields"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Expand   string                 `json:"expand,omitempty"`
}

// IssueFields contains the fields of a Jira issue
type IssueFields struct {
	Summary     string         `json:"summary"`
	Description interface{}    `json:"description,omitempty"` // Can be string or ADF format
	Project     Project        `json:"project"`
	IssueType   IssueType      `json:"issuetype"`
	Priority    *Priority      `json:"priority,omitempty"`
	Status      *Status        `json:"status,omitempty"`
	Assignee    *User          `json:"assignee,omitempty"`
	Reporter    *User          `json:"reporter,omitempty"`
	Creator     *User          `json:"creator,omitempty"`
	Created     *JiraTime      `json:"created,omitempty"`
	Updated     *JiraTime      `json:"updated,omitempty"`
	Resolved    *JiraTime      `json:"resolutiondate,omitempty"`
	Labels      []string       `json:"labels,omitempty"`
	Components  []Component    `json:"components,omitempty"`
	Versions    []Version      `json:"versions,omitempty"`
	FixVersions []Version      `json:"fixVersions,omitempty"`
	Parent      *IssueRef      `json:"parent,omitempty"`
	Subtasks    []IssueRef     `json:"subtasks,omitempty"`
	IssueLinks  []IssueLink    `json:"issuelinks,omitempty"`
	Attachment  []Attachment   `json:"attachment,omitempty"`
	Comment     *CommentResult `json:"comment,omitempty"`
	Worklog     *WorklogResult `json:"worklog,omitempty"`
	
	// Time tracking
	TimeOriginalEstimate *string `json:"timeoriginalestimate,omitempty"`
	TimeEstimate         *string `json:"timeestimate,omitempty"`
	TimeSpent            *string `json:"timespent,omitempty"`
	
	// Additional timestamp fields from Jira
	StatusCategoryChangeDate *JiraTime `json:"statuscategorychangedate,omitempty"`
	LastViewed               *JiraTime `json:"lastViewed,omitempty"`
	
	// Custom fields (dynamic)
	CustomFields map[string]interface{} `json:"-"`
}

// Project represents a Jira project
type Project struct {
	ID          string           `json:"id,omitempty"`
	Key         string           `json:"key"`
	Name        string           `json:"name,omitempty"`
	Description string           `json:"description,omitempty"`
	Lead        *User            `json:"lead,omitempty"`
	ProjectTypeKey string        `json:"projectTypeKey,omitempty"`
	Avatar      *Avatar          `json:"avatarUrls,omitempty"`
	Components  []Component      `json:"components,omitempty"`
	Versions    []Version        `json:"versions,omitempty"`
	Roles       map[string]string `json:"roles,omitempty"`
}

// IssueType represents a Jira issue type
type IssueType struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	Subtask     bool   `json:"subtask,omitempty"`
}

// Priority represents issue priority
type Priority struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	IconURL string `json:"iconUrl,omitempty"`
}

// Status represents issue status
type Status struct {
	ID             string         `json:"id,omitempty"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	IconURL        string         `json:"iconUrl,omitempty"`
	StatusCategory StatusCategory `json:"statusCategory"`
}

// StatusCategory represents status category
type StatusCategory struct {
	ID    int    `json:"id,omitempty"`
	Key   string `json:"key"`
	Name  string `json:"name"`
	Color string `json:"colorName,omitempty"`
}

// User represents a Jira user
type User struct {
	AccountID    string  `json:"accountId,omitempty"`
	Name         string  `json:"name,omitempty"`         // Deprecated in Cloud
	Key          string  `json:"key,omitempty"`          // Deprecated in Cloud
	DisplayName  string  `json:"displayName"`
	EmailAddress string  `json:"emailAddress,omitempty"`
	Avatar       *Avatar `json:"avatarUrls,omitempty"`
	Active       bool    `json:"active"`
	TimeZone     string  `json:"timeZone,omitempty"`
}

// Component represents a project component
type Component struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Lead        *User  `json:"lead,omitempty"`
}

// Version represents a project version
type Version struct {
	ID             string     `json:"id,omitempty"`
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	Archived       bool       `json:"archived,omitempty"`
	Released       bool       `json:"released,omitempty"`
	ReleaseDate    *JiraTime  `json:"releaseDate,omitempty"`
	UserReleaseDate string    `json:"userReleaseDate,omitempty"`
	ProjectID      int        `json:"projectId,omitempty"`
}

// IssueRef represents a reference to another issue
type IssueRef struct {
	ID     string                 `json:"id"`
	Key    string                 `json:"key"`
	Self   string                 `json:"self"`
	Fields map[string]interface{} `json:"fields,omitempty"`
}

// IssueLink represents a link between issues
type IssueLink struct {
	ID           string        `json:"id,omitempty"`
	Type         IssueLinkType `json:"type"`
	OutwardIssue *IssueRef     `json:"outwardIssue,omitempty"`
	InwardIssue  *IssueRef     `json:"inwardIssue,omitempty"`
}

// IssueLinkType represents the type of issue link
type IssueLinkType struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// Comment represents a comment on an issue
type Comment struct {
	ID           string                 `json:"id,omitempty"`
	Self         string                 `json:"self,omitempty"`
	Author       *User                  `json:"author,omitempty"`
	Body         interface{}            `json:"body"` // Can be string or ADF format
	UpdateAuthor *User                  `json:"updateAuthor,omitempty"`
	Created      *JiraTime              `json:"created,omitempty"`
	Updated      *JiraTime              `json:"updated,omitempty"`
	Visibility   map[string]interface{} `json:"visibility,omitempty"`
}

// CommentResult represents the comment section of an issue
type CommentResult struct {
	Comments     []Comment `json:"comments"`
	MaxResults   int       `json:"maxResults"`
	Total        int       `json:"total"`
	StartAt      int       `json:"startAt"`
}

// Attachment represents a file attachment
type Attachment struct {
	ID       string     `json:"id,omitempty"`
	Self     string     `json:"self,omitempty"`
	Filename string     `json:"filename"`
	Author   *User      `json:"author,omitempty"`
	Created  *JiraTime  `json:"created,omitempty"`
	Size     int64      `json:"size,omitempty"`
	MimeType string     `json:"mimeType,omitempty"`
	Content  string     `json:"content,omitempty"`
	Thumbnail string    `json:"thumbnail,omitempty"`
}

// Worklog represents time spent on an issue
type Worklog struct {
	ID               string                 `json:"id,omitempty"`
	Self             string                 `json:"self,omitempty"`
	Author           *User                  `json:"author,omitempty"`
	Comment          interface{}            `json:"comment,omitempty"`
	Created          *JiraTime              `json:"created,omitempty"`
	Updated          *JiraTime              `json:"updated,omitempty"`
	Started          *JiraTime              `json:"started,omitempty"`
	TimeSpent        string                 `json:"timeSpent"`
	TimeSpentSeconds int                    `json:"timeSpentSeconds"`
	Visibility       map[string]interface{} `json:"visibility,omitempty"`
}

// WorklogResult represents the worklog section of an issue
type WorklogResult struct {
	Worklogs   []Worklog `json:"worklogs"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	StartAt    int       `json:"startAt"`
}

// Avatar represents avatar URLs
type Avatar struct {
	Size16 string `json:"16x16,omitempty"`
	Size24 string `json:"24x24,omitempty"`
	Size32 string `json:"32x32,omitempty"`
	Size48 string `json:"48x48,omitempty"`
}

// ProjectType represents project type
type ProjectType struct {
	Key         string `json:"key"`
	FormattedKey string `json:"formattedKey,omitempty"`
	DescriptionI18nKey string `json:"descriptionI18nKey,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Color       string `json:"color,omitempty"`
}

// SearchResult represents search results from JQL queries
type SearchResult struct {
	Expand     string  `json:"expand,omitempty"`
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	Issues     []Issue `json:"issues"`
}

// CreateIssueRequest represents a request to create an issue
type CreateIssueRequest struct {
	Fields map[string]interface{} `json:"fields"`
}

// UpdateIssueRequest represents a request to update an issue
type UpdateIssueRequest struct {
	Fields map[string]interface{} `json:"fields,omitempty"`
	Update map[string]interface{} `json:"update,omitempty"`
}

// TransitionRequest represents a request to transition an issue
type TransitionRequest struct {
	Transition Transition             `json:"transition"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	Update     map[string]interface{} `json:"update,omitempty"`
}

// Transition represents an issue transition
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// TransitionsResult represents available transitions
type TransitionsResult struct {
	Expand      string       `json:"expand,omitempty"`
	Transitions []Transition `json:"transitions"`
}

// CreateCommentRequest represents a request to create a comment
type CreateCommentRequest struct {
	Body       interface{}            `json:"body"`
	Visibility map[string]interface{} `json:"visibility,omitempty"`
}

// ErrorResponse represents a Jira API error response
type ErrorResponse struct {
	Errors       map[string]string `json:"errors,omitempty"`
	ErrorMessages []string         `json:"errorMessages,omitempty"`
}