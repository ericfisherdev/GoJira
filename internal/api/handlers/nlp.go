package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ericfisherdev/GoJira/internal/nlp"
	"github.com/rs/zerolog/log"
)

// NLPHandler handles natural language processing requests
type NLPHandler struct {
	parser        *nlp.Parser
	disambiguator *nlp.Disambiguator
}

// NewNLPHandler creates a new NLP handler
func NewNLPHandler() *NLPHandler {
	config := &nlp.ParseConfig{
		MaxHistorySize:     100,
		MinConfidence:      0.5,
		EnableSpellCheck:   true,
		EnableContextInfer: true,
		EnableSuggestions:  true,
		MaxSuggestions:     5,
		TimeZone:          "UTC",
	}

	parser := nlp.NewParser(config)
	disambiguator := nlp.NewDisambiguator(parser)

	return &NLPHandler{
		parser:        parser,
		disambiguator: disambiguator,
	}
}

// ProcessCommandRequest represents a request to process natural language
type ProcessCommandRequest struct {
	Command   string            `json:"command" binding:"required"`
	Context   map[string]string `json:"context,omitempty"`
	SessionID string            `json:"sessionId,omitempty"`
}

// ProcessCommandResponse represents the response from processing
type ProcessCommandResponse struct {
	Success        bool                  `json:"success"`
	Intent         *nlp.Intent           `json:"intent,omitempty"`
	Clarifications []nlp.Clarification   `json:"clarifications,omitempty"`
	Suggestions    []nlp.Suggestion      `json:"suggestions,omitempty"`
	Confidence     float64               `json:"confidence"`
	Message        string                `json:"message,omitempty"`
	NextSteps      []string              `json:"nextSteps,omitempty"`
	ExecutionPlan  *ExecutionPlan        `json:"executionPlan,omitempty"`
}

// ExecutionPlan represents the plan to execute the parsed command
type ExecutionPlan struct {
	Action     string                 `json:"action"`
	Endpoint   string                 `json:"endpoint"`
	Method     string                 `json:"method"`
	Parameters map[string]interface{} `json:"parameters"`
	Steps      []ExecutionStep        `json:"steps,omitempty"`
}

