package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/ericfisherdev/GoJira/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

var sprintService *services.SprintService

// SetSprintService sets the global sprint service
func SetSprintService(service *services.SprintService) {
	sprintService = service
}

// GetActiveSprints retrieves all active sprints across boards
func GetActiveSprints(w http.ResponseWriter, r *http.Request) {
	if sprintService == nil {
		sprintService = services.NewSprintService(jiraClient)
	}
	
	ctx := r.Context()
	sprints, err := sprintService.GetActiveSprints(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get active sprints")
		render.Render(w, r, ErrInternalServer(err))
		return
	}
	
	render.JSON(w, r, map[string]interface{}{
		"sprints": sprints,
		"count":   len(sprints),
	})
}

// GetUpcomingSprints retrieves upcoming/future sprints for a board
func GetUpcomingSprints(w http.ResponseWriter, r *http.Request) {
	if sprintService == nil {
		sprintService = services.NewSprintService(jiraClient)
	}
	
	boardIDStr := r.URL.Query().Get("boardId")
	if boardIDStr == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("boardId is required")))
		return
	}
	
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid boardId")))
		return
	}
	
	ctx := r.Context()
	sprints, err := sprintService.GetUpcomingSprints(ctx, boardID)
	if err != nil {
		log.Error().Err(err).Int("boardId", boardID).Msg("Failed to get upcoming sprints")
		render.Render(w, r, ErrInternalServer(err))
		return
	}
	
	render.JSON(w, r, map[string]interface{}{
		"sprints": sprints,
		"count":   len(sprints),
		"boardId": boardID,
	})
}

// AutoStartSprint automatically starts a sprint if conditions are met
func AutoStartSprint(w http.ResponseWriter, r *http.Request) {
	if sprintService == nil {
		sprintService = services.NewSprintService(jiraClient)
	}
	
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}
	
	ctx := r.Context()
	err = sprintService.AutoStartSprint(ctx, sprintID)
	if err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Msg("Failed to auto-start sprint")
		render.Render(w, r, ErrInternalServer(err))
		return
	}
	
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Sprint auto-started successfully",
		"sprintId": sprintID,
	})
}

// CompleteSprintWithReport closes a sprint and generates a completion report
func CompleteSprintWithReport(w http.ResponseWriter, r *http.Request) {
	if sprintService == nil {
		sprintService = services.NewSprintService(jiraClient)
	}
	
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}
	
	var req struct {
		MoveIncomplete string `json:"moveIncomplete"` // "backlog", "next", or "none"
	}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		req.MoveIncomplete = "backlog" // Default to backlog
	}
	
	ctx := r.Context()
	report, err := sprintService.CompleteSprintWithReport(ctx, sprintID, req.MoveIncomplete)
	if err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Msg("Failed to complete sprint with report")
		render.Render(w, r, ErrInternalServer(err))
		return
	}
	
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"report":  report,
	})
}

// GetSprintMetrics retrieves detailed metrics for a sprint
func GetSprintMetrics(w http.ResponseWriter, r *http.Request) {
	if sprintService == nil {
		sprintService = services.NewSprintService(jiraClient)
	}
	
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}
	
	ctx := r.Context()
	metrics, err := sprintService.GetSprintMetrics(ctx, sprintID)
	if err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Msg("Failed to get sprint metrics")
		render.Render(w, r, ErrInternalServer(err))
		return
	}
	
	render.JSON(w, r, metrics)
}

// PredictSprintSuccess predicts the likelihood of sprint success
func PredictSprintSuccess(w http.ResponseWriter, r *http.Request) {
	if sprintService == nil {
		sprintService = services.NewSprintService(jiraClient)
	}
	
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}
	
	ctx := r.Context()
	prediction, err := sprintService.PredictSprintSuccess(ctx, sprintID)
	if err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Msg("Failed to predict sprint success")
		render.Render(w, r, ErrInternalServer(err))
		return
	}
	
	render.JSON(w, r, prediction)
}

// ValidateSprintRequest validates a sprint creation/update request
func ValidateSprintRequest(w http.ResponseWriter, r *http.Request) {
	if sprintService == nil {
		sprintService = services.NewSprintService(jiraClient)
	}
	
	var req jira.CreateSprintRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}
	
	err := sprintService.ValidateSprint(&req)
	if err != nil {
		render.JSON(w, r, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}
	
	render.JSON(w, r, map[string]interface{}{
		"valid": true,
		"message": "Sprint request is valid",
	})
}

