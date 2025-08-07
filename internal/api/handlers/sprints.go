package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ericfisherdev/GoJira/internal/jira"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

// GetSprints retrieves all sprints for a board
func GetSprints(w http.ResponseWriter, r *http.Request) {
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

	sprints, err := jiraClient.GetSprints(boardID)
	if err != nil {
		log.Error().Err(err).Int("boardId", boardID).Msg("Failed to get sprints")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, sprints)
}

// GetSprint retrieves a specific sprint
func GetSprint(w http.ResponseWriter, r *http.Request) {
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}

	sprint, err := jiraClient.GetSprint(sprintID)
	if err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Msg("Failed to get sprint")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, sprint)
}

// CreateSprint creates a new sprint
func CreateSprint(w http.ResponseWriter, r *http.Request) {
	var req jira.CreateSprintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate required fields
	if req.Name == "" {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("sprint name is required")))
		return
	}
	if req.OriginBoardID == 0 {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("originBoardId is required")))
		return
	}

	sprint, err := jiraClient.CreateSprint(&req)
	if err != nil {
		log.Error().Err(err).Interface("request", req).Msg("Failed to create sprint")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, sprint)
}

// UpdateSprint updates an existing sprint
func UpdateSprint(w http.ResponseWriter, r *http.Request) {
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}

	var req jira.UpdateSprintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	sprint, err := jiraClient.UpdateSprint(sprintID, &req)
	if err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Interface("request", req).Msg("Failed to update sprint")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, sprint)
}

// StartSprint starts a sprint
func StartSprint(w http.ResponseWriter, r *http.Request) {
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}

	var req struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Parse dates
	startDate, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid startDate format")))
		return
	}

	endDate, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid endDate format")))
		return
	}

	if err := jiraClient.StartSprint(sprintID, startDate, endDate); err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Msg("Failed to start sprint")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Sprint started successfully",
	})
}

// CloseSprint closes a sprint
func CloseSprint(w http.ResponseWriter, r *http.Request) {
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}

	if err := jiraClient.CloseSprint(sprintID); err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Msg("Failed to close sprint")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Sprint closed successfully",
	})
}

// GetSprintIssues retrieves issues in a sprint
func GetSprintIssues(w http.ResponseWriter, r *http.Request) {
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}

	issues, err := jiraClient.GetSprintIssues(sprintID)
	if err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Msg("Failed to get sprint issues")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, issues)
}

// MoveIssuesToSprint moves issues to a sprint
func MoveIssuesToSprint(w http.ResponseWriter, r *http.Request) {
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}

	var req struct {
		Issues []string `json:"issues"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	if len(req.Issues) == 0 {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("at least one issue is required")))
		return
	}

	if err := jiraClient.MoveIssuesToSprint(sprintID, req.Issues); err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Interface("issues", req.Issues).Msg("Failed to move issues to sprint")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Issues moved to sprint successfully",
		"count":   len(req.Issues),
	})
}

// GetSprintReport generates a sprint report
func GetSprintReport(w http.ResponseWriter, r *http.Request) {
	sprintIDStr := chi.URLParam(r, "id")
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid sprint ID")))
		return
	}

	report, err := jiraClient.GetSprintReport(sprintID)
	if err != nil {
		log.Error().Err(err).Int("sprintId", sprintID).Msg("Failed to generate sprint report")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, report)
}