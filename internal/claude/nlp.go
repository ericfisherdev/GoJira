package claude

import (
	"fmt"
	"regexp"
	"strings"
)

type CommandProcessor struct {
	patterns map[string]*regexp.Regexp
}

type ParsedCommand struct {
	Action     string            `json:"action"`
	Parameters map[string]string `json:"parameters"`
	Confidence float64           `json:"confidence"`
}

func NewCommandProcessor() *CommandProcessor {
	patterns := map[string]*regexp.Regexp{
		"create_issue":   regexp.MustCompile(`(?i)create\s+(?:a\s+)?(?:(bug|task|story|epic)\s+)?(?:ticket|issue)\s+(?:for|about|regarding)\s+(.+?)(?:\s+in\s+(?:project\s+)?([A-Z]{2,10}))?`),
		"get_issue":      regexp.MustCompile(`(?i)(?:show|get|find)\s+(?:issue\s+|ticket\s+)?([A-Z]+-\d+)`),
		"update_status":  regexp.MustCompile(`(?i)(?:move|set|change|update)\s+([A-Z]+-\d+)\s+to\s+(.+?)(?:\s|$)`),
		"search_issues":  regexp.MustCompile(`(?i)(?:find|search|show)\s+(?:all\s+)?(.+?)\s+(?:issues|tickets)`),
		"assign_issue":   regexp.MustCompile(`(?i)assign\s+([A-Z]+-\d+)\s+to\s+(.+?)(?:\s|$)`),
		"add_comment":    regexp.MustCompile(`(?i)(?:add\s+)?comment\s+(?:to\s+|on\s+)?([A-Z]+-\d+)(?:\s+saying\s+|:\s*)(.+)`),
		"list_projects":  regexp.MustCompile(`(?i)(?:list|show|get)\s+(?:all\s+)?projects`),
		"update_issue":   regexp.MustCompile(`(?i)update\s+([A-Z]+-\d+)\s+(.+)`),
		"delete_issue":   regexp.MustCompile(`(?i)delete\s+(?:issue\s+|ticket\s+)?([A-Z]+-\d+)`),
		"link_issues":    regexp.MustCompile(`(?i)link\s+([A-Z]+-\d+)\s+(?:to|with)\s+([A-Z]+-\d+)`),
		"export_search":  regexp.MustCompile(`(?i)export\s+(?:search\s+)?(?:results?\s+)?(?:to\s+)?(csv|json|markdown)`),
		"transition":     regexp.MustCompile(`(?i)(?:transition|move)\s+([A-Z]+-\d+)\s+(?:to\s+)?(.+)`),
		"watch_issue":    regexp.MustCompile(`(?i)(?:watch|follow)\s+(?:issue\s+|ticket\s+)?([A-Z]+-\d+)`),
		"unwatch_issue":  regexp.MustCompile(`(?i)(?:unwatch|unfollow|stop\s+watching)\s+(?:issue\s+|ticket\s+)?([A-Z]+-\d+)`),
	}

	return &CommandProcessor{patterns: patterns}
}

