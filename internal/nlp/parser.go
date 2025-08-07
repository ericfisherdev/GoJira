package nlp

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Parser is the main NLP parser for understanding natural language commands
type Parser struct {
	patterns      map[IntentType][]*regexp.Regexp
	entityRules   map[EntityType][]EntityRule
	projectCache  map[string]*Project
	userCache     map[string]*User
	statusCache   map[string]*Status
	priorityCache map[string]*Priority
	config        *ParseConfig
	context       *Context
}

// EntityRule defines a pattern and extraction function for entities
type EntityRule struct {
	Pattern *regexp.Regexp
	Extract func(matches []string) interface{}
	Name    string
}

// NewParser creates a new NLP parser instance
func NewParser(config *ParseConfig) *Parser {
	if config == nil {
		config = DefaultParseConfig()
	}

	p := &Parser{
		patterns:      make(map[IntentType][]*regexp.Regexp),
		entityRules:   make(map[EntityType][]EntityRule),
		projectCache:  make(map[string]*Project),
		userCache:     make(map[string]*User),
		statusCache:   make(map[string]*Status),
		priorityCache: make(map[string]*Priority),
		config:        config,
		context: &Context{
			UserPreferences: make(map[string]string),
			SessionID:       generateSessionID(),
			StartTime:       time.Now(),
		},
	}

	p.initializePatterns()
	p.initializeEntityRules()

	return p
}

// DefaultParseConfig returns default configuration for the parser
func DefaultParseConfig() *ParseConfig {
	return &ParseConfig{
		MaxHistorySize:     100,
		MinConfidence:      0.5,
		EnableSpellCheck:   true,
		EnableContextInfer: true,
		EnableSuggestions:  true,
		MaxSuggestions:     5,
		TimeZone:           "UTC",
	}
}

