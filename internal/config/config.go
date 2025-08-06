package config

import (
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Jira     JiraConfig     `mapstructure:"jira"`
	Features FeatureConfig  `mapstructure:"features"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Security SecurityConfig `mapstructure:"security"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // development, production, test
}

type JiraConfig struct {
	URL       string           `mapstructure:"url"`
	Auth      AuthConfig       `mapstructure:"auth"`
	Timeout   int              `mapstructure:"timeout"`
	Retries   int              `mapstructure:"retries"`
	Instances []InstanceConfig `mapstructure:"instances"`
}

type AuthConfig struct {
	Type         string `mapstructure:"type"`          // api_token, oauth2, pat
	Email        string `mapstructure:"email"`         // For API token
	Token        string `mapstructure:"token"`         // API token or PAT
	ClientID     string `mapstructure:"client_id"`     // OAuth2
	ClientSecret string `mapstructure:"client_secret"` // OAuth2
	RedirectURL  string `mapstructure:"redirect_url"`  // OAuth2
}

type InstanceConfig struct {
	Name string     `mapstructure:"name"`
	URL  string     `mapstructure:"url"`
	Auth AuthConfig `mapstructure:"auth"`
}

type FeatureConfig struct {
	NaturalLanguage bool `mapstructure:"natural_language"`
	Caching         bool `mapstructure:"caching"`
	AutoRetry       bool `mapstructure:"auto_retry"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // json, console
	Output string `mapstructure:"output"` // stdout, file
	File   string `mapstructure:"file"`   // Log file path if output is file
}

type SecurityConfig struct {
	RateLimit      int      `mapstructure:"rate_limit"`
	EnableCORS     bool     `mapstructure:"enable_cors"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	JWTSecret      string   `mapstructure:"jwt_secret"`
}

// Load loads configuration from various sources
func Load(configPath string) (*Config, error) {
	// Set config name and type
	viper.SetConfigName("gojira")
	viper.SetConfigType("yaml")

	// Add config paths (in order of priority)
	if configPath != "" {
		viper.AddConfigPath(configPath)
	}
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/etc/gojira")
	viper.AddConfigPath("$HOME/.gojira")
	viper.AddConfigPath("$HOME/.config/gojira")
	viper.AddConfigPath(".")

	// Environment variables
	viper.SetEnvPrefix("GOJIRA")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found is OK, we'll use defaults and env vars
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// WatchConfig watches for configuration changes and calls the callback
func WatchConfig(callback func(*Config)) error {
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Printf("Config file changed: %s\n", e.Name)

		var config Config
		if err := viper.Unmarshal(&config); err != nil {
			fmt.Printf("Failed to reload config: %v\n", err)
			return
		}

		if err := validate(&config); err != nil {
			fmt.Printf("Invalid config after reload: %v\n", err)
			return
		}

		callback(&config)
	})

	return nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "development")

	// Jira defaults
	viper.SetDefault("jira.timeout", 30)
	viper.SetDefault("jira.retries", 3)
	viper.SetDefault("jira.auth.type", "api_token")

	// Feature defaults
	viper.SetDefault("features.natural_language", false)
	viper.SetDefault("features.caching", true)
	viper.SetDefault("features.auto_retry", true)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")

	// Security defaults
	viper.SetDefault("security.rate_limit", 100)
	viper.SetDefault("security.enable_cors", true)
	viper.SetDefault("security.allowed_origins", []string{"http://localhost:*", "https://localhost:*"})
}

// validate validates the configuration
func validate(config *Config) error {
	// Validate server config
	if config.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	if config.Server.Mode != "" {
		validModes := map[string]bool{
			"development": true,
			"production":  true,
			"test":        true,
		}
		if !validModes[config.Server.Mode] {
			return fmt.Errorf("invalid server mode: %s", config.Server.Mode)
		}
	}

	// Validate logging config
	if config.Logging.Level != "" {
		validLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		if !validLevels[config.Logging.Level] {
			return fmt.Errorf("invalid logging level: %s", config.Logging.Level)
		}
	}

	if config.Logging.Format != "" {
		validFormats := map[string]bool{
			"json":    true,
			"console": true,
		}
		if !validFormats[config.Logging.Format] {
			return fmt.Errorf("invalid logging format: %s", config.Logging.Format)
		}
	}

	// Validate Jira auth config
	if config.Jira.Auth.Type != "" {
		validAuthTypes := map[string]bool{
			"api_token": true,
			"oauth2":    true,
			"pat":       true,
		}
		if !validAuthTypes[config.Jira.Auth.Type] {
			return fmt.Errorf("invalid auth type: %s", config.Jira.Auth.Type)
		}
	}

	return nil
}

// GetString returns a string configuration value
func GetString(key string) string {
	return viper.GetString(key)
}

// GetInt returns an integer configuration value
func GetInt(key string) int {
	return viper.GetInt(key)
}

// GetBool returns a boolean configuration value
func GetBool(key string) bool {
	return viper.GetBool(key)
}

// IsSet returns true if the key is set in configuration
func IsSet(key string) bool {
	return viper.IsSet(key)
}