func (cp *CommandProcessor) ParseCommand(input string) (*ParsedCommand, error) {
	input = strings.TrimSpace(input)

	for action, pattern := range cp.patterns {
		if matches := pattern.FindStringSubmatch(input); matches != nil {
			command := &ParsedCommand{
				Action:     action,
				Parameters: make(map[string]string),
				Confidence: cp.calculateConfidence(action, matches),
			}

			// Extract parameters based on action type
			switch action {
			case "create_issue":
				if len(matches) > 1 && matches[1] != "" {
					command.Parameters["type"] = matches[1]
				}
				if len(matches) > 2 {
					command.Parameters["summary"] = matches[2]
				}
				if len(matches) > 3 && matches[3] != "" {
					command.Parameters["project"] = matches[3]
				}

			case "get_issue", "delete_issue", "watch_issue", "unwatch_issue":
				if len(matches) > 1 {
					command.Parameters["key"] = matches[1]
				}

			case "update_status", "transition":
				if len(matches) > 1 {
					command.Parameters["key"] = matches[1]
				}
				if len(matches) > 2 {
					command.Parameters["status"] = matches[2]
				}

			case "assign_issue":
				if len(matches) > 1 {
					command.Parameters["key"] = matches[1]
				}
				if len(matches) > 2 {
					command.Parameters["assignee"] = matches[2]
				}

			case "add_comment":
				if len(matches) > 1 {
					command.Parameters["key"] = matches[1]
				}
				if len(matches) > 2 {
					command.Parameters["comment"] = matches[2]
				}

			case "search_issues":
				if len(matches) > 1 {
					command.Parameters["query"] = matches[1]
				}

			case "update_issue":
				if len(matches) > 1 {
					command.Parameters["key"] = matches[1]
				}
				if len(matches) > 2 {
					command.Parameters["updates"] = matches[2]
				}

			case "link_issues":
				if len(matches) > 1 {
					command.Parameters["source"] = matches[1]
				}
				if len(matches) > 2 {
					command.Parameters["target"] = matches[2]
				}

			case "export_search":
				if len(matches) > 1 {
					command.Parameters["format"] = strings.ToLower(matches[1])
				}
			}

			return command, nil
		}
	}

	// If no patterns match, try to extract issue keys for fallback
	issuePattern := regexp.MustCompile(`([A-Z]+-\d+)`)
	if issueKeys := issuePattern.FindAllString(input, -1); len(issueKeys) > 0 {
		return &ParsedCommand{
			Action: "get_issue",
			Parameters: map[string]string{
				"key": issueKeys[0],
			},
			Confidence: 0.3, // Low confidence fallback
		}, nil
	}

	return nil, fmt.Errorf("unable to parse command: %s", input)
}

func (cp *CommandProcessor) calculateConfidence(action string, matches []string) float64 {
	baseConfidence := 0.7

	// Increase confidence based on match quality
	if len(matches) > 2 {
		baseConfidence += 0.1
	}

	// Action-specific confidence adjustments
	switch action {
	case "get_issue":
		if len(matches) > 1 && regexp.MustCompile(`^[A-Z]+-\d+$`).MatchString(matches[1]) {
			baseConfidence = 0.95 // Very high confidence for exact issue key matches
		}
	case "create_issue":
		if len(matches) > 2 && len(matches[2]) > 5 {
			baseConfidence += 0.1 // Higher confidence for detailed summaries
		}
	case "search_issues":
		if len(matches) > 1 && len(matches[1]) > 3 {
			baseConfidence += 0.1 // Higher confidence for specific search terms
		}
	}

	// Cap confidence at 1.0
	if baseConfidence > 1.0 {
		baseConfidence = 1.0
	}

	return baseConfidence
}

