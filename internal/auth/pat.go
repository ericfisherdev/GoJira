package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

// PATAuth implements Personal Access Token authentication for Jira Server/Data Center
type PATAuth struct {
	token     string
	jiraURL   string
	client    *resty.Client
	user      *User
	validated bool
	validatedAt time.Time
}

// NewPATAuth creates a new Personal Access Token authenticator
func NewPATAuth(token, jiraURL string) *PATAuth {
	client := resty.New().
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)

	return &PATAuth{
		token:   token,
		jiraURL: jiraURL,
		client:  client,
	}
}

// Authenticate tests the PAT by making a call to /rest/api/2/myself
func (p *PATAuth) Authenticate(ctx context.Context) error {
	if p.jiraURL == "" {
		return fmt.Errorf("Jira URL is required for authentication")
	}

	url := fmt.Sprintf("%s/rest/api/2/myself", p.jiraURL)
	
	resp, err := p.client.R().
		SetContext(ctx).
		SetAuthToken(p.token).
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

	p.user = &user
	p.validated = true
	p.validatedAt = time.Now()

	return nil
}

// GetHeaders returns the bearer token header for PAT authentication
func (p *PATAuth) GetHeaders() map[string]string {
	if !p.validated {
		return make(map[string]string)
	}

	return map[string]string{
		"Authorization": "Bearer " + p.token,
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}
}

// Refresh re-validates the PAT (PATs don't typically need refreshing)
func (p *PATAuth) Refresh(ctx context.Context) error {
	return p.Authenticate(ctx)
}

// IsValid checks if the PAT authentication is still valid
func (p *PATAuth) IsValid() bool {
	if !p.validated {
		return false
	}
	
	// Consider authentication valid for 1 hour without re-checking
	return time.Since(p.validatedAt) < time.Hour
}

// GetUser returns the authenticated user information
func (p *PATAuth) GetUser() (*User, error) {
	if !p.validated || p.user == nil {
		return nil, fmt.Errorf("not authenticated")
	}
	return p.user, nil
}

// Type returns the authentication type
func (p *PATAuth) Type() string {
	return "pat"
}

// TestConnection tests the connection without storing credentials
func TestPATConnection(ctx context.Context, token, jiraURL string) error {
	auth := NewPATAuth(token, jiraURL)
	return auth.Authenticate(ctx)
}