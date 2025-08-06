package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestLoad(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	// Test loading with defaults
	config, err := Load("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check defaults
	if config.Server.Port != "8080" {
		t.Errorf("Expected default port 8080, got %s", config.Server.Port)
	}

	if config.Server.Mode != "development" {
		t.Errorf("Expected default mode development, got %s", config.Server.Mode)
	}

	if config.Jira.Timeout != 30 {
		t.Errorf("Expected default timeout 30, got %d", config.Jira.Timeout)
	}

	if config.Logging.Level != "info" {
		t.Errorf("Expected default log level info, got %s", config.Logging.Level)
	}
}

func TestEnvironmentVariableOverrides(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	// Set environment variables
	os.Setenv("GOJIRA_SERVER_PORT", "9000")
	os.Setenv("GOJIRA_SERVER_MODE", "production")
	os.Setenv("GOJIRA_LOGGING_LEVEL", "error")
	defer func() {
		os.Unsetenv("GOJIRA_SERVER_PORT")
		os.Unsetenv("GOJIRA_SERVER_MODE")
		os.Unsetenv("GOJIRA_LOGGING_LEVEL")
	}()

	config, err := Load("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Server.Port != "9000" {
		t.Errorf("Expected port 9000 from env var, got %s", config.Server.Port)
	}

	if config.Server.Mode != "production" {
		t.Errorf("Expected mode production from env var, got %s", config.Server.Mode)
	}

	if config.Logging.Level != "error" {
		t.Errorf("Expected log level error from env var, got %s", config.Logging.Level)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Server: ServerConfig{
					Port: "8080",
					Mode: "development",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Jira: JiraConfig{
					Auth: AuthConfig{
						Type: "api_token",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid server mode",
			config: Config{
				Server: ServerConfig{
					Port: "8080",
					Mode: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			config: Config{
				Server: ServerConfig{
					Port: "8080",
				},
				Logging: LoggingConfig{
					Level: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid auth type",
			config: Config{
				Server: ServerConfig{
					Port: "8080",
				},
				Jira: JiraConfig{
					Auth: AuthConfig{
						Type: "invalid",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}