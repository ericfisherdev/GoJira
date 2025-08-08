package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/rs/zerolog/log"
)

// SprintService provides business logic for sprint operations
type SprintService struct {
	jiraClient jira.ClientInterface
	cache      *SprintCache
}

// SprintCache provides in-memory caching for sprint data
type SprintCache struct {
	sprints    map[int]*jira.Sprint
	boardCache map[int][]int // boardID -> []sprintIDs
	lastUpdate map[string]time.Time
	ttl        time.Duration
}

// NewSprintService creates a new sprint service
func NewSprintService(jiraClient jira.ClientInterface) *SprintService {
	return &SprintService{
		jiraClient: jiraClient,
		cache: &SprintCache{
			sprints:    make(map[int]*jira.Sprint),
			boardCache: make(map[int][]int),
			lastUpdate: make(map[string]time.Time),
			ttl:        5 * time.Minute,
		},
	}
}

// SprintValidation provides sprint validation rules
type SprintValidation struct {
	MinDuration   time.Duration
	MaxDuration   time.Duration
	MaxNameLength int
	MaxGoalLength int
}

// DefaultSprintValidation returns default validation rules
func DefaultSprintValidation() *SprintValidation {
	return &SprintValidation{
		MinDuration:   7 * 24 * time.Hour,  // 1 week minimum
		MaxDuration:   30 * 24 * time.Hour, // 30 days maximum
		MaxNameLength: 255,
		MaxGoalLength: 1000,
	}
}

// ValidateSprint validates sprint data
func (s *SprintService) ValidateSprint(req *jira.CreateSprintRequest) error {
	validation := DefaultSprintValidation()
	
	// Validate name
	if len(req.Name) == 0 {
		return fmt.Errorf("sprint name is required")
	}
	if len(req.Name) > validation.MaxNameLength {
		return fmt.Errorf("sprint name exceeds maximum length of %d characters", validation.MaxNameLength)
	}
	
	// Validate goal
	if len(req.Goal) > validation.MaxGoalLength {
		return fmt.Errorf("sprint goal exceeds maximum length of %d characters", validation.MaxGoalLength)
	}
	
	// Validate dates if provided
	if req.StartDate != nil && req.EndDate != nil {
		if req.StartDate.After(*req.EndDate) {
			return fmt.Errorf("start date must be before end date")
		}
		
		duration := req.EndDate.Sub(*req.StartDate)
		if duration < validation.MinDuration {
			return fmt.Errorf("sprint duration must be at least %v", validation.MinDuration)
		}
		if duration > validation.MaxDuration {
			return fmt.Errorf("sprint duration cannot exceed %v", validation.MaxDuration)
		}
	}
	
	// Validate board ID
	if req.OriginBoardID <= 0 {
		return fmt.Errorf("valid board ID is required")
	}
	
	return nil
}

// GetActiveSprints retrieves all active sprints across boards
func (s *SprintService) GetActiveSprints(ctx context.Context) ([]*jira.Sprint, error) {
	// Get all boards first
	boards, err := s.jiraClient.GetBoards()
	if err != nil {
		return nil, fmt.Errorf("failed to get boards: %w", err)
	}
	
	var activeSprints []*jira.Sprint
	for _, board := range boards.Values {
		sprints, err := s.jiraClient.GetSprints(board.ID)
		if err != nil {
			log.Warn().Err(err).Int("boardId", board.ID).Msg("Failed to get sprints for board")
			continue
		}
		
		for _, sprint := range sprints.Values {
			if sprint.State == "active" {
				activeSprint := sprint
				activeSprints = append(activeSprints, &activeSprint)
				// Cache the sprint
				s.cache.sprints[sprint.ID] = &activeSprint
			}
		}
	}
	
	// Sort by start date
	sort.Slice(activeSprints, func(i, j int) bool {
		if activeSprints[i].StartDate == nil || activeSprints[j].StartDate == nil {
			return false
		}
		return activeSprints[i].StartDate.Before(*activeSprints[j].StartDate)
	})
	
	return activeSprints, nil
}

// GetUpcomingSprints retrieves future sprints
func (s *SprintService) GetUpcomingSprints(ctx context.Context, boardID int) ([]*jira.Sprint, error) {
	sprints, err := s.jiraClient.GetSprints(boardID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sprints: %w", err)
	}
	
	var upcomingSprints []*jira.Sprint
	now := time.Now()
	
	for _, sprint := range sprints.Values {
		if sprint.State == "future" || 
		   (sprint.StartDate != nil && sprint.StartDate.After(now)) {
			upcomingSprint := sprint
			upcomingSprints = append(upcomingSprints, &upcomingSprint)
		}
	}
	
	// Sort by planned start date
	sort.Slice(upcomingSprints, func(i, j int) bool {
		if upcomingSprints[i].StartDate == nil || upcomingSprints[j].StartDate == nil {
			return upcomingSprints[i].ID < upcomingSprints[j].ID
		}
		return upcomingSprints[i].StartDate.Before(*upcomingSprints[j].StartDate)
	})
	
	return upcomingSprints, nil
}

