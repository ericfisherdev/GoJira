package nlp

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func (p *Parser) initializeEntityRules() {
	// Issue key patterns
	p.entityRules[EntityIssueKey] = []EntityRule{
		{
			Name:    "standard_issue_key",
			Pattern: regexp.MustCompile(`\b([A-Z][A-Z0-9]{1,9}-\d+)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return matches[1]
				}
				return nil
			},
		},
	}

	// Priority patterns
	p.entityRules[EntityPriority] = []EntityRule{
		{
			Name:    "priority_level",
			Pattern: regexp.MustCompile(`(?i)\b(blocker|critical|highest|high|medium|normal|low|lowest|trivial|minor|major)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return normalizePriority(matches[1])
				}
				return nil
			},
		},
		{
			Name:    "priority_p_notation",
			Pattern: regexp.MustCompile(`(?i)\bP([0-4])\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return priorityFromP(matches[1])
				}
				return nil
			},
		},
	}

	// Issue type patterns
	p.entityRules[EntityIssueType] = []EntityRule{
		{
			Name:    "issue_type",
			Pattern: regexp.MustCompile(`(?i)\b(bug|task|story|epic|sub-?task|improvement|feature|defect|incident)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return normalizeIssueType(matches[1])
				}
				return nil
			},
		},
	}

	// Status patterns
	p.entityRules[EntityStatus] = []EntityRule{
		{
			Name:    "status",
			Pattern: regexp.MustCompile(`(?i)\b(open|in\s+progress|done|closed|resolved|reopened|blocked|ready|review|testing|todo|backlog)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return normalizeStatus(matches[1])
				}
				return nil
			},
		},
	}

	// Date patterns
	p.entityRules[EntityDate] = []EntityRule{
		{
			Name:    "iso_date",
			Pattern: regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return parseDate(matches[1])
				}
				return nil
			},
		},
		{
			Name:    "relative_date",
			Pattern: regexp.MustCompile(`(?i)\b(today|tomorrow|yesterday)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return parseRelativeDate(matches[1])
				}
				return nil
			},
		},
		{
			Name:    "next_last_date",
			Pattern: regexp.MustCompile(`(?i)\b(next|last)\s+(week|month|sprint|quarter|year)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 2 {
					return parseRelativePeriod(matches[1], matches[2])
				}
				return nil
			},
		},
		{
			Name:    "in_date",
			Pattern: regexp.MustCompile(`(?i)\bin\s+(\d+)\s+(days?|weeks?|months?)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 2 {
					return parseFutureDate(matches[1], matches[2])
				}
				return nil
			},
		},
	}

	// Sprint patterns
	p.entityRules[EntitySprint] = []EntityRule{
		{
			Name:    "sprint_number",
			Pattern: regexp.MustCompile(`(?i)\bsprint\s+(\d+)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return fmt.Sprintf("Sprint %s", matches[1])
				}
				return nil
			},
		},
		{
			Name:    "current_sprint",
			Pattern: regexp.MustCompile(`(?i)\b(current|active)\s+sprint\b`),
			Extract: func(matches []string) interface{} {
				return "current"
			},
		},
	}

	// User/Assignee patterns
	p.entityRules[EntityAssignee] = []EntityRule{
		{
			Name:    "at_mention",
			Pattern: regexp.MustCompile(`@(\w+(?:\.\w+)?)`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return matches[1]
				}
				return nil
			},
		},
		{
			Name:    "me_myself",
			Pattern: regexp.MustCompile(`(?i)\b(me|myself|my|mine)\b`),
			Extract: func(matches []string) interface{} {
				// Return special token that will be resolved to current user
				return "current_user"
			},
		},
		{
			Name:    "email",
			Pattern: regexp.MustCompile(`\b([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return matches[1]
				}
				return nil
			},
		},
		{
			Name:    "to_assignee",
			Pattern: regexp.MustCompile(`(?i)\bto\s+(\w+(?:\.\w+)?)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 && !isCommonWord(matches[1]) {
					return matches[1]
				}
				return nil
			},
		},
	}

	// Label patterns
	p.entityRules[EntityLabel] = []EntityRule{
		{
			Name:    "hashtag_label",
			Pattern: regexp.MustCompile(`#(\w+)`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return matches[1]
				}
				return nil
			},
		},
		{
			Name:    "label_keyword",
			Pattern: regexp.MustCompile(`(?i)\blabel(?:ed)?\s+(?:as\s+)?["']?(\w+(?:[-_]\w+)*)["']?`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return matches[1]
				}
				return nil
			},
		},
	}

	// Story points patterns
	p.entityRules[EntityStoryPoints] = []EntityRule{
		{
			Name:    "story_points",
			Pattern: regexp.MustCompile(`(?i)\b(\d+)\s+(?:story\s+)?points?\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					if points, err := strconv.Atoi(matches[1]); err == nil {
						return points
					}
				}
				return nil
			},
		},
		{
			Name:    "fibonacci_points",
			Pattern: regexp.MustCompile(`(?i)\bSP\s*[:=]?\s*(\d+)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					if points, err := strconv.Atoi(matches[1]); err == nil {
						return points
					}
				}
				return nil
			},
		},
	}

	// Component patterns
	p.entityRules[EntityComponent] = []EntityRule{
		{
			Name:    "component",
			Pattern: regexp.MustCompile(`(?i)\bcomponent\s+["']?(\w+(?:[-_]\w+)*)["']?`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return matches[1]
				}
				return nil
			},
		},
	}

	// Epic patterns
	p.entityRules[EntityEpic] = []EntityRule{
		{
			Name:    "epic_link",
			Pattern: regexp.MustCompile(`(?i)\bepic\s+([A-Z][A-Z0-9]{1,9}-\d+)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return matches[1]
				}
				return nil
			},
		},
	}

	// Fix version patterns
	p.entityRules[EntityFixVersion] = []EntityRule{
		{
			Name:    "version_number",
			Pattern: regexp.MustCompile(`(?i)\bversion\s+(\d+(?:\.\d+)*)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return matches[1]
				}
				return nil
			},
		},
		{
			Name:    "version_name",
			Pattern: regexp.MustCompile(`(?i)\bversion\s+["']?(\w+(?:[-_.]\w+)*)["']?`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					return matches[1]
				}
				return nil
			},
		},
	}

	// Number patterns (generic)
	p.entityRules[EntityNumber] = []EntityRule{
		{
			Name:    "number",
			Pattern: regexp.MustCompile(`\b(\d+)\b`),
			Extract: func(matches []string) interface{} {
				if len(matches) > 1 {
					if num, err := strconv.Atoi(matches[1]); err == nil {
						return num
					}
				}
				return nil
			},
		},
	}
}

