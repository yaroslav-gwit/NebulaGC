package daemon

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
)

// Config file locations
const (
	// ProductionConfigPath is the default config location for production deployments
	ProductionConfigPath = "/etc/nebulagc/config.json"

	// DevelopmentConfigPath is the optional config location for development/testing
	DevelopmentConfigPath = "./dev_config.json"

	// MinTokenLength is the minimum length for authentication tokens (HMAC-SHA256 = 41 chars)
	MinTokenLength = 41
)

// UUID validation regex (8-4-4-4-12 format)
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// DaemonConfig represents the complete daemon configuration.
type DaemonConfig struct {
	// ControlPlaneURLs is the list of control plane base URLs for HA support.
	ControlPlaneURLs []string `json:"control_plane_urls"`

	// Clusters is the list of Nebula clusters this daemon manages.
	Clusters []ClusterConfig `json:"clusters"`
}

// ClusterConfig represents configuration for a single Nebula cluster.
type ClusterConfig struct {
	// Name is a human-readable identifier for this cluster (used in logs).
	Name string `json:"name"`

	// TenantID is the UUID of the tenant this cluster belongs to.
	TenantID string `json:"tenant_id"`

	// ClusterID is the UUID of the cluster.
	ClusterID string `json:"cluster_id"`

	// NodeID is the UUID of this node in the cluster.
	NodeID string `json:"node_id"`

	// NodeToken is the authentication token for node operations.
	NodeToken string `json:"node_token"`

	// ClusterToken is the authentication token for cluster operations (optional, for admin nodes).
	ClusterToken string `json:"cluster_token,omitempty"`

	// ConfigDir is the directory where Nebula config files will be written.
	ConfigDir string `json:"config_dir"`
}

// LoadConfig loads the daemon configuration from disk.
// It checks for a development config first, then falls back to the production config.
//
// Returns:
//   - *DaemonConfig: The loaded and validated configuration
//   - error: Configuration loading or validation error
func LoadConfig() (*DaemonConfig, error) {
	// Try development config first
	if _, err := os.Stat(DevelopmentConfigPath); err == nil {
		return loadConfigFromFile(DevelopmentConfigPath)
	}

	// Fall back to production config
	return loadConfigFromFile(ProductionConfigPath)
}

// LoadConfigFromPath loads configuration from a specific file path.
//
// Parameters:
//   - path: Absolute or relative path to the configuration file
//
// Returns:
//   - *DaemonConfig: The loaded and validated configuration
//   - error: Configuration loading or validation error
func LoadConfigFromPath(path string) (*DaemonConfig, error) {
	return loadConfigFromFile(path)
}

// loadConfigFromFile reads and parses a JSON configuration file.
func loadConfigFromFile(path string) (*DaemonConfig, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var config DaemonConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// Validate checks that the daemon configuration is valid.
//
// Returns:
//   - error: Validation error describing what is wrong, or nil if valid
func (c *DaemonConfig) Validate() error {
	// Validate control plane URLs
	if len(c.ControlPlaneURLs) == 0 {
		return fmt.Errorf("control_plane_urls cannot be empty")
	}

	for i, urlStr := range c.ControlPlaneURLs {
		if urlStr == "" {
			return fmt.Errorf("control_plane_urls[%d] is empty", i)
		}

		// Validate URL format
		if _, err := url.Parse(urlStr); err != nil {
			return fmt.Errorf("control_plane_urls[%d] is invalid: %w", i, err)
		}
	}

	// Validate clusters
	if len(c.Clusters) == 0 {
		return fmt.Errorf("clusters cannot be empty")
	}

	for i, cluster := range c.Clusters {
		if err := cluster.Validate(); err != nil {
			return fmt.Errorf("clusters[%d] (%s): %w", i, cluster.Name, err)
		}
	}

	return nil
}

// Validate checks that the cluster configuration is valid.
//
// Returns:
//   - error: Validation error describing what is wrong, or nil if valid
func (c *ClusterConfig) Validate() error {
	// Validate name
	if c.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	// Validate UUIDs
	if !isValidUUID(c.TenantID) {
		return fmt.Errorf("tenant_id is not a valid UUID: %s", c.TenantID)
	}

	if !isValidUUID(c.ClusterID) {
		return fmt.Errorf("cluster_id is not a valid UUID: %s", c.ClusterID)
	}

	if !isValidUUID(c.NodeID) {
		return fmt.Errorf("node_id is not a valid UUID: %s", c.NodeID)
	}

	// Validate tokens
	if len(c.NodeToken) < MinTokenLength {
		return fmt.Errorf("node_token is too short (minimum %d characters, got %d)", MinTokenLength, len(c.NodeToken))
	}

	// Cluster token is optional, but if provided must be valid
	if c.ClusterToken != "" && len(c.ClusterToken) < MinTokenLength {
		return fmt.Errorf("cluster_token is too short (minimum %d characters, got %d)", MinTokenLength, len(c.ClusterToken))
	}

	// Validate config directory
	if c.ConfigDir == "" {
		return fmt.Errorf("config_dir cannot be empty")
	}

	// Check if config directory is an absolute path
	if !filepath.IsAbs(c.ConfigDir) {
		return fmt.Errorf("config_dir must be an absolute path: %s", c.ConfigDir)
	}

	return nil
}

// isValidUUID checks if a string matches the UUID format (8-4-4-4-12).
func isValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}