// ExecutionStep represents a single step in the execution plan
type ExecutionStep struct {
	Step        string                 `json:"step"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// EntityExtractionRequest represents a request to extract entities
type EntityExtractionRequest struct {
	Text string `json:"text" binding:"required"`
}

// EntityExtractionResponse represents the response from entity extraction
type EntityExtractionResponse struct {
	Entities map[string]nlp.Entity `json:"entities"`
	Success  bool                  `json:"success"`
	Message  string                `json:"message,omitempty"`
}

// SuggestionsRequest represents a request for command suggestions
type SuggestionsRequest struct {
	PartialCommand string `json:"partialCommand"`
	Context        string `json:"context,omitempty"`
}

// SuggestionsResponse represents command suggestions
type SuggestionsResponse struct {
	Suggestions []nlp.Suggestion `json:"suggestions"`
	Examples    []string         `json:"examples"`
}

// ProcessNaturalLanguageCommand processes a natural language command
func (h *NLPHandler) ProcessNaturalLanguageCommand(w http.ResponseWriter, r *http.Request) {
	var req ProcessCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	log.Info().
		Str("command", req.Command).
		Str("sessionId", req.SessionID).
		Msg("Processing natural language command")

	// Apply context if provided
	if req.Context != nil {
		h.applyContext(req.Context, req.SessionID)
	}

	// Parse the command
	parseResult, err := h.parser.Parse(req.Command)
	if err != nil {
		log.Error().Err(err).Str("command", req.Command).Msg("Failed to parse command")
		
		response := ProcessCommandResponse{
			Success:     false,
			Message:     err.Error(),
			Confidence:  0.0,
			Suggestions: h.generateHelpfulSuggestions(req.Command),
		}
		RespondWithJSON(w, http.StatusOK, response)
		return
	}

	// Disambiguate the intent
	disambiguatedIntent, clarifications, err := h.disambiguator.Disambiguate(parseResult.Intent)
	if err != nil {
		log.Error().Err(err).Msg("Failed to disambiguate intent")
		RespondWithError(w, http.StatusInternalServerError, "Failed to process command")
		return
	}

	// Generate execution plan
	executionPlan := h.generateExecutionPlan(disambiguatedIntent)

	// Generate next steps
	nextSteps := h.generateNextSteps(disambiguatedIntent, clarifications)

	response := ProcessCommandResponse{
		Success:        len(clarifications) == 0,
		Intent:         disambiguatedIntent,
		Clarifications: clarifications,
		Suggestions:    parseResult.Suggestions,
		Confidence:     parseResult.Confidence,
		ExecutionPlan:  executionPlan,
		NextSteps:      nextSteps,
	}

	if len(clarifications) > 0 {
		response.Message = "Additional information needed to proceed"
	} else {
		response.Message = "Command understood and ready to execute"
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// ExtractEntities extracts entities from text
func (h *NLPHandler) ExtractEntities(w http.ResponseWriter, r *http.Request) {
	var req EntityExtractionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	log.Debug().Str("text", req.Text).Msg("Extracting entities")

	// Use parser's entity extraction
	entities := h.parser.ExtractEntities(req.Text)

	response := EntityExtractionResponse{
		Entities: entities,
		Success:  true,
	}

	if len(entities) == 0 {
		response.Message = "No entities found in the text"
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// GetCommandSuggestions provides suggestions for commands
func (h *NLPHandler) GetCommandSuggestions(w http.ResponseWriter, r *http.Request) {
	var req SuggestionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, try query parameters
		req.PartialCommand = r.URL.Query().Get("q")
		req.Context = r.URL.Query().Get("context")
	}

	suggestions := h.generateCommandSuggestions(req.PartialCommand, req.Context)
	examples := h.getCommandExamples(req.Context)

	response := SuggestionsResponse{
		Suggestions: suggestions,
		Examples:    examples,
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// ValidateCommand validates a natural language command without executing
func (h *NLPHandler) ValidateCommand(w http.ResponseWriter, r *http.Request) {
	var req ProcessCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Parse the command
	parseResult, err := h.parser.Parse(req.Command)
	if err != nil {
		response := ProcessCommandResponse{
			Success:    false,
			Message:    err.Error(),
			Confidence: 0.0,
		}
		RespondWithJSON(w, http.StatusOK, response)
		return
	}

	// Validate without disambiguating
	validationResult := h.validateIntent(parseResult.Intent)

	response := ProcessCommandResponse{
		Success:    validationResult.Valid,
		Intent:     parseResult.Intent,
		Confidence: parseResult.Confidence,
		Message:    h.getValidationMessage(validationResult),
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// GetParserStatus returns the current parser status and statistics
func (h *NLPHandler) GetParserStatus(w http.ResponseWriter, r *http.Request) {
	context := h.parser.GetContext()
	
	status := map[string]interface{}{
		"sessionId":      context.SessionID,
		"startTime":      context.StartTime,
		"lastProject":    context.LastProject,
		"lastIssue":      context.LastIssue,
		"lastAssignee":   context.LastAssignee,
		"historySize":    len(context.History),
		"cacheStats": h.parser.GetCacheStats(),
	}

	RespondWithJSON(w, http.StatusOK, status)
}

// UpdateContext updates the parser context
func (h *NLPHandler) UpdateContext(w http.ResponseWriter, r *http.Request) {
	var contextData map[string]string
	if err := json.NewDecoder(r.Body).Decode(&contextData); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	h.applyContext(contextData, sessionID)

	RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Context updated successfully",
	})
}

// Helper methods

func (h *NLPHandler) applyContext(contextData map[string]string, sessionID string) {
	context := h.parser.GetContext()
	
	if sessionID != "" {
		context.SessionID = sessionID
	}

	if project, ok := contextData["project"]; ok {
		context.LastProject = project
	}
	if issue, ok := contextData["issue"]; ok {
		context.LastIssue = issue
	}
	if assignee, ok := contextData["assignee"]; ok {
		context.LastAssignee = assignee
	}
	if status, ok := contextData["status"]; ok {
		context.LastStatus = status
	}

	// Update preferences
	for key, value := range contextData {
		if strings.HasPrefix(key, "pref_") {
			context.UserPreferences[key] = value
		}
	}

	h.parser.SetContext(context)
}

func (h *NLPHandler) generateExecutionPlan(intent *nlp.Intent) *ExecutionPlan {
	plan := &ExecutionPlan{
		Action:     intent.Action,
		Parameters: make(map[string]interface{}),
	}

	// Convert entities to parameters
	for key, entity := range intent.Entities {
		plan.Parameters[key] = entity.Value
	}

	// Generate API endpoint and method based on intent type
	switch intent.Type {
	case nlp.IntentCreate:
		plan.Method = "POST"
		plan.Endpoint = "/api/v1/issues"
		plan.Steps = []ExecutionStep{
			{
				Step:        "validate_project",
				Description: "Validate project exists",
			},
			{
				Step:        "create_issue",
				Description: "Create the issue in Jira",
			},
		}

	case nlp.IntentUpdate:
		plan.Method = "PUT"
		if issueKey, ok := intent.Entities["issue_key"]; ok {
			plan.Endpoint = "/api/v1/issues/" + issueKey.Text
		}
		plan.Steps = []ExecutionStep{
			{
				Step:        "validate_issue",
				Description: "Validate issue exists",
			},
			{
				Step:        "update_issue",
				Description: "Update the issue fields",
			},
		}

	case nlp.IntentTransition:
		plan.Method = "POST"
		if issueKey, ok := intent.Entities["issue_key"]; ok {
			plan.Endpoint = "/api/v1/issues/" + issueKey.Text + "/transitions"
		}
		plan.Steps = []ExecutionStep{
			{
				Step:        "get_transitions",
				Description: "Get available transitions",
			},
			{
				Step:        "execute_transition",
				Description: "Execute the transition",
			},
		}

	case nlp.IntentSearch:
		plan.Method = "GET"
		plan.Endpoint = "/api/v1/search"
		plan.Steps = []ExecutionStep{
			{
				Step:        "build_jql",
				Description: "Build JQL query from criteria",
			},
			{
				Step:        "execute_search",
				Description: "Execute the search",
			},
		}

	case nlp.IntentAssign:
		plan.Method = "PUT"
		if issueKey, ok := intent.Entities["issue_key"]; ok {
			plan.Endpoint = "/api/v1/issues/" + issueKey.Text
		}
		plan.Steps = []ExecutionStep{
			{
				Step:        "validate_assignee",
				Description: "Validate assignee exists",
			},
			{
				Step:        "assign_issue",
				Description: "Update issue assignee",
			},
		}
	}

	return plan
}

func (h *NLPHandler) generateNextSteps(intent *nlp.Intent, clarifications []nlp.Clarification) []string {
	if len(clarifications) > 0 {
		// Return clarification steps
		steps := make([]string, len(clarifications))
		for i, clarification := range clarifications {
			steps[i] = clarification.Message
		}
		return steps
	}

	// Return execution steps based on intent
	switch intent.Type {
	case nlp.IntentCreate:
		return []string{
			"Validate project permissions",
			"Create issue with specified details",
			"Return issue key and link",
		}
	case nlp.IntentSearch:
		return []string{
			"Build JQL query from criteria",
			"Execute search against Jira",
			"Format and return results",
		}
	default:
		return []string{"Execute the command"}
	}
}

func (h *NLPHandler) generateHelpfulSuggestions(command string) []nlp.Suggestion {
	// Analyze the failed command for helpful suggestions
	lower := strings.ToLower(command)
	
	suggestions := []nlp.Suggestion{}

	if strings.Contains(lower, "create") || strings.Contains(lower, "new") {
		suggestions = append(suggestions, nlp.Suggestion{
			Text:        "create a bug in PROJECT",
			Description: "Create a new bug issue",
			Example:     "create a bug in MYPROJ with title 'Login fails'",
			Confidence:  0.8,
		})
	}

	if strings.Contains(lower, "find") || strings.Contains(lower, "search") {
		suggestions = append(suggestions, nlp.Suggestion{
			Text:        "find all bugs assigned to me",
			Description: "Search for issues",
			Example:     "find all high priority bugs in MYPROJ",
			Confidence:  0.8,
		})
	}

	if strings.Contains(lower, "move") || strings.Contains(lower, "transition") {
		suggestions = append(suggestions, nlp.Suggestion{
			Text:        "move ISSUE-123 to Done",
			Description: "Transition an issue",
			Example:     "move MYPROJ-456 to In Progress",
			Confidence:  0.8,
		})
	}

	return suggestions
}

func (h *NLPHandler) generateCommandSuggestions(partial string, context string) []nlp.Suggestion {
	suggestions := []nlp.Suggestion{
		{
			Text:        "create a bug",
			Description: "Create a new bug issue",
			Example:     "create a bug in PROJ with high priority",
			Confidence:  0.9,
		},
		{
			Text:        "find all issues assigned to me",
			Description: "Search for your assigned issues",
			Example:     "find all bugs assigned to me in current sprint",
			Confidence:  0.9,
		},
		{
			Text:        "move ISSUE-123 to Done",
			Description: "Transition an issue to new status",
			Example:     "move PROJ-456 to In Progress",
			Confidence:  0.9,
		},
		{
			Text:        "assign ISSUE-123 to john.doe",
			Description: "Assign an issue to someone",
			Example:     "assign PROJ-789 to me",
			Confidence:  0.8,
		},
		{
			Text:        "comment on ISSUE-123: Fixed the bug",
			Description: "Add a comment to an issue",
			Example:     "comment on PROJ-456: Ready for testing",
			Confidence:  0.8,
		},
	}

	// Filter suggestions based on partial input
	if partial != "" {
		filtered := []nlp.Suggestion{}
		for _, suggestion := range suggestions {
			if strings.Contains(strings.ToLower(suggestion.Text), strings.ToLower(partial)) {
				filtered = append(filtered, suggestion)
			}
		}
		return filtered
	}

	return suggestions
}

func (h *NLPHandler) getCommandExamples(context string) []string {
	return []string{
		"create a high priority bug in MYPROJ",
		"find all stories assigned to john.doe",
		"move PROJ-123 to In Progress",
		"assign PROJ-456 to me",
		"search for issues in current sprint",
		"list all open bugs with component frontend",
		"comment on PROJ-789: This is ready for review",
		"link PROJ-123 blocks PROJ-456",
		"generate sprint report for sprint 10",
	}
}

func (h *NLPHandler) validateIntent(intent *nlp.Intent) *nlp.ValidationResult {
	result := &nlp.ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Basic validation
	if intent.Confidence < 0.3 {
		result.Valid = false
		result.Errors = append(result.Errors, "Command confidence too low")
	}

	// Intent-specific validation
	switch intent.Type {
	case nlp.IntentCreate:
		if _, ok := intent.Entities["project"]; !ok {
			result.Warnings = append(result.Warnings, "No project specified")
		}
		if _, ok := intent.Entities["issue_type"]; !ok {
			result.Warnings = append(result.Warnings, "No issue type specified")
		}

	case nlp.IntentUpdate, nlp.IntentTransition:
		if _, ok := intent.Entities["issue_key"]; !ok {
			result.Valid = false
			result.Errors = append(result.Errors, "No issue key specified")
		}
	}

	return result
}

func (h *NLPHandler) getValidationMessage(result *nlp.ValidationResult) string {
	if result.Valid {
		if len(result.Warnings) > 0 {
			return "Command is valid but has warnings: " + strings.Join(result.Warnings, ", ")
		}
		return "Command is valid and ready to execute"
	}
	return "Command validation failed: " + strings.Join(result.Errors, ", ")
}