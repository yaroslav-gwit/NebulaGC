package sdk

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ClientConfig contains the configuration for creating a new SDK client.
type ClientConfig struct {
	// BaseURLs is the list of control plane URLs (e.g., ["https://cp1.example.com:8080", "https://cp2.example.com:8080"])
	// For HA setups, provide multiple URLs. The client will discover the master and fail over automatically.
	BaseURLs []string

	// TenantID is the unique identifier for the tenant (UUID v4).
	TenantID string

	// ClusterID is the unique identifier for the cluster (UUID v4).
	ClusterID string

	// NodeID is the unique identifier for this node (UUID v4).
	// Optional: only required for node-specific operations.
	NodeID string

	// NodeToken is the authentication token for node operations.
	// Optional: only required if performing node-authenticated requests.
	NodeToken string

	// ClusterToken is the authentication token for cluster operations.
	// Optional: only required if performing cluster-authenticated requests.
	ClusterToken string

	// HTTPClient is the HTTP client to use for requests.
	// Optional: if nil, a default client with reasonable timeouts will be created.
	HTTPClient *http.Client

	// RetryAttempts is the number of times to retry failed requests.
	// Default: 3
	RetryAttempts int

	// RetryWaitMin is the minimum wait time between retries.
	// Default: 1 second
	RetryWaitMin time.Duration

	// RetryWaitMax is the maximum wait time between retries.
	// Default: 30 seconds
	RetryWaitMax time.Duration

	// Timeout is the HTTP request timeout.
	// Default: 30 seconds
	Timeout time.Duration
}

// Validate checks if the client configuration is valid and sets defaults.
func (c *ClientConfig) Validate() error {
	// Check for at least one base URL
	if len(c.BaseURLs) == 0 {
		return fmt.Errorf("%w: at least one base URL is required", ErrInvalidConfig)
	}

	// Validate and normalize base URLs
	for i, url := range c.BaseURLs {
		url = strings.TrimSpace(url)
		if url == "" {
			return fmt.Errorf("%w: base URL at index %d is empty", ErrInvalidConfig, i)
		}

		// Ensure URLs don't end with a slash
		url = strings.TrimSuffix(url, "/")
		c.BaseURLs[i] = url

		// Validate URL format (must start with http:// or https://)
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return fmt.Errorf("%w: base URL must start with http:// or https://", ErrInvalidConfig)
		}
	}

	// Tenant ID is always required
	if strings.TrimSpace(c.TenantID) == "" {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidConfig)
	}

	// Cluster ID is always required
	if strings.TrimSpace(c.ClusterID) == "" {
		return fmt.Errorf("%w: cluster_id is required", ErrInvalidConfig)
	}

	// Set default retry attempts if not provided
	if c.RetryAttempts == 0 {
		c.RetryAttempts = 3
	}

	// Set default retry wait times if not provided
	if c.RetryWaitMin == 0 {
		c.RetryWaitMin = 1 * time.Second
	}
	if c.RetryWaitMax == 0 {
		c.RetryWaitMax = 30 * time.Second
	}

	// Set default timeout if not provided
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}

	// Create default HTTP client if not provided
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{
			Timeout: c.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}

	return nil
}

// HasNodeAuth returns true if node authentication credentials are available.
func (c *ClientConfig) HasNodeAuth() bool {
	return strings.TrimSpace(c.NodeToken) != ""
}

// HasClusterAuth returns true if cluster authentication credentials are available.
func (c *ClientConfig) HasClusterAuth() bool {
	return strings.TrimSpace(c.ClusterToken) != ""
}
