package claude

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ericfisherdev/GoJira/internal/nlp"
	"github.com/rs/zerolog/log"
)

// SuggestionEngine generates intelligent suggestions for Claude Code
type SuggestionEngine struct {
	patternManager *PatternManager
	sessionManager *SessionManager
	preferences    map[string]*UserPreferences
}

// SuggestionContext provides context for generating suggestions
type SuggestionContext struct {
	UserID         string
	CurrentProject string
	CurrentSprint  string
	RecentCommands []Command
	Intent         *nlp.Intent
	Entities       map[string]nlp.Entity
}

// SuggestionCategory represents different types of suggestions
type SuggestionCategory string

const (
	CategoryFollowUp    SuggestionCategory = "follow-up"
	CategoryWorkflow    SuggestionCategory = "workflow"
	CategoryProject     SuggestionCategory = "project"
	CategoryEfficiency  SuggestionCategory = "efficiency"
	CategoryBestPractice SuggestionCategory = "best-practice"
	CategoryTemplate    SuggestionCategory = "template"
)

// NewSuggestionEngine creates a new suggestion engine
func NewSuggestionEngine(patternManager *PatternManager, sessionManager *SessionManager) *SuggestionEngine {
	return &SuggestionEngine{
		patternManager: patternManager,
		sessionManager: sessionManager,
		preferences:    make(map[string]*UserPreferences),
	}
}

// GetSuggestions generates intelligent suggestions based on context
func (se *SuggestionEngine) GetSuggestions(ctx *CommandContext) []Suggestion {
	suggestions := make([]Suggestion, 0)

	// Get contextual suggestions based on current intent
	if ctx.Intent != nil {
		suggestions = append(suggestions, se.getIntentSuggestions(ctx)...)
	}

	// Add follow-up suggestions based on history
	if len(ctx.History) > 0 {
		suggestions = append(suggestions, se.getFollowUpSuggestions(ctx)...)
	}

	// Add workflow-based suggestions
	suggestions = append(suggestions, se.getWorkflowSuggestions(ctx)...)

	// Add project-specific suggestions
	if ctx.Session != nil && ctx.Session.Project != "" {
		suggestions = append(suggestions, se.getProjectSuggestions(ctx)...)
	}

	// Add efficiency suggestions
	suggestions = append(suggestions, se.getEfficiencySuggestions(ctx)...)

	// Sort by confidence and category priority
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Confidence != suggestions[j].Confidence {
			return suggestions[i].Confidence > suggestions[j].Confidence
		}
		return se.getCategoryPriority(suggestions[i].Category) > se.getCategoryPriority(suggestions[j].Category)
	})

	// Remove duplicates and limit results
	suggestions = se.deduplicateSuggestions(suggestions)
	if len(suggestions) > 8 {
		suggestions = suggestions[:8]
	}

	return suggestions
}

// getIntentSuggestions provides suggestions based on the current intent
func (se *SuggestionEngine) getIntentSuggestions(ctx *CommandContext) []Suggestion {
	suggestions := make([]Suggestion, 0)

	switch ctx.Intent.Type {
	case nlp.IntentCreate:
		suggestions = append(suggestions, se.getCreateSuggestions(ctx)...)
	case nlp.IntentSearch:
		suggestions = append(suggestions, se.getSearchSuggestions(ctx)...)
	case nlp.IntentUpdate:
		suggestions = append(suggestions, se.getUpdateSuggestions(ctx)...)
	case nlp.IntentTransition:
		suggestions = append(suggestions, se.getTransitionSuggestions(ctx)...)
	case nlp.IntentAssign:
		suggestions = append(suggestions, se.getAssignmentSuggestions(ctx)...)
	}

	return suggestions
}

