package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ericfisherdev/GoJira/internal/api/routes"
	"github.com/ericfisherdev/GoJira/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ClaudeIntegrationE2ETestSuite tests end-to-end Claude Code integration
type ClaudeIntegrationE2ETestSuite struct {
	suite.Suite
	server *httptest.Server
	client *http.Client
	config *config.Config
}

// SetupSuite initializes the E2E test environment
func (suite *ClaudeIntegrationE2ETestSuite) SetupSuite() {
	// Create test configuration
	suite.config = &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: "8080",
			Mode: "test",
		},
		Jira: config.JiraConfig{
			URL: "https://test.atlassian.net",
			Auth: config.AuthConfig{
				Type:  "api_token",
				Email: "test@example.com",
				Token: "test-token",
			},
		},
		Features: config.FeatureConfig{
			NaturalLanguage: true,
			Caching:         true,
			AutoRetry:       true,
		},
	}

	// Mock components are initialized in test setup

	// Set up HTTP client
	suite.client = &http.Client{
		Timeout: 60 * time.Second,
	}

	// Create test server
	suite.setupTestServer()
}

func (suite *ClaudeIntegrationE2ETestSuite) setupTestServer() {
	// Set up routes
	router := chi.NewRouter()
	routes.SetupRoutes(router)

	// Create test server
	suite.server = httptest.NewServer(router)
}