// extractEntities extracts all entities from the input text
func (p *Parser) extractEntities(input string) map[string]Entity {
	entities := make(map[string]Entity)

	log.Debug().Str("input", input).Msg("Extracting entities")

	// Process each entity type
	for entityType, rules := range p.entityRules {
		for _, rule := range rules {
			if matches := rule.Pattern.FindAllStringSubmatch(input, -1); matches != nil {
				for _, match := range matches {
					value := rule.Extract(match)
					if value != nil {
						// Find position of match
						position := rule.Pattern.FindStringIndex(input)
						
						// Create entity
						entity := Entity{
							Type:       entityType,
							Value:      value,
							Text:       match[0],
							Position:   position,
							Confidence: p.calculateEntityConfidence(entityType, value),
						}

						// Normalize value if applicable
						if normalized := p.normalizeEntityValue(entityType, value); normalized != nil {
							entity.Normalized = fmt.Sprintf("%v", normalized)
						}

						// Use entity type as key, or add index for multiple
						key := string(entityType)
						if _, exists := entities[key]; exists {
							// Add with index if multiple entities of same type
							for i := 1; ; i++ {
								indexedKey := fmt.Sprintf("%s_%d", key, i)
								if _, exists := entities[indexedKey]; !exists {
									entities[indexedKey] = entity
									break
								}
							}
						} else {
							entities[key] = entity
						}

						log.Debug().
							Str("type", string(entityType)).
							Interface("value", value).
							Str("text", match[0]).
							Float64("confidence", entity.Confidence).
							Msg("Entity extracted")
					}
				}
			}
		}
	}

	// Extract project from context if not found
	if _, ok := entities[string(EntityProject)]; !ok {
		if project := p.extractProjectFromContext(input); project != nil {
			entities[string(EntityProject)] = Entity{
				Type:       EntityProject,
				Value:      project,
				Confidence: 0.7,
			}
		}
	}

	return entities
}

func (p *Parser) calculateEntityConfidence(entityType EntityType, value interface{}) float64 {
	// Base confidence
	confidence := 0.8

	// Adjust based on entity type and validation
	switch entityType {
	case EntityIssueKey:
		// Issue keys are very specific, high confidence
		confidence = 0.95
	case EntityAssignee:
		// Check if user exists in cache
		if userStr, ok := value.(string); ok {
			if _, exists := p.userCache[userStr]; exists {
				confidence = 0.95
			} else {
				confidence = 0.6
			}
		}
	case EntityProject:
		// Check if project exists in cache
		if proj, ok := value.(*Project); ok {
			if _, exists := p.projectCache[proj.Key]; exists {
				confidence = 0.95
			}
		}
	case EntityDate:
		// Dates are usually clear
		confidence = 0.9
	case EntityPriority, EntityStatus:
		// These have limited valid values
		confidence = 0.85
	}

	return confidence
}

