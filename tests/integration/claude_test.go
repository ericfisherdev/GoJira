package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/api/handlers"
	"github.com/ericfisherdev/GoJira/internal/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeIntegration(t *testing.T) {
	// Setup Claude handler
	handler := handlers.NewClaudeHandler()

	t.Run("Process Claude Command", func(t *testing.T) {
		testCases := []struct {
			name           string
			command        string
			expectSuccess  bool
			expectWorkflow bool
			expectIntent   bool
		}{
			{
				name:          "Create Bug from Code",
				command:       "Create a bug for the SQL injection vulnerability in user.go line 145",
				expectSuccess: true,
				expectIntent:  true,
			},
			{
				name:          "Batch Transition",
				command:       "Move all issues in sprint 5 to Done",
				expectSuccess: true,
				expectIntent:  true,
			},
			{
				name:          "Smart Search",
				command:       "Show me critical bugs from last week",
				expectSuccess: true,
				expectIntent:  true,
			},
			{
				name:           "Start Workflow",
				command:        "create issue guided",
				expectSuccess:  true,
				expectWorkflow: true,
			},
			{
				name:           "Sprint Planning",
				command:        "plan sprint 10",
				expectSuccess:  true,
				expectWorkflow: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := handlers.ProcessClaudeCommandRequest{
					UserID:         "test-user",
					ConversationID: "test-conversation",
					Command:        tc.command,
				}

				body, err := json.Marshal(req)
				require.NoError(t, err)

				httpReq, err := http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer(body))
				require.NoError(t, err)
				httpReq.Header.Set("Content-Type", "application/json")

				rr := httptest.NewRecorder()
				handler.ProcessCommand(rr, httpReq)

				if tc.expectSuccess {
					assert.Equal(t, http.StatusOK, rr.Code)

					var response claude.IntegrationResponse
					err := json.Unmarshal(rr.Body.Bytes(), &response)
					require.NoError(t, err)

					assert.True(t, response.Success, "Expected successful response for: %s", tc.command)
					assert.Greater(t, response.Confidence, 0.0)
					assert.NotEmpty(t, response.SessionInfo)
					assert.Greater(t, response.ProcessingTime, time.Duration(0))

					if tc.expectWorkflow {
						assert.NotNil(t, response.WorkflowStatus, "Expected workflow to be started")
						assert.NotEmpty(t, response.NextSteps, "Expected next steps for workflow")
					}

					if tc.expectIntent {
						// Should have either intent from NLP or pattern matching
						assert.True(t, response.Intent != nil || response.Command != nil)
					}
				} else {
					assert.NotEqual(t, http.StatusOK, rr.Code)
				}
			})
		}
	})

	t.Run("Workflow Execution", func(t *testing.T) {
		userID := "workflow-test-user"
		conversationID := "workflow-test-conversation"

		// Step 1: Start guided issue creation workflow
		req := handlers.ProcessClaudeCommandRequest{
			UserID:         userID,
			ConversationID: conversationID,
			Command:        "create issue guided",
		}

		body, err := json.Marshal(req)
		require.NoError(t, err)

		httpReq, err := http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer(body))
		require.NoError(t, err)
		httpReq.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ProcessCommand(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response claude.IntegrationResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.NotNil(t, response.WorkflowStatus)
		assert.NotEmpty(t, response.NextSteps)
		assert.True(t, response.SessionInfo.WorkflowActive)

		// Step 2: Provide project selection
		req.Command = "TESTPROJ"
		body, err = json.Marshal(req)
		require.NoError(t, err)

		httpReq, err = http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer(body))
		require.NoError(t, err)
		httpReq.Header.Set("Content-Type", "application/json")

		rr = httptest.NewRecorder()
		handler.ProcessCommand(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should still be in workflow, asking for next step
		assert.True(t, response.SessionInfo.WorkflowActive)
		assert.NotEmpty(t, response.NextSteps)

		// Step 3: Cancel workflow
		req.Command = "cancel workflow"
		body, err = json.Marshal(req)
		require.NoError(t, err)

		httpReq, err = http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer(body))
		require.NoError(t, err)
		httpReq.Header.Set("Content-Type", "application/json")

		rr = httptest.NewRecorder()
		handler.ProcessCommand(rr, httpReq)

		// Workflow should be cancelled or completed
		if rr.Code == http.StatusOK {
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)
		}
	})

	t.Run("Session Management", func(t *testing.T) {
		userID := "session-test-user"
		conversationID := "session-test-conversation"

		// Process a command to create session
		req := handlers.ProcessClaudeCommandRequest{
			UserID:         userID,
			ConversationID: conversationID,
			Command:        "find all bugs assigned to me",
		}

		body, err := json.Marshal(req)
		require.NoError(t, err)

		httpReq, err := http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer(body))
		require.NoError(t, err)
		httpReq.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ProcessCommand(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Get session status
		httpReq, err = http.NewRequest("GET", "/api/v1/claude/session?userId="+userID+"&conversationId="+conversationID, nil)
		require.NoError(t, err)

		rr = httptest.NewRecorder()
		handler.GetSessionStatus(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		var sessionInfo claude.SessionInfo
		err = json.Unmarshal(rr.Body.Bytes(), &sessionInfo)
		require.NoError(t, err)

		assert.NotEmpty(t, sessionInfo.ID)
		assert.Equal(t, 1, sessionInfo.CommandCount)
		assert.False(t, sessionInfo.WorkflowActive)
	})

	t.Run("User Preferences", func(t *testing.T) {
		userID := "prefs-test-user"
		conversationID := "prefs-test-conversation"

		// First create a session
		cmdReq := handlers.ProcessClaudeCommandRequest{
			UserID:         userID,
			ConversationID: conversationID,
			Command:        "show me all issues",
		}

		body, err := json.Marshal(cmdReq)
		require.NoError(t, err)

		httpReq, err := http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer(body))
		require.NoError(t, err)
		httpReq.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ProcessCommand(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Update preferences
		prefs := &claude.UserPreferences{
			DefaultProject:      "MYPROJ",
			AutoAssign:          true,
			NotifyOnCreate:      false,
			PreferredPriority:   "High",
			VerboseResponses:    false,
			ShowSuggestions:     true,
		}

		prefsReq := handlers.UpdatePreferencesRequest{
			UserID:         userID,
			ConversationID: conversationID,
			Preferences:    prefs,
		}

		body, err = json.Marshal(prefsReq)
		require.NoError(t, err)

		httpReq, err = http.NewRequest("POST", "/api/v1/claude/preferences", bytes.NewBuffer(body))
		require.NoError(t, err)
		httpReq.Header.Set("Content-Type", "application/json")

		rr = httptest.NewRecorder()
		handler.UpdatePreferences(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response map[string]string
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Preferences updated successfully", response["message"])
	})

	t.Run("Available Commands", func(t *testing.T) {
		httpReq, err := http.NewRequest("GET", "/api/v1/claude/commands", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.GetAvailableCommands(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		var commands map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &commands)
		require.NoError(t, err)

		// Should have pattern-based commands
		assert.Contains(t, commands, "CreateBugFromCode")
		assert.Contains(t, commands, "BatchTransition")
		assert.Contains(t, commands, "SmartSearch")

		// Should have workflows
		assert.Contains(t, commands, "workflows")
		workflows := commands["workflows"].([]interface{})
		assert.Greater(t, len(workflows), 0)
	})

	t.Run("Workflow Management", func(t *testing.T) {
		// Get available workflows
		httpReq, err := http.NewRequest("GET", "/api/v1/claude/workflows", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.GetWorkflows(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		workflows := response["workflows"].([]interface{})
		assert.Greater(t, len(workflows), 0)

		// Verify workflow structure
		workflow := workflows[0].(map[string]interface{})
		assert.Contains(t, workflow, "id")
		assert.Contains(t, workflow, "name")
		assert.Contains(t, workflow, "description")
		assert.Contains(t, workflow, "triggers")
		assert.Contains(t, workflow, "category")
		assert.Contains(t, workflow, "steps")
	})

	t.Run("Pattern Filtering", func(t *testing.T) {
		// Get patterns for specific category
		httpReq, err := http.NewRequest("GET", "/api/v1/claude/patterns?category=development", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.GetPatterns(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		var patterns map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &patterns)
		require.NoError(t, err)

		// Should only contain development patterns
		for _, pattern := range patterns {
			if patternMap, ok := pattern.(map[string]interface{}); ok {
				if category, exists := patternMap["category"]; exists {
					assert.Equal(t, "development", category)
				}
			}
		}
	})

	t.Run("Suggestions", func(t *testing.T) {
		userID := "suggestions-test-user"
		conversationID := "suggestions-test-conversation"

		// Test generic suggestions (no session)
		httpReq, err := http.NewRequest("GET", "/api/v1/claude/suggestions?userId=new-user&conversationId=new-conversation", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.GetSuggestions(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		suggestions := response["suggestions"].([]interface{})
		assert.Greater(t, len(suggestions), 0)
		assert.False(t, response["contextual"].(bool))

		// Create a session first
		cmdReq := handlers.ProcessClaudeCommandRequest{
			UserID:         userID,
			ConversationID: conversationID,
			Command:        "create a bug in PROJ",
		}

		body, err := json.Marshal(cmdReq)
		require.NoError(t, err)

		httpReq, err = http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer(body))
		require.NoError(t, err)
		httpReq.Header.Set("Content-Type", "application/json")

		rr = httptest.NewRecorder()
		handler.ProcessCommand(rr, httpReq)

		// Now get contextual suggestions
		httpReq, err = http.NewRequest("GET", "/api/v1/claude/suggestions?userId="+userID+"&conversationId="+conversationID, nil)
		require.NoError(t, err)

		rr = httptest.NewRecorder()
		handler.GetSuggestions(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		suggestions = response["suggestions"].([]interface{})
		assert.Greater(t, len(suggestions), 0)
		assert.True(t, response["contextual"].(bool))
		assert.Contains(t, response, "sessionInfo")
	})

	t.Run("Manager Stats", func(t *testing.T) {
		httpReq, err := http.NewRequest("GET", "/api/v1/claude/stats", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.GetManagerStats(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		var stats map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &stats)
		require.NoError(t, err)

		// Should contain expected statistics
		assert.Contains(t, stats, "sessions")
		assert.Contains(t, stats, "patterns")
		assert.Contains(t, stats, "workflows")
		assert.Contains(t, stats, "config")

		sessionStats := stats["sessions"].(map[string]interface{})
		assert.Contains(t, sessionStats, "totalSessions")
		assert.Contains(t, sessionStats, "activeSessions")
	})

	t.Run("Health Check", func(t *testing.T) {
		httpReq, err := http.NewRequest("GET", "/api/v1/claude/health", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler.Health(rr, httpReq)

		assert.Equal(t, http.StatusOK, rr.Code)

		var health map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &health)
		require.NoError(t, err)

		assert.Equal(t, "healthy", health["status"])
		assert.Equal(t, "claude-integration", health["service"])
		assert.Contains(t, health, "statistics")
		assert.Contains(t, health, "timestamp")
	})

	t.Run("Error Handling", func(t *testing.T) {
		// Test invalid JSON
		httpReq, err := http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer([]byte("invalid json")))
		require.NoError(t, err)
		httpReq.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ProcessCommand(rr, httpReq)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		// Test missing parameters
		httpReq, err = http.NewRequest("GET", "/api/v1/claude/session", nil)
		require.NoError(t, err)

		rr = httptest.NewRecorder()
		handler.GetSessionStatus(rr, httpReq)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		// Test session not found
		httpReq, err = http.NewRequest("GET", "/api/v1/claude/session?userId=nonexistent&conversationId=nonexistent", nil)
		require.NoError(t, err)

		rr = httptest.NewRecorder()
		handler.GetSessionStatus(rr, httpReq)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestClaudePatternMatching(t *testing.T) {
	// Test Claude-specific pattern matching
	handler := handlers.NewClaudeHandler()

	testCases := []struct {
		name         string
		command      string
		expectedPattern string
		minConfidence   float64
	}{
		{
			name:            "Create Bug Pattern",
			command:         "Create a bug for the memory leak in worker.py line 89",
			expectedPattern: "CreateBugFromCode",
			minConfidence:   0.7,
		},
		{
			name:            "Batch Transition Pattern",
			command:         "Move all issues in sprint 5 to In Progress",
			expectedPattern: "BatchTransition",
			minConfidence:   0.7,
		},
		{
			name:            "Smart Search Pattern",
			command:         "Find all unassigned issues in the current sprint",
			expectedPattern: "SmartSearch",
			minConfidence:   0.6,
		},
		{
			name:            "Quick Assign Pattern",
			command:         "Assign PROJ-123 to the frontend team lead",
			expectedPattern: "QuickAssign",
			minConfidence:   0.6,
		},
		{
			name:            "Sprint Management Pattern",
			command:         "Close current sprint and move incomplete items",
			expectedPattern: "SprintManagement",
			minConfidence:   0.6,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := handlers.ProcessClaudeCommandRequest{
				UserID:         "pattern-test-user",
				ConversationID: "pattern-test-conversation",
				Command:        tc.command,
			}

			body, err := json.Marshal(req)
			require.NoError(t, err)

			httpReq, err := http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer(body))
			require.NoError(t, err)
			httpReq.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.ProcessCommand(rr, httpReq)

			assert.Equal(t, http.StatusOK, rr.Code)

			var response claude.IntegrationResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.True(t, response.Success, "Command should succeed: %s", tc.command)
			assert.GreaterOrEqual(t, response.Confidence, tc.minConfidence, "Confidence should meet minimum for: %s", tc.command)

			// Check if the expected pattern was matched (if pattern info is in metadata)
			if response.Metadata != nil {
				if pattern, exists := response.Metadata["pattern"]; exists {
					assert.Equal(t, tc.expectedPattern, pattern, "Should match expected pattern for: %s", tc.command)
				}
			}
		})
	}
}

func BenchmarkClaudeCommand(b *testing.B) {
	handler := handlers.NewClaudeHandler()
	
	req := handlers.ProcessClaudeCommandRequest{
		UserID:         "bench-user",
		ConversationID: "bench-conversation",
		Command:        "create a bug in PROJ",
	}

	body, _ := json.Marshal(req)

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		httpReq, _ := http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ProcessCommand(rr, httpReq)

		if rr.Code != http.StatusOK {
			b.Errorf("Expected status 200, got %d", rr.Code)
		}
	}
}

func BenchmarkPatternMatching(b *testing.B) {
	handler := handlers.NewClaudeHandler()
	
	commands := []string{
		"Create a bug for the SQL injection in user.go",
		"Move all issues in sprint 5 to Done", 
		"Show me critical bugs from last week",
		"Assign PROJ-123 to john.doe",
		"Find all unassigned issues",
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		command := commands[i%len(commands)]
		
		req := handlers.ProcessClaudeCommandRequest{
			UserID:         "bench-user",
			ConversationID: "bench-conversation",
			Command:        command,
		}

		body, _ := json.Marshal(req)
		
		httpReq, _ := http.NewRequest("POST", "/api/v1/claude/command", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.ProcessCommand(rr, httpReq)
	}
}