package auth

import (
	"testing"

	"github.com/ericfisherdev/GoJira/internal/config"
)

func TestNewAuthenticator(t *testing.T) {
	tests := []struct {
		name        string
		authType    string
		credentials map[string]string
		jiraURL     string
		wantErr     bool
	}{
		{
			name:     "valid api token",
			authType: "api_token",
			credentials: map[string]string{
				"email": "test@example.com",
				"token": "test-token",
			},
			jiraURL: "https://test.atlassian.net",
			wantErr: false,
		},
		{
			name:     "missing email for api token",
			authType: "api_token",
			credentials: map[string]string{
				"token": "test-token",
			},
			jiraURL: "https://test.atlassian.net",
			wantErr: true,
		},
		{
			name:     "valid oauth2",
			authType: "oauth2",
			credentials: map[string]string{
				"client_id":     "test-client",
				"client_secret": "test-secret",
				"redirect_url":  "http://localhost:8080/callback",
			},
			jiraURL: "https://test.atlassian.net",
			wantErr: false,
		},
		{
			name:     "valid pat",
			authType: "pat",
			credentials: map[string]string{
				"token": "test-pat-token",
			},
			jiraURL: "https://test.atlassian.net",
			wantErr: false,
		},
		{
			name:        "unsupported auth type",
			authType:    "unsupported",
			credentials: map[string]string{},
			jiraURL:     "https://test.atlassian.net",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := NewAuthenticator(tt.authType, tt.credentials, tt.jiraURL)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAuthenticator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && auth == nil {
				t.Error("NewAuthenticator() returned nil authenticator")
			}
			
			if !tt.wantErr && auth.Type() != tt.authType {
				t.Errorf("NewAuthenticator() type = %v, want %v", auth.Type(), tt.authType)
			}
		})
	}
}

func TestManager(t *testing.T) {
	cfg := &config.Config{}
	manager := NewManager(cfg)

	// Test adding authenticator
	auth := &APITokenAuth{
		email:   "test@example.com",
		token:   "test-token",
		jiraURL: "https://test.atlassian.net",
	}

	manager.AddAuthenticator("test", auth)

	// Test setting current
	err := manager.SetCurrent("test")
	if err != nil {
		t.Errorf("SetCurrent() error = %v", err)
	}

	// Test getting current
	current := manager.GetCurrent()
	if current != auth {
		t.Error("GetCurrent() returned wrong authenticator")
	}

	// Test setting non-existent authenticator
	err = manager.SetCurrent("non-existent")
	if err == nil {
		t.Error("SetCurrent() should have returned error for non-existent authenticator")
	}

	// Test authentication status
	if !manager.IsAuthenticated() {
		// This is expected since we haven't actually authenticated
		t.Log("Manager correctly reports not authenticated (expected)")
	}
}