func (p *Parser) normalizeEntityValue(entityType EntityType, value interface{}) interface{} {
	switch entityType {
	case EntityPriority:
		if priority, ok := value.(string); ok {
			return normalizePriority(priority)
		}
	case EntityStatus:
		if status, ok := value.(string); ok {
			return normalizeStatus(status)
		}
	case EntityIssueType:
		if issueType, ok := value.(string); ok {
			return normalizeIssueType(issueType)
		}
	}
	return nil
}

func (p *Parser) extractProjectFromContext(input string) *Project {
	upperInput := strings.ToUpper(input)
	
	// Look for project keys in cache
	for key, project := range p.projectCache {
		if strings.Contains(upperInput, strings.ToUpper(key)) {
			return project
		}
		if strings.Contains(upperInput, strings.ToUpper(project.Name)) {
			return project
		}
	}

	// Check context for last project
	if p.context.LastProject != "" {
		if project, ok := p.projectCache[p.context.LastProject]; ok {
			return project
		}
	}

	// Check config for default project
	if p.config.DefaultProject != "" {
		if project, ok := p.projectCache[p.config.DefaultProject]; ok {
			return project
		}
	}

	return nil
}

// Helper functions for normalization

func normalizePriority(priority string) string {
	lower := strings.ToLower(priority)
	switch lower {
	case "blocker", "highest", "critical":
		return "Highest"
	case "high", "major":
		return "High"
	case "medium", "normal":
		return "Medium"
	case "low", "minor":
		return "Low"
	case "lowest", "trivial":
		return "Lowest"
	default:
		return priority
	}
}

func priorityFromP(level string) string {
	switch level {
	case "0":
		return "Highest"
	case "1":
		return "High"
	case "2":
		return "Medium"
	case "3":
		return "Low"
	case "4":
		return "Lowest"
	default:
		return "Medium"
	}
}

func normalizeIssueType(issueType string) string {
	lower := strings.ToLower(issueType)
	switch lower {
	case "bug", "defect", "incident":
		return "Bug"
	case "story", "feature":
		return "Story"
	case "task":
		return "Task"
	case "epic":
		return "Epic"
	case "sub-task", "subtask":
		return "Sub-task"
	case "improvement":
		return "Improvement"
	default:
		// Capitalize first letter
		if len(issueType) > 0 {
			return strings.ToUpper(issueType[:1]) + strings.ToLower(issueType[1:])
		}
		return issueType
	}
}

func normalizeStatus(status string) string {
	lower := strings.ToLower(status)
	switch lower {
	case "open", "todo", "backlog":
		return "To Do"
	case "in progress", "doing":
		return "In Progress"
	case "done", "closed", "resolved":
		return "Done"
	case "review", "reviewing":
		return "In Review"
	case "testing", "test":
		return "Testing"
	case "blocked":
		return "Blocked"
	case "reopened":
		return "Reopened"
	default:
		// Capitalize words
		words := strings.Fields(status)
		for i, word := range words {
			if len(word) > 0 {
				words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
			}
		}
		return strings.Join(words, " ")
	}
}

func parseDate(dateStr string) time.Time {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

func parseRelativeDate(relative string) time.Time {
	now := time.Now()
	lower := strings.ToLower(relative)
	
	switch lower {
	case "today":
		return now
	case "tomorrow":
		return now.AddDate(0, 0, 1)
	case "yesterday":
		return now.AddDate(0, 0, -1)
	default:
		return now
	}
}

func parseRelativePeriod(direction, period string) time.Time {
	now := time.Now()
	multiplier := 1
	if strings.ToLower(direction) == "last" {
		multiplier = -1
	}

	switch strings.ToLower(period) {
	case "week":
		return now.AddDate(0, 0, 7*multiplier)
	case "month":
		return now.AddDate(0, 1*multiplier, 0)
	case "quarter":
		return now.AddDate(0, 3*multiplier, 0)
	case "year":
		return now.AddDate(1*multiplier, 0, 0)
	case "sprint":
		// Assume 2-week sprints
		return now.AddDate(0, 0, 14*multiplier)
	default:
		return now
	}
}

func parseFutureDate(amount, unit string) time.Time {
	now := time.Now()
	num, err := strconv.Atoi(amount)
	if err != nil {
		return now
	}

	switch strings.ToLower(unit) {
	case "day", "days":
		return now.AddDate(0, 0, num)
	case "week", "weeks":
		return now.AddDate(0, 0, num*7)
	case "month", "months":
		return now.AddDate(0, num, 0)
	default:
		return now
	}
}

func isCommonWord(word string) bool {
	commonWords := []string{"the", "a", "an", "in", "on", "at", "to", "for", "of", "with", "by", "from", "as", "is", "was", "are", "were"}
	lower := strings.ToLower(word)
	for _, common := range commonWords {
		if lower == common {
			return true
		}
	}
	return false
}