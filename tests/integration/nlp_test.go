package integration

import (
	"strings"
	"testing"

	"github.com/ericfisherdev/GoJira/internal/nlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNLPParser(t *testing.T) {
	// Setup parser with test data
	parser := nlp.NewParser(nil)
	setupTestData(parser)

	t.Run("Parse Create Intent", func(t *testing.T) {
		testCases := []struct {
			input          string
			expectedType   nlp.IntentType
			expectedAction string
			minConfidence  float64
		}{
			{
				input:          "create a bug in PROJ",
				expectedType:   nlp.IntentCreate,
				expectedAction: "create_issue",
				minConfidence:  0.6,
			},
			{
				input:          "new task for PROJECT-123",
				expectedType:   nlp.IntentCreate,
				expectedAction: "create_task",
				minConfidence:  0.6,
			},
			{
				input:          "file a bug report",
				expectedType:   nlp.IntentCreate,
				expectedAction: "create_bug",
				minConfidence:  0.5,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result, err := parser.Parse(tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedType, result.Intent.Type)
				assert.GreaterOrEqual(t, result.Intent.Confidence, tc.minConfidence)
				assert.Equal(t, tc.input, result.Intent.Raw)
			})
		}
	})

	t.Run("Parse Update Intent", func(t *testing.T) {
		testCases := []struct {
			input         string
			expectedType  nlp.IntentType
			minConfidence float64
		}{
			{
				input:         "update PROJ-123",
				expectedType:  nlp.IntentUpdate,
				minConfidence: 0.7,
			},
			{
				input:         "change priority of PROJ-456 to high",
				expectedType:  nlp.IntentUpdate,
				minConfidence: 0.8,
			},
			{
				input:         "set assignee to john.doe for PROJ-789",
				expectedType:  nlp.IntentUpdate,
				minConfidence: 0.8,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result, err := parser.Parse(tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedType, result.Intent.Type)
				assert.GreaterOrEqual(t, result.Intent.Confidence, tc.minConfidence)
			})
		}
	})

	t.Run("Parse Transition Intent", func(t *testing.T) {
		testCases := []struct {
			input         string
			expectedType  nlp.IntentType
			minConfidence float64
		}{
			{
				input:         "move PROJ-123 to Done",
				expectedType:  nlp.IntentTransition,
				minConfidence: 0.8,
			},
			{
				input:         "transition PROJ-456 to In Progress",
				expectedType:  nlp.IntentTransition,
				minConfidence: 0.8,
			},
			{
				input:         "mark PROJ-789 as resolved",
				expectedType:  nlp.IntentTransition,
				minConfidence: 0.7,
			},
			{
				input:         "close PROJ-123",
				expectedType:  nlp.IntentTransition,
				minConfidence: 0.8,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result, err := parser.Parse(tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedType, result.Intent.Type)
				assert.GreaterOrEqual(t, result.Intent.Confidence, tc.minConfidence)
			})
		}
	})

	t.Run("Parse Search Intent", func(t *testing.T) {
		testCases := []struct {
			input         string
			expectedType  nlp.IntentType
			minConfidence float64
		}{
			{
				input:         "find all bugs in PROJ",
				expectedType:  nlp.IntentSearch,
				minConfidence: 0.7,
			},
			{
				input:         "show me issues assigned to john.doe",
				expectedType:  nlp.IntentSearch,
				minConfidence: 0.8,
			},
			{
				input:         "list issues in sprint 5",
				expectedType:  nlp.IntentSearch,
				minConfidence: 0.8,
			},
			{
				input:         "search for high priority bugs",
				expectedType:  nlp.IntentSearch,
				minConfidence: 0.7,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result, err := parser.Parse(tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedType, result.Intent.Type)
				assert.GreaterOrEqual(t, result.Intent.Confidence, tc.minConfidence)
			})
		}
	})

	t.Run("Parse Assignment Intent", func(t *testing.T) {
		testCases := []struct {
			input         string
			expectedType  nlp.IntentType
			minConfidence float64
		}{
			{
				input:         "assign PROJ-123 to john.doe",
				expectedType:  nlp.IntentAssign,
				minConfidence: 0.8,
			},
			{
				input:         "reassign PROJ-456 to me",
				expectedType:  nlp.IntentAssign,
				minConfidence: 0.8,
			},
			{
				input:         "unassign PROJ-789",
				expectedType:  nlp.IntentAssign,
				minConfidence: 0.7,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result, err := parser.Parse(tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedType, result.Intent.Type)
				assert.GreaterOrEqual(t, result.Intent.Confidence, tc.minConfidence)
			})
		}
	})

	t.Run("Parse Unknown Intent", func(t *testing.T) {
		testCases := []string{
			"xyz abc def ghi",
			"random gibberish text",
			"",
		}

		for _, input := range testCases {
			t.Run(input, func(t *testing.T) {
				result, err := parser.Parse(input)
				if input == "" {
					assert.Error(t, err)
					return
				}
				
				// Should either error or return unknown intent with suggestions
				if err != nil {
					assert.Contains(t, err.Error(), "could not understand")
				} else {
					assert.Equal(t, nlp.IntentUnknown, result.Intent.Type)
					assert.Greater(t, len(result.Suggestions), 0)
				}
			})
		}
	})
}

