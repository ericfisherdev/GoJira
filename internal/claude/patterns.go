package claude

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ericfisherdev/GoJira/internal/nlp"
	"github.com/rs/zerolog/log"
)

// CommandPattern represents a Claude-specific command pattern
type CommandPattern struct {
	Name        string
	Description string
	Examples    []string
	Pattern     *regexp.Regexp
	Handler     CommandHandler
	Priority    int
	Category    string
}

// CommandHandler processes a command context and returns a result
type CommandHandler func(ctx *CommandContext) (*CommandResult, error)

// CommandContext provides context for command execution
type CommandContext struct {
	Input       string
	Intent      *nlp.Intent
	Session     *Session
	UserID      string
	History     []Command
	Suggestions []Suggestion
}

// Command represents an executed command
type Command struct {
	ID        string
	Input     string
	Intent    *nlp.Intent
	Result    *CommandResult
	Timestamp time.Time
}

// CommandResult represents the result of command execution
type CommandResult struct {
	Success     bool                   `json:"success"`
	Data        interface{}            `json:"data,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Suggestions []Suggestion           `json:"suggestions,omitempty"`
	NextSteps   []string               `json:"nextSteps,omitempty"`
	Markdown    string                 `json:"markdown,omitempty"`
	Actions     []ActionItem           `json:"actions,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ActionItem represents a follow-up action
type ActionItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Priority    int    `json:"priority"`
}

// Suggestion represents a command suggestion
type Suggestion struct {
	Command     string   `json:"command"`
	Description string   `json:"description"`
	Confidence  float64  `json:"confidence"`
	Category    string   `json:"category"`
	Examples    []string `json:"examples,omitempty"`
	Parameters  []string `json:"parameters,omitempty"`
}

// PatternManager manages Claude-specific command patterns
type PatternManager struct {
	patterns []CommandPattern
}

// NewPatternManager creates a new pattern manager with initialized patterns
func NewPatternManager() *PatternManager {
	pm := &PatternManager{
		patterns: InitializePatterns(),
	}
	
	// Sort patterns by priority (descending)
	sort.Slice(pm.patterns, func(i, j int) bool {
		return pm.patterns[i].Priority > pm.patterns[j].Priority
	})
	
	return pm
}

