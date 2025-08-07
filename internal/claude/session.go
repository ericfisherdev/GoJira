package claude

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Session represents a conversation session with Claude Code
type Session struct {
	ID              string                 `json:"id"`
	UserID          string                 `json:"userId"`
	Project         string                 `json:"project,omitempty"`
	StartTime       time.Time              `json:"startTime"`
	LastActivity    time.Time              `json:"lastActivity"`
	CommandHistory  []Command              `json:"commandHistory"`
	Context         map[string]interface{} `json:"context"`
	Preferences     *UserPreferences       `json:"preferences,omitempty"`
	IsActive        bool                   `json:"isActive"`
	ConversationID  string                 `json:"conversationId,omitempty"`
	WorkflowState   *WorkflowState         `json:"workflowState,omitempty"`
}

// WorkflowState tracks multi-step command workflows
type WorkflowState struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	CurrentStep   int                    `json:"currentStep"`
	TotalSteps    int                    `json:"totalSteps"`
	StepData      map[string]interface{} `json:"stepData"`
	CompletedAt   time.Time              `json:"completedAt,omitempty"`
	IsCompleted   bool                   `json:"isCompleted"`
}

// UserPreferences stores user-specific preferences for Claude interactions
type UserPreferences struct {
	DefaultProject      string            `json:"defaultProject,omitempty"`
	AutoAssign          bool              `json:"autoAssign"`
	NotifyOnCreate      bool              `json:"notifyOnCreate"`
	PreferredPriority   string            `json:"preferredPriority"`
	TimezoneOffset      int               `json:"timezoneOffset"`
	VerboseResponses    bool              `json:"verboseResponses"`
	ShowSuggestions     bool              `json:"showSuggestions"`
	CustomCommands      map[string]string `json:"customCommands,omitempty"`
	WorkingHours        *WorkingHours     `json:"workingHours,omitempty"`
}

// WorkingHours defines user's preferred working hours
type WorkingHours struct {
	StartHour int      `json:"startHour"`
	EndHour   int      `json:"endHour"`
	Weekdays  []string `json:"weekdays"`
	Timezone  string   `json:"timezone"`
}

// SessionManager manages conversation sessions
type SessionManager struct {
	sessions    map[string]*Session
	userSessions map[string][]string // userID -> sessionIDs
	mu          sync.RWMutex
	ttl         time.Duration
	cleanup     *time.Ticker
	stopCh      chan struct{}
}

// NewSessionManager creates a new session manager
func NewSessionManager(sessionTTL time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions:     make(map[string]*Session),
		userSessions: make(map[string][]string),
		ttl:          sessionTTL,
		stopCh:       make(chan struct{}),
	}

	// Start cleanup goroutine
	sm.cleanup = time.NewTicker(sessionTTL / 4)
	go sm.cleanupExpiredSessions()

	return sm
}

// CreateSession creates a new conversation session
func (sm *SessionManager) CreateSession(userID, conversationID string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionID := generateSessionID()
	now := time.Now()

	session := &Session{
		ID:              sessionID,
		UserID:          userID,
		StartTime:       now,
		LastActivity:    now,
		CommandHistory:  make([]Command, 0),
		Context:         make(map[string]interface{}),
		IsActive:        true,
		ConversationID:  conversationID,
		Preferences:     sm.getDefaultPreferences(),
	}

	// Initialize context with defaults
	session.Context["sessionStart"] = now
	session.Context["commandCount"] = 0

	sm.sessions[sessionID] = session

	// Track session by user
	if sm.userSessions[userID] == nil {
		sm.userSessions[userID] = make([]string, 0)
	}
	sm.userSessions[userID] = append(sm.userSessions[userID], sessionID)

	log.Info().
		Str("sessionId", sessionID).
		Str("userId", userID).
		Str("conversationId", conversationID).
		Msg("Created new Claude Code session")

	return session
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if exists && sm.isSessionExpired(session) {
		return nil, false
	}

	return session, exists
}