// getCreateSuggestions provides suggestions for create operations
func (se *SuggestionEngine) getCreateSuggestions(ctx *CommandContext) []Suggestion {
	suggestions := []Suggestion{
		{
			Command:     "Add it to the current sprint",
			Description: "Add the created issue to the active sprint",
			Confidence:  0.9,
			Category:    string(CategoryWorkflow),
			Examples:    []string{"add PROJ-123 to current sprint"},
		},
		{
			Command:     "Assign it to me",
			Description: "Assign the created issue to yourself",
			Confidence:  0.85,
			Category:    string(CategoryWorkflow),
			Examples:    []string{"assign PROJ-123 to me"},
		},
		{
			Command:     "Set priority based on impact",
			Description: "Automatically determine priority based on description",
			Confidence:  0.8,
			Category:    string(CategoryEfficiency),
			Examples:    []string{"analyze PROJ-123 and set priority"},
		},
		{
			Command:     "Link to related issues",
			Description: "Find and link similar or related issues",
			Confidence:  0.75,
			Category:    string(CategoryBestPractice),
			Examples:    []string{"link PROJ-123 to related issues"},
		},
	}

	// Add project-specific suggestions
	if project, ok := ctx.Intent.Entities["project"]; ok {
		if projectStr, ok := project.Value.(string); ok {
			suggestions = append(suggestions, Suggestion{
				Command:     fmt.Sprintf("Use %s issue templates", projectStr),
				Description: "Apply project-specific issue templates",
				Confidence:  0.7,
				Category:    string(CategoryTemplate),
				Examples:    []string{fmt.Sprintf("apply %s bug template to PROJ-123", projectStr)},
			})
		}
	}

	// Add issue type specific suggestions
	if issueType, ok := ctx.Intent.Entities["issue_type"]; ok {
		if typeStr, ok := issueType.Value.(string); ok {
			switch strings.ToLower(typeStr) {
			case "bug":
				suggestions = append(suggestions, Suggestion{
					Command:     "Add reproduction steps template",
					Description: "Include standard bug reproduction template",
					Confidence:  0.8,
					Category:    string(CategoryTemplate),
					Examples:    []string{"add reproduction steps to PROJ-123"},
				})
			case "story":
				suggestions = append(suggestions, Suggestion{
					Command:     "Add acceptance criteria template",
					Description: "Include standard acceptance criteria",
					Confidence:  0.8,
					Category:    string(CategoryTemplate),
					Examples:    []string{"add acceptance criteria to PROJ-123"},
				})
			}
		}
	}

	return suggestions
}

// getSearchSuggestions provides suggestions for search operations
func (se *SuggestionEngine) getSearchSuggestions(ctx *CommandContext) []Suggestion {
	return []Suggestion{
		{
			Command:     "Save this search as a filter",
			Description: "Create a reusable filter from this search",
			Confidence:  0.8,
			Category:    string(CategoryEfficiency),
			Examples:    []string{"save search as 'My Open Issues'"},
		},
		{
			Command:     "Export results to spreadsheet",
			Description: "Export search results for analysis",
			Confidence:  0.7,
			Category:    string(CategoryEfficiency),
			Examples:    []string{"export search results to CSV"},
		},
		{
			Command:     "Set up notifications for this query",
			Description: "Get notified when matching issues change",
			Confidence:  0.6,
			Category:    string(CategoryWorkflow),
			Examples:    []string{"notify me about new critical bugs"},
		},
		{
			Command:     "Generate report from results",
			Description: "Create a formatted report from the search",
			Confidence:  0.75,
			Category:    string(CategoryEfficiency),
			Examples:    []string{"generate weekly status report"},
		},
	}
}

// getUpdateSuggestions provides suggestions for update operations
func (se *SuggestionEngine) getUpdateSuggestions(ctx *CommandContext) []Suggestion {
	suggestions := []Suggestion{
		{
			Command:     "Add a comment explaining the change",
			Description: "Document why this update was made",
			Confidence:  0.85,
			Category:    string(CategoryBestPractice),
			Examples:    []string{"comment on PROJ-123: Updated priority due to customer escalation"},
		},
		{
			Command:     "Notify stakeholders of the change",
			Description: "Let relevant people know about this update",
			Confidence:  0.8,
			Category:    string(CategoryWorkflow),
			Examples:    []string{"notify watchers about PROJ-123 update"},
		},
	}

	// Add field-specific suggestions
	if priority, ok := ctx.Intent.Entities["priority"]; ok {
		if strings.Contains(strings.ToLower(priority.Text), "high") {
			suggestions = append(suggestions, Suggestion{
				Command:     "Add to current sprint",
				Description: "High priority issues should be in the active sprint",
				Confidence:  0.9,
				Category:    string(CategoryBestPractice),
				Examples:    []string{"move PROJ-123 to current sprint"},
			})
		}
	}

	return suggestions
}

// getTransitionSuggestions provides suggestions for transition operations
func (se *SuggestionEngine) getTransitionSuggestions(ctx *CommandContext) []Suggestion {
	return []Suggestion{
		{
			Command:     "Log time spent on this issue",
			Description: "Track time investment for reporting",
			Confidence:  0.8,
			Category:    string(CategoryBestPractice),
			Examples:    []string{"log 2h work on PROJ-123"},
		},
		{
			Command:     "Update remaining estimate",
			Description: "Adjust time estimates based on progress",
			Confidence:  0.7,
			Category:    string(CategoryBestPractice),
			Examples:    []string{"set remaining estimate to 4h for PROJ-123"},
		},
		{
			Command:     "Link to pull request",
			Description: "Connect the issue to related code changes",
			Confidence:  0.85,
			Category:    string(CategoryWorkflow),
			Examples:    []string{"link PROJ-123 to PR #456"},
		},
	}
}