// AutoStartSprint automatically starts a sprint if conditions are met
func (s *SprintService) AutoStartSprint(ctx context.Context, sprintID int) error {
	sprint, err := s.jiraClient.GetSprint(sprintID)
	if err != nil {
		return fmt.Errorf("failed to get sprint: %w", err)
	}
	
	// Check if sprint can be started
	if sprint.State != "future" {
		return fmt.Errorf("sprint is not in future state (current: %s)", sprint.State)
	}
	
	// Check for active sprints on the same board
	boardSprints, err := s.jiraClient.GetSprints(sprint.OriginBoardID)
	if err != nil {
		return fmt.Errorf("failed to check board sprints: %w", err)
	}
	
	for _, bs := range boardSprints.Values {
		if bs.State == "active" && bs.ID != sprintID {
			return fmt.Errorf("board already has an active sprint: %s", bs.Name)
		}
	}
	
	// Set default dates if not provided
	startDate := time.Now()
	endDate := startDate.AddDate(0, 0, 14) // 2 weeks by default
	
	if sprint.StartDate != nil {
		startDate = *sprint.StartDate
	}
	if sprint.EndDate != nil {
		endDate = *sprint.EndDate
	}
	
	// Start the sprint
	err = s.jiraClient.StartSprint(sprintID, startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to start sprint: %w", err)
	}
	
	log.Info().
		Int("sprintId", sprintID).
		Str("name", sprint.Name).
		Time("startDate", startDate).
		Time("endDate", endDate).
		Msg("Sprint auto-started")
	
	return nil
}

// CompleteSprintWithReport closes a sprint and generates a completion report
func (s *SprintService) CompleteSprintWithReport(ctx context.Context, sprintID int, moveIncomplete string) (*SprintCompletionReport, error) {
	// Get sprint details
	sprint, err := s.jiraClient.GetSprint(sprintID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sprint: %w", err)
	}
	
	if sprint.State != "active" {
		return nil, fmt.Errorf("sprint is not active (current: %s)", sprint.State)
	}
	
	// Get sprint issues before closing
	issues, err := s.jiraClient.GetSprintIssues(sprintID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sprint issues: %w", err)
	}
	
	// Analyze completion
	report := s.analyzeSprintCompletion(sprint, issues)
	
	// Handle incomplete issues
	if len(report.IncompleteIssues) > 0 {
		switch moveIncomplete {
		case "backlog":
			// Move to backlog
			var issueKeys []string
			for _, issue := range report.IncompleteIssues {
				issueKeys = append(issueKeys, issue.Key)
			}
			if err := s.jiraClient.MoveIssuesToBacklog(issueKeys); err != nil {
				log.Warn().Err(err).Msg("Failed to move issues to backlog")
			}
		case "next":
			// Move to next sprint
			nextSprint, err := s.findNextSprint(ctx, sprint.OriginBoardID)
			if err == nil && nextSprint != nil {
				var issueKeys []string
				for _, issue := range report.IncompleteIssues {
					issueKeys = append(issueKeys, issue.Key)
				}
				if err := s.jiraClient.MoveIssuesToSprint(nextSprint.ID, issueKeys); err != nil {
					log.Warn().Err(err).Msg("Failed to move issues to next sprint")
				}
			}
		}
	}
	
	// Close the sprint
	if err := s.jiraClient.CloseSprint(sprintID); err != nil {
		return nil, fmt.Errorf("failed to close sprint: %w", err)
	}
	
	report.CompletedAt = time.Now()
	
	return report, nil
}

// SprintCompletionReport provides detailed sprint completion metrics
type SprintCompletionReport struct {
	Sprint           *jira.Sprint
	CompletedIssues  []jira.SprintIssue
	IncompleteIssues []jira.SprintIssue
	TotalIssues      int
	CompletionRate   float64
	Velocity         float64
	CompletedAt      time.Time
	Duration         time.Duration
}

func (s *SprintService) analyzeSprintCompletion(sprint *jira.Sprint, issues *jira.SprintIssueList) *SprintCompletionReport {
	report := &SprintCompletionReport{
		Sprint:      sprint,
		TotalIssues: len(issues.Issues),
	}
	
	for _, issue := range issues.Issues {
		if issue.Fields.Status.StatusCategory.Key == "done" {
			report.CompletedIssues = append(report.CompletedIssues, issue)
		} else {
			report.IncompleteIssues = append(report.IncompleteIssues, issue)
		}
	}
	
	if report.TotalIssues > 0 {
		report.CompletionRate = float64(len(report.CompletedIssues)) / float64(report.TotalIssues) * 100
	}
	
	// Calculate velocity (simplified - count of completed issues)
	report.Velocity = float64(len(report.CompletedIssues))
	
	// Calculate duration
	if sprint.StartDate != nil && sprint.EndDate != nil {
		report.Duration = sprint.EndDate.Sub(*sprint.StartDate)
	}
	
	return report
}

