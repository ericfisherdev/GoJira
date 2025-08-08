package integration

import (
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

// AdvancedFeaturesIntegrationTestSuite provides comprehensive testing for advanced GoJira features
type AdvancedFeaturesIntegrationTestSuite struct {
	suite.Suite
	server *httptest.Server
	client *http.Client
	config *config.Config
}

// SetupSuite initializes the test environment
func (suite *AdvancedFeaturesIntegrationTestSuite) SetupSuite() {
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

	// Set up HTTP client
	suite.client = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create test server with routing
	suite.setupTestServer()
}

func (suite *AdvancedFeaturesIntegrationTestSuite) setupTestServer() {
	// Set up routes
	router := chi.NewRouter()
	routes.SetupRoutes(router)

	// Create test server
	suite.server = httptest.NewServer(router)
}

// TearDownSuite cleans up test resources
func (suite *AdvancedFeaturesIntegrationTestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// TestSprintLifecycleManagement tests complete sprint operations
func (suite *AdvancedFeaturesIntegrationTestSuite) TestSprintLifecycleManagement() {
	suite.T().Log("Testing Sprint Lifecycle Management")

	// Test data
	boardID := "10001"
	sprintName := "Phase 3 Integration Test Sprint"

	// Step 1: Create Sprint
	suite.T().Log("Step 1: Creating sprint")
	createReq := fmt.Sprintf(`{
		"name": "%s",
		"goal": "Test sprint lifecycle management",
		"boardId": %s,
		"startDate": "%s",
		"endDate": "%s"
	}`, sprintName, boardID, 
		time.Now().Format(time.RFC3339),
		time.Now().Add(14*24*time.Hour).Format(time.RFC3339))

	resp, err := suite.makeRequest("POST", "/api/v1/sprints", createReq)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	var createResult map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&createResult)
	require.NoError(suite.T(), err)
	resp.Body.Close()

	sprintID, ok := createResult["id"].(string)
	require.True(suite.T(), ok, "Sprint ID should be returned")
	suite.T().Logf("Created sprint with ID: %s", sprintID)

	// Step 2: Get Sprint Details
	suite.T().Log("Step 2: Getting sprint details")
	resp, err = suite.makeRequest("GET", fmt.Sprintf("/api/v1/sprints/%s", sprintID), "")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var sprintDetails map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&sprintDetails)
	require.NoError(suite.T(), err)
	resp.Body.Close()

	assert.Equal(suite.T(), sprintName, sprintDetails["name"])
	assert.Equal(suite.T(), "FUTURE", sprintDetails["state"])

	// Step 3: Start Sprint  
	suite.T().Log("Step 3: Starting sprint")
	startReq := `{"state": "ACTIVE"}`
	resp, err = suite.makeRequest("PUT", fmt.Sprintf("/api/v1/sprints/%s/start", sprintID), startReq)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Step 4: Verify Sprint is Active
	suite.T().Log("Step 4: Verifying sprint is active")
	resp, err = suite.makeRequest("GET", fmt.Sprintf("/api/v1/sprints/%s", sprintID), "")
	require.NoError(suite.T(), err)

	err = json.NewDecoder(resp.Body).Decode(&sprintDetails)
	require.NoError(suite.T(), err)
	resp.Body.Close()

	assert.Equal(suite.T(), "ACTIVE", sprintDetails["state"])
	
	// Step 5: Get Sprint Report
	suite.T().Log("Step 5: Getting sprint report")
	resp, err = suite.makeRequest("GET", fmt.Sprintf("/api/v1/sprints/%s/report", sprintID), "")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var reportData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&reportData)
	require.NoError(suite.T(), err)
	resp.Body.Close()

	assert.Contains(suite.T(), reportData, "summary")
	assert.Contains(suite.T(), reportData, "completedIssues")
	assert.Contains(suite.T(), reportData, "incompleteIssues")

	suite.T().Log("Sprint lifecycle management test completed successfully")
}

