package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

// GetBoards retrieves all boards
func GetBoards(w http.ResponseWriter, r *http.Request) {
	boards, err := jiraClient.GetBoards()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get boards")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, boards)
}

// GetBoard retrieves a specific board
func GetBoard(w http.ResponseWriter, r *http.Request) {
	boardIDStr := chi.URLParam(r, "id")
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid board ID")))
		return
	}

	board, err := jiraClient.GetBoard(boardID)
	if err != nil {
		log.Error().Err(err).Int("boardId", boardID).Msg("Failed to get board")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, board)
}

// GetBoardConfiguration retrieves board configuration
func GetBoardConfiguration(w http.ResponseWriter, r *http.Request) {
	boardIDStr := chi.URLParam(r, "id")
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid board ID")))
		return
	}

	config, err := jiraClient.GetBoardConfiguration(boardID)
	if err != nil {
		log.Error().Err(err).Int("boardId", boardID).Msg("Failed to get board configuration")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, config)
}

// GetBoardIssues retrieves issues on a board
func GetBoardIssues(w http.ResponseWriter, r *http.Request) {
	boardIDStr := chi.URLParam(r, "id")
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid board ID")))
		return
	}

	issues, err := jiraClient.GetBoardIssues(boardID)
	if err != nil {
		log.Error().Err(err).Int("boardId", boardID).Msg("Failed to get board issues")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, issues)
}

// GetBoardBacklog retrieves issues in the board backlog
func GetBoardBacklog(w http.ResponseWriter, r *http.Request) {
	boardIDStr := chi.URLParam(r, "id")
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid board ID")))
		return
	}

	backlog, err := jiraClient.GetBoardBacklog(boardID)
	if err != nil {
		log.Error().Err(err).Int("boardId", boardID).Msg("Failed to get board backlog")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, backlog)
}

// GetBoardSprints retrieves sprints for a board
func GetBoardSprints(w http.ResponseWriter, r *http.Request) {
	boardIDStr := chi.URLParam(r, "id")
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid board ID")))
		return
	}

	sprints, err := jiraClient.GetBoardSprints(boardID)
	if err != nil {
		log.Error().Err(err).Int("boardId", boardID).Msg("Failed to get board sprints")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, sprints)
}

// MoveIssuesToBacklog moves issues to the backlog
func MoveIssuesToBacklog(w http.ResponseWriter, r *http.Request) {
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

	if err := jiraClient.MoveIssuesToBacklog(req.Issues); err != nil {
		log.Error().Err(err).Interface("issues", req.Issues).Msg("Failed to move issues to backlog")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Issues moved to backlog successfully",
		"count":   len(req.Issues),
	})
}

// MoveIssuesToBoard moves issues to a specific position on a board
func MoveIssuesToBoard(w http.ResponseWriter, r *http.Request) {
	boardIDStr := chi.URLParam(r, "id")
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid board ID")))
		return
	}

	var req struct {
		Issues   []string `json:"issues"`
		Position string   `json:"position,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	if len(req.Issues) == 0 {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("at least one issue is required")))
		return
	}

	if err := jiraClient.MoveIssuesToBoard(boardID, req.Issues, req.Position); err != nil {
		log.Error().Err(err).Int("boardId", boardID).Interface("issues", req.Issues).Msg("Failed to move issues on board")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Issues moved on board successfully",
		"count":   len(req.Issues),
	})
}