// getAssignmentSuggestions provides suggestions for assignment operations
func (se *SuggestionEngine) getAssignmentSuggestions(ctx *CommandContext) []Suggestion {
	return []Suggestion{
		{
			Command:     "Check assignee's current workload",
			Description: "Ensure the assignee isn't overloaded",
			Confidence:  0.8,
			Category:    string(CategoryBestPractice),
			Examples:    []string{"show john.doe's current workload"},
		},
		{
			Command:     "Set realistic due date",
			Description: "Add appropriate deadline for the assignee",
			Confidence:  0.75,
			Category:    string(CategoryBestPractice),
			Examples:    []string{"set due date based on priority and workload"},
		},
		{
			Command:     "Add to assignee's current sprint",
			Description: "Include in their active sprint for visibility",
			Confidence:  0.7,
			Category:    string(CategoryWorkflow),
			Examples:    []string{"add PROJ-123 to john.doe's sprint"},
		},
	}
}

// getFollowUpSuggestions provides suggestions based on command history
func (se *SuggestionEngine) getFollowUpSuggestions(ctx *CommandContext) []Suggestion {
	suggestions := make([]Suggestion, 0)
	
	if len(ctx.History) == 0 {
		return suggestions
	}

	lastCommand := ctx.History[len(ctx.History)-1]
	
	// Analyze last command result
	if lastCommand.Result != nil && lastCommand.Result.Success {
		switch {
		case strings.Contains(strings.ToLower(lastCommand.Input), "create"):
			suggestions = append(suggestions, Suggestion{
				Command:     "View the created issue",
				Description: "Check the details of the issue just created",
				Confidence:  0.9,
				Category:    string(CategoryFollowUp),
				Examples:    []string{"show PROJ-123"},
			})
			
		case strings.Contains(strings.ToLower(lastCommand.Input), "search"):
			suggestions = append(suggestions, Suggestion{
				Command:     "Refine the search results",
				Description: "Add more filters to narrow down results",
				Confidence:  0.8,
				Category:    string(CategoryFollowUp),
				Examples:    []string{"add priority=high to search"},
			})
			
		case strings.Contains(strings.ToLower(lastCommand.Input), "assign"):
			suggestions = append(suggestions, Suggestion{
				Command:     "Set due date for assigned issue",
				Description: "Add deadline for the assigned work",
				Confidence:  0.85,
				Category:    string(CategoryFollowUp),
				Examples:    []string{"set due date to next Friday"},
			})
		}
	}

	// Pattern-based follow-ups
	if len(ctx.History) >= 2 {
		patterns := se.detectCommandPatterns(ctx.History)
		for _, pattern := range patterns {
			suggestions = append(suggestions, pattern)
		}
	}

	return suggestions
}

// getWorkflowSuggestions provides workflow optimization suggestions
func (se *SuggestionEngine) getWorkflowSuggestions(ctx *CommandContext) []Suggestion {
	suggestions := []Suggestion{
		{
			Command:     "Set up daily standup reminders",
			Description: "Get daily summaries of your assigned work",
			Confidence:  0.6,
			Category:    string(CategoryWorkflow),
			Examples:    []string{"enable daily standup notifications"},
		},
		{
			Command:     "Create a dashboard for your work",
			Description: "Get a personalized view of your issues",
			Confidence:  0.65,
			Category:    string(CategoryEfficiency),
			Examples:    []string{"create my work dashboard"},
		},
	}

	// Add time-based suggestions
	now := time.Now()
	if now.Hour() >= 9 && now.Hour() <= 17 && now.Weekday() >= time.Monday && now.Weekday() <= time.Friday {
		suggestions = append(suggestions, Suggestion{
			Command:     "Start a work session timer",
			Description: "Track time spent on current task",
			Confidence:  0.7,
			Category:    string(CategoryEfficiency),
			Examples:    []string{"start timer for PROJ-123"},
		})
	}

	return suggestions
}

// getProjectSuggestions provides project-specific suggestions
func (se *SuggestionEngine) getProjectSuggestions(ctx *CommandContext) []Suggestion {
	if ctx.Session == nil || ctx.Session.Project == "" {
		return []Suggestion{}
	}

	project := ctx.Session.Project
	
	return []Suggestion{
		{
			Command:     fmt.Sprintf("Show %s project metrics", project),
			Description: "View current project health and progress",
			Confidence:  0.7,
			Category:    string(CategoryProject),
			Examples:    []string{fmt.Sprintf("generate %s status report", project)},
		},
		{
			Command:     fmt.Sprintf("List %s team members", project),
			Description: "See who's working on this project",
			Confidence:  0.6,
			Category:    string(CategoryProject),
			Examples:    []string{fmt.Sprintf("show %s team assignments", project)},
		},
	}
}

