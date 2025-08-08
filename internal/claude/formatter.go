package claude

import (
	"fmt"
	"strings"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
)

// ResponseFormat defines the output format for responses
type ResponseFormat string

const (
	FormatJSON     ResponseFormat = "json"
	FormatMarkdown ResponseFormat = "markdown"
	FormatTable    ResponseFormat = "table"
	FormatSummary  ResponseFormat = "summary"
	FormatCompact  ResponseFormat = "compact"
)

type ResponseFormatter struct {
	config FormatterConfig
}

type FormatterConfig struct {
	IncludeMetadata      bool           `json:"includeMetadata"`
	UseMarkdown          bool           `json:"useMarkdown"`
	SummarizeResults     bool           `json:"summarizeResults"`
	MaxDescriptionLength int            `json:"maxDescriptionLength"`
	DefaultFormat        ResponseFormat `json:"defaultFormat"`
	Verbose              bool           `json:"verbose"`
	MaxResults           int            `json:"maxResults"`
	Fields               []string       `json:"fields"`
	SortBy               string         `json:"sortBy"`
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
	if config.MaxResults == 0 {
		config.MaxResults = 50
	}
	if config.DefaultFormat == "" {
		config.DefaultFormat = FormatJSON
	}

	return &ResponseFormatter{config: config}
}

// Format formats data according to the specified format
func (rf *ResponseFormatter) Format(data interface{}, format ResponseFormat) (string, error) {
	switch format {
	case FormatJSON:
		return rf.formatJSON(data)
	case FormatMarkdown:
		return rf.formatMarkdown(data)
	case FormatTable:
		return rf.formatTable(data)
	case FormatSummary:
		return rf.formatSummary(data)
	case FormatCompact:
		return rf.formatCompact(data)
	default:
		return "", fmt.Errorf("unknown format: %s", format)
	}
}

// formatMarkdown formats data as markdown
func (rf *ResponseFormatter) formatMarkdown(data interface{}) (string, error) {
	var buf strings.Builder
	
	switch v := data.(type) {
	case *jira.Issue:
		rf.formatIssueMarkdown(&buf, v)
	case []jira.Issue:
		rf.formatIssueListMarkdown(&buf, v)
	case *jira.ExtendedSearchResult:
		rf.formatSearchResultMarkdown(&buf, v)
	case *ClaudeResponse:
		rf.formatClaudeResponseMarkdown(&buf, v)
	default:
		return "", fmt.Errorf("unsupported type for markdown formatting: %T", data)
	}
	
	return buf.String(), nil
}

// formatIssueMarkdown formats a single issue as markdown
func (rf *ResponseFormatter) formatIssueMarkdown(buf *strings.Builder, issue *jira.Issue) {
	buf.WriteString(fmt.Sprintf("## %s: %s\n\n", issue.Key, issue.Fields.Summary))
	
	// Status badge
	buf.WriteString(fmt.Sprintf("**Status:** `%s`", issue.Fields.Status.Name))
	if issue.Fields.Status.StatusCategory.Name != "" {
		buf.WriteString(fmt.Sprintf(" (%s)", issue.Fields.Status.StatusCategory.Name))
	}
	buf.WriteString("\n")
	
	// Basic info table
	buf.WriteString("| Field | Value |\n")
	buf.WriteString("|-------|-------|\n")
	buf.WriteString(fmt.Sprintf("| **Type** | %s |\n", issue.Fields.IssueType.Name))
	
	if issue.Fields.Priority != nil {
		buf.WriteString(fmt.Sprintf("| **Priority** | %s |\n", issue.Fields.Priority.Name))
	}
	
	if issue.Fields.Assignee != nil {
		buf.WriteString(fmt.Sprintf("| **Assignee** | %s |\n", issue.Fields.Assignee.DisplayName))
	} else {
		buf.WriteString("| **Assignee** | Unassigned |\n")
	}
	
	buf.WriteString(fmt.Sprintf("| **Project** | %s |\n", issue.Fields.Project.Key))
	
	if issue.Fields.Created != nil {
		buf.WriteString(fmt.Sprintf("| **Created** | %s |\n", 
			issue.Fields.Created.Time.Format("2006-01-02 15:04")))
	}
	
	if issue.Fields.Updated != nil {
		buf.WriteString(fmt.Sprintf("| **Updated** | %s |\n", 
			issue.Fields.Updated.Time.Format("2006-01-02 15:04")))
	}
	
	// Description
	if desc, ok := issue.Fields.Description.(string); ok && desc != "" {
		buf.WriteString("\n### Description\n")
		if len(desc) > rf.config.MaxDescriptionLength {
			desc = desc[:rf.config.MaxDescriptionLength] + "..."
		}
		buf.WriteString(desc)
		buf.WriteString("\n")
	}
	
	// Labels and Components
	if len(issue.Fields.Labels) > 0 {
		buf.WriteString("\n**Labels:** ")
		for i, label := range issue.Fields.Labels {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(fmt.Sprintf("`%s`", label))
		}
		buf.WriteString("\n")
	}
	
	if len(issue.Fields.Components) > 0 {
		buf.WriteString("\n**Components:** ")
		for i, comp := range issue.Fields.Components {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(fmt.Sprintf("`%s`", comp.Name))
		}
		buf.WriteString("\n")
	}
	
	if issue.Self != "" {
		buf.WriteString(fmt.Sprintf("\n[View in Jira](%s)\n", issue.Self))
	}
}