func (p *Parser) initializePatterns() {
	// Create issue patterns
	p.patterns[IntentCreate] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)create\s+(a\s+)?(\w+)?\s*(issue|ticket|bug|story|task|epic)`),
		regexp.MustCompile(`(?i)new\s+(\w+)?\s*(issue|ticket|bug|story|task|epic)\s*(in|for)?\s*(\w+)?`),
		regexp.MustCompile(`(?i)add\s+(a\s+)?(\w+)?\s*(issue|ticket|bug|story|task|epic)\s+to\s+(\w+)`),
		regexp.MustCompile(`(?i)file\s+a\s+(\w+)?\s*(bug|issue|ticket)`),
		regexp.MustCompile(`(?i)report\s+a?\s*(\w+)?\s*(bug|issue|problem)`),
	}

	// Update issue patterns
	p.patterns[IntentUpdate] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)update\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)change\s+(\w+)\s+of\s+(\w+-\d+)\s+to\s+(.+)`),
		regexp.MustCompile(`(?i)set\s+(\w+)\s+to\s+(.+)\s+for\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)modify\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)edit\s+(\w+-\d+)`),
	}

	// Transition patterns
	p.patterns[IntentTransition] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)move\s+(\w+-\d+)\s+to\s+(\w+)`),
		regexp.MustCompile(`(?i)transition\s+(\w+-\d+)\s+to\s+(\w+)`),
		regexp.MustCompile(`(?i)mark\s+(\w+-\d+)\s+as\s+(\w+)`),
		regexp.MustCompile(`(?i)close\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)resolve\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)reopen\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)start\s+(?:work\s+on\s+)?(\w+-\d+)`),
	}

	// Search patterns
	p.patterns[IntentSearch] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)find\s+all?\s*(\w+)?\s*(issues?)?\s*(in|for)?\s*(\w+)?`),
		regexp.MustCompile(`(?i)show\s+(?:me\s+)?(\w+)?\s*(issues?)?\s*assigned\s+to\s+(\w+)`),
		regexp.MustCompile(`(?i)list\s+(\w+)?\s*(issues?)?\s*in\s+sprint\s+(.+)`),
		regexp.MustCompile(`(?i)search\s+for\s+(.+)`),
		regexp.MustCompile(`(?i)get\s+all?\s*(\w+)?\s*(issues?)?`),
		regexp.MustCompile(`(?i)what\s+(?:issues?\s+)?(?:are|is)\s+(\w+)\s+working\s+on`),
	}

	// Assignment patterns
	p.patterns[IntentAssign] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)assign\s+(\w+-\d+)\s+to\s+(\w+)`),
		regexp.MustCompile(`(?i)reassign\s+(\w+-\d+)\s+to\s+(\w+)`),
		regexp.MustCompile(`(?i)give\s+(\w+-\d+)\s+to\s+(\w+)`),
		regexp.MustCompile(`(?i)unassign\s+(\w+-\d+)`),
	}

	// Comment patterns
	p.patterns[IntentComment] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)comment\s+on\s+(\w+-\d+)\s*:?\s*(.+)`),
		regexp.MustCompile(`(?i)add\s+comment\s+to\s+(\w+-\d+)\s*:?\s*(.+)`),
		regexp.MustCompile(`(?i)note\s+on\s+(\w+-\d+)\s*:?\s*(.+)`),
	}

	// Delete patterns
	p.patterns[IntentDelete] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)delete\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)remove\s+(\w+-\d+)`),
	}

	// Link patterns
	p.patterns[IntentLink] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)link\s+(\w+-\d+)\s+to\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)connect\s+(\w+-\d+)\s+(?:to|with)\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)(\w+-\d+)\s+blocks\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)(\w+-\d+)\s+is\s+blocked\s+by\s+(\w+-\d+)`),
		regexp.MustCompile(`(?i)(\w+-\d+)\s+relates\s+to\s+(\w+-\d+)`),
	}

	// Report patterns
	p.patterns[IntentReport] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:generate\s+)?report\s+(?:for\s+)?(.+)`),
		regexp.MustCompile(`(?i)sprint\s+report\s*(\d+)?`),
		regexp.MustCompile(`(?i)velocity\s+(?:report|chart)`),
		regexp.MustCompile(`(?i)burndown\s+(?:report|chart)`),
	}

	// Help patterns
	p.patterns[IntentHelp] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)help\s*(.+)?`),
		regexp.MustCompile(`(?i)how\s+(?:do\s+I|to)\s+(.+)`),
		regexp.MustCompile(`(?i)what\s+can\s+(?:I|you)\s+do`),
		regexp.MustCompile(`(?i)show\s+(?:me\s+)?commands`),
	}
}

// Parse analyzes the input text and returns a parsed intent
func (p *Parser) Parse(input string) (*ParseResult, error) {
	// Normalize and clean input
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}

	log.Debug().Str("input", input).Msg("Parsing natural language input")

	// Detect intent
	intent := p.detectIntent(input)
	if intent == nil {
		// Try to provide helpful suggestions
		suggestions := p.generateSuggestions(input)
		return &ParseResult{
			Intent: &Intent{
				Type:       IntentUnknown,
				Raw:        input,
				Timestamp:  time.Now(),
				Confidence: 0.0,
			},
			Suggestions: suggestions,
			Confidence:  0.0,
		}, fmt.Errorf("could not understand command: %s", input)
	}

	// Extract entities
	entities := p.extractEntities(input)
	intent.Entities = entities

	// Add context
	intent.Context = p.buildContext(input, intent)

	// Generate parse result
	result := &ParseResult{
		Intent:     intent,
		Confidence: intent.Confidence,
	}

	// Check for clarifications needed
	if p.config.EnableContextInfer {
		clarifications := p.checkClarifications(intent)
		result.Clarifications = clarifications
	}

	// Generate suggestions if enabled
	if p.config.EnableSuggestions && intent.Confidence < 0.8 {
		result.Suggestions = p.generateSuggestions(input)
	}

	// Update context with this intent
	p.updateContext(intent)

	return result, nil
}

