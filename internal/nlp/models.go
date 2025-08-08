package nlp

import (
	"time"
)

// Intent represents a parsed user intent from natural language
type Intent struct {
	Type       IntentType             `json:"type"`
	Confidence float64                `json:"confidence"`
	Action     string                 `json:"action"`
	Entities   map[string]Entity      `json:"entities"`
	Context    map[string]interface{} `json:"context"`
	Raw        string                 `json:"raw"`
	Timestamp  time.Time              `json:"timestamp"`
}

// IntentType represents the type of action the user wants to perform
type IntentType string

const (
	IntentCreate     IntentType = "CREATE"
	IntentUpdate     IntentType = "UPDATE"
	IntentSearch     IntentType = "SEARCH"
	IntentTransition IntentType = "TRANSITION"
	IntentReport     IntentType = "REPORT"
	IntentDelete     IntentType = "DELETE"
	IntentAssign     IntentType = "ASSIGN"
	IntentComment    IntentType = "COMMENT"
	IntentLink       IntentType = "LINK"
	IntentHelp       IntentType = "HELP"
	IntentUnknown    IntentType = "UNKNOWN"
)

// Entity represents an extracted entity from the input
type Entity struct {
	Type       EntityType  `json:"type"`
	Value      interface{} `json:"value"`
	Text       string      `json:"text"`
	Position   []int       `json:"position"`
	Confidence float64     `json:"confidence"`
	Normalized string      `json:"normalized,omitempty"`
}

// EntityType represents different types of entities we can extract
type EntityType string

const (
	EntityProject     EntityType = "PROJECT"
	EntityIssueType   EntityType = "ISSUE_TYPE"
	EntityIssueKey    EntityType = "ISSUE_KEY"
	EntityPriority    EntityType = "PRIORITY"
	EntityAssignee    EntityType = "ASSIGNEE"
	EntityReporter    EntityType = "REPORTER"
	EntitySprint      EntityType = "SPRINT"
	EntityStatus      EntityType = "STATUS"
	EntityLabel       EntityType = "LABEL"
	EntityComponent   EntityType = "COMPONENT"
	EntityDate        EntityType = "DATE"
	EntityTime        EntityType = "TIME"
	EntityDuration    EntityType = "DURATION"
	EntityNumber      EntityType = "NUMBER"
	EntityText        EntityType = "TEXT"
	EntityFixVersion  EntityType = "FIX_VERSION"
	EntityEpic        EntityType = "EPIC"
	EntityStoryPoints EntityType = "STORY_POINTS"
)

// ParseResult represents the complete result of parsing natural language input
type ParseResult struct {
	Intent          *Intent         `json:"intent"`
	Clarifications  []Clarification `json:"clarifications,omitempty"`
	Suggestions     []Suggestion    `json:"suggestions,omitempty"`
	Confidence      float64         `json:"confidence"`
	AlternateIntents []Intent       `json:"alternateIntents,omitempty"`
}

// Clarification represents a request for clarification from the user
type Clarification struct {
	Field       string   `json:"field"`
	Message     string   `json:"message"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required"`
	EntityType  EntityType `json:"entityType"`
}

// Suggestion represents a suggestion for the user
type Suggestion struct {
	Text        string  `json:"text"`
	Description string  `json:"description,omitempty"`
	Example     string  `json:"example,omitempty"`
	Confidence  float64 `json:"confidence"`
}

// Context maintains conversation context for better understanding
type Context struct {
	LastProject     string            `json:"lastProject,omitempty"`
	LastIssue       string            `json:"lastIssue,omitempty"`
	LastSprint      string            `json:"lastSprint,omitempty"`
	LastAssignee    string            `json:"lastAssignee,omitempty"`
	LastStatus      string            `json:"lastStatus,omitempty"`
	LastSearch      string            `json:"lastSearch,omitempty"`
	History         []Intent          `json:"history,omitempty"`
	UserPreferences map[string]string `json:"userPreferences,omitempty"`
	SessionID       string            `json:"sessionId"`
	StartTime       time.Time         `json:"startTime"`
}

// Project represents a simplified Jira project for NLP context
type Project struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
}

// User represents a simplified Jira user for NLP context
type User struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
	Active      bool   `json:"active"`
}

// Priority represents issue priority levels
type Priority struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Level       int    `json:"level"`
}

// IssueType represents different issue types
type IssueType struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Subtask     bool   `json:"subtask"`
}

// Status represents issue status
type Status struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

// ValidationResult represents the result of validating an intent
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// CommandType represents specific command types for better routing
type CommandType string

const (
	CommandSimple  CommandType = "SIMPLE"
	CommandComplex CommandType = "COMPLEX"
	CommandBatch   CommandType = "BATCH"
	CommandQuery   CommandType = "QUERY"
)

// ParseConfig contains configuration for the parser
type ParseConfig struct {
	MaxHistorySize      int     `json:"maxHistorySize"`
	MinConfidence       float64 `json:"minConfidence"`
	EnableSpellCheck    bool    `json:"enableSpellCheck"`
	EnableContextInfer  bool    `json:"enableContextInfer"`
	EnableSuggestions   bool    `json:"enableSuggestions"`
	MaxSuggestions      int     `json:"maxSuggestions"`
	DefaultProject      string  `json:"defaultProject,omitempty"`
	DefaultAssignee     string  `json:"defaultAssignee,omitempty"`
	TimeZone            string  `json:"timeZone,omitempty"`
}