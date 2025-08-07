package claude

import (
	"fmt"
	"strings"
	"sort"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
)

// Summarizer provides intelligent summarization of Jira data for Claude Code
type Summarizer struct {
	maxLength int
	emphasis  []string
}

// SummaryType defines different types of summaries
type SummaryType string

const (
	SummaryBrief    SummaryType = "brief"
	SummaryDetailed SummaryType = "detailed"
	SummaryMetrics  SummaryType = "metrics"
	SummaryTrends   SummaryType = "trends"
)

// SummaryOptions configures summarization behavior
type SummaryOptions struct {
	Type         SummaryType
	MaxLength    int
	IncludeStats bool
	GroupBy      []string
	Emphasis     []string
	TimeRange    *TimeRange
}

// TimeRange defines a time period for analysis
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// IssueMetrics contains statistical information about issues
type IssueMetrics struct {
	Total          int                    `json:"total"`
	ByStatus       map[string]int         `json:"byStatus"`
	ByPriority     map[string]int         `json:"byPriority"`
	ByAssignee     map[string]int         `json:"byAssignee"`
	ByProject      map[string]int         `json:"byProject"`
	ByType         map[string]int         `json:"byType"`
	CompletionRate float64                `json:"completionRate"`
	AverageAge     time.Duration          `json:"averageAge"`
	RecentActivity []RecentActivityItem   `json:"recentActivity"`
}

// RecentActivityItem represents recent activity on an issue
type RecentActivityItem struct {
	Key       string    `json:"key"`
	Action    string    `json:"action"`
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user,omitempty"`
}

// SprintMetrics contains sprint-specific metrics
type SprintMetrics struct {
	Velocity       VelocityMetrics `json:"velocity"`
	Burndown       BurndownMetrics `json:"burndown"`
	Health         HealthMetrics   `json:"health"`
	Completion     CompletionMetrics `json:"completion"`
}

// VelocityMetrics tracks sprint velocity
type VelocityMetrics struct {
	Planned   float64 `json:"planned"`
	Completed float64 `json:"completed"`
	Rate      float64 `json:"rate"`
}

// BurndownMetrics tracks burndown progress
type BurndownMetrics struct {
	Ideal           []float64  `json:"ideal"`
	Actual          []float64  `json:"actual"`
	PredictedFinish *time.Time `json:"predictedFinish,omitempty"`
	OnTrack         bool       `json:"onTrack"`
}

