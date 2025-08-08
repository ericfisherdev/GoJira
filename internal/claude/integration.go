package claude

import (
	"fmt"
	"time"

	"github.com/ericfisherdev/GoJira/internal/cache"
	"github.com/ericfisherdev/GoJira/internal/nlp"
	"github.com/rs/zerolog/log"
)

// IntegrationManager is the main Claude Code integration manager
type IntegrationManager struct {
	patternManager    *PatternManager
	sessionManager    *SessionManager
	workflowEngine    *WorkflowEngine
	suggestionEngine  *SuggestionEngine
	nlpParser         *nlp.Parser
	config            *ClaudeConfig
	responseCache     *cache.ResponseCache
	formatter         *ResponseFormatter
	summarizer        *Summarizer
}

// ClaudeConfig holds configuration for Claude Code integration
type ClaudeConfig struct {
	SessionTTL         time.Duration  `json:"sessionTTL"`
	EnableWorkflows    bool           `json:"enableWorkflows"`
	EnableSuggestions  bool           `json:"enableSuggestions"`
	MaxSessionsPerUser int            `json:"maxSessionsPerUser"`
	WorkflowTimeout    time.Duration  `json:"workflowTimeout"`
	SuggestionLimit    int            `json:"suggestionLimit"`
	VerboseResponses   bool           `json:"verboseResponses"`
	EnableContextCache bool           `json:"enableContextCache"`
	EnableResponseCache bool          `json:"enableResponseCache"`
	ResponseCacheSize  int            `json:"responseCacheSize"`
	ResponseCacheTTL   time.Duration  `json:"responseCacheTTL"`
	DefaultFormat      ResponseFormat `json:"defaultFormat"`
	OptimizeForClaude  bool           `json:"optimizeForClaude"`
}

