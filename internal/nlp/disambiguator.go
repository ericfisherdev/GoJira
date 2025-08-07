package nlp

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
)

// Disambiguator handles command disambiguation and clarification
type Disambiguator struct {
	parser  *Parser
	context *Context
}

// NewDisambiguator creates a new disambiguator instance
func NewDisambiguator(parser *Parser) *Disambiguator {
	return &Disambiguator{
		parser:  parser,
		context: parser.GetContext(),
	}
}

// Disambiguate processes an intent and returns clarifications if needed
func (d *Disambiguator) Disambiguate(intent *Intent) (*Intent, []Clarification, error) {
	log.Debug().
		Str("intentType", string(intent.Type)).
		Float64("confidence", intent.Confidence).
		Msg("Disambiguating intent")

	clarifications := make([]Clarification, 0)

	// Check for missing required entities
	requiredEntities := d.getRequiredEntities(intent.Type)
	
	for _, required := range requiredEntities {
		if _, exists := intent.Entities[required]; !exists {
			// Try to infer from context
			if inferredEntity := d.inferFromContext(required); inferredEntity != nil {
				intent.Entities[required] = *inferredEntity
				log.Debug().
					Str("entity", required).
					Interface("value", inferredEntity.Value).
					Msg("Inferred entity from context")
			} else {
				clarifications = append(clarifications, Clarification{
					Field:      required,
					Message:    d.getClarificationMessage(required),
					Options:    d.getSuggestions(required),
					Required:   true,
					EntityType: EntityType(strings.ToUpper(required)),
				})
			}
		}
	}

	// Validate existing entities
	for key, entity := range intent.Entities {
		if validationError := d.validateEntity(entity); validationError != nil {
			// Try to find similar valid options
			suggestions := d.findSimilarOptions(entity)
			clarifications = append(clarifications, Clarification{
				Field:      key,
				Message:    fmt.Sprintf("'%s' is not a valid %s. %s", entity.Text, entity.Type, validationError.Error()),
				Options:    suggestions,
				Required:   false,
				EntityType: entity.Type,
			})
		}
	}

	// Check for ambiguous entities
	ambiguousEntities := d.findAmbiguousEntities(intent)
	for _, ambiguous := range ambiguousEntities {
		clarifications = append(clarifications, ambiguous)
	}

	// Resolve entity references
	d.resolveEntityReferences(intent)

	return intent, clarifications, nil
}

func (d *Disambiguator) getRequiredEntities(intentType IntentType) []string {
	switch intentType {
	case IntentCreate:
		return []string{"project", "issue_type"}
	case IntentUpdate:
		return []string{"issue_key"}
	case IntentTransition:
		return []string{"issue_key", "status"}
	case IntentAssign:
		return []string{"issue_key", "assignee"}
	case IntentComment:
		return []string{"issue_key"}
	case IntentDelete:
		return []string{"issue_key"}
	case IntentLink:
		// Need at least two issue keys or one issue key and a target
		return []string{"issue_key"}
	case IntentSearch:
		// Search can work without specific entities
		return []string{}
	case IntentReport:
		// Reports might need project or sprint context
		return []string{}
	default:
		return []string{}
	}
}

