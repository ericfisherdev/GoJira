package handlers

import (
	"net/http"

	"github.com/go-chi/render"
)

type ConnectRequest struct {
	Type        string            `json:"type" validate:"required,oneof=api_token oauth2"`
	Credentials map[string]string `json:"credentials" validate:"required"`
}

type ConnectResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (cr *ConnectResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type StatusResponse struct {
	Connected  bool   `json:"connected"`
	Connection string `json:"connection,omitempty"`
	User       string `json:"user,omitempty"`
}

func (sr *StatusResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func Connect(w http.ResponseWriter, r *http.Request) {
	var req ConnectRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, &ErrorResponse{
			HTTPStatusCode: http.StatusBadRequest,
			StatusText:     "Invalid request",
			ErrorText:      err.Error(),
		})
		return
	}

	// TODO: Implement actual authentication logic
	response := &ConnectResponse{
		Success: true,
		Message: "Connected to Jira successfully (placeholder)",
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

func (cr *ConnectRequest) Bind(r *http.Request) error {
	// Validation could be added here
	return nil
}

func Disconnect(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement disconnect logic
	response := &ConnectResponse{
		Success: true,
		Message: "Disconnected from Jira",
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

func Status(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement actual status check
	response := &StatusResponse{
		Connected:  false,
		Connection: "Not configured",
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}