// formatIssueListMarkdown formats a list of issues as markdown
func (rf *ResponseFormatter) formatIssueListMarkdown(buf *strings.Builder, issues []jira.Issue) {
	if len(issues) == 0 {
		buf.WriteString("No issues found.\n")
		return
	}
	
	buf.WriteString(fmt.Sprintf("# Issues (%d)\n\n", len(issues)))
	
	// Summary table
	buf.WriteString("| Key | Summary | Status | Assignee | Priority |\n")
	buf.WriteString("|-----|---------|--------|----------|----------|\n")
	
	for _, issue := range issues {
		assignee := "Unassigned"
		if issue.Fields.Assignee != nil {
			assignee = issue.Fields.Assignee.DisplayName
		}
		
		priority := "None"
		if issue.Fields.Priority != nil {
			priority = issue.Fields.Priority.Name
		}
		
		summary := issue.Fields.Summary
		if len(summary) > 50 {
			summary = summary[:47] + "..."
		}
		
		buf.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			issue.Key, summary, issue.Fields.Status.Name, assignee, priority))
	}
}

// formatSearchResultMarkdown formats search results as markdown
func (rf *ResponseFormatter) formatSearchResultMarkdown(buf *strings.Builder, result *jira.ExtendedSearchResult) {
	buf.WriteString(fmt.Sprintf("# Search Results\n\n"))
	buf.WriteString(fmt.Sprintf("**Total Results:** %d\n", result.Total))
	buf.WriteString(fmt.Sprintf("**Showing:** %d-%d\n\n", 
		result.StartAt+1, result.StartAt+len(result.Issues)))
	
	if len(result.Issues) > 0 {
		rf.formatIssueListMarkdown(buf, result.Issues)
		
		// Add stats if more than 3 issues
		if len(result.Issues) > 3 {
			buf.WriteString("\n## Summary\n\n")
			rf.addSearchStatistics(buf, result.Issues)
		}
	}
}

// formatClaudeResponseMarkdown formats a ClaudeResponse as markdown
func (rf *ResponseFormatter) formatClaudeResponseMarkdown(buf *strings.Builder, response *ClaudeResponse) {
	if response.Success {
		buf.WriteString("✅ ")
	} else {
		buf.WriteString("❌ ")
	}
	buf.WriteString(fmt.Sprintf("# %s\n\n", response.Summary))
	
	// Format details based on type
	if response.Details != nil {
		if detailStr, err := rf.formatMarkdown(response.Details); err == nil {
			buf.WriteString(detailStr)
		}
	}
	
	// Add suggestions
	if len(response.Suggestions) > 0 {
		buf.WriteString("\n## Suggestions\n\n")
		for _, suggestion := range response.Suggestions {
			buf.WriteString(fmt.Sprintf("- %s\n", suggestion))
		}
	}
}

// formatTable formats data as a table
func (rf *ResponseFormatter) formatTable(data interface{}) (string, error) {
	switch v := data.(type) {
	case []jira.Issue:
		return rf.formatIssueTable(v), nil
	case *jira.ExtendedSearchResult:
		return rf.formatIssueTable(v.Issues), nil
	default:
		return "", fmt.Errorf("table format not supported for type: %T", data)
	}
}

// formatIssueTable formats issues as a simple table
func (rf *ResponseFormatter) formatIssueTable(issues []jira.Issue) string {
	if len(issues) == 0 {
		return "No issues found."
	}
	
	var buf strings.Builder
	
	// Determine columns based on config
	columns := rf.config.Fields
	if len(columns) == 0 {
		columns = []string{"key", "summary", "status", "assignee", "priority"}
	}
	
	// Header
	for i, col := range columns {
		if i > 0 {
			buf.WriteString(" | ")
		}
		buf.WriteString(strings.Title(col))
	}
	buf.WriteString("\n")
	
	// Separator
	for i := range columns {
		if i > 0 {
			buf.WriteString("-+-")
		}
		buf.WriteString(strings.Repeat("-", 15))
	}
	buf.WriteString("\n")
	
	// Rows
	for _, issue := range issues {
		for i, col := range columns {
			if i > 0 {
				buf.WriteString(" | ")
			}
			
			var value string
			switch col {
			case "key":
				value = issue.Key
			case "summary":
				value = issue.Fields.Summary
				if len(value) > 30 {
					value = value[:27] + "..."
				}
			case "status":
				value = issue.Fields.Status.Name
			case "assignee":
				if issue.Fields.Assignee != nil {
					value = issue.Fields.Assignee.DisplayName
				} else {
					value = "Unassigned"
				}
			case "priority":
				if issue.Fields.Priority != nil {
					value = issue.Fields.Priority.Name
				} else {
					value = "None"
				}
			case "project":
				value = issue.Fields.Project.Key
			case "type":
				value = issue.Fields.IssueType.Name
			default:
				value = "-"
			}
			
			// Truncate to fit column width
			if len(value) > 15 {
				value = value[:12] + "..."
			}
			
			buf.WriteString(fmt.Sprintf("%-15s", value))
		}
		buf.WriteString("\n")
	}
	
	return buf.String()
}

