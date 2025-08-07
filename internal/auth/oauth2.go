package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"golang.org/x/oauth2"
)

// OAuth2Auth implements OAuth 2.0 authentication for Jira
type OAuth2Auth struct {
	clientID     string
	clientSecret string
	redirectURL  string
	jiraURL      string
	config       *oauth2.Config
	token        *oauth2.Token
	client       *resty.Client
	user         *User
	validated    bool
}

// NewOAuth2Auth creates a new OAuth2 authenticator
func NewOAuth2Auth(clientID, clientSecret, redirectURL, jiraURL string) *OAuth2Auth {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"read:jira-work", "write:jira-work", "manage:jira-project"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/rest/oauth2/authorize", jiraURL),
			TokenURL: fmt.Sprintf("%s/rest/oauth2/token", jiraURL),
		},
	}

	client := resty.New().
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)

	return &OAuth2Auth{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		jiraURL:      jiraURL,
		config:       config,
		client:       client,
	}
}

// GetAuthURL returns the OAuth2 authorization URL
func (o *OAuth2Auth) GetAuthURL(state string) string {
	return o.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeCodeForToken exchanges the authorization code for access token
func (o *OAuth2Auth) ExchangeCodeForToken(ctx context.Context, code string) error {
	token, err := o.config.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange code for token: %w", err)
	}

	o.token = token
	return nil
}

// Authenticate verifies the OAuth2 token by making a test API call
func (o *OAuth2Auth) Authenticate(ctx context.Context) error {
	if o.token == nil {
		return fmt.Errorf("no OAuth2 token available")
	}

	if !o.token.Valid() {
		return fmt.Errorf("OAuth2 token is not valid")
	}

	// Test the token by getting user information
	url := fmt.Sprintf("%s/rest/api/2/myself", o.jiraURL)
	
	resp, err := o.client.R().
		SetContext(ctx).
		SetAuthToken(o.token.AccessToken).
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return fmt.Errorf("authentication test failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode(), resp.String())
	}

	// Parse user information
	var user User
	if err := json.Unmarshal(resp.Body(), &user); err != nil {
		return fmt.Errorf("failed to parse user information: %w", err)
	}

	o.user = &user
	o.validated = true

	return nil
}

// GetHeaders returns the OAuth2 bearer token header
func (o *OAuth2Auth) GetHeaders() map[string]string {
	if !o.validated || o.token == nil {
		return make(map[string]string)
	}

	return map[string]string{
		"Authorization": "Bearer " + o.token.AccessToken,
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}
}

// Refresh refreshes the OAuth2 token if possible
func (o *OAuth2Auth) Refresh(ctx context.Context) error {
	if o.token == nil {
		return fmt.Errorf("no token to refresh")
	}

	tokenSource := o.config.TokenSource(ctx, o.token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	o.token = newToken
	o.validated = false

	// Re-authenticate with the new token
	return o.Authenticate(ctx)
}

// IsValid checks if the OAuth2 token is valid
func (o *OAuth2Auth) IsValid() bool {
	if !o.validated || o.token == nil {
		return false
	}
	return o.token.Valid()
}

// GetUser returns the authenticated user information
func (o *OAuth2Auth) GetUser() (*User, error) {
	if !o.validated || o.user == nil {
		return nil, fmt.Errorf("not authenticated")
	}
	return o.user, nil
}

// Type returns the authentication type
func (o *OAuth2Auth) Type() string {
	return "oauth2"
}

// SetToken sets the OAuth2 token (for cases where token is obtained externally)
func (o *OAuth2Auth) SetToken(token *oauth2.Token) {
	o.token = token
	o.validated = false
}