// InitializePatterns creates the default Claude-specific patterns
func InitializePatterns() []CommandPattern {
	return []CommandPattern{
		{
			Name:        "CreateBugFromCode",
			Description: "Create a bug ticket from code review findings",
			Category:    "development",
			Examples: []string{
				"Create a bug for the SQL injection vulnerability in user.go line 145",
				"Report a security issue found in authentication.js",
				"File a bug for the memory leak in worker.py line 89",
			},
			Pattern:  regexp.MustCompile(`(?i)create\s+a?\s*(bug|security\s+issue|vulnerability).*\s+in\s+(\S+)(?:\s+line\s+(\d+))?`),
			Handler:  handleCreateBugFromCode,
			Priority: 10,
		},
		{
			Name:        "BatchTransition",
			Description: "Transition multiple issues at once",
			Category:    "batch",
			Examples: []string{
				"Move all issues in sprint 5 to In Progress",
				"Transition all bugs assigned to me to Done",
				"Set all critical issues to In Review",
			},
			Pattern:  regexp.MustCompile(`(?i)(move|transition)\s+all\s+(.+?)\s+to\s+(\w+(?:\s+\w+)*)`),
			Handler:  handleBatchTransition,
			Priority: 9,
		},
		{
			Name:        "SmartSearch",
			Description: "Natural language search for issues",
			Category:    "search",
			Examples: []string{
				"Show me critical bugs from last week",
				"Find all unassigned issues in the current sprint",
				"List high priority tasks for the frontend team",
			},
			Pattern:  regexp.MustCompile(`(?i)(show|find|list|get)\s+(me\s+)?(.+)`),
			Handler:  handleSmartSearch,
			Priority: 7,
		},
		{
			Name:        "QuickAssign",
			Description: "Assign issues with smart defaults",
			Category:    "assignment",
			Examples: []string{
				"Assign PROJ-123 to the frontend team lead",
				"Give this issue to whoever worked on similar bugs",
				"Assign to the person with least workload",
			},
			Pattern:  regexp.MustCompile(`(?i)assign\s+(\w+-\d+)\s+to\s+(.+)`),
			Handler:  handleQuickAssign,
			Priority: 8,
		},
		{
			Name:        "SprintManagement",
			Description: "Manage sprint operations intelligently",
			Category:    "sprint",
			Examples: []string{
				"Start the next sprint with these issues",
				"Close current sprint and move incomplete items",
				"Add high priority issues to active sprint",
			},
			Pattern:  regexp.MustCompile(`(?i)(start|close|add\s+to)\s+.*(sprint|iteration)`),
			Handler:  handleSprintManagement,
			Priority: 8,
		},
		{
			Name:        "StatusSummary",
			Description: "Generate status summaries and reports",
			Category:    "reporting",
			Examples: []string{
				"Give me a summary of today's progress",
				"Show the team's velocity this sprint",
				"Generate a status report for management",
			},
			Pattern:  regexp.MustCompile(`(?i)(summary|report|status).*\s+(today|this\s+week|this\s+sprint|progress)`),
			Handler:  handleStatusSummary,
			Priority: 6,
		},
		{
			Name:        "LinkRelatedIssues",
			Description: "Smart linking of related issues",
			Category:    "linking",
			Examples: []string{
				"Link this issue to similar bugs",
				"Find and link related tasks",
				"Connect this to the parent epic",
			},
			Pattern:  regexp.MustCompile(`(?i)(link|connect).*\s+(similar|related|parent)`),
			Handler:  handleLinkRelatedIssues,
			Priority: 7,
		},
		{
			Name:        "PriorityTriage",
			Description: "Intelligent priority assignment",
			Category:    "triage",
			Examples: []string{
				"Set priority based on impact and urgency",
				"Triage this issue automatically",
				"Prioritize based on business value",
			},
			Pattern:  regexp.MustCompile(`(?i)(prioritize|triage|set\s+priority).*\s+(automatic|based\s+on|business)`),
			Handler:  handlePriorityTriage,
			Priority: 6,
		},
		{
			Name:        "ComponentAnalysis",
			Description: "Analyze and suggest components",
			Category:    "analysis",
			Examples: []string{
				"Suggest components for this issue",
				"Which team should handle this bug?",
				"Route this to the right component",
			},
			Pattern:  regexp.MustCompile(`(?i)(suggest|route|which\s+team).*\s+(component|team|handle)`),
			Handler:  handleComponentAnalysis,
			Priority: 5,
		},
		{
			Name:        "EstimationHelper",
			Description: "Help with story point estimation",
			Category:    "estimation",
			Examples: []string{
				"Estimate story points for this issue",
				"Compare complexity with similar tasks",
				"Suggest effort based on description",
			},
			Pattern:  regexp.MustCompile(`(?i)(estimate|complexity|effort).*\s+(story\s+points|similar|based\s+on)`),
			Handler:  handleEstimationHelper,
			Priority: 5,
		},
	}
}

// MatchCommand finds the best matching pattern for the input
func (pm *PatternManager) MatchCommand(input string, intent *nlp.Intent) (*CommandPattern, float64) {
	bestMatch := (*CommandPattern)(nil)
	highestScore := 0.0

	for i := range pm.patterns {
		pattern := &pm.patterns[i]
		
		// Pattern matching
		if pattern.Pattern.MatchString(input) {
			score := pm.calculatePatternScore(input, pattern, intent)
			
			log.Debug().
				Str("pattern", pattern.Name).
				Float64("score", score).
				Msg("Pattern match score")
			
			if score > highestScore {
				highestScore = score
				bestMatch = pattern
			}
		}
	}

	return bestMatch, highestScore
}