// TestBulkOperations tests bulk update operations with verification
func (suite *AdvancedFeaturesIntegrationTestSuite) TestBulkOperations() {
	suite.T().Log("Testing Bulk Operations")

	// Create test issues for bulk operations
	testIssues := suite.createTestIssues(10)
	
	// Step 1: Bulk Update Priority
	suite.T().Log("Step 1: Bulk updating issue priorities")
	bulkUpdateReq := map[string]interface{}{
		"issueKeys": testIssues,
		"update": map[string]interface{}{
			"priority": map[string]interface{}{
				"name": "High",
			},
		},
	}

	reqBody, err := json.Marshal(bulkUpdateReq)
	require.NoError(suite.T(), err)

	resp, err := suite.makeRequest("POST", "/api/v1/bulk/update", string(reqBody))
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var bulkResult map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&bulkResult)
	require.NoError(suite.T(), err)
	resp.Body.Close()

	assert.Contains(suite.T(), bulkResult, "successful")
	assert.Contains(suite.T(), bulkResult, "failed")
	
	successful := int(bulkResult["successful"].(float64))
	assert.Equal(suite.T(), len(testIssues), successful)

	// Step 2: Verify Updates
	suite.T().Log("Step 2: Verifying bulk updates")
	for _, issueKey := range testIssues {
		resp, err := suite.makeRequest("GET", fmt.Sprintf("/api/v1/issues/%s", issueKey), "")
		require.NoError(suite.T(), err)

		var issue map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&issue)
		require.NoError(suite.T(), err)
		resp.Body.Close()

		fields := issue["fields"].(map[string]interface{})
		priority := fields["priority"].(map[string]interface{})
		assert.Equal(suite.T(), "High", priority["name"])
	}

	// Step 3: Bulk Transition Issues
	suite.T().Log("Step 3: Bulk transitioning issues")
	transitionReq := map[string]interface{}{
		"issueKeys": testIssues[:5], // Only first 5 issues
		"transition": map[string]interface{}{
			"id": "21", // Transition to "In Progress"
		},
	}

	reqBody, err = json.Marshal(transitionReq)
	require.NoError(suite.T(), err)

	resp, err = suite.makeRequest("POST", "/api/v1/bulk/transition", string(reqBody))
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Step 4: Verify Transitions
	suite.T().Log("Step 4: Verifying bulk transitions")
	time.Sleep(2 * time.Second) // Allow for eventual consistency

	for i, issueKey := range testIssues[:5] {
		resp, err := suite.makeRequest("GET", fmt.Sprintf("/api/v1/issues/%s", issueKey), "")
		require.NoError(suite.T(), err)

		var issue map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&issue)
		require.NoError(suite.T(), err)
		resp.Body.Close()

		fields := issue["fields"].(map[string]interface{})
		status := fields["status"].(map[string]interface{})
		
		suite.T().Logf("Issue %d (%s) status: %s", i+1, issueKey, status["name"])
		// Status should be "In Progress" or equivalent
		assert.Contains(suite.T(), []string{"In Progress", "In Development"}, status["name"])
	}

	suite.T().Log("Bulk operations test completed successfully")
}

// TestWorkflowOperations tests workflow state management
func (suite *AdvancedFeaturesIntegrationTestSuite) TestWorkflowOperations() {
	suite.T().Log("Testing Workflow Operations")

	projectKey := "TEST"
	
	// Step 1: Get Available Workflows
	suite.T().Log("Step 1: Getting available workflows")
	resp, err := suite.makeRequest("GET", fmt.Sprintf("/api/v1/workflows?projectKey=%s", projectKey), "")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var workflows []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&workflows)
	require.NoError(suite.T(), err)
	resp.Body.Close()

	assert.Greater(suite.T(), len(workflows), 0, "Should have at least one workflow")

	// Step 2: Get Workflow Transitions
	workflowID := workflows[0]["id"].(string)
	suite.T().Log("Step 2: Getting workflow transitions for workflow:", workflowID)
	
	resp, err = suite.makeRequest("GET", fmt.Sprintf("/api/v1/workflows/%s/transitions", workflowID), "")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var transitions []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&transitions)
	require.NoError(suite.T(), err)
	resp.Body.Close()

	assert.Greater(suite.T(), len(transitions), 0, "Should have transitions")

	// Step 3: Advanced Workflow Query
	suite.T().Log("Step 3: Testing advanced workflow query")
	advancedReq := map[string]interface{}{
		"query": "Find all issues in TODO state older than 7 days",
		"projectKey": projectKey,
		"maxResults": 50,
	}

	reqBody, err := json.Marshal(advancedReq)
	require.NoError(suite.T(), err)

	resp, err = suite.makeRequest("POST", "/api/v1/workflows/advanced/query", string(reqBody))
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var queryResult map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&queryResult)
	require.NoError(suite.T(), err)
	resp.Body.Close()

	assert.Contains(suite.T(), queryResult, "issues")
	assert.Contains(suite.T(), queryResult, "total")

	suite.T().Log("Workflow operations test completed successfully")
}

