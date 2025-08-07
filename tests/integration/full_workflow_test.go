package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/ericfisherdev/GoJira/internal/api/routes"
	"github.com/ericfisherdev/GoJira/internal/config"
	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/go-chi/chi/v5"
)

type IntegrationTestSuite struct {
	suite.Suite
	server     *httptest.Server
	jiraClient *jira.Client
	testConfig *config.Config
	router     *chi.Mux
}

func (suite *IntegrationTestSuite) SetupSuite() {
	// Load test configuration
	suite.testConfig = &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: "8080",
			Mode: "test",
		},
		Jira: config.JiraConfig{
			URL:     "https://test.atlassian.net",
			Timeout: 30,
			Retries: 3,
		},
	}

	// Set up test router
	suite.router = chi.NewRouter()
	routes.SetupRoutes(suite.router)
	suite.server = httptest.NewServer(suite.router)
}

func (suite *IntegrationTestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Close()
	}
}

func (suite *IntegrationTestSuite) TestHealthEndpoints() {
	// Test health check
	healthResp := suite.makeRequest("GET", "/health", nil)
	suite.Equal(200, healthResp.Code)

	var healthResult map[string]interface{}
	json.Unmarshal(healthResp.Body.Bytes(), &healthResult)
	suite.Equal("ok", healthResult["status"])

	// Test readiness check
	readyResp := suite.makeRequest("GET", "/ready", nil)
	suite.Equal(200, readyResp.Code)

	var readyResult map[string]interface{}
	json.Unmarshal(readyResp.Body.Bytes(), &readyResult)
	suite.Equal("ready", readyResult["status"])
}

func (suite *IntegrationTestSuite) TestSearchValidation() {
	// Test JQL validation endpoint
	validationResp := suite.makeRequest("GET", "/api/v1/search/validate?jql=project=TEST", nil)
	suite.Equal(200, validationResp.Code)

	var validationResult map[string]interface{}
	json.Unmarshal(validationResp.Body.Bytes(), &validationResult)
	suite.True(validationResult["success"].(bool))
}

func (suite *IntegrationTestSuite) TestClaudeEndpoints() {
	// Test command suggestions
	suggestionsResp := suite.makeRequest("GET", "/api/v1/claude/suggestions?input=create", nil)
	suite.Equal(200, suggestionsResp.Code)

	var suggestionsResult map[string]interface{}
	json.Unmarshal(suggestionsResp.Body.Bytes(), &suggestionsResult)
	suite.True(suggestionsResult["success"].(bool))

	// Test JQL generation
	jqlReq := map[string]interface{}{
		"query": "find all open issues",
	}

	jqlBody, _ := json.Marshal(jqlReq)
	jqlResp := suite.makeRequest("POST", "/api/v1/claude/jql", jqlBody)
	suite.Equal(200, jqlResp.Code)

	var jqlResult map[string]interface{}
	json.Unmarshal(jqlResp.Body.Bytes(), &jqlResult)
	suite.True(jqlResult["success"].(bool))

	// Test command processing
	commandReq := map[string]interface{}{
		"command": "show issue TEST-1",
	}

	commandBody, _ := json.Marshal(commandReq)
	commandResp := suite.makeRequest("POST", "/api/v1/claude/command", commandBody)
	suite.Equal(200, commandResp.Code)

	var commandResult map[string]interface{}
	json.Unmarshal(commandResp.Body.Bytes(), &commandResult)
	suite.True(commandResult["success"].(bool))
}

func (suite *IntegrationTestSuite) TestAdvancedSearchEndpoints() {
	// Test advanced search
	searchReq := map[string]interface{}{
		"jql":        "project = TEST ORDER BY created DESC",
		"maxResults": 50,
		"startAt":    0,
		"fields":     []string{"key", "summary", "status"},
	}

	searchBody, _ := json.Marshal(searchReq)
	searchResp := suite.makeRequest("POST", "/api/v1/search/advanced", searchBody)
	suite.Equal(200, searchResp.Code)

	var searchResult map[string]interface{}
	json.Unmarshal(searchResp.Body.Bytes(), &searchResult)
	suite.True(searchResult["success"].(bool))

	// Test paginated search
	paginatedBody, _ := json.Marshal(searchReq)
	paginatedResp := suite.makeRequest("POST", "/api/v1/search/paginated", paginatedBody)
	suite.Equal(200, paginatedResp.Code)

	var paginatedResult map[string]interface{}
	json.Unmarshal(paginatedResp.Body.Bytes(), &paginatedResult)
	suite.True(paginatedResult["success"].(bool))
}

func (suite *IntegrationTestSuite) TestExportFunctionality() {
	// Test search export
	exportReq := map[string]interface{}{
		"jql":    "project = TEST",
		"format": "json",
		"fields": []string{"key", "summary", "status"},
	}

	exportBody, _ := json.Marshal(exportReq)
	exportResp := suite.makeRequest("POST", "/api/v1/search/export", exportBody)
	suite.Equal(200, exportResp.Code)

	var exportResult map[string]interface{}
	json.Unmarshal(exportResp.Body.Bytes(), &exportResult)
	suite.True(exportResult["success"].(bool))
}

func (suite *IntegrationTestSuite) TestPerformanceUnderLoad() {
	concurrency := 10
	requestsPerWorker := 25 // Reduced for faster testing

	start := time.Now()

	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency*requestsPerWorker)

	for i := 0; i < concurrency; i++ {
		go func(worker int) {
			defer func() { done <- true }()

			for j := 0; j < requestsPerWorker; j++ {
				resp := suite.makeRequest("GET", "/health", nil)
				if resp.Code != 200 {
					errors <- fmt.Errorf("worker %d, request %d failed: %d", worker, j, resp.Code)
				}

				// Test readiness endpoint
				resp = suite.makeRequest("GET", "/ready", nil)
				if resp.Code != 200 {
					errors <- fmt.Errorf("worker %d, ready request %d failed: %d", worker, j, resp.Code)
				}
			}
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}

	duration := time.Since(start)
	totalRequests := concurrency * requestsPerWorker * 2 // 2 requests per iteration

	suite.T().Logf("Completed %d requests in %v (%.2f req/sec)",
		totalRequests, duration, float64(totalRequests)/duration.Seconds())

	// Check for errors
	close(errors)
	errorCount := 0
	for err := range errors {
		errorCount++
		suite.T().Logf("Error: %v", err)
	}

	suite.Equal(0, errorCount, "Should have no errors under load")
	suite.Less(duration, 30*time.Second, "Should complete within 30 seconds")

	// Verify we achieved reasonable throughput
	throughput := float64(totalRequests) / duration.Seconds()
	suite.Greater(throughput, 50.0, "Should achieve at least 50 requests per second")
}

func (suite *IntegrationTestSuite) TestConcurrentRequests() {
	concurrency := 5
	iterations := 10

	done := make(chan bool, concurrency)
	results := make(chan int, concurrency*iterations)

	for i := 0; i < concurrency; i++ {
		go func(worker int) {
			defer func() { done <- true }()

			for j := 0; j < iterations; j++ {
				resp := suite.makeRequest("GET", "/api/v1/search/validate?jql=project=TEST", nil)
				results <- resp.Code
			}
		}(i)
	}

	// Wait for completion
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// Check results
	close(results)
	successCount := 0
	for code := range results {
		if code == 200 {
			successCount++
		}
	}

	expectedResults := concurrency * iterations
	suite.Equal(expectedResults, successCount, "All concurrent requests should succeed")
}

func (suite *IntegrationTestSuite) makeRequest(method, path string, body []byte) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		req, _ = http.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	return w
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}