// IntegrationResponse represents a response optimized for Claude Code integration
type IntegrationResponse struct {
	Success         bool                   `json:"success"`
	Intent          *nlp.Intent            `json:"intent,omitempty"`
	Command         *CommandResult         `json:"command,omitempty"`
	Suggestions     []Suggestion           `json:"suggestions,omitempty"`
	WorkflowStatus  *WorkflowExecution     `json:"workflowStatus,omitempty"`
	SessionInfo     *SessionInfo           `json:"sessionInfo,omitempty"`
	NextSteps       []string               `json:"nextSteps,omitempty"`
	ErrorMessage    string                 `json:"errorMessage,omitempty"`
	Confidence      float64                `json:"confidence"`
	ProcessingTime  time.Duration          `json:"processingTime"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	// Response optimization fields
	FormattedResponse string                `json:"formattedResponse,omitempty"`
	Format            ResponseFormat        `json:"format,omitempty"`
	Summary           string                `json:"summary,omitempty"`
	CacheHit          bool                  `json:"cacheHit,omitempty"`
	OptimizationStats OptimizationStats     `json:"optimizationStats,omitempty"`
}

// OptimizationStats tracks response optimization metrics
type OptimizationStats struct {
	FormattingTime time.Duration `json:"formattingTime"`
	CacheCheckTime time.Duration `json:"cacheCheckTime"`
	CompressionRatio float64     `json:"compressionRatio,omitempty"`
	TokenEstimate   int          `json:"tokenEstimate,omitempty"`
}

// SessionInfo provides context about the current session
type SessionInfo struct {
	ID               string                 `json:"id"`
	CommandCount     int                    `json:"commandCount"`
	WorkflowActive   bool                   `json:"workflowActive"`
	WorkflowStep     string                 `json:"workflowStep,omitempty"`
	WorkflowProgress float64                `json:"workflowProgress,omitempty"`
	Context          map[string]interface{} `json:"context,omitempty"`
}

// NewIntegrationManager creates a new Claude Code integration manager
func NewIntegrationManager(nlpParser *nlp.Parser, config *ClaudeConfig) *IntegrationManager {
	if config == nil {
		config = DefaultClaudeConfig()
	}

	patternManager := NewPatternManager()
	sessionManager := NewSessionManager(config.SessionTTL)
	workflowEngine := NewWorkflowEngine(sessionManager, patternManager)
	suggestionEngine := NewSuggestionEngine(patternManager, sessionManager)

	// Initialize response optimization components
	var responseCache *cache.ResponseCache
	if config.EnableResponseCache {
		responseCache = cache.NewResponseCache(config.ResponseCacheSize, config.ResponseCacheTTL)
	}

	// Configure formatter for Claude Code optimization
	formatterConfig := FormatterConfig{
		IncludeMetadata:      true,
		UseMarkdown:          config.OptimizeForClaude,
		SummarizeResults:     true,
		MaxDescriptionLength: 300,
		DefaultFormat:        config.DefaultFormat,
		Verbose:              config.VerboseResponses,
		MaxResults:           50,
	}

	formatter := NewResponseFormatter(formatterConfig)
	summarizer := NewSummarizer(500, []string{"status", "priority", "assignee"})

	return &IntegrationManager{
		patternManager:   patternManager,
		sessionManager:   sessionManager,
		workflowEngine:   workflowEngine,
		suggestionEngine: suggestionEngine,
		nlpParser:        nlpParser,
		config:           config,
		responseCache:    responseCache,
		formatter:        formatter,
		summarizer:       summarizer,
	}
}

// DefaultClaudeConfig returns default configuration for Claude Code integration
func DefaultClaudeConfig() *ClaudeConfig {
	return &ClaudeConfig{
		SessionTTL:          time.Hour * 2,
		EnableWorkflows:     true,
		EnableSuggestions:   true,
		MaxSessionsPerUser:  5,
		WorkflowTimeout:     time.Minute * 30,
		SuggestionLimit:     8,
		VerboseResponses:    true,
		EnableContextCache:  true,
		EnableResponseCache: true,
		ResponseCacheSize:   1000,
		ResponseCacheTTL:    time.Minute * 15,
		DefaultFormat:       FormatMarkdown,
		OptimizeForClaude:   true,
	}
}

// ProcessCommand is the main entry point for processing Claude Code commands
func (m *IntegrationManager) ProcessCommand(userID, conversationID, input string) (*IntegrationResponse, error) {
	return m.ProcessCommandWithFormat(userID, conversationID, input, m.config.DefaultFormat)
}

// ProcessCommandWithFormat processes a command with a specific output format
func (m *IntegrationManager) ProcessCommandWithFormat(userID, conversationID, input string, format ResponseFormat) (*IntegrationResponse, error) {
	startTime := time.Now()
	
	log.Info().
		Str("userId", userID).
		Str("conversationId", conversationID).
		Str("input", input).
		Str("format", string(format)).
		Msg("Processing Claude Code command")

	// Check cache first if enabled
	var cacheKey string
	var cacheCheckStart time.Time
	if m.config.EnableResponseCache && m.responseCache != nil {
		cacheCheckStart = time.Now()
		params := map[string]string{
			"input":          input,
			"conversationId": conversationID,
		}
		cacheKey = cache.GenerateUserKey(userID, "command", params, string(format))
		
		if cachedResponse, exists := m.responseCache.Get(cacheKey); exists {
			if response, ok := cachedResponse.(*IntegrationResponse); ok {
				// Update cache hit metrics
				response.CacheHit = true
				response.OptimizationStats.CacheCheckTime = time.Since(cacheCheckStart)
				response.ProcessingTime = time.Since(startTime)
				
				log.Debug().
					Str("cacheKey", cacheKey).
					Msg("Cache hit for command")
				
				return response, nil
			}
		}
	}

	// Get or create session
	session := m.getOrCreateSession(userID, conversationID)
	
	// Build command context
	ctx, err := m.sessionManager.GetSessionContext(session.ID)
	if err != nil {
		return m.errorResponseWithOptimization(fmt.Sprintf("Failed to get session context: %v", err), startTime, format), err
	}

	ctx.Input = input
	ctx.UserID = userID

	// Parse with NLP
	parseResult, err := m.nlpParser.Parse(input)
	if err != nil {
		log.Debug().Err(err).Str("input", input).Msg("NLP parsing failed, trying Claude patterns")
		
		// Fallback to Claude-specific patterns
		return m.processWithPatternsOptimized(ctx, startTime, format, cacheKey)
	}

	ctx.Intent = parseResult.Intent

	// Check for workflow trigger first
	if m.config.EnableWorkflows {
		if workflowID := m.workflowEngine.DetectWorkflowTrigger(input); workflowID != "" {
			return m.handleWorkflowCommandOptimized(ctx, workflowID, startTime, format, cacheKey)
		}

		// Handle ongoing workflow
		if session.IsWorkflowActive() {
			return m.continueWorkflowOptimized(ctx, startTime, format, cacheKey)
		}
	}

	// Try Claude-specific patterns
	if claudeResponse := m.tryClaudePatternsOptimized(ctx, startTime, format, cacheKey); claudeResponse != nil {
		return claudeResponse, nil
	}

	// Handle standard NLP result
	return m.processNLPResultOptimized(ctx, parseResult, startTime, format, cacheKey)
}

// getOrCreateSession gets existing session or creates new one
func (m *IntegrationManager) getOrCreateSession(userID, conversationID string) *Session {
	// Try to find existing session for this conversation
	userSessions := m.sessionManager.GetUserSessions(userID)
	for _, session := range userSessions {
		if session.ConversationID == conversationID && session.IsActive {
			return session
		}
	}

	// Create new session
	return m.sessionManager.CreateSession(userID, conversationID)
}

// processWithPatterns handles input using Claude-specific patterns
func (m *IntegrationManager) processWithPatterns(ctx *CommandContext, startTime time.Time) (*IntegrationResponse, error) {
	// Try to match Claude patterns without NLP
	pattern, score := m.patternManager.MatchCommand(ctx.Input, nil)
	
	if pattern == nil || score < 0.3 {
		return m.errorResponse("Could not understand command. Please try rephrasing.", startTime), nil
	}

	// Execute pattern handler
	result, err := pattern.Handler(ctx)
	if err != nil {
		return m.errorResponse(fmt.Sprintf("Command execution failed: %v", err), startTime), err
	}

	// Update session
	command := Command{
		ID:        generateIntegrationCommandID(),
		Input:     ctx.Input,
		Result:    result,
		Timestamp: time.Now(),
	}
	m.sessionManager.UpdateSession(ctx.Session.ID, command)

	// Build response
	response := &IntegrationResponse{
		Success:        result.Success,
		Command:        result,
		Confidence:     score,
		ProcessingTime: time.Since(startTime),
		SessionInfo:    m.buildSessionInfo(ctx.Session),
	}

	// Add suggestions if enabled
	if m.config.EnableSuggestions {
		suggestions := m.suggestionEngine.GetSuggestions(ctx)
		response.Suggestions = m.limitSuggestions(suggestions)
	}

	return response, nil
}

// tryClaudePatterns attempts to match and execute Claude-specific patterns
func (m *IntegrationManager) tryClaudePatterns(ctx *CommandContext, startTime time.Time) *IntegrationResponse {
	pattern, score := m.patternManager.MatchCommand(ctx.Input, ctx.Intent)
	
	if pattern == nil || score < 0.5 {
		return nil
	}

	log.Debug().
		Str("pattern", pattern.Name).
		Float64("score", score).
		Msg("Matched Claude pattern")

	// Execute pattern handler
	result, err := pattern.Handler(ctx)
	if err != nil {
		log.Error().Err(err).Str("pattern", pattern.Name).Msg("Pattern handler failed")
		return m.errorResponse(fmt.Sprintf("Command failed: %v", err), startTime)
	}

	// Update session with command
	command := Command{
		ID:        generateIntegrationCommandID(),
		Input:     ctx.Input,
		Intent:    ctx.Intent,
		Result:    result,
		Timestamp: time.Now(),
	}
	m.sessionManager.UpdateSession(ctx.Session.ID, command)

	// Build enhanced response
	response := &IntegrationResponse{
		Success:        result.Success,
		Intent:         ctx.Intent,
		Command:        result,
		Confidence:     score,
		ProcessingTime: time.Since(startTime),
		SessionInfo:    m.buildSessionInfo(ctx.Session),
		NextSteps:      result.NextSteps,
		Metadata: map[string]interface{}{
			"pattern": pattern.Name,
			"category": pattern.Category,
		},
	}

	// Add suggestions
	if m.config.EnableSuggestions && len(result.Suggestions) > 0 {
		response.Suggestions = m.limitSuggestions(result.Suggestions)
	} else if m.config.EnableSuggestions {
		suggestions := m.suggestionEngine.GetSuggestions(ctx)
		response.Suggestions = m.limitSuggestions(suggestions)
	}

	return response
}

// handleWorkflowCommand starts a new workflow
func (m *IntegrationManager) handleWorkflowCommand(ctx *CommandContext, workflowID string, startTime time.Time) (*IntegrationResponse, error) {
	execution, err := m.workflowEngine.StartWorkflow(ctx, workflowID)
	if err != nil {
		return m.errorResponse(fmt.Sprintf("Failed to start workflow: %v", err), startTime), err
	}

	// Get first step prompt
	templates := m.workflowEngine.GetAvailableWorkflows()
	var template *WorkflowTemplate
	for _, t := range templates {
		if t.ID == workflowID {
			template = t
			break
		}
	}

	response := &IntegrationResponse{
		Success:         true,
		WorkflowStatus:  execution,
		ProcessingTime:  time.Since(startTime),
		SessionInfo:     m.buildSessionInfo(ctx.Session),
		Confidence:      0.9,
		Metadata: map[string]interface{}{
			"workflowStarted": true,
		},
	}

	if template != nil {
		response.Metadata["workflowName"] = template.Name
		
		// Add first step prompt
		if len(template.Steps) > 0 {
			firstStep := template.Steps[0]
			response.NextSteps = []string{firstStep.Prompt}
			response.Metadata["currentStep"] = firstStep.Name
		}
	}

	return response, nil
}

// continueWorkflow processes the next step in an active workflow
func (m *IntegrationManager) continueWorkflow(ctx *CommandContext, startTime time.Time) (*IntegrationResponse, error) {
	execution, err := m.workflowEngine.GetWorkflowStatus(ctx.Session.ID)
	if err != nil {
		return m.errorResponse(fmt.Sprintf("Failed to get workflow status: %v", err), startTime), err
	}

	// Process the workflow step
	stepResult, err := m.workflowEngine.ProcessWorkflowStep(execution, ctx.Input)
	if err != nil {
		log.Error().Err(err).Str("sessionId", ctx.Session.ID).Msg("Workflow step failed")
		return m.errorResponse(fmt.Sprintf("Workflow step failed: %v", err), startTime), err
	}

	response := &IntegrationResponse{
		Success:         stepResult.Status == StatusCompleted,
		WorkflowStatus:  execution,
		ProcessingTime:  time.Since(startTime),
		SessionInfo:     m.buildSessionInfo(ctx.Session),
		Confidence:      0.9,
		Metadata: map[string]interface{}{
			"stepCompleted": stepResult.StepID,
			"stepResult":    stepResult.Data,
		},
	}

	// Add next step or completion message
	if execution.Status == StatusCompleted {
		response.NextSteps = []string{"Workflow completed successfully!"}
		response.Metadata["workflowCompleted"] = true
	} else if execution.Status == StatusFailed {
		response.Success = false
		response.ErrorMessage = execution.ErrorMessage
	} else {
		// Get next step prompt
		templates := m.workflowEngine.GetAvailableWorkflows()
		for _, template := range templates {
			if template.ID == execution.TemplateID && execution.CurrentStep < len(template.Steps) {
				nextStep := template.Steps[execution.CurrentStep]
				response.NextSteps = []string{nextStep.Prompt}
				response.Metadata["currentStep"] = nextStep.Name
				break
			}
		}
	}

	return response, nil
}

// processNLPResult handles standard NLP parsing results
func (m *IntegrationManager) processNLPResult(ctx *CommandContext, parseResult *nlp.ParseResult, startTime time.Time) (*IntegrationResponse, error) {
	// Create a generic command result
	result := &CommandResult{
		Success: true,
		Message: fmt.Sprintf("Understood %s intent", parseResult.Intent.Type),
		Data: map[string]interface{}{
			"intentType":   parseResult.Intent.Type,
			"confidence":   parseResult.Intent.Confidence,
			"entities":     parseResult.Intent.Entities,
		},
	}

	// Update session
	command := Command{
		ID:        generateIntegrationCommandID(),
		Input:     ctx.Input,
		Intent:    parseResult.Intent,
		Result:    result,
		Timestamp: time.Now(),
	}
	m.sessionManager.UpdateSession(ctx.Session.ID, command)

	response := &IntegrationResponse{
		Success:        true,
		Intent:         parseResult.Intent,
		Command:        result,
		Confidence:     parseResult.Intent.Confidence,
		ProcessingTime: time.Since(startTime),
		SessionInfo:    m.buildSessionInfo(ctx.Session),
		NextSteps:      []string{"Execute the identified action"},
	}

	// Add suggestions
	if m.config.EnableSuggestions {
		suggestions := m.suggestionEngine.GetSuggestions(ctx)
		response.Suggestions = m.limitSuggestions(suggestions)
	}

	return response, nil
}

// buildSessionInfo creates session information for response
func (m *IntegrationManager) buildSessionInfo(session *Session) *SessionInfo {
	info := &SessionInfo{
		ID:           session.ID,
		CommandCount: len(session.CommandHistory),
		Context:      session.Context,
	}

	if session.IsWorkflowActive() {
		info.WorkflowActive = true
		info.WorkflowProgress = session.GetWorkflowProgress()
		
		step, total := session.GetCurrentWorkflowStep()
		if total > 0 {
			info.WorkflowStep = fmt.Sprintf("%d/%d", step, total)
		}
	}

	return info
}

// limitSuggestions limits suggestions based on configuration
func (m *IntegrationManager) limitSuggestions(suggestions []Suggestion) []Suggestion {
	if len(suggestions) <= m.config.SuggestionLimit {
		return suggestions
	}
	return suggestions[:m.config.SuggestionLimit]
}

// errorResponse creates an error response
func (m *IntegrationManager) errorResponse(message string, startTime time.Time) *IntegrationResponse {
	return &IntegrationResponse{
		Success:        false,
		ErrorMessage:   message,
		Confidence:     0.0,
		ProcessingTime: time.Since(startTime),
	}
}

// GetSessionStatus returns current session status
func (m *IntegrationManager) GetSessionStatus(userID, conversationID string) (*SessionInfo, error) {
	userSessions := m.sessionManager.GetUserSessions(userID)
	for _, session := range userSessions {
		if session.ConversationID == conversationID && session.IsActive {
			return m.buildSessionInfo(session), nil
		}
	}

	return nil, fmt.Errorf("session not found for conversation: %s", conversationID)
}

// UpdateUserPreferences updates preferences for a user's session
func (m *IntegrationManager) UpdateUserPreferences(userID, conversationID string, prefs *UserPreferences) error {
	userSessions := m.sessionManager.GetUserSessions(userID)
	for _, session := range userSessions {
		if session.ConversationID == conversationID && session.IsActive {
			return m.sessionManager.UpdateUserPreferences(session.ID, prefs)
		}
	}

	return fmt.Errorf("session not found for conversation: %s", conversationID)
}

// GetAvailableCommands returns available commands and examples
func (m *IntegrationManager) GetAvailableCommands() map[string]interface{} {
	patterns := m.patternManager.GetAllPatterns()
	workflows := m.workflowEngine.GetAvailableWorkflows()

	commands := make(map[string]interface{})
	
	// Add pattern-based commands
	for _, pattern := range patterns {
		commands[pattern.Name] = map[string]interface{}{
			"description": pattern.Description,
			"examples":    pattern.Examples,
			"category":    pattern.Category,
		}
	}

	// Add workflow commands
	workflowCommands := make([]map[string]interface{}, len(workflows))
	for i, workflow := range workflows {
		workflowCommands[i] = map[string]interface{}{
			"id":          workflow.ID,
			"name":        workflow.Name,
			"description": workflow.Description,
			"triggers":    workflow.Triggers,
			"category":    workflow.Category,
			"steps":       len(workflow.Steps),
		}
	}
	
	commands["workflows"] = workflowCommands
	
	return commands
}

// GetStats returns manager statistics
func (m *IntegrationManager) GetStats() map[string]interface{} {
	sessionStats := m.sessionManager.GetStats()
	
	stats := map[string]interface{}{
		"sessions":   sessionStats,
		"patterns":   len(m.patternManager.GetAllPatterns()),
		"workflows":  len(m.workflowEngine.GetAvailableWorkflows()),
		"config":     m.config,
	}

	return stats
}

// Shutdown gracefully shuts down the integration manager
func (m *IntegrationManager) Shutdown() {
	log.Info().Msg("Shutting down Claude Code integration manager")
	m.sessionManager.Stop()
	if m.responseCache != nil {
		m.responseCache.Stop()
	}
}

// Response optimization methods

// optimizeResponse applies formatting and caching to a response
func (m *IntegrationManager) optimizeResponse(response *IntegrationResponse, format ResponseFormat, cacheKey string) *IntegrationResponse {
	formatStart := time.Now()
	
	// Apply formatting if requested and different from default JSON
	if format != FormatJSON && m.formatter != nil {
		if response.Command != nil && response.Command.Data != nil {
			if formatted, err := m.formatter.Format(response.Command.Data, format); err == nil {
				response.FormattedResponse = formatted
				response.Format = format
			}
		}
		
		// Generate summary if not present
		if response.Summary == "" && m.summarizer != nil {
			if response.Command != nil && response.Command.Data != nil {
				opts := DefaultSummaryOptions()
				if m.config.VerboseResponses {
					opts.Type = SummaryDetailed
				}
				response.Summary = m.summarizer.SummarizeIssues(nil, opts) // Would need proper data conversion
			}
		}
	}
	
	// Update optimization stats
	response.OptimizationStats.FormattingTime = time.Since(formatStart)
	response.OptimizationStats.TokenEstimate = m.estimateTokens(response)
	
	// Cache the response if caching is enabled
	if m.config.EnableResponseCache && m.responseCache != nil && cacheKey != "" && response.Success {
		tags := []string{"command"}
		if response.Intent != nil {
			tags = append(tags, string(response.Intent.Type))
		}
		
		m.responseCache.Set(cacheKey, response, 
			cache.WithFormat(string(format)),
			cache.WithTags(tags...),
			cache.WithTTL(m.config.ResponseCacheTTL))
	}
	
	return response
}

// processWithPatternsOptimized handles input using Claude-specific patterns with optimization
func (m *IntegrationManager) processWithPatternsOptimized(ctx *CommandContext, startTime time.Time, format ResponseFormat, cacheKey string) (*IntegrationResponse, error) {
	response, err := m.processWithPatterns(ctx, startTime)
	if err != nil {
		return response, err
	}
	
	return m.optimizeResponse(response, format, cacheKey), nil
}

// tryClaudePatternsOptimized attempts to match and execute Claude-specific patterns with optimization
func (m *IntegrationManager) tryClaudePatternsOptimized(ctx *CommandContext, startTime time.Time, format ResponseFormat, cacheKey string) *IntegrationResponse {
	response := m.tryClaudePatterns(ctx, startTime)
	if response == nil {
		return nil
	}
	
	return m.optimizeResponse(response, format, cacheKey)
}

// handleWorkflowCommandOptimized starts a new workflow with optimization
func (m *IntegrationManager) handleWorkflowCommandOptimized(ctx *CommandContext, workflowID string, startTime time.Time, format ResponseFormat, cacheKey string) (*IntegrationResponse, error) {
	response, err := m.handleWorkflowCommand(ctx, workflowID, startTime)
	if err != nil {
		return response, err
	}
	
	return m.optimizeResponse(response, format, cacheKey), nil
}

// continueWorkflowOptimized processes the next step in an active workflow with optimization
func (m *IntegrationManager) continueWorkflowOptimized(ctx *CommandContext, startTime time.Time, format ResponseFormat, cacheKey string) (*IntegrationResponse, error) {
	response, err := m.continueWorkflow(ctx, startTime)
	if err != nil {
		return response, err
	}
	
	return m.optimizeResponse(response, format, cacheKey), nil
}

// processNLPResultOptimized handles standard NLP parsing results with optimization
func (m *IntegrationManager) processNLPResultOptimized(ctx *CommandContext, parseResult *nlp.ParseResult, startTime time.Time, format ResponseFormat, cacheKey string) (*IntegrationResponse, error) {
	response, err := m.processNLPResult(ctx, parseResult, startTime)
	if err != nil {
		return response, err
	}
	
	return m.optimizeResponse(response, format, cacheKey), nil
}

// errorResponseWithOptimization creates an error response with optimization features
func (m *IntegrationManager) errorResponseWithOptimization(message string, startTime time.Time, format ResponseFormat) *IntegrationResponse {
	response := m.errorResponse(message, startTime)
	
	// Convert to optimized response format
	optimizedResponse := &IntegrationResponse{
		Success:        response.Success,
		ErrorMessage:   response.ErrorMessage,
		Confidence:     response.Confidence,
		ProcessingTime: response.ProcessingTime,
		Format:         format,
		CacheHit:       false,
		OptimizationStats: OptimizationStats{
			FormattingTime: 0,
			CacheCheckTime: 0,
		},
	}
	
	// Apply error formatting if needed
	if format == FormatMarkdown {
		optimizedResponse.FormattedResponse = fmt.Sprintf("❌ **Error**: %s", message)
	}
	
	return optimizedResponse
}

// estimateTokens provides a rough estimate of token count for Claude
func (m *IntegrationManager) estimateTokens(response *IntegrationResponse) int {
	// Rough estimation: 1 token ≈ 4 characters for English text
	tokenCount := 0
	
	if response.FormattedResponse != "" {
		tokenCount += len(response.FormattedResponse) / 4
	} else if response.Command != nil && response.Command.Message != "" {
		tokenCount += len(response.Command.Message) / 4
	}
	
	if response.Summary != "" {
		tokenCount += len(response.Summary) / 4
	}
	
	// Add estimates for other fields
	tokenCount += len(response.NextSteps) * 10 // Rough estimate for next steps
	tokenCount += len(response.Suggestions) * 15 // Rough estimate for suggestions
	
	return tokenCount
}

// GetCacheStats returns cache performance statistics
func (m *IntegrationManager) GetCacheStats() map[string]interface{} {
	stats := map[string]interface{}{
		"cacheEnabled": m.config.EnableResponseCache,
	}
	
	if m.responseCache != nil {
		cacheStats := m.responseCache.GetStats()
		stats["hits"] = cacheStats.Hits
		stats["misses"] = cacheStats.Misses
		stats["hitRate"] = cacheStats.HitRate
		stats["size"] = cacheStats.Size
		stats["memoryUsed"] = cacheStats.MemoryUsed
		stats["evictions"] = cacheStats.Evictions
	}
	
	return stats
}

// InvalidateCache provides cache invalidation capabilities
func (m *IntegrationManager) InvalidateCache(pattern string) int {
	if m.responseCache == nil {
		return 0
	}
	
	return m.responseCache.InvalidatePattern(pattern)
}

// InvalidateCacheByTag provides tag-based cache invalidation
func (m *IntegrationManager) InvalidateCacheByTag(tag string) int {
	if m.responseCache == nil {
		return 0
	}
	
	return m.responseCache.InvalidateTag(tag)
}

func generateIntegrationCommandID() string {
	return fmt.Sprintf("cmd_%d", time.Now().UnixNano())
}