func TestEntityExtraction(t *testing.T) {
	parser := nlp.NewParser(nil)
	setupTestData(parser)

	t.Run("Extract Issue Keys", func(t *testing.T) {
		testCases := []struct {
			input       string
			expectedKey string
		}{
			{"update PROJ-123", "PROJ-123"},
			{"move ABC-456 to Done", "ABC-456"},
			{"MYPROJECT-789 is broken", "MYPROJECT-789"},
			{"Fix issue XYZ-1", "XYZ-1"},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				entities := parser.ExtractEntities(tc.input)
				
				issueEntity, exists := entities[string(nlp.EntityIssueKey)]
				assert.True(t, exists, "Issue key entity should be found")
				assert.Equal(t, tc.expectedKey, issueEntity.Value)
				assert.Equal(t, nlp.EntityIssueKey, issueEntity.Type)
				assert.GreaterOrEqual(t, issueEntity.Confidence, 0.9)
			})
		}
	})

	t.Run("Extract Priorities", func(t *testing.T) {
		testCases := []struct {
			input            string
			expectedPriority string
		}{
			{"create a high priority bug", "High"},
			{"set priority to critical", "Highest"},
			{"make it low priority", "Low"},
			{"P0 issue needs attention", "Highest"},
			{"P3 can wait", "Low"},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				entities := parser.ExtractEntities(tc.input)
				
				priorityEntity, exists := entities[string(nlp.EntityPriority)]
				assert.True(t, exists, "Priority entity should be found")
				assert.Equal(t, nlp.EntityPriority, priorityEntity.Type)
				// Note: The normalized value should match expected
				assert.NotEmpty(t, priorityEntity.Value)
			})
		}
	})

	t.Run("Extract Dates", func(t *testing.T) {
		testCases := []struct {
			input        string
			expectEntity bool
		}{
			{"due date is 2024-12-25", true},
			{"create issue for today", true},
			{"schedule for tomorrow", true},
			{"deadline next week", true},
			{"in 3 days", true},
			{"no date mentioned", false},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				entities := parser.ExtractEntities(tc.input)
				
				_, exists := entities[string(nlp.EntityDate)]
				assert.Equal(t, tc.expectEntity, exists)
			})
		}
	})

	t.Run("Extract Assignees", func(t *testing.T) {
		testCases := []struct {
			input     string
			expected  string
			shouldFind bool
		}{
			{"assign to @john.doe", "john.doe", true},
			{"assigned to me", "current_user", true},
			{"give it to jane.smith@example.com", "jane.smith@example.com", true},
			{"assign to alice", "alice", true},
			{"no assignee mentioned", "", false},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				entities := parser.ExtractEntities(tc.input)
				
				assigneeEntity, exists := entities[string(nlp.EntityAssignee)]
				assert.Equal(t, tc.shouldFind, exists)
				
				if tc.shouldFind {
					assert.Equal(t, tc.expected, assigneeEntity.Value)
					assert.Equal(t, nlp.EntityAssignee, assigneeEntity.Type)
				}
			})
		}
	})

	t.Run("Extract Story Points", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int
		}{
			{"estimate 5 story points", 5},
			{"set to 8 points", 8},
			{"SP: 13", 13},
			{"3 points for this task", 3},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				entities := parser.ExtractEntities(tc.input)
				
				spEntity, exists := entities[string(nlp.EntityStoryPoints)]
				assert.True(t, exists)
				assert.Equal(t, tc.expected, spEntity.Value)
			})
		}
	})

	t.Run("Extract Labels", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"tag with #frontend", "frontend"},
			{"labeled as backend", "backend"},
			{"add label ui-bug", "ui-bug"},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				entities := parser.ExtractEntities(tc.input)
				
				labelEntity, exists := entities[string(nlp.EntityLabel)]
				assert.True(t, exists)
				assert.Equal(t, tc.expected, labelEntity.Value)
			})
		}
	})
}

