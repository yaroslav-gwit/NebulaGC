package logging

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Environment represents the deployment environment.
type Environment string

const (
	// EnvironmentProduction is for production deployments with JSON logging.
	EnvironmentProduction Environment = "production"

	// EnvironmentDevelopment is for development with console logging.
	EnvironmentDevelopment Environment = "development"
)

// Config holds the configuration for the logger.
type Config struct {
	// Level is the minimum enabled logging level (debug, info, warn, error).
	Level string

	// Environment determines the log format (production = JSON, development = console).
	Environment Environment

	// OutputPaths is a list of URLs or file paths to write logging output to.
	OutputPaths []string

	// ErrorOutputPaths is a list of URLs or file paths to write internal logger errors to.
	ErrorOutputPaths []string

	// DisableCaller disables automatic caller information.
	DisableCaller bool

	// DisableStacktrace disables automatic stacktrace capturing.
	DisableStacktrace bool
}

// DefaultConfig returns a default configuration for development.
func DefaultConfig() Config {
	return Config{
		Level:             "info",
		Environment:       EnvironmentDevelopment,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
		DisableCaller:     false,
		DisableStacktrace: false,
	}
}

// NewLogger creates a new zap logger based on the provided configuration.
func NewLogger(cfg Config) (*zap.Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", cfg.Level, err)
	}

	// Create encoder config
	var encoderConfig zapcore.EncoderConfig
	if cfg.Environment == EnvironmentProduction {
		encoderConfig = zap.NewProductionEncoderConfig()
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Create zap config
	zapConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       cfg.Environment == EnvironmentDevelopment,
		DisableCaller:     cfg.DisableCaller,
		DisableStacktrace: cfg.DisableStacktrace,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         encodingFromEnvironment(cfg.Environment),
		EncoderConfig:    encoderConfig,
		OutputPaths:      cfg.OutputPaths,
		ErrorOutputPaths: cfg.ErrorOutputPaths,
	}

	logger, err := zapConfig.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger, nil
}

// NewDevelopmentLogger creates a logger optimized for development with console output.
func NewDevelopmentLogger() (*zap.Logger, error) {
	cfg := DefaultConfig()
	cfg.Environment = EnvironmentDevelopment
	cfg.Level = "debug"
	return NewLogger(cfg)
}

// NewProductionLogger creates a logger optimized for production with JSON output.
func NewProductionLogger(level string) (*zap.Logger, error) {
	if level == "" {
		level = "info"
	}

	cfg := Config{
		Level:             level,
		Environment:       EnvironmentProduction,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
		DisableCaller:     false,
		DisableStacktrace: false,
	}

	return NewLogger(cfg)
}

// encodingFromEnvironment returns the encoding format based on environment.
func encodingFromEnvironment(env Environment) string {
	if env == EnvironmentProduction {
		return "json"
	}
	return "console"
}

// ParseLevel converts a string level to zapcore.Level.
func ParseLevel(level string) (zapcore.Level, error) {
	return zapcore.ParseLevel(strings.ToLower(level))
}

// MustNewLogger creates a new logger and panics if there's an error.
// This should only be used during application startup.
func MustNewLogger(cfg Config) *zap.Logger {
	logger, err := NewLogger(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}
	return logger
}