func (d *Disambiguator) inferFromContext(entityType string) *Entity {
	log.Debug().Str("entityType", entityType).Msg("Attempting to infer entity from context")

	switch entityType {
	case "project":
		// Try from last used project
		if d.context.LastProject != "" {
			if project, exists := d.parser.projectCache[d.context.LastProject]; exists {
				return &Entity{
					Type:       EntityProject,
					Value:      project,
					Text:       d.context.LastProject,
					Confidence: 0.7,
				}
			}
		}
		// Try from default project in config
		if d.parser.config.DefaultProject != "" {
			if project, exists := d.parser.projectCache[d.parser.config.DefaultProject]; exists {
				return &Entity{
					Type:       EntityProject,
					Value:      project,
					Text:       d.parser.config.DefaultProject,
					Confidence: 0.6,
				}
			}
		}

	case "assignee":
		// Check for "me" or current user context
		if d.parser.config.DefaultAssignee != "" {
			return &Entity{
				Type:       EntityAssignee,
				Value:      d.parser.config.DefaultAssignee,
				Text:       d.parser.config.DefaultAssignee,
				Confidence: 0.6,
			}
		}

	case "issue_key":
		// Try from last mentioned issue
		if d.context.LastIssue != "" {
			return &Entity{
				Type:       EntityIssueKey,
				Value:      d.context.LastIssue,
				Text:       d.context.LastIssue,
				Confidence: 0.7,
			}
		}

	case "sprint":
		// Try from last sprint context
		if d.context.LastSprint != "" {
			return &Entity{
				Type:       EntitySprint,
				Value:      d.context.LastSprint,
				Text:       d.context.LastSprint,
				Confidence: 0.7,
			}
		}

	case "status":
		// For transitions, we might infer common target states
		if d.context.LastStatus != "" {
			nextStates := d.getCommonTransitions(d.context.LastStatus)
			if len(nextStates) == 1 {
				return &Entity{
					Type:       EntityStatus,
					Value:      nextStates[0],
					Text:       nextStates[0],
					Confidence: 0.6,
				}
			}
		}
	}

	return nil
}

func (d *Disambiguator) getClarificationMessage(entityType string) string {
	switch entityType {
	case "project":
		return "Which project should this be created in?"
	case "issue_key":
		return "Which issue would you like to work with?"
	case "issue_type":
		return "What type of issue would you like to create?"
	case "assignee":
		return "Who should this be assigned to?"
	case "status":
		return "What status should the issue be moved to?"
	case "priority":
		return "What priority should this issue have?"
	case "sprint":
		return "Which sprint are you referring to?"
	case "component":
		return "Which component does this relate to?"
	default:
		return fmt.Sprintf("Please specify the %s", strings.ReplaceAll(entityType, "_", " "))
	}
}

func (d *Disambiguator) getSuggestions(entityType string) []string {
	switch entityType {
	case "project":
		return d.getProjectSuggestions()
	case "issue_type":
		return []string{"Bug", "Task", "Story", "Epic", "Sub-task"}
	case "priority":
		return []string{"Highest", "High", "Medium", "Low", "Lowest"}
	case "status":
		return []string{"To Do", "In Progress", "In Review", "Testing", "Done"}
	case "assignee":
		return d.getUserSuggestions()
	case "component":
		return d.getComponentSuggestions()
	default:
		return []string{}
	}
}

func (d *Disambiguator) getProjectSuggestions() []string {
	suggestions := make([]string, 0, len(d.parser.projectCache))
	for key, project := range d.parser.projectCache {
		suggestions = append(suggestions, fmt.Sprintf("%s (%s)", key, project.Name))
	}
	sort.Strings(suggestions)
	
	// Limit to top 10
	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}
	
	return suggestions
}

func (d *Disambiguator) getUserSuggestions() []string {
	suggestions := make([]string, 0, len(d.parser.userCache))
	for username, user := range d.parser.userCache {
		if user.Active {
			suggestions = append(suggestions, fmt.Sprintf("%s (%s)", username, user.DisplayName))
		}
	}
	sort.Strings(suggestions)
	
	// Limit to top 10
	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}
	
	return suggestions
}

func (d *Disambiguator) getComponentSuggestions() []string {
	// This would ideally come from Jira API or cache
	// For now, return common component names
	return []string{"frontend", "backend", "api", "database", "ui", "core", "auth", "payments"}
}