func (suite *ClaudeIntegrationE2ETestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// TestNaturalLanguageCommandProcessing tests various natural language commands
func (suite *ClaudeIntegrationE2ETestSuite) TestNaturalLanguageCommandProcessing() {
	testCases := []struct {
		name            string
		command         string
		expectedAction  string
		expectedStatus  int
		validateResult  func(result map[string]interface{}) bool
	}{
		{
			name:           "Create Critical Bug",
			command:        "Create a critical bug for SQL injection in auth.go line 145",
			expectedAction: "CREATE_ISSUE",
			expectedStatus: 200,
			validateResult: func(result map[string]interface{}) bool {
				action := result["action"].(map[string]interface{})
				params := action["parameters"].(map[string]interface{})
				return params["issueType"] == "Bug" && 
				       params["priority"] == "Critical" &&
				       strings.Contains(params["summary"].(string), "SQL injection")
			},
		},
		{
			name:           "Bulk Status Update",
			command:        "Move all bugs in sprint 5 to In Progress",
			expectedAction: "BULK_UPDATE",
			expectedStatus: 200,
			validateResult: func(result map[string]interface{}) bool {
				action := result["action"].(map[string]interface{})
				params := action["parameters"].(map[string]interface{})
				return params["status"] == "In Progress" &&
				       strings.Contains(params["jql"].(string), "sprint = 5")
			},
		},
		{
			name:           "Search Query",
			command:        "Show me critical bugs from last week",
			expectedAction: "SEARCH_ISSUES",
			expectedStatus: 200,
			validateResult: func(result map[string]interface{}) bool {
				action := result["action"].(map[string]interface{})
				params := action["parameters"].(map[string]interface{})
				jql := params["jql"].(string)
				return strings.Contains(jql, "priority = Critical") &&
				       strings.Contains(jql, "type = Bug") &&
				       strings.Contains(jql, "created >= -1w")
			},
		},
		{
			name:           "Sprint Management",
			command:        "Start sprint 'Phase 3 Testing' with 2 week duration",
			expectedAction: "START_SPRINT",
			expectedStatus: 200,
			validateResult: func(result map[string]interface{}) bool {
				action := result["action"].(map[string]interface{})
				params := action["parameters"].(map[string]interface{})
				return strings.Contains(params["name"].(string), "Phase 3 Testing")
			},
		},
		{
			name:           "Complex Workflow Query",
			command:        "Find all high priority tasks assigned to John that are stuck in review for more than 3 days",
			expectedAction: "SEARCH_ISSUES",
			expectedStatus: 200,
			validateResult: func(result map[string]interface{}) bool {
				action := result["action"].(map[string]interface{})
				params := action["parameters"].(map[string]interface{})
				jql := params["jql"].(string)
				return strings.Contains(jql, "priority = High") &&
				       strings.Contains(jql, "assignee") &&
				       strings.Contains(jql, "status")
			},
		},
		{
			name:           "Comment Addition",
			command:        "Add comment 'Fixed security vulnerability' to ticket PROJ-123",
			expectedAction: "ADD_COMMENT",
			expectedStatus: 200,
			validateResult: func(result map[string]interface{}) bool {
				action := result["action"].(map[string]interface{})
				params := action["parameters"].(map[string]interface{})
				return params["issueKey"] == "PROJ-123" &&
				       strings.Contains(params["comment"].(string), "Fixed security vulnerability")
			},
		},
		{
			name:           "Transition Issue",
			command:        "Move ticket PROJ-456 to Done",
			expectedAction: "TRANSITION_ISSUE",
			expectedStatus: 200,
			validateResult: func(result map[string]interface{}) bool {
				action := result["action"].(map[string]interface{})
				params := action["parameters"].(map[string]interface{})
				return params["issueKey"] == "PROJ-456" &&
				       params["status"] == "Done"
			},
		},
		{
			name:           "Ambiguous Command",
			command:        "Do something with the tickets",
			expectedAction: "CLARIFICATION_NEEDED",
			expectedStatus: 200,
			validateResult: func(result map[string]interface{}) bool {
				suggestions := result["suggestions"].([]interface{})
				return len(suggestions) > 0
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			t.Logf("Testing command: %s", tc.command)

			// Prepare request
			request := map[string]interface{}{
				"command": tc.command,
				"context": map[string]interface{}{
					"currentProject": "TEST",
					"currentUser":    "testuser@example.com",
				},
			}

			reqBody, err := json.Marshal(request)
			require.NoError(t, err)

			// Make request
			resp, err := suite.makeRequest("POST", "/api/v1/claude/interpret", string(reqBody))
			require.NoError(t, err)
			defer resp.Body.Close()

			// Verify response status
			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			// Parse response
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			// Verify command interpretation
			assert.Contains(t, result, "understood")
			assert.True(t, result["understood"].(bool), "Command should be understood")

			// Verify expected action
			assert.Contains(t, result, "action")
			action := result["action"].(map[string]interface{})
			assert.Equal(t, tc.expectedAction, action["type"])

			// Run custom validation
			if tc.validateResult != nil {
				assert.True(t, tc.validateResult(result), "Custom validation failed")
			}

			// Verify response time
			assert.Contains(t, result, "processingTime")
			processingTime := result["processingTime"].(float64)
			assert.Less(t, processingTime, 500.0, "Processing should be under 500ms")

			t.Logf("Command processed successfully in %.2fms", processingTime)
		})
	}
}

// TestCommandAccuracyAndPrecision tests the accuracy of command interpretation
func (suite *ClaudeIntegrationE2ETestSuite) TestCommandAccuracyAndPrecision() {
	suite.T().Log("Testing Command Accuracy and Precision")

	accuracyTests := []struct {
		command  string
		expected map[string]interface{}
	}{
		{
			command: "Create a Story with title 'User Authentication' in project CORE with 5 story points",
			expected: map[string]interface{}{
				"action": "CREATE_ISSUE",
				"issueType": "Story",
				"project": "CORE",
				"storyPoints": 5,
			},
		},
		{
			command: "Update the priority of all open bugs to high",
			expected: map[string]interface{}{
				"action": "BULK_UPDATE",
				"priority": "High",
				"issueType": "Bug",
				"status": "Open",
			},
		},
		{
			command: "Show me all issues assigned to sarah.jones@company.com created this month",
			expected: map[string]interface{}{
				"action": "SEARCH_ISSUES",
				"assignee": "sarah.jones@company.com",
				"timeframe": "this month",
			},
		},
	}

	totalTests := len(accuracyTests)
	correctInterpretations := 0

	for i, test := range accuracyTests {
		suite.T().Logf("Accuracy test %d/%d: %s", i+1, totalTests, test.command)

		request := map[string]interface{}{
			"command": test.command,
		}

		reqBody, err := json.Marshal(request)
		require.NoError(suite.T(), err)

		resp, err := suite.makeRequest("POST", "/api/v1/claude/interpret", string(reqBody))
		require.NoError(suite.T(), err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(suite.T(), err)

		if suite.validateInterpretation(result, test.expected) {
			correctInterpretations++
		}
	}

	accuracy := float64(correctInterpretations) / float64(totalTests) * 100
	suite.T().Logf("Command interpretation accuracy: %.1f%% (%d/%d)", accuracy, correctInterpretations, totalTests)
	
	// Require at least 85% accuracy as per Day 10 requirements
	assert.GreaterOrEqual(suite.T(), accuracy, 85.0, "Command interpretation accuracy should be >= 85%")
}

// TestConversationalFlow tests multi-turn conversation handling
func (suite *ClaudeIntegrationE2ETestSuite) TestConversationalFlow() {
	suite.T().Log("Testing Conversational Flow")

	sessionID := "test-session-" + fmt.Sprintf("%d", time.Now().Unix())

	conversationFlow := []struct {
		step    int
		command string
		expectsClarification bool
	}{
		{1, "Create a new issue", true},
		{2, "Make it a bug", false},
		{3, "Set priority to critical", false},
		{4, "Assign it to john.doe@company.com", false},
		{5, "Add description about login failure", false},
		{6, "Create the issue in project SECURITY", false},
	}

	for _, step := range conversationFlow {
		suite.T().Logf("Step %d: %s", step.step, step.command)

		request := map[string]interface{}{
			"command": step.command,
			"sessionId": sessionID,
			"context": map[string]interface{}{
				"step": step.step,
			},
		}

		reqBody, err := json.Marshal(request)
		require.NoError(suite.T(), err)

		resp, err := suite.makeRequest("POST", "/api/v1/claude/interpret", string(reqBody))
		require.NoError(suite.T(), err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(suite.T(), err)

		if step.expectsClarification {
			assert.Contains(suite.T(), result, "clarification")
			assert.True(suite.T(), len(result["clarification"].(string)) > 0)
		} else {
			assert.Contains(suite.T(), result, "action")
		}

		// Verify conversation context is maintained
		assert.Contains(suite.T(), result, "sessionId")
		assert.Equal(suite.T(), sessionID, result["sessionId"])
	}

	suite.T().Log("Conversational flow test completed successfully")
}

// TestPerformanceRequirements tests performance criteria for Day 10
func (suite *ClaudeIntegrationE2ETestSuite) TestPerformanceRequirements() {
	suite.T().Log("Testing Performance Requirements")

	// Test 1: NLP Processing Time < 500ms
	suite.T().Log("Test 1: NLP Processing Time")
	start := time.Now()
	
	request := map[string]interface{}{
		"command": "Show me all critical bugs assigned to the backend team created in the last 2 weeks",
	}

	reqBody, _ := json.Marshal(request)
	resp, err := suite.makeRequest("POST", "/api/v1/claude/interpret", string(reqBody))
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	processingTime := time.Since(start)
	suite.T().Logf("NLP processing time: %v", processingTime)
	assert.Less(suite.T(), processingTime, 500*time.Millisecond, "NLP processing should be under 500ms")

	// Test 2: Concurrent Command Processing
	suite.T().Log("Test 2: Concurrent Command Processing")
	numConcurrent := 10
	results := make(chan time.Duration, numConcurrent)

	commands := []string{
		"Create a bug issue",
		"Show me open tasks",
		"Update issue priority",
		"Start new sprint",
		"Search for stories",
		"Add comment to ticket",
		"Transition issue to done",
		"Get issue details",
		"List project members",
		"Show sprint report",
	}

	for i := 0; i < numConcurrent; i++ {
		go func(index int) {
			start := time.Now()
			
			request := map[string]interface{}{
				"command": commands[index%len(commands)],
				"sessionId": fmt.Sprintf("concurrent-session-%d", index),
			}

			reqBody, _ := json.Marshal(request)
			resp, err := suite.makeRequest("POST", "/api/v1/claude/interpret", string(reqBody))
			if err == nil {
				resp.Body.Close()
			}
			
			results <- time.Since(start)
		}(i)
	}

	// Collect results
	totalTime := time.Duration(0)
	for i := 0; i < numConcurrent; i++ {
		duration := <-results
		totalTime += duration
	}

	avgTime := totalTime / time.Duration(numConcurrent)
	suite.T().Logf("Average concurrent processing time: %v", avgTime)
	assert.Less(suite.T(), avgTime, time.Second, "Average concurrent processing should be under 1 second")

	// Test 3: Memory Usage During Processing
	suite.T().Log("Test 3: Memory Usage Test")
	// This would typically use runtime.ReadMemStats() to monitor memory usage
	// For now, we'll test that we can handle multiple sessions without issues

	sessionCount := 20
	for i := 0; i < sessionCount; i++ {
		request := map[string]interface{}{
			"command": fmt.Sprintf("Create issue number %d", i),
			"sessionId": fmt.Sprintf("memory-test-session-%d", i),
		}

		reqBody, _ := json.Marshal(request)
		resp, err := suite.makeRequest("POST", "/api/v1/claude/interpret", string(reqBody))
		require.NoError(suite.T(), err)
		resp.Body.Close()
	}

	suite.T().Logf("Successfully processed %d different sessions", sessionCount)
}

// TestErrorRecoveryAndEdgeCases tests error handling and edge cases
func (suite *ClaudeIntegrationE2ETestSuite) TestErrorRecoveryAndEdgeCases() {
	suite.T().Log("Testing Error Recovery and Edge Cases")

	edgeCases := []struct {
		name        string
		command     string
		expectError bool
		expectFallback bool
	}{
		{
			name:        "Empty Command",
			command:     "",
			expectError: true,
			expectFallback: false,
		},
		{
			name:        "Very Long Command",
			command:     strings.Repeat("create issue with very long description ", 100),
			expectError: false,
			expectFallback: false,
		},
		{
			name:        "Special Characters",
			command:     "Create issue with title '!@#$%^&*()_+-=[]{}|;:,.<>?' and description",
			expectError: false,
			expectFallback: false,
		},
		{
			name:        "Non-English Command",
			command:     "Créer un nouveau ticket de type bug avec priorité haute",
			expectError: false,
			expectFallback: true,
		},
		{
			name:        "Nonsense Command",
			command:     "fjdksjfkldsj fkdsjflkds jfkldsjfkl dsjfklsdj",
			expectError: false,
			expectFallback: true,
		},
	}

	for _, tc := range edgeCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			request := map[string]interface{}{
				"command": tc.command,
			}

			reqBody, err := json.Marshal(request)
			require.NoError(t, err)

			resp, err := suite.makeRequest("POST", "/api/v1/claude/interpret", string(reqBody))
			require.NoError(t, err)
			defer resp.Body.Close()

			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			if tc.expectError {
				assert.Contains(t, result, "error")
			} else {
				assert.Contains(t, result, "understood")
				
				if tc.expectFallback {
					// Should provide suggestions or clarification
					assert.True(t, 
						result["understood"] == false ||
						result["action"].(map[string]interface{})["type"] == "CLARIFICATION_NEEDED",
						"Should trigger fallback behavior")
				}
			}
		})
	}
}

