package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

// APITokenAuth implements authentication using Jira API tokens
type APITokenAuth struct {
	email     string
	token     string
	jiraURL   string
	client    *resty.Client
	user      *User
	validated bool
	validatedAt time.Time
}

// NewAPITokenAuth creates a new API token authenticator
func NewAPITokenAuth(email, token, jiraURL string) *APITokenAuth {
	client := resty.New().
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)

	return &APITokenAuth{
		email:   email,
		token:   token,
		jiraURL: jiraURL,
		client:  client,
	}
}

// Authenticate tests the API token by making a call to /rest/api/2/myself
func (a *APITokenAuth) Authenticate(ctx context.Context) error {
	if a.jiraURL == "" {
		return fmt.Errorf("Jira URL is required for authentication")
	}

	url := fmt.Sprintf("%s/rest/api/2/myself", a.jiraURL)
	
	resp, err := a.client.R().
		SetContext(ctx).
		SetBasicAuth(a.email, a.token).
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode(), resp.String())
	}

	// Parse user information
	var user User
	if err := json.Unmarshal(resp.Body(), &user); err != nil {
		return fmt.Errorf("failed to parse user information: %w", err)
	}

	a.user = &user
	a.validated = true
	a.validatedAt = time.Now()

	return nil
}

// GetHeaders returns the basic auth header for API token authentication
func (a *APITokenAuth) GetHeaders() map[string]string {
	if !a.validated {
		return make(map[string]string)
	}

	auth := base64.StdEncoding.EncodeToString(
		[]byte(fmt.Sprintf("%s:%s", a.email, a.token)),
	)
	
	return map[string]string{
		"Authorization": "Basic " + auth,
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}
}

// Refresh is a no-op for API token authentication as tokens don't expire
func (a *APITokenAuth) Refresh(ctx context.Context) error {
	// API tokens don't need refreshing, but we can re-validate
	return a.Authenticate(ctx)
}

// IsValid checks if the API token authentication is still valid
func (a *APITokenAuth) IsValid() bool {
	if !a.validated {
		return false
	}
	
	// Consider authentication valid for 1 hour without re-checking
	return time.Since(a.validatedAt) < time.Hour
}

// GetUser returns the authenticated user information
func (a *APITokenAuth) GetUser() (*User, error) {
	if !a.validated || a.user == nil {
		return nil, fmt.Errorf("not authenticated")
	}
	return a.user, nil
}

// Type returns the authentication type
func (a *APITokenAuth) Type() string {
	return "api_token"
}

// TestConnection tests the connection without storing credentials
func TestAPITokenConnection(ctx context.Context, email, token, jiraURL string) error {
	auth := NewAPITokenAuth(email, token, jiraURL)
	return auth.Authenticate(ctx)
}