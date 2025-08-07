package claude

import (
	"fmt"
	"strings"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
)

type ResponseFormatter struct {
	config FormatterConfig
}

type FormatterConfig struct {
	IncludeMetadata      bool `json:"includeMetadata"`
	UseMarkdown          bool `json:"useMarkdown"`
	SummarizeResults     bool `json:"summarizeResults"`
	MaxDescriptionLength int  `json:"maxDescriptionLength"`
}

type ClaudeResponse struct {
	Success     bool                   `json:"success"`
	Summary     string                 `json:"summary"`
	Details     interface{}            `json:"details"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Metadata    ResponseMetadata       `json:"metadata,omitempty"`
}

type ResponseMetadata struct {
	Timestamp    time.Time `json:"timestamp"`
	Duration     string    `json:"duration,omitempty"`
	ResultCount  int       `json:"resultCount,omitempty"`
	JiraInstance string    `json:"jiraInstance,omitempty"`
}

func NewResponseFormatter(config FormatterConfig) *ResponseFormatter {
	if config.MaxDescriptionLength == 0 {
		config.MaxDescriptionLength = 200
	}

	return &ResponseFormatter{config: config}
}

func (rf *ResponseFormatter) FormatIssueResponse(issue *jira.Issue, operation string) *ClaudeResponse {
	response := &ClaudeResponse{
		Success: true,
		Details: rf.formatIssueDetails(issue),
	}

	// Generate summary based on operation
	switch operation {
	case "create":
		response.Summary = fmt.Sprintf("✅ Created issue %s: %s", issue.Key, issue.Fields.Summary)
	case "update":
		response.Summary = fmt.Sprintf("✅ Updated issue %s: %s", issue.Key, issue.Fields.Summary)
	case "get":
		response.Summary = rf.generateIssueSummary(issue)
	default:
		response.Summary = fmt.Sprintf("Issue %s: %s", issue.Key, issue.Fields.Summary)
	}

	// Add suggestions
	response.Suggestions = rf.generateIssueSuggestions(issue)

	// Add context
	response.Context = map[string]interface{}{
		"issueKey": issue.Key,
		"project":  issue.Fields.Project.Key,
		"status":   issue.Fields.Status.Name,
	}

	if rf.config.IncludeMetadata {
		response.Metadata = ResponseMetadata{
			Timestamp: time.Now(),
		}
	}

	return response
}

func (rf *ResponseFormatter) FormatSearchResponse(result *jira.ExtendedSearchResult, jql string) *ClaudeResponse {
	response := &ClaudeResponse{
		Success: true,
		Details: rf.formatSearchDetails(result),
	}

	// Generate summary
	if result.Total == 0 {
		response.Summary = "No issues found matching your criteria."
	} else if result.Total == 1 {
		response.Summary = "Found 1 issue:"
		issue := &result.Issues[0]
		response.Summary += fmt.Sprintf(" %s - %s", issue.Key, issue.Fields.Summary)
	} else {
		response.Summary = fmt.Sprintf("Found %d issues", result.Total)
		if len(result.Issues) < result.Total {
			response.Summary += fmt.Sprintf(" (showing first %d)", len(result.Issues))
		}
	}

	// Add suggestions for search refinement
	response.Suggestions = rf.generateSearchSuggestions(result, jql)

	// Add context
	response.Context = map[string]interface{}{
		"jql":          jql,
		"totalResults": result.Total,
		"currentPage":  result.StartAt/result.MaxResults + 1,
	}

	if rf.config.IncludeMetadata {
		response.Metadata = ResponseMetadata{
			Timestamp:   time.Now(),
			ResultCount: result.Total,
		}
	}

	return response
}

func (rf *ResponseFormatter) formatIssueDetails(issue *jira.Issue) interface{} {
	details := map[string]interface{}{
		"key":     issue.Key,
		"summary": issue.Fields.Summary,
		"status":  issue.Fields.Status.Name,
		"project": issue.Fields.Project.Key,
		"type":    issue.Fields.IssueType.Name,
	}

	if issue.Fields.Assignee != nil {
		details["assignee"] = issue.Fields.Assignee.DisplayName
	} else {
		details["assignee"] = "Unassigned"
	}

	if issue.Fields.Priority != nil {
		details["priority"] = issue.Fields.Priority.Name
	}

	// Truncate description if too long
	if desc, ok := issue.Fields.Description.(string); ok && desc != "" {
		description := desc
		if len(description) > rf.config.MaxDescriptionLength {
			description = description[:rf.config.MaxDescriptionLength] + "..."
		}
		details["description"] = description
	}

	if issue.Fields.Created != nil {
		details["created"] = issue.Fields.Created.Time.Format("2006-01-02 15:04")
	}
	if issue.Fields.Updated != nil {
		details["updated"] = issue.Fields.Updated.Time.Format("2006-01-02 15:04")
	}

	if len(issue.Fields.Labels) > 0 {
		details["labels"] = issue.Fields.Labels
	}

	if len(issue.Fields.Components) > 0 {
		var componentNames []string
		for _, comp := range issue.Fields.Components {
			componentNames = append(componentNames, comp.Name)
		}
		details["components"] = componentNames
	}

	return details
}

func (rf *ResponseFormatter) formatSearchDetails(result *jira.ExtendedSearchResult) interface{} {
	if rf.config.SummarizeResults && len(result.Issues) > 5 {
		return rf.formatSearchSummary(result)
	}

	var issues []interface{}
	for _, issue := range result.Issues {
		issues = append(issues, rf.formatIssueDetails(&issue))
	}

	return map[string]interface{}{
		"issues":     issues,
		"total":      result.Total,
		"startAt":    result.StartAt,
		"maxResults": result.MaxResults,
	}
}

func (rf *ResponseFormatter) formatSearchSummary(result *jira.ExtendedSearchResult) interface{} {
	summary := map[string]interface{}{
		"total":      result.Total,
		"startAt":    result.StartAt,
		"maxResults": result.MaxResults,
	}

	// Group by status
	statusCounts := make(map[string]int)
	priorityCounts := make(map[string]int)
	projectCounts := make(map[string]int)

	var recentIssues []interface{}

	for i, issue := range result.Issues {
		statusCounts[issue.Fields.Status.Name]++
		projectCounts[issue.Fields.Project.Key]++

		if issue.Fields.Priority != nil {
			priorityCounts[issue.Fields.Priority.Name]++
		}

		// Include first 3 issues as examples
		if i < 3 {
			recentIssues = append(recentIssues, map[string]interface{}{
				"key":     issue.Key,
				"summary": issue.Fields.Summary,
				"status":  issue.Fields.Status.Name,
			})
		}
	}

	summary["statusBreakdown"] = statusCounts
	summary["priorityBreakdown"] = priorityCounts
	summary["projectBreakdown"] = projectCounts
	summary["sampleIssues"] = recentIssues

	return summary
}

func (rf *ResponseFormatter) generateIssueSummary(issue *jira.Issue) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("**%s**: %s", issue.Key, issue.Fields.Summary))
	parts = append(parts, fmt.Sprintf("Status: %s", issue.Fields.Status.Name))

	if issue.Fields.Assignee != nil {
		parts = append(parts, fmt.Sprintf("Assignee: %s", issue.Fields.Assignee.DisplayName))
	} else {
		parts = append(parts, "Assignee: Unassigned")
	}

	if issue.Fields.Priority != nil {
		parts = append(parts, fmt.Sprintf("Priority: %s", issue.Fields.Priority.Name))
	}

	return strings.Join(parts, " | ")
}

func (rf *ResponseFormatter) generateIssueSuggestions(issue *jira.Issue) []string {
	var suggestions []string

	// Status-based suggestions
	if issue.Fields.Status.Name == "To Do" {
		suggestions = append(suggestions, "Start work on this issue")
		suggestions = append(suggestions, "Assign to a team member")
	} else if issue.Fields.Status.Name == "In Progress" {
		suggestions = append(suggestions, "Add a comment with progress update")
		suggestions = append(suggestions, "Move to Done when complete")
	}

	// Assignee-based suggestions
	if issue.Fields.Assignee == nil {
		suggestions = append(suggestions, "Assign this issue to someone")
	}

	// Add related suggestions
	suggestions = append(suggestions, "View related issues")
	suggestions = append(suggestions, "Add a comment")
	suggestions = append(suggestions, "Create a subtask")

	return suggestions
}

func (rf *ResponseFormatter) generateSearchSuggestions(result *jira.ExtendedSearchResult, jql string) []string {
	var suggestions []string

	if result.Total > result.MaxResults {
		suggestions = append(suggestions, "Load more results")
		suggestions = append(suggestions, "Export all results to CSV")
	}

	if result.Total == 0 {
		suggestions = append(suggestions, "Try broadening your search criteria")
		suggestions = append(suggestions, "Check spelling in your query")
	} else if result.Total > 100 {
		suggestions = append(suggestions, "Narrow down your search")
		suggestions = append(suggestions, "Add more specific filters")
	}

	suggestions = append(suggestions, "Save this search as favorite")
	suggestions = append(suggestions, "Export results")
	suggestions = append(suggestions, "Create bulk operation")

	return suggestions
}

// FormatErrorResponse creates a Claude-optimized error response
func (rf *ResponseFormatter) FormatErrorResponse(err error, operation string) *ClaudeResponse {
	return &ClaudeResponse{
		Success: false,
		Summary: fmt.Sprintf("❌ %s failed: %s", operation, err.Error()),
		Details: map[string]interface{}{
			"error": err.Error(),
		},
		Suggestions: []string{
			"Check your Jira connection",
			"Verify your permissions",
			"Try the operation again",
		},
		Context: map[string]interface{}{
			"operation": operation,
			"success":   false,
		},
		Metadata: ResponseMetadata{
			Timestamp: time.Now(),
		},
	}
}

// FormatGenericResponse creates a generic Claude response for any data
func (rf *ResponseFormatter) FormatGenericResponse(data interface{}, summary string, operation string) *ClaudeResponse {
	return &ClaudeResponse{
		Success: true,
		Summary: summary,
		Details: data,
		Context: map[string]interface{}{
			"operation": operation,
			"success":   true,
		},
		Metadata: ResponseMetadata{
			Timestamp: time.Now(),
		},
	}
}