// HealthMetrics provides sprint health indicators
type HealthMetrics struct {
	Status      string  `json:"status"` // healthy, at-risk, critical
	Score       float64 `json:"score"`  // 0-100
	Issues      []string `json:"issues,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// CompletionMetrics tracks completion statistics
type CompletionMetrics struct {
	Completed    int     `json:"completed"`
	InProgress   int     `json:"inProgress"`
	NotStarted   int     `json:"notStarted"`
	PercentDone  float64 `json:"percentDone"`
}

// NewSummarizer creates a new summarizer with default settings
func NewSummarizer(maxLength int, emphasis []string) *Summarizer {
	if maxLength == 0 {
		maxLength = 500
	}
	
	return &Summarizer{
		maxLength: maxLength,
		emphasis:  emphasis,
	}
}

// SummarizeIssues creates a summary of issues based on the specified options
func (s *Summarizer) SummarizeIssues(issues []jira.Issue, opts SummaryOptions) string {
	if len(issues) == 0 {
		return "No issues found"
	}
	
	switch opts.Type {
	case SummaryBrief:
		return s.createBriefSummary(issues, opts)
	case SummaryDetailed:
		return s.createDetailedSummary(issues, opts)
	case SummaryMetrics:
		return s.createMetricsSummary(issues, opts)
	case SummaryTrends:
		return s.createTrendsSummary(issues, opts)
	default:
		return s.createBriefSummary(issues, opts)
	}
}

// SummarizeSprintMetrics creates a summary of sprint performance
func (s *Summarizer) SummarizeSprintMetrics(metrics *SprintMetrics) string {
	var summary strings.Builder
	
	summary.WriteString("**Sprint Performance Summary:**\n\n")
	
	// Velocity status
	velocityStatus := "on track"
	if metrics.Velocity.Rate < 0.8 {
		velocityStatus = "behind"
	} else if metrics.Velocity.Rate > 1.2 {
		velocityStatus = "ahead"
	}
	
	summary.WriteString(fmt.Sprintf("📊 **Velocity:** %.1f/%.1f points (%.0f%% - %s)\n", 
		metrics.Velocity.Completed, 
		metrics.Velocity.Planned,
		metrics.Velocity.Rate*100,
		velocityStatus))
	
	// Health status with emoji
	healthEmoji := "🟢"
	switch metrics.Health.Status {
	case "at-risk":
		healthEmoji = "🟡"
	case "critical":
		healthEmoji = "🔴"
	}
	
	summary.WriteString(fmt.Sprintf("%s **Health:** %s (%.0f/100)\n", 
		healthEmoji, strings.Title(metrics.Health.Status), metrics.Health.Score))
	
	// Completion status
	summary.WriteString(fmt.Sprintf("✅ **Completion:** %.1f%% (%d done, %d in progress, %d not started)\n",
		metrics.Completion.PercentDone,
		metrics.Completion.Completed,
		metrics.Completion.InProgress,
		metrics.Completion.NotStarted))
	
	// Burndown status
	if metrics.Burndown.PredictedFinish != nil {
		finishStatus := "on time"
		if metrics.Burndown.PredictedFinish.After(time.Now().AddDate(0, 0, 7)) { // Assuming 1 week sprint
			finishStatus = "may be late"
		}
		
		summary.WriteString(fmt.Sprintf("📈 **Burndown:** %s (predicted finish: %s)\n",
			finishStatus,
			metrics.Burndown.PredictedFinish.Format("Jan 2, 15:04")))
	}
	
	// Health issues and suggestions
	if len(metrics.Health.Issues) > 0 {
		summary.WriteString("\n**⚠️ Issues:**\n")
		for _, issue := range metrics.Health.Issues {
			summary.WriteString(fmt.Sprintf("- %s\n", issue))
		}
	}
	
	if len(metrics.Health.Suggestions) > 0 {
		summary.WriteString("\n**💡 Suggestions:**\n")
		for _, suggestion := range metrics.Health.Suggestions {
			summary.WriteString(fmt.Sprintf("- %s\n", suggestion))
		}
	}
	
	return summary.String()
}

// GenerateIssueMetrics calculates metrics from a list of issues
func (s *Summarizer) GenerateIssueMetrics(issues []jira.Issue) *IssueMetrics {
	metrics := &IssueMetrics{
		Total:      len(issues),
		ByStatus:   make(map[string]int),
		ByPriority: make(map[string]int),
		ByAssignee: make(map[string]int),
		ByProject:  make(map[string]int),
		ByType:     make(map[string]int),
	}
	
	if len(issues) == 0 {
		return metrics
	}
	
	var totalAge time.Duration
	completed := 0
	
	for _, issue := range issues {
		// Count by status
		metrics.ByStatus[issue.Fields.Status.Name]++
		
		// Count by priority
		if issue.Fields.Priority != nil {
			metrics.ByPriority[issue.Fields.Priority.Name]++
		}
		
		// Count by assignee
		if issue.Fields.Assignee != nil {
			metrics.ByAssignee[issue.Fields.Assignee.DisplayName]++
		} else {
			metrics.ByAssignee["Unassigned"]++
		}
		
		// Count by project
		metrics.ByProject[issue.Fields.Project.Key]++
		
		// Count by type
		metrics.ByType[issue.Fields.IssueType.Name]++
		
		// Calculate age and completion
		if issue.Fields.Created != nil {
			age := time.Since(issue.Fields.Created.Time)
			totalAge += age
		}
		
		// Check completion (assuming Done, Resolved, Closed are completed states)
		status := strings.ToLower(issue.Fields.Status.Name)
		if status == "done" || status == "resolved" || status == "closed" {
			completed++
		}
	}
	
	// Calculate averages
	metrics.AverageAge = totalAge / time.Duration(len(issues))
	metrics.CompletionRate = float64(completed) / float64(len(issues))
	
	return metrics
}

// createBriefSummary creates a short, concise summary
func (s *Summarizer) createBriefSummary(issues []jira.Issue, opts SummaryOptions) string {
	if len(issues) == 1 {
		issue := issues[0]
		return fmt.Sprintf("1 issue: %s - %s [%s]", 
			issue.Key, s.truncateText(issue.Fields.Summary, 40), issue.Fields.Status.Name)
	}
	
	metrics := s.GenerateIssueMetrics(issues)
	var parts []string
	
	parts = append(parts, fmt.Sprintf("%d issues", metrics.Total))
	
	// Add most significant status breakdown
	if len(metrics.ByStatus) > 1 {
		topStatuses := s.getTopCounts(metrics.ByStatus, 2)
		for status, count := range topStatuses {
			parts = append(parts, fmt.Sprintf("%d %s", count, status))
		}
	}
	
	// Add completion rate if significant
	if metrics.CompletionRate > 0 {
		parts = append(parts, fmt.Sprintf("%.0f%% complete", metrics.CompletionRate*100))
	}
	
	return strings.Join(parts, ", ")
}

// createDetailedSummary creates a comprehensive summary
func (s *Summarizer) createDetailedSummary(issues []jira.Issue, opts SummaryOptions) string {
	metrics := s.GenerateIssueMetrics(issues)
	var summary strings.Builder
	
	summary.WriteString(fmt.Sprintf("**Issue Summary (%d total)**\n\n", metrics.Total))
	
	// Status breakdown
	if len(metrics.ByStatus) > 0 {
		summary.WriteString("**By Status:**\n")
		statusList := s.getSortedCounts(metrics.ByStatus)
		for _, item := range statusList {
			percentage := float64(item.count) / float64(metrics.Total) * 100
			summary.WriteString(fmt.Sprintf("- %s: %d (%.0f%%)\n", item.name, item.count, percentage))
		}
		summary.WriteString("\n")
	}
	
	// Priority breakdown if varied
	if len(metrics.ByPriority) > 1 {
		summary.WriteString("**By Priority:**\n")
		priorityList := s.getSortedCounts(metrics.ByPriority)
		for _, item := range priorityList {
			summary.WriteString(fmt.Sprintf("- %s: %d\n", item.name, item.count))
		}
		summary.WriteString("\n")
	}
	
	// Assignee breakdown (top 5)
	if len(metrics.ByAssignee) > 1 {
		summary.WriteString("**Top Assignees:**\n")
		topAssignees := s.getTopCounts(metrics.ByAssignee, 5)
		for assignee, count := range topAssignees {
			summary.WriteString(fmt.Sprintf("- %s: %d\n", assignee, count))
		}
		summary.WriteString("\n")
	}
	
	// Key insights
	summary.WriteString("**Key Insights:**\n")
	if metrics.CompletionRate > 0 {
		summary.WriteString(fmt.Sprintf("- %.0f%% completion rate\n", metrics.CompletionRate*100))
	}
	
	if metrics.AverageAge > 24*time.Hour {
		summary.WriteString(fmt.Sprintf("- Average issue age: %.0f days\n", metrics.AverageAge.Hours()/24))
	}
	
	return s.truncateText(summary.String(), s.maxLength)
}

// createMetricsSummary focuses on statistical information
func (s *Summarizer) createMetricsSummary(issues []jira.Issue, opts SummaryOptions) string {
	metrics := s.GenerateIssueMetrics(issues)
	var summary strings.Builder
	
	summary.WriteString(fmt.Sprintf("**Metrics Dashboard (%d issues)**\n\n", metrics.Total))
	
	// Key performance indicators
	summary.WriteString("**KPIs:**\n")
	summary.WriteString(fmt.Sprintf("📊 Total Issues: %d\n", metrics.Total))
	summary.WriteString(fmt.Sprintf("✅ Completion Rate: %.1f%%\n", metrics.CompletionRate*100))
	summary.WriteString(fmt.Sprintf("⏱️ Average Age: %.0f days\n", metrics.AverageAge.Hours()/24))
	
	// Distribution metrics
	summary.WriteString("\n**Distribution:**\n")
	summary.WriteString(fmt.Sprintf("🏷️ Projects: %d\n", len(metrics.ByProject)))
	summary.WriteString(fmt.Sprintf("👥 Assignees: %d\n", len(metrics.ByAssignee)))
	summary.WriteString(fmt.Sprintf("📋 Issue Types: %d\n", len(metrics.ByType)))
	
	// Top categories
	if len(metrics.ByStatus) > 0 {
		summary.WriteString("\n**Status Distribution:**\n")
		for status, count := range metrics.ByStatus {
			percentage := float64(count) / float64(metrics.Total) * 100
			summary.WriteString(fmt.Sprintf("- %s: %d (%.0f%%)\n", status, count, percentage))
		}
	}
	
	return summary.String()
}

// createTrendsSummary analyzes trends and patterns
func (s *Summarizer) createTrendsSummary(issues []jira.Issue, opts SummaryOptions) string {
	var summary strings.Builder
	
	summary.WriteString(fmt.Sprintf("**Trends Analysis (%d issues)**\n\n", len(issues)))
	
	// Time-based analysis would require more historical data
	// For now, provide basic pattern analysis
	
	metrics := s.GenerateIssueMetrics(issues)
	
	// Workload distribution
	summary.WriteString("**Workload Distribution:**\n")
	unassignedCount := metrics.ByAssignee["Unassigned"]
	if unassignedCount > 0 {
		percentage := float64(unassignedCount) / float64(metrics.Total) * 100
		if percentage > 30 {
			summary.WriteString(fmt.Sprintf("⚠️ High unassigned rate: %.0f%% (%d issues)\n", percentage, unassignedCount))
		} else {
			summary.WriteString(fmt.Sprintf("✅ Reasonable assignment: %.0f%% unassigned\n", percentage))
		}
	}
	
	// Priority distribution analysis
	if len(metrics.ByPriority) > 0 {
		highPriority := metrics.ByPriority["High"] + metrics.ByPriority["Highest"] + metrics.ByPriority["Critical"]
		if highPriority > 0 {
			percentage := float64(highPriority) / float64(metrics.Total) * 100
			if percentage > 50 {
				summary.WriteString(fmt.Sprintf("⚠️ High priority issues: %.0f%% may indicate planning issues\n", percentage))
			}
		}
	}
	
	// Status analysis
	if len(metrics.ByStatus) > 0 {
		inProgress := metrics.ByStatus["In Progress"] + metrics.ByStatus["In Development"] + metrics.ByStatus["In Review"]
		if inProgress > 0 {
			percentage := float64(inProgress) / float64(metrics.Total) * 100
			summary.WriteString(fmt.Sprintf("🔄 Work in progress: %.0f%% (%d issues)\n", percentage, inProgress))
		}
	}
	
	return summary.String()
}

// Helper methods

type countItem struct {
	name  string
	count int
}

func (s *Summarizer) getSortedCounts(counts map[string]int) []countItem {
	items := make([]countItem, 0, len(counts))
	for name, count := range counts {
		items = append(items, countItem{name: name, count: count})
	}
	
	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
	})
	
	return items
}

func (s *Summarizer) getTopCounts(counts map[string]int, limit int) map[string]int {
	items := s.getSortedCounts(counts)
	
	result := make(map[string]int)
	for i, item := range items {
		if i >= limit {
			break
		}
		result[item.name] = item.count
	}
	
	return result
}

func (s *Summarizer) truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	
	// Try to truncate at word boundary
	truncated := text[:maxLength]
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > maxLength/2 {
		truncated = text[:lastSpace]
	}
	
	return truncated + "..."
}

// DefaultSummaryOptions returns sensible default options
func DefaultSummaryOptions() SummaryOptions {
	return SummaryOptions{
		Type:         SummaryBrief,
		MaxLength:    300,
		IncludeStats: true,
		GroupBy:      []string{"status", "priority"},
	}
}