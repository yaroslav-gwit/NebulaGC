// Package main provides the NebulaGC control plane server.
//
// This is the main entrypoint for the nebulagc-server binary which runs
// the control plane HTTP API for managing Nebula overlay networks.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	_ "modernc.org/sqlite"

	"nebulagc.io/server/internal/api"
	"nebulagc.io/server/internal/ha"
	"nebulagc.io/server/internal/service"
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

	// Mode indicates whether the server runs as master or replica.
	Mode ha.Mode

	// PublicURL is the externally reachable URL for this instance.
	PublicURL string
}

// parseFlags parses command-line flags and environment variables.
func parseFlags() *Config {
	config := &Config{}

	modeEnv := strings.ToLower(getEnv("NEBULAGC_MODE", ""))
	defaultMaster := modeEnv == "master"
	defaultReplica := modeEnv == "replica"

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
	flag.StringVar(&config.PublicURL, "public-url", getEnv("NEBULAGC_PUBLIC_URL", ""),
		"Public URL for this instance (e.g., https://cp1.example.com:8080)")

	masterFlag := flag.Bool("master", defaultMaster, "Run in master mode (write-enabled)")
	replicaFlag := flag.Bool("replica", defaultReplica, "Run in replica mode (read-only)")

	flag.Parse()

	// Resolve HA mode after parsing flags
	if *masterFlag != *replicaFlag {
		if *masterFlag {
			config.Mode = ha.ModeMaster
		} else {
			config.Mode = ha.ModeReplica
		}
	}

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

	// Validate HA mode
	if !ha.ValidateMode(config.Mode) {
		return fmt.Errorf("must specify exactly one of --master or --replica (or NEBULAGC_MODE)")
	}

	// Validate public URL
	if config.PublicURL == "" {
		return fmt.Errorf("public URL is required for replica registry (set NEBULAGC_PUBLIC_URL or use -public-url)")
	}

	parsedURL, err := url.Parse(config.PublicURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("invalid public URL %q: must include scheme and host", config.PublicURL)
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

	parts := strings.Split(origins, ",")
	var result []string
	for _, origin := range parts {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			result = append(result, trimmed)
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
		zap.String("mode", string(config.Mode)),
		zap.String("public_url", config.PublicURL),
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

	// Initialize services
	replicaService := service.NewReplicaService(db, logger)

	haConfig := ha.DefaultConfig(config.InstanceID, config.PublicURL, config.Mode)
	haManager := ha.NewManager(haConfig, replicaService, logger)

	if err := haManager.Start(); err != nil {
		logger.Fatal("failed to start HA manager", zap.Error(err))
	}

	// Setup HTTP router
	router := api.SetupRouter(&api.RouterConfig{
		DB:                db,
		Logger:            logger,
		HMACSecret:        config.HMACSecret,
		InstanceID:        config.InstanceID,
		AllowOrigins:      parseCORSOrigins(config.AllowOrigins),
		DisableWriteGuard: config.DisableWriteGuard,
		HAManager:         haManager,
	})

	// Start HTTP server
	logger.Info("server listening", zap.String("addr", config.ListenAddr))
	server := &http.Server{
		Addr:    config.ListenAddr,
		Handler: router,
	}

	// Run server in background to enable graceful shutdown
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received, stopping server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", zap.Error(err))
	}

	if err := haManager.Stop(); err != nil {
		logger.Error("failed to stop HA manager", zap.Error(err))
	}
}
