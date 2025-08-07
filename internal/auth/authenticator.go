package auth

import (
	"context"
	"fmt"

	"github.com/ericfisherdev/GoJira/internal/config"
)

// Authenticator defines the interface for Jira authentication methods
type Authenticator interface {
	// Authenticate performs the authentication process
	Authenticate(ctx context.Context) error
	
	// GetHeaders returns HTTP headers for authenticated requests
	GetHeaders() map[string]string
	
	// Refresh refreshes the authentication if applicable
	Refresh(ctx context.Context) error
	
	// IsValid checks if the current authentication is valid
	IsValid() bool
	
	// GetUser returns information about the authenticated user
	GetUser() (*User, error)
	
	// Type returns the authentication type
	Type() string
}

// User represents a Jira user
type User struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Active       bool   `json:"active"`
}

// Manager manages multiple authenticators and handles switching between them
type Manager struct {
	authenticators map[string]Authenticator
	current        Authenticator
	config         *config.Config
}

// NewManager creates a new authentication manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		authenticators: make(map[string]Authenticator),
		config:         cfg,
	}
}

// NewAuthenticator creates a new authenticator based on the configuration
func NewAuthenticator(authType string, credentials map[string]string, jiraURL string) (Authenticator, error) {
	switch authType {
	case "api_token":
		email, ok := credentials["email"]
		if !ok {
			return nil, fmt.Errorf("email is required for API token authentication")
		}
		token, ok := credentials["token"]
		if !ok {
			return nil, fmt.Errorf("token is required for API token authentication")
		}
		return NewAPITokenAuth(email, token, jiraURL), nil
		
	case "oauth2":
		clientID, ok := credentials["client_id"]
		if !ok {
			return nil, fmt.Errorf("client_id is required for OAuth2 authentication")
		}
		clientSecret, ok := credentials["client_secret"]
		if !ok {
			return nil, fmt.Errorf("client_secret is required for OAuth2 authentication")
		}
		redirectURL := credentials["redirect_url"]
		return NewOAuth2Auth(clientID, clientSecret, redirectURL, jiraURL), nil
		
	case "pat":
		token, ok := credentials["token"]
		if !ok {
			return nil, fmt.Errorf("token is required for PAT authentication")
		}
		return NewPATAuth(token, jiraURL), nil
		
	default:
		return nil, fmt.Errorf("unsupported authentication type: %s", authType)
	}
}

// AddAuthenticator adds an authenticator to the manager
func (m *Manager) AddAuthenticator(name string, auth Authenticator) {
	m.authenticators[name] = auth
}

// SetCurrent sets the current active authenticator
func (m *Manager) SetCurrent(name string) error {
	auth, exists := m.authenticators[name]
	if !exists {
		return fmt.Errorf("authenticator %s not found", name)
	}
	m.current = auth
	return nil
}

// GetCurrent returns the current authenticator
func (m *Manager) GetCurrent() Authenticator {
	return m.current
}

// Authenticate using the current authenticator
func (m *Manager) Authenticate(ctx context.Context) error {
	if m.current == nil {
		return fmt.Errorf("no authenticator set")
	}
	return m.current.Authenticate(ctx)
}

// GetHeaders returns headers from the current authenticator
func (m *Manager) GetHeaders() map[string]string {
	if m.current == nil {
		return make(map[string]string)
	}
	return m.current.GetHeaders()
}

// IsAuthenticated checks if there's a valid current authentication
func (m *Manager) IsAuthenticated() bool {
	return m.current != nil && m.current.IsValid()
}