// calculatePatternScore determines how well a pattern matches the input
func (pm *PatternManager) calculatePatternScore(input string, pattern *CommandPattern, intent *nlp.Intent) float64 {
	score := 0.0

	// Base score from pattern priority
	score += float64(pattern.Priority) * 0.1

	// Pattern match coverage
	matches := pattern.Pattern.FindStringSubmatch(input)
	if len(matches) > 1 {
		coverage := float64(len(matches[0])) / float64(len(input))
		score += coverage * 0.3
	}

	// Intent type alignment
	if intent != nil {
		score += pm.getIntentAlignmentScore(pattern, intent) * 0.4
	}

	// Category bonus for specific contexts
	if pattern.Category != "" {
		score += 0.1
	}

	return score
}

// getIntentAlignmentScore checks how well the pattern aligns with detected intent
func (pm *PatternManager) getIntentAlignmentScore(pattern *CommandPattern, intent *nlp.Intent) float64 {
	alignmentMap := map[string]map[nlp.IntentType]float64{
		"development": {
			nlp.IntentCreate: 0.8,
			nlp.IntentUpdate: 0.6,
		},
		"batch": {
			nlp.IntentTransition: 0.9,
			nlp.IntentUpdate:     0.7,
		},
		"search": {
			nlp.IntentSearch: 0.9,
			nlp.IntentReport: 0.6,
		},
		"assignment": {
			nlp.IntentAssign:  0.9,
			nlp.IntentUpdate:  0.5,
		},
		"sprint": {
			nlp.IntentTransition: 0.8,
			nlp.IntentUpdate:     0.7,
		},
		"reporting": {
			nlp.IntentReport: 0.9,
			nlp.IntentSearch: 0.6,
		},
		"linking": {
			nlp.IntentLink:   0.9,
			nlp.IntentCreate: 0.5,
		},
		"triage": {
			nlp.IntentUpdate: 0.8,
			nlp.IntentCreate: 0.6,
		},
		"analysis": {
			nlp.IntentSearch: 0.7,
			nlp.IntentReport: 0.8,
		},
		"estimation": {
			nlp.IntentUpdate: 0.7,
			nlp.IntentCreate: 0.5,
		},
	}

	if categoryScores, ok := alignmentMap[pattern.Category]; ok {
		if score, ok := categoryScores[intent.Type]; ok {
			return score
		}
	}

	return 0.0
}

// GetPatternsForCategory returns patterns in a specific category
func (pm *PatternManager) GetPatternsForCategory(category string) []CommandPattern {
	var patterns []CommandPattern
	for _, pattern := range pm.patterns {
		if pattern.Category == category {
			patterns = append(patterns, pattern)
		}
	}
	return patterns
}

// GetAllPatterns returns all available patterns
func (pm *PatternManager) GetAllPatterns() []CommandPattern {
	return pm.patterns
}

// Command handler implementations (placeholder implementations)