// Helper methods

func (suite *ClaudeIntegrationE2ETestSuite) makeRequest(method, endpoint, body string) (*http.Response, error) {
	url := suite.server.URL + endpoint
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return suite.client.Do(req)
}

// Mock helper functions for testing

func (suite *ClaudeIntegrationE2ETestSuite) validateInterpretation(result, expected map[string]interface{}) bool {
	if !result["understood"].(bool) {
		return false
	}

	action := result["action"].(map[string]interface{})
	
	// Check each expected field
	for key, expectedValue := range expected {
		if key == "action" {
			if action["type"].(string) != expectedValue.(string) {
				return false
			}
		} else {
			params := action["parameters"].(map[string]interface{})
			if params[key] != expectedValue {
				return false
			}
		}
	}

	return true
}

// Test suite runner
func TestClaudeIntegrationE2E(t *testing.T) {
	suite.Run(t, new(ClaudeIntegrationE2ETestSuite))
}

// Individual test functions
func TestNaturalLanguageProcessing(t *testing.T) {
	suite := &ClaudeIntegrationE2ETestSuite{}
	suite.SetT(t)
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.TestNaturalLanguageCommandProcessing()
}

func TestCommandAccuracy(t *testing.T) {
	suite := &ClaudeIntegrationE2ETestSuite{}
	suite.SetT(t)
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.TestCommandAccuracyAndPrecision()
}

func TestConversation(t *testing.T) {
	suite := &ClaudeIntegrationE2ETestSuite{}
	suite.SetT(t)
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.TestConversationalFlow()
}

func TestNLPPerformance(t *testing.T) {
	suite := &ClaudeIntegrationE2ETestSuite{}
	suite.SetT(t)
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.TestPerformanceRequirements()
}

func TestErrorHandling(t *testing.T) {
	suite := &ClaudeIntegrationE2ETestSuite{}
	suite.SetT(t)
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.TestErrorRecoveryAndEdgeCases()
}