// UpdateSession updates session activity and context
func (sm *SessionManager) UpdateSession(sessionID string, command Command) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	now := time.Now()
	session.LastActivity = now

	// Add command to history
	session.CommandHistory = append(session.CommandHistory, command)

	// Limit history size
	maxHistory := 50
	if len(session.CommandHistory) > maxHistory {
		session.CommandHistory = session.CommandHistory[len(session.CommandHistory)-maxHistory:]
	}

	// Update context
	session.Context["lastCommand"] = command.Input
	if command.Intent != nil {
		session.Context["lastIntent"] = command.Intent.Type
		
		// Update project context if available
		if project, ok := command.Intent.Entities["project"]; ok {
			session.Project = project.Value.(string)
			session.Context["currentProject"] = session.Project
		}
	}
	session.Context["commandCount"] = len(session.CommandHistory)

	// Update workflow state if in progress
	if session.WorkflowState != nil && !session.WorkflowState.IsCompleted {
		sm.updateWorkflowState(session, command)
	}

	log.Debug().
		Str("sessionId", sessionID).
		Str("command", command.Input).
		Msg("Updated session with command")

	return nil
}

// GetUserSessions retrieves all active sessions for a user
func (sm *SessionManager) GetUserSessions(userID string) []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessionIDs := sm.userSessions[userID]
	if sessionIDs == nil {
		return []*Session{}
	}

	sessions := make([]*Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if session, exists := sm.sessions[sessionID]; exists && !sm.isSessionExpired(session) {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// StartWorkflow initiates a multi-step workflow
func (sm *SessionManager) StartWorkflow(sessionID, workflowType string, totalSteps int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	workflowID := generateWorkflowID()
	session.WorkflowState = &WorkflowState{
		ID:          workflowID,
		Type:        workflowType,
		CurrentStep: 1,
		TotalSteps:  totalSteps,
		StepData:    make(map[string]interface{}),
		IsCompleted: false,
	}

	log.Info().
		Str("sessionId", sessionID).
		Str("workflowId", workflowID).
		Str("workflowType", workflowType).
		Int("totalSteps", totalSteps).
		Msg("Started multi-step workflow")

	return nil
}

// CompleteWorkflowStep advances workflow to next step
func (sm *SessionManager) CompleteWorkflowStep(sessionID string, stepData map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.WorkflowState == nil {
		return fmt.Errorf("no active workflow in session: %s", sessionID)
	}

	workflow := session.WorkflowState

	// Store step data
	for key, value := range stepData {
		workflow.StepData[key] = value
	}

	// Advance to next step
	workflow.CurrentStep++

	// Check if workflow is completed
	if workflow.CurrentStep > workflow.TotalSteps {
		workflow.IsCompleted = true
		workflow.CompletedAt = time.Now()

		log.Info().
			Str("sessionId", sessionID).
			Str("workflowId", workflow.ID).
			Msg("Completed multi-step workflow")
	}

	return nil
}

// GetSessionContext builds context for command processing
func (sm *SessionManager) GetSessionContext(sessionID string) (*CommandContext, error) {
	session, exists := sm.GetSession(sessionID)
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Build command history
	history := make([]Command, len(session.CommandHistory))
	copy(history, session.CommandHistory)

	return &CommandContext{
		Session: session,
		UserID:  session.UserID,
		History: history,
	}, nil
}

// UpdateUserPreferences updates user preferences for a session
func (sm *SessionManager) UpdateUserPreferences(sessionID string, prefs *UserPreferences) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Preferences = prefs
	session.LastActivity = time.Now()

	log.Debug().
		Str("sessionId", sessionID).
		Msg("Updated user preferences")

	return nil
}

// CloseSession marks a session as inactive
func (sm *SessionManager) CloseSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.IsActive = false

	// Complete any active workflow
	if session.WorkflowState != nil && !session.WorkflowState.IsCompleted {
		session.WorkflowState.IsCompleted = true
		session.WorkflowState.CompletedAt = time.Now()
	}

	log.Info().
		Str("sessionId", sessionID).
		Str("userId", session.UserID).
		Dur("duration", time.Since(session.StartTime)).
		Int("commands", len(session.CommandHistory)).
		Msg("Closed Claude Code session")

	return nil
}

