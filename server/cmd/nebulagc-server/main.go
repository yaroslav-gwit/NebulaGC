// Package main provides the NebulaGC control plane server.
//
// This is the main entrypoint for the nebulagc-server binary which runs
// the control plane HTTP API for managing Nebula overlay networks.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"nebulagc.io/server/internal/api"
)

// Config holds server configuration from flags and environment variables.
type Config struct {
	// ListenAddr is the address to listen on (e.g., ":8080").
	ListenAddr string

	// DatabasePath is the path to the SQLite database file.
	DatabasePath string

	// HMACSecret is the secret key for token validation.
	HMACSecret string

	// InstanceID is this control plane instance's UUID.
	InstanceID string

	// LogLevel is the logging level (debug, info, warn, error).
	LogLevel string

	// LogFormat is the log format (json, console).
	LogFormat string

	// AllowOrigins is comma-separated list of allowed CORS origins.
	AllowOrigins string

	// DisableWriteGuard disables replica write guard (for single-instance mode).
	DisableWriteGuard bool
}

// parseFlags parses command-line flags and environment variables.
func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.ListenAddr, "listen", getEnv("NEBULAGC_LISTEN_ADDR", ":8080"),
		"Address to listen on")
	flag.StringVar(&config.DatabasePath, "db", getEnv("NEBULAGC_DB_PATH", "./nebula.db"),
		"Path to SQLite database file")
	flag.StringVar(&config.HMACSecret, "secret", getEnv("NEBULAGC_HMAC_SECRET", ""),
		"HMAC secret for token validation (required, min 32 bytes)")
	flag.StringVar(&config.InstanceID, "instance-id", getEnv("NEBULAGC_INSTANCE_ID", ""),
		"Control plane instance UUID (auto-generated if not provided)")
	flag.StringVar(&config.LogLevel, "log-level", getEnv("NEBULAGC_LOG_LEVEL", "info"),
		"Log level (debug, info, warn, error)")
	flag.StringVar(&config.LogFormat, "log-format", getEnv("NEBULAGC_LOG_FORMAT", "console"),
		"Log format (json, console)")
	flag.StringVar(&config.AllowOrigins, "cors-origins", getEnv("NEBULAGC_CORS_ORIGINS", ""),
		"Comma-separated list of allowed CORS origins (* for all)")
	flag.BoolVar(&config.DisableWriteGuard, "disable-write-guard",
		getEnv("NEBULAGC_DISABLE_WRITE_GUARD", "") == "true",
		"Disable replica write guard (single-instance mode)")

	flag.Parse()

	return config
}

// getEnv retrieves an environment variable with a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// validateConfig validates the server configuration.
func validateConfig(config *Config) error {
	// Validate HMAC secret
	if config.HMACSecret == "" {
		return fmt.Errorf("HMAC secret is required (set NEBULAGC_HMAC_SECRET or use -secret flag)")
	}
	if len(config.HMACSecret) < 32 {
		return fmt.Errorf("HMAC secret must be at least 32 bytes (got %d)", len(config.HMACSecret))
	}

	// Generate instance ID if not provided
	if config.InstanceID == "" {
		config.InstanceID = uuid.New().String()
	}

	// Validate instance ID format
	if _, err := uuid.Parse(config.InstanceID); err != nil {
		return fmt.Errorf("invalid instance ID format: %w", err)
	}

	return nil
}

// setupLogger creates a Zap logger based on configuration.
func setupLogger(config *Config) (*zap.Logger, error) {
	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(config.LogLevel)); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", config.LogLevel, err)
	}

	// Create logger config
	var zapConfig zap.Config
	if config.LogFormat == "json" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	zapConfig.Level = zap.NewAtomicLevelAt(level)

	// Build logger
	logger, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return logger, nil
}

// openDatabase opens a connection to the SQLite database.
func openDatabase(path string, logger *zap.Logger) (*sql.DB, error) {
	// Open database with WAL mode for better concurrency
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_timeout=5000&_foreign_keys=on", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("database connection established", zap.String("path", path))
	return db, nil
}

// parseCORSOrigins parses the comma-separated CORS origins string.
func parseCORSOrigins(origins string) []string {
	if origins == "" {
		return nil
	}

	// Simple split by comma - in production you might want more sophisticated parsing
	var result []string
	for _, origin := range []string{origins} {
		if origin != "" {
			result = append(result, origin)
		}
	}

	return result
}

func main() {
	// Parse configuration
	config := parseFlags()

	// Validate configuration
	if err := validateConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger, err := setupLogger(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting nebulagc-server",
		zap.String("version", "0.1.0"),
		zap.String("instance_id", config.InstanceID),
		zap.String("listen_addr", config.ListenAddr),
		zap.String("log_level", config.LogLevel),
		zap.Bool("write_guard", !config.DisableWriteGuard),
	)

	// Open database
	db, err := openDatabase(config.DatabasePath, logger)
	if err != nil {
		logger.Fatal("failed to open database", zap.Error(err))
	}
	defer db.Close()

	// Setup HTTP router
	router := api.SetupRouter(&api.RouterConfig{
		DB:                db,
		Logger:            logger,
		HMACSecret:        config.HMACSecret,
		InstanceID:        config.InstanceID,
		AllowOrigins:      parseCORSOrigins(config.AllowOrigins),
		DisableWriteGuard: config.DisableWriteGuard,
	})

	// Start HTTP server
	logger.Info("server listening", zap.String("addr", config.ListenAddr))
	if err := router.Run(config.ListenAddr); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}