// GetSprintHealthCheck performs a health check on active sprints
func GetSprintHealthCheck(w http.ResponseWriter, r *http.Request) {
	if sprintService == nil {
		sprintService = services.NewSprintService(jiraClient)
	}
	
	ctx := r.Context()
	activeSprints, err := sprintService.GetActiveSprints(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get active sprints for health check")
		render.Render(w, r, ErrInternalServer(err))
		return
	}
	
	healthStatus := make([]map[string]interface{}, 0)
	
	for _, sprint := range activeSprints {
		metrics, err := sprintService.GetSprintMetrics(ctx, sprint.ID)
		if err != nil {
			log.Warn().Err(err).Int("sprintId", sprint.ID).Msg("Failed to get metrics for sprint")
			continue
		}
		
		prediction, _ := sprintService.PredictSprintSuccess(ctx, sprint.ID)
		
		status := "healthy"
		if metrics.CompletionPercentage < 30 && sprint.StartDate != nil {
			// If less than 30% complete and past halfway point
			daysSinceStart := time.Since(*sprint.StartDate).Hours() / 24
			if sprint.EndDate != nil {
				totalDays := sprint.EndDate.Sub(*sprint.StartDate).Hours() / 24
				if daysSinceStart > totalDays/2 {
					status = "at-risk"
				}
			}
		}
		
		health := map[string]interface{}{
			"sprintId":   sprint.ID,
			"sprintName": sprint.Name,
			"status":     status,
			"metrics": map[string]interface{}{
				"completionPercentage": metrics.CompletionPercentage,
				"burndownRate":        metrics.BurndownRate,
				"totalIssues":         metrics.TotalIssues,
				"completedIssues":     metrics.CompletedIssues,
			},
		}
		
		if prediction != nil {
			health["prediction"] = map[string]interface{}{
				"successProbability": prediction.SuccessProbability,
				"riskLevel":         prediction.RiskLevel,
			}
		}
		
		healthStatus = append(healthStatus, health)
	}
	
	overallHealth := "healthy"
	atRiskCount := 0
	for _, status := range healthStatus {
		if status["status"] == "at-risk" {
			atRiskCount++
		}
	}
	if atRiskCount > 0 {
		overallHealth = fmt.Sprintf("%d sprints at risk", atRiskCount)
	}
	
	render.JSON(w, r, map[string]interface{}{
		"overallHealth": overallHealth,
		"sprints":       healthStatus,
		"totalActive":   len(activeSprints),
		"atRisk":        atRiskCount,
		"timestamp":     time.Now(),
	})
}

// CloneSpring creates a new sprint based on an existing one
func CloneSprint(w http.ResponseWriter, r *http.Request) {
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}
	
	var req struct {
		Name      string `json:"name"`
		CopyGoal  bool   `json:"copyGoal"`
		CopyDates bool   `json:"copyDates"`
	}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}
	
	// Get original sprint
	originalSprint, err := jiraClient.GetSprint(sprintID)
	if err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Msg("Failed to get original sprint")
		render.Render(w, r, ErrInternalServer(err))
		return
	}
	
	// Create clone request
	cloneReq := jira.CreateSprintRequest{
		Name:          req.Name,
		OriginBoardID: originalSprint.OriginBoardID,
	}
	
	if req.Name == "" {
		cloneReq.Name = fmt.Sprintf("%s (Copy)", originalSprint.Name)
	}
	
	if req.CopyGoal {
		cloneReq.Goal = originalSprint.Goal
	}
	
	if req.CopyDates && originalSprint.StartDate != nil && originalSprint.EndDate != nil {
		// Shift dates to future
		duration := originalSprint.EndDate.Sub(*originalSprint.StartDate)
		newStart := time.Now().AddDate(0, 0, 1) // Start tomorrow
		newEnd := newStart.Add(duration)
		cloneReq.StartDate = &newStart
		cloneReq.EndDate = &newEnd
	}
	
	// Create the cloned sprint
	newSprint, err := jiraClient.CreateSprint(&cloneReq)
	if err != nil {
		log.Error().Err(err).Msg("Failed to clone sprint")
		render.Render(w, r, ErrInternalServer(err))
		return
	}
	
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]interface{}{
		"success":        true,
		"originalId":     sprintID,
		"clonedSprint":   newSprint,
		"message":        fmt.Sprintf("Sprint '%s' cloned successfully", newSprint.Name),
	})
}