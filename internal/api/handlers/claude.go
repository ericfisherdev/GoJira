package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ericfisherdev/GoJira/internal/claude"
	"github.com/ericfisherdev/GoJira/internal/nlp"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// ClaudeHandler handles Claude Code integration endpoints
type ClaudeHandler struct {
	integrationManager *claude.IntegrationManager
}

// NewClaudeHandler creates a new Claude integration handler
func NewClaudeHandler() *ClaudeHandler {
	// Initialize NLP parser
	nlpParser := nlp.NewParser(nil)
	
	// Initialize Claude integration manager
	config := claude.DefaultClaudeConfig()
	integrationManager := claude.NewIntegrationManager(nlpParser, config)

	return &ClaudeHandler{
		integrationManager: integrationManager,
	}
}

// ProcessClaudeCommandRequest represents a Claude Code command request
type ProcessClaudeCommandRequest struct {
	UserID         string `json:"userId" binding:"required"`
	ConversationID string `json:"conversationId" binding:"required"`
	Command        string `json:"command" binding:"required"`
}

// SessionStatusRequest represents a session status request
type SessionStatusRequest struct {
	UserID         string `json:"userId" binding:"required"`
	ConversationID string `json:"conversationId" binding:"required"`
}

// UpdatePreferencesRequest represents a user preferences update request
type UpdatePreferencesRequest struct {
	UserID         string                     `json:"userId" binding:"required"`
	ConversationID string                     `json:"conversationId" binding:"required"`
	Preferences    *claude.UserPreferences    `json:"preferences" binding:"required"`
}

// ProcessCommand processes a Claude Code command
func (h *ClaudeHandler) ProcessCommand(w http.ResponseWriter, r *http.Request) {
	var req ProcessClaudeCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	log.Info().
		Str("userId", req.UserID).
		Str("conversationId", req.ConversationID).
		Str("command", req.Command).
		Msg("Processing Claude Code command")

	// Process command with integration manager
	response, err := h.integrationManager.ProcessCommand(req.UserID, req.ConversationID, req.Command)
	if err != nil {
		log.Error().Err(err).Msg("Failed to process Claude command")
		RespondWithError(w, http.StatusInternalServerError, "Failed to process command")
		return
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// GetSessionStatus returns the current session status
func (h *ClaudeHandler) GetSessionStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	conversationID := r.URL.Query().Get("conversationId")

	if userID == "" || conversationID == "" {
		RespondWithError(w, http.StatusBadRequest, "userId and conversationId are required")
		return
	}

	sessionInfo, err := h.integrationManager.GetSessionStatus(userID, conversationID)
	if err != nil {
		log.Debug().Err(err).Msg("Session not found")
		RespondWithError(w, http.StatusNotFound, "Session not found")
		return
	}

	RespondWithJSON(w, http.StatusOK, sessionInfo)
}

// UpdatePreferences updates user preferences for a session
func (h *ClaudeHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	var req UpdatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	err := h.integrationManager.UpdateUserPreferences(req.UserID, req.ConversationID, req.Preferences)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update user preferences")
		RespondWithError(w, http.StatusInternalServerError, "Failed to update preferences")
		return
	}

	RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Preferences updated successfully",
	})
}

// GetAvailableCommands returns all available Claude Code commands and workflows
func (h *ClaudeHandler) GetAvailableCommands(w http.ResponseWriter, r *http.Request) {
	commands := h.integrationManager.GetAvailableCommands()
	RespondWithJSON(w, http.StatusOK, commands)
}

// GetManagerStats returns Claude integration manager statistics
func (h *ClaudeHandler) GetManagerStats(w http.ResponseWriter, r *http.Request) {
	stats := h.integrationManager.GetStats()
	RespondWithJSON(w, http.StatusOK, stats)
}

// GetPatterns returns available Claude-specific patterns
func (h *ClaudeHandler) GetPatterns(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	
	// Get pattern manager through integration manager
	commands := h.integrationManager.GetAvailableCommands()
	
	// Filter by category if specified
	if category != "" {
		filteredCommands := make(map[string]interface{})
		for name, command := range commands {
			if cmdMap, ok := command.(map[string]interface{}); ok {
				if cmdCategory, exists := cmdMap["category"]; exists && cmdCategory == category {
					filteredCommands[name] = command
				}
			}
		}
		RespondWithJSON(w, http.StatusOK, filteredCommands)
		return
	}

	RespondWithJSON(w, http.StatusOK, commands)
}

// GetWorkflows returns available Claude workflows
func (h *ClaudeHandler) GetWorkflows(w http.ResponseWriter, r *http.Request) {
	commands := h.integrationManager.GetAvailableCommands()
	
	if workflows, ok := commands["workflows"]; ok {
		RespondWithJSON(w, http.StatusOK, map[string]interface{}{
			"workflows": workflows,
		})
		return
	}

	RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"workflows": []interface{}{},
	})
}