// formatSummary creates a concise summary
func (rf *ResponseFormatter) formatSummary(data interface{}) (string, error) {
	switch v := data.(type) {
	case []jira.Issue:
		return rf.summarizeIssues(v), nil
	case *jira.ExtendedSearchResult:
		return rf.summarizeSearchResult(v), nil
	case *ClaudeResponse:
		return v.Summary, nil
	default:
		return fmt.Sprintf("Summary not available for type: %T", data), nil
	}
}

// formatCompact creates the most concise format
func (rf *ResponseFormatter) formatCompact(data interface{}) (string, error) {
	switch v := data.(type) {
	case *jira.Issue:
		return fmt.Sprintf("%s: %s [%s]", v.Key, v.Fields.Summary, v.Fields.Status.Name), nil
	case []jira.Issue:
		if len(v) == 0 {
			return "No issues", nil
		}
		if len(v) == 1 {
			return rf.formatCompact(&v[0])
		}
		return fmt.Sprintf("%d issues found", len(v)), nil
	case *jira.ExtendedSearchResult:
		return fmt.Sprintf("%d total results", v.Total), nil
	default:
		return fmt.Sprintf("Data: %T", data), nil
	}
}

// formatJSON returns JSON representation (delegated to existing logic)
func (rf *ResponseFormatter) formatJSON(data interface{}) (string, error) {
	// This would typically use json.Marshal, but for now return a placeholder
	return fmt.Sprintf("JSON format for %T", data), nil
}

// Helper methods

func (rf *ResponseFormatter) addSearchStatistics(buf *strings.Builder, issues []jira.Issue) {
	// Group by status
	statusCounts := make(map[string]int)
	priorityCounts := make(map[string]int)
	assigneeCounts := make(map[string]int)
	
	for _, issue := range issues {
		statusCounts[issue.Fields.Status.Name]++
		
		if issue.Fields.Priority != nil {
			priorityCounts[issue.Fields.Priority.Name]++
		}
		
		if issue.Fields.Assignee != nil {
			assigneeCounts[issue.Fields.Assignee.DisplayName]++
		} else {
			assigneeCounts["Unassigned"]++
		}
	}
	
	// Status breakdown
	if len(statusCounts) > 1 {
		buf.WriteString("**By Status:**\n")
		for status, count := range statusCounts {
			buf.WriteString(fmt.Sprintf("- %s: %d\n", status, count))
		}
		buf.WriteString("\n")
	}
	
	// Priority breakdown
	if len(priorityCounts) > 1 {
		buf.WriteString("**By Priority:**\n")
		for priority, count := range priorityCounts {
			buf.WriteString(fmt.Sprintf("- %s: %d\n", priority, count))
		}
		buf.WriteString("\n")
	}
}

func (rf *ResponseFormatter) summarizeIssues(issues []jira.Issue) string {
	if len(issues) == 0 {
		return "No issues found"
	}
	
	if len(issues) == 1 {
		issue := issues[0]
		return fmt.Sprintf("1 issue: %s - %s [%s]", 
			issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
	}
	
	// Group by status
	statusCounts := make(map[string]int)
	for _, issue := range issues {
		statusCounts[issue.Fields.Status.Name]++
	}
	
	var parts []string
	parts = append(parts, fmt.Sprintf("%d issues total", len(issues)))
	
	// Add status breakdown if varied
	if len(statusCounts) > 1 {
		for status, count := range statusCounts {
			parts = append(parts, fmt.Sprintf("%d %s", count, status))
		}
	}
	
	return strings.Join(parts, ", ")
}

func (rf *ResponseFormatter) summarizeSearchResult(result *jira.ExtendedSearchResult) string {
	summary := rf.summarizeIssues(result.Issues)
	
	if result.Total > len(result.Issues) {
		summary += fmt.Sprintf(" (showing %d of %d total)", len(result.Issues), result.Total)
	}
	
	return summary
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