func handleCreateBugFromCode(ctx *CommandContext) (*CommandResult, error) {
	// Extract file and line information from the intent
	_ = ctx.Intent.Entities
	
	var filename string
	var lineNumber int
	var issueType = "Bug"
	
	// Parse the matched pattern
	if len(ctx.Intent.Raw) > 0 {
		// This is a simplified extraction - in reality, you'd use the pattern matches
		parts := strings.Fields(ctx.Intent.Raw)
		for i, part := range parts {
			if strings.Contains(part, ".") && (strings.Contains(part, ".go") || 
				strings.Contains(part, ".js") || strings.Contains(part, ".py")) {
				filename = part
				// Check if next part is "line" followed by number
				if i+2 < len(parts) && parts[i+1] == "line" {
					if num, err := strconv.Atoi(parts[i+2]); err == nil {
						lineNumber = num
					}
				}
				break
			}
		}
	}

	// Build issue description
	description := fmt.Sprintf("Security vulnerability found in %s", filename)
	if lineNumber > 0 {
		description += fmt.Sprintf(" at line %d", lineNumber)
	}

	result := &CommandResult{
		Success: true,
		Message: fmt.Sprintf("Ready to create %s issue for code review finding", strings.ToLower(issueType)),
		Data: map[string]interface{}{
			"issueType":   issueType,
			"summary":     fmt.Sprintf("Security issue in %s", filename),
			"description": description,
			"priority":    "High",
			"labels":      []string{"security", "code-review"},
		},
		NextSteps: []string{
			"Specify the project where this issue should be created",
			"Add more details about the vulnerability",
			"Set assignee if known",
		},
		Actions: []ActionItem{
			{
				ID:          "create-issue",
				Title:       "Create Issue",
				Description: "Create the bug issue with the gathered information",
				Command:     fmt.Sprintf("create issue %s in PROJECT", issueType),
				Priority:    1,
			},
		},
	}

	return result, nil
}

func handleBatchTransition(ctx *CommandContext) (*CommandResult, error) {
	// This is a placeholder - in reality would perform the batch operation
	return &CommandResult{
		Success: true,
		Message: "Batch transition operation identified",
		Data: map[string]interface{}{
			"operation": "batch_transition",
			"scope":     "multiple_issues",
		},
		NextSteps: []string{
			"Confirm the issues to be transitioned",
			"Verify target status is valid",
			"Execute the batch operation",
		},
	}, nil
}

func handleSmartSearch(ctx *CommandContext) (*CommandResult, error) {
	// This is a placeholder - in reality would execute intelligent search
	return &CommandResult{
		Success: true,
		Message: "Smart search query processed",
		Data: map[string]interface{}{
			"searchType": "natural_language",
			"query":      ctx.Input,
		},
		NextSteps: []string{
			"Execute search with inferred criteria",
			"Present results in requested format",
		},
	}, nil
}

func handleQuickAssign(ctx *CommandContext) (*CommandResult, error) {
	return &CommandResult{
		Success: true,
		Message: "Quick assignment processed",
		Data: map[string]interface{}{
			"operation": "smart_assignment",
		},
	}, nil
}

func handleSprintManagement(ctx *CommandContext) (*CommandResult, error) {
	return &CommandResult{
		Success: true,
		Message: "Sprint management operation identified",
		Data: map[string]interface{}{
			"operation": "sprint_management",
		},
	}, nil
}

func handleStatusSummary(ctx *CommandContext) (*CommandResult, error) {
	return &CommandResult{
		Success: true,
		Message: "Status summary request processed",
		Data: map[string]interface{}{
			"operation": "generate_summary",
		},
	}, nil
}

func handleLinkRelatedIssues(ctx *CommandContext) (*CommandResult, error) {
	return &CommandResult{
		Success: true,
		Message: "Link related issues operation",
		Data: map[string]interface{}{
			"operation": "link_issues",
		},
	}, nil
}

func handlePriorityTriage(ctx *CommandContext) (*CommandResult, error) {
	return &CommandResult{
		Success: true,
		Message: "Priority triage operation",
		Data: map[string]interface{}{
			"operation": "priority_triage",
		},
	}, nil
}

func handleComponentAnalysis(ctx *CommandContext) (*CommandResult, error) {
	return &CommandResult{
		Success: true,
		Message: "Component analysis operation",
		Data: map[string]interface{}{
			"operation": "component_analysis",
		},
	}, nil
}

func handleEstimationHelper(ctx *CommandContext) (*CommandResult, error) {
	return &CommandResult{
		Success: true,
		Message: "Estimation helper operation",
		Data: map[string]interface{}{
			"operation": "estimation_helper",
		},
	}, nil
}