// getEfficiencySuggestions provides efficiency improvement suggestions
func (se *SuggestionEngine) getEfficiencySuggestions(ctx *CommandContext) []Suggestion {
	return []Suggestion{
		{
			Command:     "Use keyboard shortcuts",
			Description: "Learn shortcuts to work faster",
			Confidence:  0.5,
			Category:    string(CategoryEfficiency),
			Examples:    []string{"show keyboard shortcuts"},
		},
		{
			Command:     "Batch similar operations",
			Description: "Group similar tasks for efficiency",
			Confidence:  0.6,
			Category:    string(CategoryEfficiency),
			Examples:    []string{"transition all ready issues to done"},
		},
	}
}

// detectCommandPatterns analyzes command history for patterns
func (se *SuggestionEngine) detectCommandPatterns(history []Command) []Suggestion {
	suggestions := make([]Suggestion, 0)
	
	if len(history) < 2 {
		return suggestions
	}

	// Look for create -> assign pattern
	lastTwo := history[len(history)-2:]
	if strings.Contains(strings.ToLower(lastTwo[0].Input), "create") &&
		strings.Contains(strings.ToLower(lastTwo[1].Input), "assign") {
		
		suggestions = append(suggestions, Suggestion{
			Command:     "Add to sprint and set priority",
			Description: "Complete the issue setup workflow",
			Confidence:  0.8,
			Category:    string(CategoryFollowUp),
			Examples:    []string{"add to current sprint and set high priority"},
		})
	}

	return suggestions
}

// getCategoryPriority returns priority weight for suggestion categories
func (se *SuggestionEngine) getCategoryPriority(category string) int {
	priorities := map[string]int{
		string(CategoryFollowUp):    10,
		string(CategoryWorkflow):    9,
		string(CategoryBestPractice): 8,
		string(CategoryEfficiency):  7,
		string(CategoryProject):     6,
		string(CategoryTemplate):    5,
	}
	
	if priority, ok := priorities[category]; ok {
		return priority
	}
	return 0
}

// deduplicateSuggestions removes duplicate suggestions
func (se *SuggestionEngine) deduplicateSuggestions(suggestions []Suggestion) []Suggestion {
	seen := make(map[string]bool)
	result := make([]Suggestion, 0)

	for _, suggestion := range suggestions {
		key := fmt.Sprintf("%s:%s", suggestion.Command, suggestion.Category)
		if !seen[key] {
			seen[key] = true
			result = append(result, suggestion)
		}
	}

	return result
}

// GenerateContextualHelp provides help suggestions based on current context
func (se *SuggestionEngine) GenerateContextualHelp(ctx *CommandContext) []Suggestion {
	help := []Suggestion{
		{
			Command:     "create a bug in PROJECT",
			Description: "Create a new bug issue",
			Confidence:  1.0,
			Category:    "help",
			Examples:    []string{"create a high priority bug in MYPROJ"},
		},
		{
			Command:     "find all issues assigned to me",
			Description: "Search for your assigned issues",
			Confidence:  1.0,
			Category:    "help",
			Examples:    []string{"find all critical issues assigned to me"},
		},
		{
			Command:     "move PROJ-123 to Done",
			Description: "Transition an issue to a new status",
			Confidence:  1.0,
			Category:    "help",
			Examples:    []string{"move PROJ-123 to In Progress"},
		},
		{
			Command:     "assign PROJ-123 to john.doe",
			Description: "Assign an issue to someone",
			Confidence:  1.0,
			Category:    "help",
			Examples:    []string{"assign PROJ-123 to me"},
		},
	}

	// Add context-specific help
	if ctx.Session != nil && ctx.Session.Project != "" {
		help = append(help, Suggestion{
			Command:     fmt.Sprintf("show %s project status", ctx.Session.Project),
			Description: "Get project overview and metrics",
			Confidence:  0.9,
			Category:    "help",
			Examples:    []string{fmt.Sprintf("generate %s weekly report", ctx.Session.Project)},
		})
	}

	return help
}

// UpdateUserPreferences stores user preferences for better suggestions
func (se *SuggestionEngine) UpdateUserPreferences(userID string, prefs *UserPreferences) {
	se.preferences[userID] = prefs
	log.Debug().Str("userID", userID).Msg("Updated user preferences for suggestions")
}

// GetUserPreferences retrieves user preferences
func (se *SuggestionEngine) GetUserPreferences(userID string) *UserPreferences {
	if prefs, ok := se.preferences[userID]; ok {
		return prefs
	}
	
	// Return default preferences
	return &UserPreferences{
		AutoAssign:        false,
		NotifyOnCreate:    true,
		PreferredPriority: "Medium",
	}
}