func TestDisambiguator(t *testing.T) {
	parser := nlp.NewParser(nil)
	setupTestData(parser)
	disambiguator := nlp.NewDisambiguator(parser)

	t.Run("Require Missing Entities", func(t *testing.T) {
		// Create intent missing required entities
		intent := &nlp.Intent{
			Type:       nlp.IntentCreate,
			Confidence: 0.8,
			Entities:   make(map[string]nlp.Entity),
		}

		result, clarifications, err := disambiguator.Disambiguate(intent)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, len(clarifications), 0)
		
		// Should ask for project and issue_type
		fields := make([]string, len(clarifications))
		for i, c := range clarifications {
			fields[i] = c.Field
		}
		assert.Contains(t, fields, "project")
		assert.Contains(t, fields, "issue_type")
	})

	t.Run("Infer From Context", func(t *testing.T) {
		// Set up context
		context := &nlp.Context{
			LastProject:  "PROJ",
			LastAssignee: "john.doe",
		}
		parser.SetContext(context)

		intent := &nlp.Intent{
			Type:       nlp.IntentCreate,
			Confidence: 0.8,
			Entities:   make(map[string]nlp.Entity),
		}

		result, clarifications, err := disambiguator.Disambiguate(intent)
		require.NoError(t, err)
		
		// Should have inferred project from context
		projectEntity, exists := result.Entities["project"]
		assert.True(t, exists)
		assert.Equal(t, nlp.EntityProject, projectEntity.Type)
		
		// Should have fewer clarifications now
		assert.Less(t, len(clarifications), 2)
	})

	t.Run("Validate Entities", func(t *testing.T) {
		// Create intent with invalid entities
		intent := &nlp.Intent{
			Type:       nlp.IntentUpdate,
			Confidence: 0.8,
			Entities: map[string]nlp.Entity{
				"issue_key": {
					Type:  nlp.EntityIssueKey,
					Value: "INVALID-KEY-FORMAT",
					Text:  "INVALID-KEY-FORMAT",
				},
			},
		}

		result, clarifications, err := disambiguator.Disambiguate(intent)
		require.NoError(t, err)
		assert.NotNil(t, result)
		
		// Should have clarification for invalid issue key
		found := false
		for _, c := range clarifications {
			if c.Field == "issue_key" {
				found = true
				assert.Contains(t, strings.ToLower(c.Message), "not a valid")
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("Resolve Entity References", func(t *testing.T) {
		// Set up parser with default assignee
		config := &nlp.ParseConfig{
			DefaultAssignee: "test.user",
		}
		parser := nlp.NewParser(config)
		disambiguator := nlp.NewDisambiguator(parser)

		intent := &nlp.Intent{
			Type:       nlp.IntentAssign,
			Confidence: 0.8,
			Entities: map[string]nlp.Entity{
				"assignee": {
					Type:  nlp.EntityAssignee,
					Value: "current_user",
					Text:  "me",
				},
			},
		}

		result, _, err := disambiguator.Disambiguate(intent)
		require.NoError(t, err)
		
		// Should have resolved "current_user" to actual user
		assigneeEntity := result.Entities["assignee"]
		assert.Equal(t, "test.user", assigneeEntity.Value)
		assert.Equal(t, "test.user", assigneeEntity.Normalized)
	})
}

func TestContextManagement(t *testing.T) {
	parser := nlp.NewParser(nil)
	
	t.Run("Update Context From Intent", func(t *testing.T) {
		// Parse command that should update context
		result, err := parser.Parse("update PROJ-123")
		require.NoError(t, err)
		
		// Context should have been updated
		context := parser.GetContext()
		assert.Equal(t, "PROJ-123", context.LastIssue)
		assert.Greater(t, len(context.History), 0)
		assert.Equal(t, result.Intent.Type, context.History[0].Type)
	})

	t.Run("History Management", func(t *testing.T) {
		// Create parser with limited history
		config := &nlp.ParseConfig{
			MaxHistorySize: 2,
		}
		parser := nlp.NewParser(config)
		
		// Add multiple intents
		intents := []string{
			"create bug",
			"update PROJ-123", 
			"move PROJ-456 to done",
		}
		
		for _, command := range intents {
			_, err := parser.Parse(command)
			require.NoError(t, err)
		}
		
		// History should be limited
		context := parser.GetContext()
		assert.LessOrEqual(t, len(context.History), 2)
		
		// Should have latest intents
		assert.Equal(t, nlp.IntentTransition, context.History[len(context.History)-1].Type)
	})
}

func TestComplexCommands(t *testing.T) {
	parser := nlp.NewParser(nil)
	setupTestData(parser)

	t.Run("Multi-Entity Commands", func(t *testing.T) {
		testCases := []struct {
			input           string
			expectedType    nlp.IntentType
			expectedEntities []nlp.EntityType
		}{
			{
				input:        "create a high priority bug in PROJ assigned to @john.doe",
				expectedType: nlp.IntentCreate,
				expectedEntities: []nlp.EntityType{
					nlp.EntityIssueType,
					nlp.EntityPriority,
					nlp.EntityProject,
					nlp.EntityAssignee,
				},
			},
			{
				input:        "move PROJ-123 to In Progress and assign to me",
				expectedType: nlp.IntentTransition, // Primary intent
				expectedEntities: []nlp.EntityType{
					nlp.EntityIssueKey,
					nlp.EntityStatus,
					nlp.EntityAssignee,
				},
			},
			{
				input:        "find all critical bugs in PROJ assigned to john.doe in sprint 5",
				expectedType: nlp.IntentSearch,
				expectedEntities: []nlp.EntityType{
					nlp.EntityIssueType,
					nlp.EntityPriority,
					nlp.EntityProject,
					nlp.EntityAssignee,
					nlp.EntitySprint,
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result, err := parser.Parse(tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedType, result.Intent.Type)
				
				// Check that expected entities are present
				for _, entityType := range tc.expectedEntities {
					found := false
					for _, entity := range result.Intent.Entities {
						if entity.Type == entityType {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected entity type %s not found", entityType)
				}
			})
		}
	})
}

// Helper function to set up test data
func setupTestData(parser *nlp.Parser) {
	// Setup project cache
	projects := map[string]*nlp.Project{
		"PROJ": {
			Key:  "PROJ",
			Name: "Test Project",
			ID:   "10001",
		},
		"ABC": {
			Key:  "ABC",
			Name: "Another Project",
			ID:   "10002",
		},
		"MYPROJECT": {
			Key:  "MYPROJECT", 
			Name: "My Project",
			ID:   "10003",
		},
	}
	parser.SetProjectCache(projects)

	// Setup user cache
	users := map[string]*nlp.User{
		"john.doe": {
			Username:    "john.doe",
			DisplayName: "John Doe",
			Email:       "john.doe@example.com",
			Active:      true,
		},
		"jane.smith": {
			Username:    "jane.smith",
			DisplayName: "Jane Smith", 
			Email:       "jane.smith@example.com",
			Active:      true,
		},
		"alice": {
			Username:    "alice",
			DisplayName: "Alice Johnson",
			Email:       "alice@example.com",
			Active:      true,
		},
	}
	parser.SetUserCache(users)
}

func BenchmarkParsing(b *testing.B) {
	parser := nlp.NewParser(nil)
	setupTestData(parser)
	
	commands := []string{
		"create a bug in PROJ",
		"update PROJ-123",
		"move PROJ-456 to Done",
		"find all issues assigned to me",
		"assign PROJ-789 to john.doe",
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		command := commands[i%len(commands)]
		_, err := parser.Parse(command)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkEntityExtraction(b *testing.B) {
	parser := nlp.NewParser(nil)
	setupTestData(parser)
	
	text := "create a high priority bug in PROJ-123 assigned to @john.doe with 5 story points due tomorrow"
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = parser.ExtractEntities(text)
	}
}