func (p *Parser) detectIntent(input string) *Intent {
	bestMatch := &Intent{
		Confidence: 0.0,
		Raw:        input,
		Timestamp:  time.Now(),
	}

	for intentType, patterns := range p.patterns {
		for _, pattern := range patterns {
			if matches := pattern.FindStringSubmatch(input); matches != nil {
				confidence := p.calculateConfidence(input, pattern, matches)

				if confidence > bestMatch.Confidence {
					bestMatch = &Intent{
						Type:       intentType,
						Confidence: confidence,
						Action:     p.extractAction(intentType, matches),
						Raw:        input,
						Timestamp:  time.Now(),
					}
				}
			}
		}
	}

	if bestMatch.Confidence < p.config.MinConfidence {
		return nil
	}

	return bestMatch
}

func (p *Parser) calculateConfidence(input string, pattern *regexp.Regexp, matches []string) float64 {
	// Base confidence from pattern match
	confidence := 0.6

	// Increase confidence based on match coverage
	matchLength := len(matches[0])
	inputLength := len(input)
	coverage := float64(matchLength) / float64(inputLength)
	confidence += coverage * 0.2

	// Increase confidence if we have more captured groups
	if len(matches) > 2 {
		confidence += 0.1
	}

	// Check for known entities
	if p.containsKnownEntities(input) {
		confidence += 0.1
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

func (p *Parser) extractAction(intentType IntentType, matches []string) string {
	// Extract the main action from the matches
	switch intentType {
	case IntentCreate:
		if len(matches) > 2 {
			return fmt.Sprintf("create_%s", strings.ToLower(matches[2]))
		}
		return "create_issue"
	case IntentUpdate:
		return "update_issue"
	case IntentTransition:
		return "transition_issue"
	case IntentSearch:
		return "search_issues"
	case IntentAssign:
		return "assign_issue"
	case IntentComment:
		return "add_comment"
	case IntentDelete:
		return "delete_issue"
	case IntentLink:
		return "link_issues"
	case IntentReport:
		return "generate_report"
	case IntentHelp:
		return "show_help"
	default:
		return "unknown"
	}
}

func (p *Parser) containsKnownEntities(input string) bool {
	upperInput := strings.ToUpper(input)

	// Check for issue keys
	issueKeyPattern := regexp.MustCompile(`\b[A-Z]+-\d+\b`)
	if issueKeyPattern.MatchString(upperInput) {
		return true
	}

	// Check for known projects
	for key := range p.projectCache {
		if strings.Contains(upperInput, strings.ToUpper(key)) {
			return true
		}
	}

	// Check for known users
	for username := range p.userCache {
		if strings.Contains(strings.ToLower(input), strings.ToLower(username)) {
			return true
		}
	}

	return false
}

func (p *Parser) buildContext(input string, intent *Intent) map[string]interface{} {
	context := make(map[string]interface{})

	// Add session context
	context["sessionId"] = p.context.SessionID
	context["timestamp"] = time.Now()

	// Add last used values from context
	if p.context.LastProject != "" {
		context["lastProject"] = p.context.LastProject
	}
	if p.context.LastIssue != "" {
		context["lastIssue"] = p.context.LastIssue
	}
	if p.context.LastAssignee != "" {
		context["lastAssignee"] = p.context.LastAssignee
	}

	// Add user preferences
	if p.config.DefaultProject != "" {
		context["defaultProject"] = p.config.DefaultProject
	}
	if p.config.DefaultAssignee != "" {
		context["defaultAssignee"] = p.config.DefaultAssignee
	}

	return context
}

func (p *Parser) updateContext(intent *Intent) {
	// Update context based on the intent
	if project, ok := intent.Entities["project"]; ok {
		p.context.LastProject = project.Text
	}
	if issue, ok := intent.Entities["issue_key"]; ok {
		p.context.LastIssue = issue.Text
	}
	if assignee, ok := intent.Entities["assignee"]; ok {
		p.context.LastAssignee = assignee.Text
	}
	if status, ok := intent.Entities["status"]; ok {
		p.context.LastStatus = status.Text
	}

	// Add to history
	if p.config.MaxHistorySize > 0 {
		p.context.History = append(p.context.History, *intent)
		if len(p.context.History) > p.config.MaxHistorySize {
			p.context.History = p.context.History[1:]
		}
	}
}

func (p *Parser) checkClarifications(intent *Intent) []Clarification {
	clarifications := []Clarification{}

	// Check based on intent type
	switch intent.Type {
	case IntentCreate:
		if _, ok := intent.Entities["project"]; !ok && p.config.DefaultProject == "" {
			clarifications = append(clarifications, Clarification{
				Field:      "project",
				Message:    "Which project should the issue be created in?",
				Required:   true,
				EntityType: EntityProject,
				Options:    p.getProjectOptions(),
			})
		}
		if _, ok := intent.Entities["issue_type"]; !ok {
			clarifications = append(clarifications, Clarification{
				Field:      "issue_type",
				Message:    "What type of issue would you like to create?",
				Required:   true,
				EntityType: EntityIssueType,
				Options:    []string{"Bug", "Task", "Story", "Epic"},
			})
		}
	case IntentUpdate, IntentTransition, IntentAssign, IntentComment:
		if _, ok := intent.Entities["issue_key"]; !ok {
			clarifications = append(clarifications, Clarification{
				Field:      "issue_key",
				Message:    "Which issue would you like to update?",
				Required:   true,
				EntityType: EntityIssueKey,
			})
		}
		
		// For assign intent, also check for assignee
		if intent.Type == IntentAssign {
			if _, ok := intent.Entities["assignee"]; !ok {
				clarifications = append(clarifications, Clarification{
					Field:      "assignee",
					Message:    "Who should the issue be assigned to?",
					Required:   true,
					EntityType: EntityAssignee,
					Options:    p.getUserOptions(),
				})
			}
		}
	}

	return clarifications
}

func (p *Parser) generateSuggestions(input string) []Suggestion {
	suggestions := []Suggestion{}

	// Suggest based on partial matches
	lowerInput := strings.ToLower(input)

	if strings.Contains(lowerInput, "create") || strings.Contains(lowerInput, "new") {
		suggestions = append(suggestions, Suggestion{
			Text:        "Create a new issue",
			Description: "Create a new issue in a project",
			Example:     "create a bug in PROJECT-KEY",
			Confidence:  0.7,
		})
	}

	if strings.Contains(lowerInput, "find") || strings.Contains(lowerInput, "search") {
		suggestions = append(suggestions, Suggestion{
			Text:        "Search for issues",
			Description: "Find issues based on criteria",
			Example:     "find all bugs assigned to me",
			Confidence:  0.7,
		})
	}

	if strings.Contains(lowerInput, "assign") {
		suggestions = append(suggestions, Suggestion{
			Text:        "Assign an issue",
			Description: "Assign an issue to someone",
			Example:     "assign PROJ-123 to john.doe",
			Confidence:  0.7,
		})
	}

	// Limit suggestions
	if len(suggestions) > p.config.MaxSuggestions {
		suggestions = suggestions[:p.config.MaxSuggestions]
	}

	return suggestions
}

func (p *Parser) getProjectOptions() []string {
	options := []string{}
	for key := range p.projectCache {
		options = append(options, key)
	}
	return options
}

func (p *Parser) getUserOptions() []string {
	options := []string{}
	for username := range p.userCache {
		options = append(options, username)
	}
	return options
}

// SetProjectCache updates the project cache
func (p *Parser) SetProjectCache(projects map[string]*Project) {
	p.projectCache = projects
}

// SetUserCache updates the user cache
func (p *Parser) SetUserCache(users map[string]*User) {
	p.userCache = users
}

// SetContext sets a new context
func (p *Parser) SetContext(context *Context) {
	p.context = context
}

// GetContext returns the current context
func (p *Parser) GetContext() *Context {
	return p.context
}

// ExtractEntities is a public wrapper for extractEntities
func (p *Parser) ExtractEntities(input string) map[string]Entity {
	return p.extractEntities(input)
}

// GetCacheStats returns statistics about the parser caches
func (p *Parser) GetCacheStats() map[string]int {
	return map[string]int{
		"projects": len(p.projectCache),
		"users":    len(p.userCache),
		"statuses": len(p.statusCache),
	}
}

func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}