// GetStats returns session manager statistics
func (sm *SessionManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	activeCount := 0
	totalCommands := 0
	activeWorkflows := 0

	for _, session := range sm.sessions {
		if session.IsActive && !sm.isSessionExpired(session) {
			activeCount++
			totalCommands += len(session.CommandHistory)
			if session.WorkflowState != nil && !session.WorkflowState.IsCompleted {
				activeWorkflows++
			}
		}
	}

	return map[string]interface{}{
		"totalSessions":    len(sm.sessions),
		"activeSessions":   activeCount,
		"totalUsers":       len(sm.userSessions),
		"totalCommands":    totalCommands,
		"activeWorkflows":  activeWorkflows,
		"sessionTTL":       sm.ttl.String(),
	}
}

// Helper methods

func (sm *SessionManager) isSessionExpired(session *Session) bool {
	return time.Since(session.LastActivity) > sm.ttl
}

func (sm *SessionManager) updateWorkflowState(session *Session, command Command) {
	// Update workflow context based on command
	workflow := session.WorkflowState
	stepKey := fmt.Sprintf("step_%d", workflow.CurrentStep)

	workflow.StepData[stepKey] = map[string]interface{}{
		"command":   command.Input,
		"intent":    command.Intent.Type,
		"timestamp": command.Timestamp,
		"success":   command.Result != nil && command.Result.Success,
	}
}

func (sm *SessionManager) cleanupExpiredSessions() {
	for {
		select {
		case <-sm.cleanup.C:
			sm.performCleanup()
		case <-sm.stopCh:
			sm.cleanup.Stop()
			return
		}
	}
}

func (sm *SessionManager) performCleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	expiredCount := 0

	// Find and remove expired sessions
	for sessionID, session := range sm.sessions {
		if time.Since(session.LastActivity) > sm.ttl {
			// Remove from user sessions
			userSessions := sm.userSessions[session.UserID]
			for i, id := range userSessions {
				if id == sessionID {
					sm.userSessions[session.UserID] = append(userSessions[:i], userSessions[i+1:]...)
					break
				}
			}

			// Remove session
			delete(sm.sessions, sessionID)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		log.Debug().
			Int("expiredSessions", expiredCount).
			Msg("Cleaned up expired sessions")
	}
}

func (sm *SessionManager) getDefaultPreferences() *UserPreferences {
	return &UserPreferences{
		AutoAssign:        false,
		NotifyOnCreate:    true,
		PreferredPriority: "Medium",
		TimezoneOffset:    0,
		VerboseResponses:  true,
		ShowSuggestions:   true,
		CustomCommands:    make(map[string]string),
		WorkingHours: &WorkingHours{
			StartHour: 9,
			EndHour:   17,
			Weekdays:  []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"},
			Timezone:  "UTC",
		},
	}
}

func (sm *SessionManager) Stop() {
	close(sm.stopCh)
}

func generateWorkflowID() string {
	return fmt.Sprintf("workflow_%d", time.Now().UnixNano())
}

// IsWorkflowActive checks if session has an active workflow
func (s *Session) IsWorkflowActive() bool {
	return s.WorkflowState != nil && !s.WorkflowState.IsCompleted
}

// GetCurrentWorkflowStep returns current workflow step info
func (s *Session) GetCurrentWorkflowStep() (int, int) {
	if s.WorkflowState == nil {
		return 0, 0
	}
	return s.WorkflowState.CurrentStep, s.WorkflowState.TotalSteps
}

// GetWorkflowProgress returns workflow completion percentage
func (s *Session) GetWorkflowProgress() float64 {
	if s.WorkflowState == nil || s.WorkflowState.TotalSteps == 0 {
		return 0.0
	}

	if s.WorkflowState.IsCompleted {
		return 1.0
	}

	return float64(s.WorkflowState.CurrentStep-1) / float64(s.WorkflowState.TotalSteps)
}

// GetRecentCommands returns the last N commands from history
func (s *Session) GetRecentCommands(count int) []Command {
	if len(s.CommandHistory) <= count {
		return s.CommandHistory
	}
	return s.CommandHistory[len(s.CommandHistory)-count:]
}

func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}