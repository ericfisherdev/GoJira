package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ericfisherdev/GoJira/internal/auth"
	"github.com/go-chi/render"
)

var authManager *auth.Manager

// SetAuthManager sets the global auth manager
func SetAuthManager(manager *auth.Manager) {
	authManager = manager
}

type ConnectRequest struct {
	Type        string            `json:"type" validate:"required,oneof=api_token oauth2 pat"`
	Credentials map[string]string `json:"credentials" validate:"required"`
	JiraURL     string            `json:"jira_url,omitempty"`
}

type ConnectResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	User    *auth.User  `json:"user,omitempty"`
}

func (cr *ConnectResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type StatusResponse struct {
	Connected  bool       `json:"connected"`
	AuthType   string     `json:"auth_type,omitempty"`
	User       *auth.User `json:"user,omitempty"`
	JiraURL    string     `json:"jira_url,omitempty"`
}

func (sr *StatusResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type OAuth2StartResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

func (o *OAuth2StartResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func Connect(w http.ResponseWriter, r *http.Request) {
	var req ConnectRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Use Jira URL from request or get from context/config
	jiraURL := req.JiraURL
	if jiraURL == "" {
		// TODO: Get from config or context
		jiraURL = "https://placeholder.atlassian.net"
	}

	// Create authenticator
	authenticator, err := auth.NewAuthenticator(req.Type, req.Credentials, jiraURL)
	if err != nil {
		render.Render(w, r, &ErrorResponse{
			HTTPStatusCode: http.StatusBadRequest,
			StatusText:     "Invalid credentials",
			ErrorText:      err.Error(),
		})
		return
	}

	// Authenticate with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := authenticator.Authenticate(ctx); err != nil {
		render.Render(w, r, &ErrorResponse{
			HTTPStatusCode: http.StatusUnauthorized,
			StatusText:     "Authentication failed",
			ErrorText:      err.Error(),
		})
		return
	}

	// Store authenticator in manager
	if authManager != nil {
		authManager.AddAuthenticator("current", authenticator)
		authManager.SetCurrent("current")
	}

	// Get user info
	user, err := authenticator.GetUser()
	if err != nil {
		// Log error but don't fail the request
		user = nil
	}

	response := &ConnectResponse{
		Success: true,
		Message: "Connected to Jira successfully",
		User:    user,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

func (cr *ConnectRequest) Bind(r *http.Request) error {
	// Basic validation
	if cr.Type == "" {
		return fmt.Errorf("authentication type is required")
	}

	validTypes := map[string]bool{
		"api_token": true,
		"oauth2":    true,
		"pat":       true,
	}
	if !validTypes[cr.Type] {
		return fmt.Errorf("invalid authentication type: %s", cr.Type)
	}

	if len(cr.Credentials) == 0 {
		return fmt.Errorf("credentials are required")
	}

	return nil
}

func Disconnect(w http.ResponseWriter, r *http.Request) {
	// Clear current authenticator
	if authManager != nil {
		authManager.SetCurrent("")
	}

	response := &ConnectResponse{
		Success: true,
		Message: "Disconnected from Jira",
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

func Status(w http.ResponseWriter, r *http.Request) {
	response := &StatusResponse{
		Connected: false,
	}

	if authManager != nil && authManager.IsAuthenticated() {
		current := authManager.GetCurrent()
		response.Connected = true
		response.AuthType = current.Type()

		// Get user info if available
		if user, err := current.GetUser(); err == nil {
			response.User = user
		}
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

type OAuth2StartRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
	JiraURL      string `json:"jira_url"`
}

func (o *OAuth2StartRequest) Bind(r *http.Request) error {
	if o.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if o.ClientSecret == "" {
		return fmt.Errorf("client_secret is required")
	}
	if o.JiraURL == "" {
		return fmt.Errorf("jira_url is required")
	}
	return nil
}

// OAuth2Start initiates OAuth2 authentication flow
func OAuth2Start(w http.ResponseWriter, r *http.Request) {
	var req OAuth2StartRequest

	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	oauth2Auth := auth.NewOAuth2Auth(req.ClientID, req.ClientSecret, req.RedirectURL, req.JiraURL)
	state := generateState() // TODO: Implement proper state generation

	response := &OAuth2StartResponse{
		AuthURL: oauth2Auth.GetAuthURL(state),
		State:   state,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// OAuth2Callback handles OAuth2 callback
func OAuth2Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		render.Render(w, r, &ErrorResponse{
			HTTPStatusCode: http.StatusBadRequest,
			StatusText:     "Missing authorization code",
		})
		return
	}

	// TODO: Validate state parameter
	_ = state

	// TODO: Retrieve OAuth2 authenticator from session/storage
	// For now, return success
	response := &ConnectResponse{
		Success: true,
		Message: "OAuth2 callback received",
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// generateState generates a random state parameter for OAuth2
func generateState() string {
	// TODO: Implement proper random state generation
	return "random_state_123"
}