func (d *Disambiguator) validateEntity(entity Entity) error {
	switch entity.Type {
	case EntityIssueKey:
		// Validate issue key format
		if !isValidIssueKey(entity.Text) {
			return fmt.Errorf("invalid issue key format")
		}
		
	case EntityProject:
		// Check if project exists in cache
		if project, ok := entity.Value.(*Project); ok {
			if _, exists := d.parser.projectCache[project.Key]; !exists {
				return fmt.Errorf("project not found")
			}
		}
		
	case EntityAssignee:
		// Check if user exists and is active
		if username, ok := entity.Value.(string); ok {
			if username == "current_user" {
				// This is valid - will be resolved later
				return nil
			}
			if user, exists := d.parser.userCache[username]; !exists {
				return fmt.Errorf("user not found")
			} else if !user.Active {
				return fmt.Errorf("user is not active")
			}
		}
		
	case EntityPriority:
		// Validate priority level
		if !isValidPriority(fmt.Sprintf("%v", entity.Value)) {
			return fmt.Errorf("invalid priority level")
		}
		
	case EntityStatus:
		// Check if status exists in cache
		if statusName, ok := entity.Value.(string); ok {
			if _, exists := d.parser.statusCache[statusName]; !exists {
				return fmt.Errorf("status not found")
			}
		}
	}
	
	return nil
}

func (d *Disambiguator) findSimilarOptions(entity Entity) []string {
	suggestions := []string{}
	
	switch entity.Type {
	case EntityProject:
		// Find projects with similar names
		for key, project := range d.parser.projectCache {
			if similarity := calculateStringSimilarity(entity.Text, key); similarity > 0.6 {
				suggestions = append(suggestions, key)
			} else if similarity := calculateStringSimilarity(entity.Text, project.Name); similarity > 0.6 {
				suggestions = append(suggestions, fmt.Sprintf("%s (%s)", key, project.Name))
			}
		}
		
	case EntityAssignee:
		// Find users with similar names
		for username, user := range d.parser.userCache {
			if similarity := calculateStringSimilarity(entity.Text, username); similarity > 0.6 {
				suggestions = append(suggestions, fmt.Sprintf("%s (%s)", username, user.DisplayName))
			} else if similarity := calculateStringSimilarity(entity.Text, user.DisplayName); similarity > 0.6 {
				suggestions = append(suggestions, fmt.Sprintf("%s (%s)", username, user.DisplayName))
			}
		}
		
	case EntityStatus:
		// Find similar status names
		for statusName := range d.parser.statusCache {
			if similarity := calculateStringSimilarity(entity.Text, statusName); similarity > 0.6 {
				suggestions = append(suggestions, statusName)
			}
		}
	}
	
	// Sort by similarity (this is a simplified sort)
	sort.Strings(suggestions)
	
	// Limit suggestions
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}
	
	return suggestions
}

func (d *Disambiguator) findAmbiguousEntities(intent *Intent) []Clarification {
	clarifications := []Clarification{}
	
	// Check for multiple potential matches
	for key, entity := range intent.Entities {
		switch entity.Type {
		case EntityProject:
			// If confidence is low, it might be ambiguous
			if entity.Confidence < 0.8 {
				alternatives := d.findAlternativeProjects(entity.Text)
				if len(alternatives) > 1 {
					clarifications = append(clarifications, Clarification{
						Field:      key,
						Message:    fmt.Sprintf("Multiple projects match '%s'. Which one did you mean?", entity.Text),
						Options:    alternatives,
						Required:   true,
						EntityType: EntityProject,
					})
				}
			}
			
		case EntityAssignee:
			// Check for ambiguous user names
			if entity.Confidence < 0.8 {
				alternatives := d.findAlternativeUsers(entity.Text)
				if len(alternatives) > 1 {
					clarifications = append(clarifications, Clarification{
						Field:      key,
						Message:    fmt.Sprintf("Multiple users match '%s'. Which one did you mean?", entity.Text),
						Options:    alternatives,
						Required:   true,
						EntityType: EntityAssignee,
					})
				}
			}
		}
	}
	
	return clarifications
}

func (d *Disambiguator) findAlternativeProjects(text string) []string {
	alternatives := []string{}
	
	for key, project := range d.parser.projectCache {
		if strings.Contains(strings.ToLower(key), strings.ToLower(text)) ||
		   strings.Contains(strings.ToLower(project.Name), strings.ToLower(text)) {
			alternatives = append(alternatives, fmt.Sprintf("%s (%s)", key, project.Name))
		}
	}
	
	return alternatives
}

