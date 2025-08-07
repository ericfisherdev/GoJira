package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ericfisherdev/GoJira/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Fields represents structured logging fields
type Fields map[string]interface{}

// Logger wraps zerolog with additional functionality
type Logger struct {
	logger zerolog.Logger
	config *config.LoggingConfig
}

// New creates a new logger instance
func New(cfg *config.LoggingConfig) (*Logger, error) {
	var output io.Writer = os.Stdout

	// Set output destination
	switch cfg.Output {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	case "file":
		if cfg.File == "" {
			return nil, fmt.Errorf("log file path is required when output is 'file'")
		}
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		output = file
	default:
		output = os.Stdout
	}

	// Configure output format
	switch cfg.Format {
	case "console":
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	case "json":
		// JSON is the default format for zerolog
	default:
		// Default to JSON
	}

	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	// Create logger
	logger := zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Str("service", "gojira").
		Logger()

	return &Logger{
		logger: logger,
		config: cfg,
	}, nil
}

// InitGlobal initializes the global logger
func InitGlobal(cfg *config.LoggingConfig, version string) error {
	// Set global log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure output
	var output io.Writer = os.Stdout

	switch cfg.Output {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	case "file":
		if cfg.File != "" {
			file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				return fmt.Errorf("failed to open log file: %w", err)
			}
			output = file
		}
	}

	// Configure format
	switch cfg.Format {
	case "console":
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
		}
	}

	// Set global logger
	log.Logger = zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Str("service", "gojira").
		Str("version", version).
		Logger()

	return nil
}

// WithFields adds structured fields to the logger
func (l *Logger) WithFields(fields Fields) *Logger {
	event := l.logger.With()
	for k, v := range fields {
		event = event.Interface(k, v)
	}
	return &Logger{
		logger: event.Logger(),
		config: l.config,
	}
}

// WithContext adds context information to the logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		logger: l.logger.With().Ctx(ctx).Logger(),
		config: l.config,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...Fields) {
	event := l.logger.Debug()
	l.addFields(event, fields...)
	event.Msg(msg)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...Fields) {
	event := l.logger.Info()
	l.addFields(event, fields...)
	event.Msg(msg)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...Fields) {
	event := l.logger.Warn()
	l.addFields(event, fields...)
	event.Msg(msg)
}

// Error logs an error message
func (l *Logger) Error(err error, msg string, fields ...Fields) {
	event := l.logger.Error().Err(err)
	l.addFields(event, fields...)
	event.Msg(msg)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(err error, msg string, fields ...Fields) {
	event := l.logger.Fatal().Err(err)
	l.addFields(event, fields...)
	event.Msg(msg)
}

// addFields adds fields to a log event
func (l *Logger) addFields(event *zerolog.Event, fields ...Fields) {
	for _, fieldSet := range fields {
		for k, v := range fieldSet {
			event = event.Interface(k, v)
		}
	}
}

// HTTPLogger creates a logger for HTTP requests
func HTTPLogger() func(r interface{}) {
	return func(r interface{}) {
		// This will be used with the HTTP middleware
		log.Info().Interface("request", r).Msg("HTTP request")
	}
}

// ContextLogger returns a logger from context or creates a new one
func ContextLogger(ctx context.Context) zerolog.Logger {
	if logger := zerolog.Ctx(ctx); logger != nil && logger.GetLevel() != zerolog.Disabled {
		return *logger
	}
	return log.Logger
}

// Global logging functions for convenience
func Debug(msg string) {
	log.Debug().Msg(msg)
}

func Info(msg string) {
	log.Info().Msg(msg)
}

func Warn(msg string) {
	log.Warn().Msg(msg)
}

func Error(err error, msg string) {
	log.Error().Err(err).Msg(msg)
}

func Fatal(err error, msg string) {
	log.Fatal().Err(err).Msg(msg)
}

// With structured fields
func WithFields(fields Fields) zerolog.Logger {
	event := log.With()
	for k, v := range fields {
		event = event.Interface(k, v)
	}
	return event.Logger()
}

func WithError(err error) zerolog.Logger {
	return log.With().Err(err).Logger()
}