// GenerateJQLFromNaturalLanguage converts natural language to JQL
func (cp *CommandProcessor) GenerateJQLFromNaturalLanguage(input string) (string, error) {
	input = strings.ToLower(strings.TrimSpace(input))

	var jqlParts []string

	// Status patterns
	if strings.Contains(input, "open") || strings.Contains(input, "to do") {
		jqlParts = append(jqlParts, `status = "To Do"`)
	} else if strings.Contains(input, "in progress") || strings.Contains(input, "working on") {
		jqlParts = append(jqlParts, `status = "In Progress"`)
	} else if strings.Contains(input, "done") || strings.Contains(input, "completed") {
		jqlParts = append(jqlParts, `status = Done`)
	}

	// Priority patterns
	if strings.Contains(input, "high priority") || strings.Contains(input, "urgent") {
		jqlParts = append(jqlParts, `priority = High`)
	} else if strings.Contains(input, "low priority") {
		jqlParts = append(jqlParts, `priority = Low`)
	}

	// Assignee patterns
	if strings.Contains(input, "unassigned") {
		jqlParts = append(jqlParts, `assignee is EMPTY`)
	} else if strings.Contains(input, "assigned to me") || strings.Contains(input, "my issues") {
		jqlParts = append(jqlParts, `assignee = currentUser()`)
	}

	// Issue type patterns
	if strings.Contains(input, "bugs") {
		jqlParts = append(jqlParts, `issuetype = Bug`)
	} else if strings.Contains(input, "tasks") {
		jqlParts = append(jqlParts, `issuetype = Task`)
	} else if strings.Contains(input, "stories") {
		jqlParts = append(jqlParts, `issuetype = Story`)
	}

	// Time patterns
	if strings.Contains(input, "created today") {
		jqlParts = append(jqlParts, `created >= -1d`)
	} else if strings.Contains(input, "created this week") {
		jqlParts = append(jqlParts, `created >= -7d`)
	} else if strings.Contains(input, "updated today") {
		jqlParts = append(jqlParts, `updated >= -1d`)
	}

	// Project patterns
	projectPattern := regexp.MustCompile(`(?:project\s+|in\s+)([A-Z]{2,10})`)
	if projectMatch := projectPattern.FindStringSubmatch(input); len(projectMatch) > 1 {
		jqlParts = append(jqlParts, fmt.Sprintf(`project = %s`, projectMatch[1]))
	}

	// Text search patterns
	textPattern := regexp.MustCompile(`containing\s+"([^"]+)"`)
	if textMatch := textPattern.FindStringSubmatch(input); len(textMatch) > 1 {
		jqlParts = append(jqlParts, fmt.Sprintf(`text ~ "%s"`, textMatch[1]))
	}

	if len(jqlParts) == 0 {
		return "", fmt.Errorf("unable to generate JQL from input: %s", input)
	}

	return strings.Join(jqlParts, " AND "), nil
}

// SuggestCommands provides command suggestions based on partial input
func (cp *CommandProcessor) SuggestCommands(partialInput string) []string {
	partialInput = strings.ToLower(strings.TrimSpace(partialInput))

	suggestions := []string{}

	commandTemplates := map[string]string{
		"create": "create issue about [description] in [PROJECT]",
		"get":    "get issue [KEY]",
		"show":   "show issue [KEY]",
		"find":   "find issues about [search term]",
		"search": "search for [criteria] issues",
		"assign": "assign [KEY] to [username]",
		"update": "update [KEY] status to [status]",
		"move":   "move [KEY] to [status]",
		"comment": "comment on [KEY]: [message]",
		"link":   "link [KEY1] to [KEY2]",
		"export": "export results to csv/json/markdown",
		"list":   "list all projects",
	}

	for command, template := range commandTemplates {
		if strings.HasPrefix(command, partialInput) || strings.Contains(command, partialInput) {
			suggestions = append(suggestions, template)
		}
	}

	return suggestions
}

// ValidateCommand checks if a parsed command has all required parameters
func (cp *CommandProcessor) ValidateCommand(cmd *ParsedCommand) []string {
	var missingParams []string

	switch cmd.Action {
	case "create_issue":
		if _, ok := cmd.Parameters["summary"]; !ok {
			missingParams = append(missingParams, "summary")
		}
	case "get_issue", "delete_issue", "watch_issue", "unwatch_issue":
		if _, ok := cmd.Parameters["key"]; !ok {
			missingParams = append(missingParams, "issue key")
		}
	case "assign_issue":
		if _, ok := cmd.Parameters["key"]; !ok {
			missingParams = append(missingParams, "issue key")
		}
		if _, ok := cmd.Parameters["assignee"]; !ok {
			missingParams = append(missingParams, "assignee")
		}
	case "add_comment":
		if _, ok := cmd.Parameters["key"]; !ok {
			missingParams = append(missingParams, "issue key")
		}
		if _, ok := cmd.Parameters["comment"]; !ok {
			missingParams = append(missingParams, "comment text")
		}
	case "update_status", "transition":
		if _, ok := cmd.Parameters["key"]; !ok {
			missingParams = append(missingParams, "issue key")
		}
		if _, ok := cmd.Parameters["status"]; !ok {
			missingParams = append(missingParams, "target status")
		}
	case "link_issues":
		if _, ok := cmd.Parameters["source"]; !ok {
			missingParams = append(missingParams, "source issue key")
		}
		if _, ok := cmd.Parameters["target"]; !ok {
			missingParams = append(missingParams, "target issue key")
		}
	}

	return missingParams
}