func (s *SprintService) findNextSprint(ctx context.Context, boardID int) (*jira.Sprint, error) {
	sprints, err := s.jiraClient.GetSprints(boardID)
	if err != nil {
		return nil, err
	}
	
	// Find the first future sprint
	for _, sprint := range sprints.Values {
		if sprint.State == "future" {
			return &sprint, nil
		}
	}
	
	return nil, fmt.Errorf("no future sprint found")
}

// GetSprintMetrics calculates advanced metrics for a sprint
func (s *SprintService) GetSprintMetrics(ctx context.Context, sprintID int) (*SprintMetrics, error) {
	sprint, err := s.jiraClient.GetSprint(sprintID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sprint: %w", err)
	}
	
	issues, err := s.jiraClient.GetSprintIssues(sprintID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sprint issues: %w", err)
	}
	
	metrics := &SprintMetrics{
		SprintID:   sprintID,
		SprintName: sprint.Name,
		State:      sprint.State,
	}
	
	// Calculate metrics
	metrics.TotalIssues = len(issues.Issues)
	
	statusCounts := make(map[string]int)
	for _, issue := range issues.Issues {
		status := issue.Fields.Status.Name
		statusCounts[status]++
		
		if issue.Fields.Status.StatusCategory.Key == "done" {
			metrics.CompletedIssues++
		} else if issue.Fields.Status.StatusCategory.Key == "indeterminate" {
			metrics.InProgressIssues++
		} else {
			metrics.TodoIssues++
		}
	}
	
	metrics.StatusDistribution = statusCounts
	
	// Calculate completion percentage
	if metrics.TotalIssues > 0 {
		metrics.CompletionPercentage = float64(metrics.CompletedIssues) / float64(metrics.TotalIssues) * 100
	}
	
	// Calculate burndown rate
	if sprint.StartDate != nil && sprint.State == "active" {
		daysSinceStart := time.Since(*sprint.StartDate).Hours() / 24
		if daysSinceStart > 0 {
			metrics.BurndownRate = float64(metrics.CompletedIssues) / daysSinceStart
		}
	}
	
	// Estimate completion
	if metrics.BurndownRate > 0 && metrics.TodoIssues+metrics.InProgressIssues > 0 {
		daysToComplete := float64(metrics.TodoIssues+metrics.InProgressIssues) / metrics.BurndownRate
		metrics.EstimatedCompletion = time.Now().AddDate(0, 0, int(daysToComplete))
	}
	
	return metrics, nil
}

// SprintMetrics provides detailed sprint metrics
type SprintMetrics struct {
	SprintID             int
	SprintName           string
	State                string
	TotalIssues          int
	CompletedIssues      int
	InProgressIssues     int
	TodoIssues           int
	CompletionPercentage float64
	BurndownRate         float64 // Issues per day
	EstimatedCompletion  time.Time
	StatusDistribution   map[string]int
}

// PredictSprintSuccess uses historical data to predict sprint success
func (s *SprintService) PredictSprintSuccess(ctx context.Context, sprintID int) (*SprintPrediction, error) {
	metrics, err := s.GetSprintMetrics(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	
	sprint, err := s.jiraClient.GetSprint(sprintID)
	if err != nil {
		return nil, err
	}
	
	prediction := &SprintPrediction{
		SprintID:   sprintID,
		SprintName: sprint.Name,
	}
	
	// Calculate success probability based on current progress
	if sprint.StartDate != nil && sprint.EndDate != nil {
		totalDays := sprint.EndDate.Sub(*sprint.StartDate).Hours() / 24
		elapsedDays := time.Since(*sprint.StartDate).Hours() / 24
		remainingDays := totalDays - elapsedDays
		
		if remainingDays > 0 && metrics.BurndownRate > 0 {
			requiredRate := float64(metrics.TodoIssues+metrics.InProgressIssues) / remainingDays
			prediction.RequiredBurndownRate = requiredRate
			
			// Simple prediction based on current vs required rate
			if metrics.BurndownRate >= requiredRate {
				prediction.SuccessProbability = min(95, 60+metrics.CompletionPercentage*0.35)
				prediction.RiskLevel = "Low"
			} else if metrics.BurndownRate >= requiredRate*0.7 {
				prediction.SuccessProbability = min(70, 40+metrics.CompletionPercentage*0.3)
				prediction.RiskLevel = "Medium"
			} else {
				prediction.SuccessProbability = min(40, 20+metrics.CompletionPercentage*0.2)
				prediction.RiskLevel = "High"
			}
		}
		
		// Add recommendations
		if prediction.RiskLevel == "High" {
			prediction.Recommendations = append(prediction.Recommendations,
				"Consider moving some issues to the next sprint",
				"Increase team velocity or add resources",
				"Re-evaluate issue priorities")
		} else if prediction.RiskLevel == "Medium" {
			prediction.Recommendations = append(prediction.Recommendations,
				"Monitor progress closely",
				"Consider descoping lower priority items if needed")
		}
	}
	
	return prediction, nil
}

// SprintPrediction provides sprint success predictions
type SprintPrediction struct {
	SprintID             int
	SprintName           string
	SuccessProbability   float64
	RequiredBurndownRate float64
	RiskLevel            string // Low, Medium, High
	Recommendations      []string
}

// Helper function for min
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}