// StartWorkflow manually starts a specific workflow
func (h *ClaudeHandler) StartWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowId")
	if workflowID == "" {
		RespondWithError(w, http.StatusBadRequest, "workflowId is required")
		return
	}

	var req ProcessClaudeCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Create a command that would trigger this workflow
	triggerCommand := fmt.Sprintf("start workflow %s", workflowID)
	
	response, err := h.integrationManager.ProcessCommand(req.UserID, req.ConversationID, triggerCommand)
	if err != nil {
		log.Error().Err(err).Str("workflowId", workflowID).Msg("Failed to start workflow")
		RespondWithError(w, http.StatusInternalServerError, "Failed to start workflow")
		return
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// ProcessWorkflowStep processes a step in an active workflow
func (h *ClaudeHandler) ProcessWorkflowStep(w http.ResponseWriter, r *http.Request) {
	var req ProcessClaudeCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Process the workflow step input
	response, err := h.integrationManager.ProcessCommand(req.UserID, req.ConversationID, req.Command)
	if err != nil {
		log.Error().Err(err).Msg("Failed to process workflow step")
		RespondWithError(w, http.StatusInternalServerError, "Failed to process workflow step")
		return
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// CancelWorkflow cancels an active workflow
func (h *ClaudeHandler) CancelWorkflow(w http.ResponseWriter, r *http.Request) {
	var req SessionStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Process cancel command
	response, err := h.integrationManager.ProcessCommand(req.UserID, req.ConversationID, "cancel workflow")
	if err != nil {
		log.Error().Err(err).Msg("Failed to cancel workflow")
		RespondWithError(w, http.StatusInternalServerError, "Failed to cancel workflow")
		return
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// GetSuggestions provides contextual suggestions for Claude Code
func (h *ClaudeHandler) GetSuggestions(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	conversationID := r.URL.Query().Get("conversationId")
	context := r.URL.Query().Get("context")

	if userID == "" || conversationID == "" {
		RespondWithError(w, http.StatusBadRequest, "userId and conversationId are required")
		return
	}

	// Get session status to build context for suggestions
	sessionInfo, err := h.integrationManager.GetSessionStatus(userID, conversationID)
	if err != nil {
		log.Debug().Err(err).Msg("Session not found, providing generic suggestions")
		
		// Provide generic suggestions if no session exists
		suggestions := h.getGenericSuggestions()
		RespondWithJSON(w, http.StatusOK, map[string]interface{}{
			"suggestions": suggestions,
			"contextual":  false,
		})
		return
	}

	// For now, return available commands as suggestions
	commands := h.integrationManager.GetAvailableCommands()
	
	suggestions := make([]map[string]interface{}, 0)
	for name, command := range commands {
		if name == "workflows" {
			continue // Skip workflows in suggestions
		}
		
		if cmdMap, ok := command.(map[string]interface{}); ok {
			suggestion := map[string]interface{}{
				"command":     name,
				"description": cmdMap["description"],
				"category":    cmdMap["category"],
				"examples":    cmdMap["examples"],
			}
			suggestions = append(suggestions, suggestion)
		}
	}

	response := map[string]interface{}{
		"suggestions": suggestions,
		"contextual":  true,
		"sessionInfo": sessionInfo,
	}

	if context != "" {
		response["context"] = context
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// getGenericSuggestions returns basic suggestions when no session context is available
func (h *ClaudeHandler) getGenericSuggestions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"command":     "Create a bug from code review findings",
			"description": "Quickly create bug tickets from security vulnerabilities or issues found in code",
			"example":     "Create a bug for the SQL injection vulnerability in user.go line 145",
			"category":    "development",
		},
		{
			"command":     "Find all issues assigned to me",
			"description": "Search for your assigned work across projects",
			"example":     "Show me all critical bugs assigned to me",
			"category":    "search",
		},
		{
			"command":     "Transition multiple issues at once",
			"description": "Batch operations for moving issues to new statuses",
			"example":     "Move all issues in sprint 5 to Done",
			"category":    "batch",
		},
		{
			"command":     "Start guided issue creation",
			"description": "Step-by-step issue creation with intelligent suggestions",
			"example":     "create issue guided",
			"category":    "creation",
		},
		{
			"command":     "Plan a sprint",
			"description": "Guided sprint planning and issue organization",
			"example":     "plan sprint 10",
			"category":    "sprint",
		},
	}
}

// Health check endpoint for Claude integration
func (h *ClaudeHandler) Health(w http.ResponseWriter, r *http.Request) {
	stats := h.integrationManager.GetStats()
	
	health := map[string]interface{}{
		"status":     "healthy",
		"service":    "claude-integration",
		"statistics": stats,
		"timestamp":  time.Now().UTC(),
	}

	RespondWithJSON(w, http.StatusOK, health)
}

// Shutdown gracefully shuts down the Claude handler
func (h *ClaudeHandler) Shutdown() {
	log.Info().Msg("Shutting down Claude handler")
	h.integrationManager.Shutdown()
}