// TestPerformanceAndCaching tests caching and performance features
func (suite *AdvancedFeaturesIntegrationTestSuite) TestPerformanceAndCaching() {
	suite.T().Log("Testing Performance and Caching")

	// Step 1: Test Cache Performance
	suite.T().Log("Step 1: Testing cache performance")
	issueKey := "TEST-123"
	
	// First request - cache miss
	start := time.Now()
	resp, err := suite.makeRequest("GET", fmt.Sprintf("/api/v1/issues/%s", issueKey), "")
	require.NoError(suite.T(), err)
	resp.Body.Close()
	firstDuration := time.Since(start)

	// Second request - should be cached
	start = time.Now()
	resp, err = suite.makeRequest("GET", fmt.Sprintf("/api/v1/issues/%s", issueKey), "")
	require.NoError(suite.T(), err)
	resp.Body.Close()
	secondDuration := time.Since(start)

	// Cache hit should be faster
	assert.True(suite.T(), secondDuration < firstDuration, "Cached request should be faster")
	suite.T().Logf("First request: %v, Second request (cached): %v", firstDuration, secondDuration)

	// Step 2: Test Concurrent Operations
	suite.T().Log("Step 2: Testing concurrent operations")
	numConcurrent := 10
	results := make(chan time.Duration, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(index int) {
			start := time.Now()
			resp, err := suite.makeRequest("GET", fmt.Sprintf("/api/v1/issues/TEST-%d", index), "")
			if err == nil {
				resp.Body.Close()
			}
			results <- time.Since(start)
		}(i)
	}

	// Collect results
	totalDuration := time.Duration(0)
	for i := 0; i < numConcurrent; i++ {
		duration := <-results
		totalDuration += duration
	}

	avgDuration := totalDuration / time.Duration(numConcurrent)
	suite.T().Logf("Average duration for %d concurrent requests: %v", numConcurrent, avgDuration)
	assert.Less(suite.T(), avgDuration, 2*time.Second, "Average response time should be reasonable")

	suite.T().Log("Performance and caching test completed successfully")
}

// TestErrorHandlingAndResilience tests error handling and resilience features
func (suite *AdvancedFeaturesIntegrationTestSuite) TestErrorHandlingAndResilience() {
	suite.T().Log("Testing Error Handling and Resilience")

	// Step 1: Test Invalid Issue Key
	suite.T().Log("Step 1: Testing invalid issue key handling")
	resp, err := suite.makeRequest("GET", "/api/v1/issues/INVALID-KEY", "")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// Step 2: Test Invalid Sprint ID
	suite.T().Log("Step 2: Testing invalid sprint ID handling")
	resp, err = suite.makeRequest("GET", "/api/v1/sprints/99999", "")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// Step 3: Test Malformed Request
	suite.T().Log("Step 3: Testing malformed request handling")
	resp, err = suite.makeRequest("POST", "/api/v1/sprints", "invalid json{")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// Step 4: Test Rate Limiting
	suite.T().Log("Step 4: Testing rate limiting")
	// Make rapid requests to trigger rate limiting
	for i := 0; i < 100; i++ {
		resp, err := suite.makeRequest("GET", "/api/v1/health", "")
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			suite.T().Log("Rate limiting triggered successfully")
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}

	suite.T().Log("Error handling and resilience test completed successfully")
}

// Helper methods

func (suite *AdvancedFeaturesIntegrationTestSuite) makeRequest(method, endpoint, body string) (*http.Response, error) {
	url := suite.server.URL + endpoint
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return suite.client.Do(req)
}

func (suite *AdvancedFeaturesIntegrationTestSuite) createTestIssues(count int) []string {
	issues := make([]string, count)
	for i := 0; i < count; i++ {
		issues[i] = fmt.Sprintf("TEST-%d", 1000+i)
	}
	return issues
}

// Test suite runner
func TestAdvancedFeaturesIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AdvancedFeaturesIntegrationTestSuite))
}

// Individual test functions for granular testing
func TestSprintLifecycleComplete(t *testing.T) {
	suite := &AdvancedFeaturesIntegrationTestSuite{}
	suite.SetT(t)
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.TestSprintLifecycleManagement()
}

func TestBulkUpdateOperations(t *testing.T) {
	suite := &AdvancedFeaturesIntegrationTestSuite{}
	suite.SetT(t)
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.TestBulkOperations()
}

func TestAdvancedWorkflowOperations(t *testing.T) {
	suite := &AdvancedFeaturesIntegrationTestSuite{}
	suite.SetT(t)
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.TestWorkflowOperations()
}

func TestPerformanceAndCacheIntegration(t *testing.T) {
	suite := &AdvancedFeaturesIntegrationTestSuite{}
	suite.SetT(t)
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.TestPerformanceAndCaching()
}

func TestErrorHandlingAndSystemResilience(t *testing.T) {
	suite := &AdvancedFeaturesIntegrationTestSuite{}
	suite.SetT(t)
	suite.SetupSuite()
	defer suite.TearDownSuite()

	suite.TestErrorHandlingAndResilience()
}