func (d *Disambiguator) findAlternativeUsers(text string) []string {
	alternatives := []string{}
	
	for username, user := range d.parser.userCache {
		if user.Active && (strings.Contains(strings.ToLower(username), strings.ToLower(text)) ||
		   strings.Contains(strings.ToLower(user.DisplayName), strings.ToLower(text))) {
			alternatives = append(alternatives, fmt.Sprintf("%s (%s)", username, user.DisplayName))
		}
	}
	
	return alternatives
}

func (d *Disambiguator) resolveEntityReferences(intent *Intent) {
	for key, entity := range intent.Entities {
		switch entity.Type {
		case EntityAssignee:
			if entity.Value == "current_user" {
				// Resolve to actual current user
				if d.parser.config.DefaultAssignee != "" {
					intent.Entities[key] = Entity{
						Type:       EntityAssignee,
						Value:      d.parser.config.DefaultAssignee,
						Text:       "me",
						Confidence: 0.9,
						Normalized: d.parser.config.DefaultAssignee,
					}
				}
			}
		}
	}
}

func (d *Disambiguator) getCommonTransitions(fromStatus string) []string {
	// Common workflow transitions
	transitions := map[string][]string{
		"To Do":       {"In Progress"},
		"In Progress": {"In Review", "Testing", "Done", "Blocked"},
		"In Review":   {"In Progress", "Testing", "Done"},
		"Testing":     {"In Progress", "Done", "In Review"},
		"Blocked":     {"To Do", "In Progress"},
		"Done":        {"Reopened"},
	}
	
	if nextStates, exists := transitions[fromStatus]; exists {
		return nextStates
	}
	
	return []string{}
}

// Utility functions

func isValidIssueKey(key string) bool {
	// Check if it matches the pattern: PROJECT-NUMBER
	matched, _ := regexp.Match(`^[A-Z][A-Z0-9]{1,9}-\d+$`, []byte(key))
	return matched
}

func isValidPriority(priority string) bool {
	validPriorities := []string{"Highest", "High", "Medium", "Low", "Lowest", "Blocker", "Critical", "Major", "Minor", "Trivial"}
	for _, valid := range validPriorities {
		if strings.EqualFold(priority, valid) {
			return true
		}
	}
	return false
}

func calculateStringSimilarity(a, b string) float64 {
	// Simple similarity calculation using common prefixes
	// This is a simplified version - in production, you might use more sophisticated algorithms
	
	if a == b {
		return 1.0
	}
	
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return 0.8
	}
	
	// Check for common prefix
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	
	commonPrefix := 0
	for i := 0; i < minLen; i++ {
		if a[i] == b[i] {
			commonPrefix++
		} else {
			break
		}
	}
	
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	
	return float64(commonPrefix) / float64(maxLen)
}

// UpdateContext updates the disambiguation context
func (d *Disambiguator) UpdateContext(intent *Intent) {
	// Update last used values for future inference
	if project, ok := intent.Entities["project"]; ok {
		if proj, ok := project.Value.(*Project); ok {
			d.context.LastProject = proj.Key
		}
	}
	
	if issue, ok := intent.Entities["issue_key"]; ok {
		d.context.LastIssue = issue.Text
	}
	
	if assignee, ok := intent.Entities["assignee"]; ok {
		d.context.LastAssignee = assignee.Text
	}
	
	if status, ok := intent.Entities["status"]; ok {
		d.context.LastStatus = status.Text
	}
	
	// Update history
	d.context.History = append(d.context.History, *intent)
	
	// Trim history if too long
	if len(d.context.History) > 50 {
		d.context.History = d.context.History[1:]
	}
	
	log.Debug().
		Str("lastProject", d.context.LastProject).
		Str("lastIssue", d.context.LastIssue).
		Str("lastAssignee", d.context.LastAssignee).
		Int("historySize", len(d.context.History)).
		Msg("Updated disambiguation context")
}