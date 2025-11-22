package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client is the main SDK client for interacting with the NebulaGC control plane.
// It supports high availability with automatic master discovery and failover.
type Client struct {
	// BaseURLs is the list of control plane URLs for HA support.
	BaseURLs []string

	// TenantID is the unique identifier for the tenant.
	TenantID string

	// ClusterID is the unique identifier for the cluster.
	ClusterID string

	// NodeID is the unique identifier for this node (optional).
	NodeID string

	// NodeToken is the authentication token for node operations (optional).
	NodeToken string

	// ClusterToken is the authentication token for cluster operations (optional).
	ClusterToken string

	// HTTPClient is the HTTP client used for requests.
	HTTPClient *http.Client

	// RetryAttempts is the number of times to retry failed requests.
	RetryAttempts int

	// RetryWaitMin is the minimum wait time between retries.
	RetryWaitMin time.Duration

	// RetryWaitMax is the maximum wait time between retries.
	RetryWaitMax time.Duration

	// masterURL is the cached URL of the current master (protected by mutex).
	masterURL string

	// mu protects concurrent access to masterURL.
	mu sync.RWMutex
}

// NewClient creates a new SDK client with the given configuration.
// It validates the configuration and optionally discovers the master instance.
func NewClient(config ClientConfig) (*Client, error) {
	// Validate and set defaults
	if err := config.Validate(); err != nil {
		return nil, err
	}

	client := &Client{
		BaseURLs:      config.BaseURLs,
		TenantID:      config.TenantID,
		ClusterID:     config.ClusterID,
		NodeID:        config.NodeID,
		NodeToken:     config.NodeToken,
		ClusterToken:  config.ClusterToken,
		HTTPClient:    config.HTTPClient,
		RetryAttempts: config.RetryAttempts,
		RetryWaitMin:  config.RetryWaitMin,
		RetryWaitMax:  config.RetryWaitMax,
	}

	return client, nil
}

// DiscoverMaster attempts to discover which control plane instance is the master.
// It caches the result for future requests. Returns ErrNoMasterFound if no master is available.
func (c *Client) DiscoverMaster(ctx context.Context) error {
	for _, baseURL := range c.BaseURLs {
		url := fmt.Sprintf("%s/api/v1/check-master", baseURL)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			continue
		}

		defer drainAndCloseBody(resp)

		if resp.StatusCode == http.StatusOK {
			c.mu.Lock()
			c.masterURL = baseURL
			c.mu.Unlock()
			return nil
		}
	}

	return ErrNoMasterFound
}

// getMasterURL returns the cached master URL, or empty string if not discovered.
func (c *Client) getMasterURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.masterURL
}

// clearMasterCache clears the cached master URL, forcing rediscovery on next request.
func (c *Client) clearMasterCache() {
	c.mu.Lock()
	c.masterURL = ""
	c.mu.Unlock()
}

// doRequest performs an HTTP request to the control plane with automatic failover.
// If preferMaster is true, it will attempt to use the cached master URL first.
// authType specifies which authentication headers to include.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, authType AuthType, preferMaster bool) (*http.Response, error) {
	// Build list of URLs to try
	urls := c.buildURLList(preferMaster)

	if len(urls) == 0 {
		return nil, ErrNoBaseURLs
	}

	var lastErr error

	for _, baseURL := range urls {
		// Build full URL
		fullURL := fmt.Sprintf("%s%s", baseURL, path)

		// Create request
		req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		// Add authentication headers
		if err := c.addAuthHeaders(req, authType); err != nil {
			return nil, err
		}

		// Set common headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		// Perform request with retry logic
		resp, err := c.doRequestWithRetry(ctx, req)
		if err != nil {
			lastErr = err
			// If this was the master URL and it failed, clear the cache
			if baseURL == c.getMasterURL() {
				c.clearMasterCache()
			}
			continue
		}

		// Check for authentication errors
		if resp.StatusCode == http.StatusUnauthorized {
			drainAndCloseBody(resp)
			return nil, ErrUnauthorized
		}

		// Check for rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			drainAndCloseBody(resp)
			return nil, ErrRateLimited
		}

		// Success or client error (4xx other than 401/429)
		return resp, nil
	}

	// All instances failed
	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrAllInstancesFailed, lastErr)
	}

	return nil, ErrAllInstancesFailed
}

// buildURLList builds a prioritized list of URLs to try for a request.
// If preferMaster is true and a master is cached, it will be first in the list.
func (c *Client) buildURLList(preferMaster bool) []string {
	if preferMaster {
		masterURL := c.getMasterURL()
		if masterURL != "" {
			// Master URL first, then others
			urls := []string{masterURL}
			for _, url := range c.BaseURLs {
				if url != masterURL {
					urls = append(urls, url)
				}
			}
			return urls
		}
	}

	// Return all URLs in order
	return c.BaseURLs
}

// parseJSONResponse parses a JSON response body into the provided destination.
func (c *Client) parseJSONResponse(resp *http.Response, dest interface{}) error {
	defer drainAndCloseBody(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return nil
}

// parseErrorResponse attempts to parse an error response from the server.
func (c *Client) parseErrorResponse(resp *http.Response) error {
	var apiErr APIResponse
	if err := c.parseJSONResponse(resp, &apiErr); err != nil {
		// Failed to parse error response, return generic error
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	if apiErr.Error != "" {
		return fmt.Errorf("API error: %s", apiErr.Error)
	}

	return fmt.Errorf("request failed with status %d", resp.StatusCode)
}

// doJSONRequest is a convenience method that performs a request with JSON body and parses the JSON response.
func (c *Client) doJSONRequest(ctx context.Context, method, path string, reqBody, respBody interface{}, authType AuthType, preferMaster bool) error {
	var body io.Reader
	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	resp, err := c.doRequest(ctx, method, path, body, authType, preferMaster)
	if err != nil {
		return err
	}

	// Check for success status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.parseErrorResponse(resp)
	}

	// Parse response if a destination was provided
	if respBody != nil {
		return c.parseJSONResponse(resp, respBody)
	}

	// No response body expected, just close
	drainAndCloseBody(resp)
	return nil
}

// ============================================================================
// Node Management Methods
// ============================================================================

// CreateNode creates a new node in the cluster with the specified configuration.
// It returns the node credentials including the node ID, node token, and Nebula IP.
// The node token is only returned once and must be stored securely.
//
// This operation requires cluster token authentication and is executed on the master instance.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - name: Human-readable name for the node (1-255 characters)
//   - isAdmin: Whether the node should have administrative privileges
//   - mtu: Maximum Transmission Unit for the node (576-9000)
//
// Returns:
//   - *NodeCredentials: The created node's credentials (ID, token, IP)
//   - error: ErrUnauthorized if cluster token is invalid, ErrRateLimited if rate limited,
//     or other errors for validation failures or network issues
func (c *Client) CreateNode(ctx context.Context, name string, isAdmin bool, mtu int) (*NodeCredentials, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/nodes", c.TenantID, c.ClusterID)

	reqBody := map[string]interface{}{
		"name":     name,
		"is_admin": isAdmin,
		"mtu":      mtu,
	}

	var credentials NodeCredentials
	if err := c.doJSONRequest(ctx, http.MethodPost, path, reqBody, &credentials, AuthTypeCluster, true); err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	return &credentials, nil
}

// DeleteNode removes a node from the cluster.
// This operation is irreversible and will invalidate the node's authentication token.
//
// This operation requires cluster token authentication and is executed on the master instance.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - nodeID: The unique identifier of the node to delete
//
// Returns:
//   - error: ErrUnauthorized if cluster token is invalid, ErrNotFound if node doesn't exist,
//     ErrRateLimited if rate limited, or other errors for network issues
func (c *Client) DeleteNode(ctx context.Context, nodeID string) error {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/nodes/%s", c.TenantID, c.ClusterID, nodeID)

	if err := c.doJSONRequest(ctx, http.MethodDelete, path, nil, nil, AuthTypeCluster, true); err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	return nil
}

// ListNodes retrieves a paginated list of nodes in the cluster.
// This operation can be executed on any control plane instance (master or replica).
//
// This operation requires cluster token authentication.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - page: Page number (1-based, use 1 for first page)
//   - pageSize: Number of nodes per page (1-1000)
//
// Returns:
//   - []NodeSummary: List of nodes in the cluster
//   - error: ErrUnauthorized if cluster token is invalid, ErrRateLimited if rate limited,
//     or other errors for validation failures or network issues
func (c *Client) ListNodes(ctx context.Context, page, pageSize int) ([]NodeSummary, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/nodes?page=%d&page_size=%d",
		c.TenantID, c.ClusterID, page, pageSize)

	var nodes []NodeSummary
	if err := c.doJSONRequest(ctx, http.MethodGet, path, nil, &nodes, AuthTypeCluster, false); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	return nodes, nil
}

// UpdateMTU updates the Maximum Transmission Unit for a specific node.
// The new MTU must be between 576 and 9000 bytes.
//
// This operation requires cluster token authentication and is executed on the master instance.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - nodeID: The unique identifier of the node to update
//   - mtu: The new MTU value (576-9000)
//
// Returns:
//   - error: ErrUnauthorized if cluster token is invalid, ErrNotFound if node doesn't exist,
//     ErrRateLimited if rate limited, or other errors for validation failures or network issues
func (c *Client) UpdateMTU(ctx context.Context, nodeID string, mtu int) error {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/nodes/%s/mtu", c.TenantID, c.ClusterID, nodeID)

	reqBody := map[string]interface{}{
		"mtu": mtu,
	}

	if err := c.doJSONRequest(ctx, http.MethodPut, path, reqBody, nil, AuthTypeCluster, true); err != nil {
		return fmt.Errorf("failed to update MTU: %w", err)
	}

	return nil
}

// RotateNodeToken generates a new authentication token for the specified node.
// The old token is immediately invalidated. The new token is only returned once
// and must be provided to the node daemon to maintain connectivity.
//
// This operation requires cluster token authentication and is executed on the master instance.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - nodeID: The unique identifier of the node whose token should be rotated
//
// Returns:
//   - string: The new authentication token (store securely, only returned once)
//   - error: ErrUnauthorized if cluster token is invalid, ErrNotFound if node doesn't exist,
//     ErrRateLimited if rate limited, or other errors for network issues
func (c *Client) RotateNodeToken(ctx context.Context, nodeID string) (string, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/nodes/%s/rotate-token", c.TenantID, c.ClusterID, nodeID)

	var response TokenRotationResponse
	if err := c.doJSONRequest(ctx, http.MethodPost, path, nil, &response, AuthTypeCluster, true); err != nil {
		return "", fmt.Errorf("failed to rotate node token: %w", err)
	}

	return response.Token, nil
}

// ============================================================================
// Config Bundle Methods
// ============================================================================

// GetLatestVersion retrieves the current config bundle version for the cluster.
// This is used by the daemon to check if a new config version is available without
// downloading the full bundle.
//
// This operation requires node token authentication and can be executed on any
// control plane instance (master or replica).
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//
// Returns:
//   - int64: The current config bundle version number
//   - error: ErrUnauthorized if node token is invalid, ErrRateLimited if rate limited,
//     or other errors for network issues
func (c *Client) GetLatestVersion(ctx context.Context) (int64, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/config/version", c.TenantID, c.ClusterID)

	var versionResp VersionResponse
	if err := c.doJSONRequest(ctx, http.MethodGet, path, nil, &versionResp, AuthTypeNode, false); err != nil {
		return 0, fmt.Errorf("failed to get latest version: %w", err)
	}

	return versionResp.Version, nil
}

// DownloadBundle downloads the config bundle if a newer version is available.
// It supports HTTP 304 Not Modified responses to avoid unnecessary downloads.
//
// This operation requires node token authentication and can be executed on any
// control plane instance (master or replica).
//
// If the server returns 304 Not Modified (currentVersion is up to date):
//   - Returns (nil, currentVersion, nil)
//
// If a new version is available:
//   - Returns (bundleData, newVersion, nil)
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - currentVersion: The version currently installed on the node
//
// Returns:
//   - []byte: The bundle data as a tar.gz archive, or nil if no update
//   - int64: The new version number, or currentVersion if no update
//   - error: ErrUnauthorized if node token is invalid, ErrRateLimited if rate limited,
//     or other errors for network issues
func (c *Client) DownloadBundle(ctx context.Context, currentVersion int64) ([]byte, int64, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/config/bundle?current_version=%d",
		c.TenantID, c.ClusterID, currentVersion)

	// Build URL list
	urls := c.buildURLList(false)
	if len(urls) == 0 {
		return nil, 0, ErrNoBaseURLs
	}

	var lastErr error

	for _, baseURL := range urls {
		fullURL := fmt.Sprintf("%s%s", baseURL, path)

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		// Add authentication headers
		if err := c.addAuthHeaders(req, AuthTypeNode); err != nil {
			return nil, 0, err
		}

		// Set headers
		req.Header.Set("Accept", "application/octet-stream")

		// Perform request with retry
		resp, err := c.doRequestWithRetry(ctx, req)
		if err != nil {
			lastErr = err
			if baseURL == c.getMasterURL() {
				c.clearMasterCache()
			}
			continue
		}

		// Check for 304 Not Modified
		if resp.StatusCode == http.StatusNotModified {
			drainAndCloseBody(resp)
			return nil, currentVersion, nil
		}

		// Check for authentication errors
		if resp.StatusCode == http.StatusUnauthorized {
			drainAndCloseBody(resp)
			return nil, 0, ErrUnauthorized
		}

		// Check for rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			drainAndCloseBody(resp)
			return nil, 0, ErrRateLimited
		}

		// Check for success
		if resp.StatusCode != http.StatusOK {
			err := c.parseErrorResponse(resp)
			lastErr = err
			continue
		}

		// Read bundle data
		data, err := io.ReadAll(resp.Body)
		drainAndCloseBody(resp)
		if err != nil {
			lastErr = fmt.Errorf("failed to read bundle data: %w", err)
			continue
		}

		// Get new version from header
		versionHeader := resp.Header.Get("X-Config-Version")
		if versionHeader == "" {
			lastErr = fmt.Errorf("missing X-Config-Version header in response")
			continue
		}

		newVersion, err := parseVersion(versionHeader)
		if err != nil {
			lastErr = fmt.Errorf("invalid version header: %w", err)
			continue
		}

		return data, newVersion, nil
	}

	// All instances failed
	if lastErr != nil {
		return nil, 0, fmt.Errorf("failed to download bundle: %w", lastErr)
	}

	return nil, 0, ErrAllInstancesFailed
}

// UploadBundle uploads a new config bundle to the control plane.
// This operation is typically restricted to admin nodes.
//
// This operation requires node token authentication and is executed on the master instance.
//
// The bundle must be a valid tar.gz archive containing the required Nebula configuration files.
// The server will validate the bundle and increment the version number automatically.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - data: The bundle data as a tar.gz archive
//
// Returns:
//   - int64: The new version number assigned to this bundle
//   - error: ErrUnauthorized if node token is invalid or node lacks admin privileges,
//     ErrRateLimited if rate limited, or other errors for validation failures or network issues
func (c *Client) UploadBundle(ctx context.Context, data []byte) (int64, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/config/bundle", c.TenantID, c.ClusterID)

	// Build URL list preferring master
	urls := c.buildURLList(true)
	if len(urls) == 0 {
		return 0, ErrNoBaseURLs
	}

	var lastErr error

	for _, baseURL := range urls {
		fullURL := fmt.Sprintf("%s%s", baseURL, path)

		// Create request with binary body
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(data))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		// Add authentication headers
		if err := c.addAuthHeaders(req, AuthTypeNode); err != nil {
			return 0, err
		}

		// Set headers for binary upload
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Accept", "application/json")

		// Perform request with retry
		resp, err := c.doRequestWithRetry(ctx, req)
		if err != nil {
			lastErr = err
			if baseURL == c.getMasterURL() {
				c.clearMasterCache()
			}
			continue
		}

		// Check for authentication errors
		if resp.StatusCode == http.StatusUnauthorized {
			drainAndCloseBody(resp)
			return 0, ErrUnauthorized
		}

		// Check for rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			drainAndCloseBody(resp)
			return 0, ErrRateLimited
		}

		// Check for success
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			err := c.parseErrorResponse(resp)
			lastErr = err
			continue
		}

		// Parse response to get new version
		var versionResp VersionResponse
		if err := c.parseJSONResponse(resp, &versionResp); err != nil {
			lastErr = fmt.Errorf("failed to parse response: %w", err)
			continue
		}

		return versionResp.Version, nil
	}

	// All instances failed
	if lastErr != nil {
		return 0, fmt.Errorf("failed to upload bundle: %w", lastErr)
	}

	return 0, ErrAllInstancesFailed
}

// parseVersion parses a version string into an int64.
func parseVersion(versionStr string) (int64, error) {
	version, err := parseInt64(versionStr)
	if err != nil {
		return 0, fmt.Errorf("invalid version format: %w", err)
	}
	return version, nil
}

// parseInt64 parses a string into an int64.
func parseInt64(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	if err != nil {
		return 0, err
	}
	return result, nil
}

// ============================================================================
// Topology Management Methods
// ============================================================================

// RegisterRoutes registers or updates the routes advertised by this node.
// Routes should be specified in CIDR notation (e.g., "10.100.0.0/24").
//
// This operation requires node token authentication and is executed on the master instance.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - routes: List of CIDR routes this node will advertise to the cluster
//
// Returns:
//   - error: ErrUnauthorized if node token is invalid, ErrRateLimited if rate limited,
//     or other errors for validation failures or network issues
func (c *Client) RegisterRoutes(ctx context.Context, routes []string) error {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/nodes/%s/routes", c.TenantID, c.ClusterID, c.NodeID)

	reqBody := map[string]interface{}{
		"routes": routes,
	}

	if err := c.doJSONRequest(ctx, http.MethodPut, path, reqBody, nil, AuthTypeNode, true); err != nil {
		return fmt.Errorf("failed to register routes: %w", err)
	}

	return nil
}

// GetRoutes retrieves the routes currently advertised by this node.
//
// This operation requires node token authentication and can be executed on any
// control plane instance (master or replica).
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//
// Returns:
//   - []string: List of CIDR routes advertised by this node
//   - error: ErrUnauthorized if node token is invalid, ErrRateLimited if rate limited,
//     or other errors for network issues
func (c *Client) GetRoutes(ctx context.Context) ([]string, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/nodes/%s/routes", c.TenantID, c.ClusterID, c.NodeID)

	var response struct {
		Routes []string `json:"routes"`
	}

	if err := c.doJSONRequest(ctx, http.MethodGet, path, nil, &response, AuthTypeNode, false); err != nil {
		return nil, fmt.Errorf("failed to get routes: %w", err)
	}

	return response.Routes, nil
}

// ListClusterRoutes retrieves all routes advertised by all nodes in the cluster.
// This provides a complete view of the cluster's routing table.
//
// This operation requires cluster token authentication and can be executed on any
// control plane instance (master or replica).
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//
// Returns:
//   - []NodeRoutes: List of all nodes and their advertised routes
//   - error: ErrUnauthorized if cluster token is invalid, ErrRateLimited if rate limited,
//     or other errors for network issues
func (c *Client) ListClusterRoutes(ctx context.Context) ([]NodeRoutes, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/routes", c.TenantID, c.ClusterID)

	var routes []NodeRoutes
	if err := c.doJSONRequest(ctx, http.MethodGet, path, nil, &routes, AuthTypeCluster, false); err != nil {
		return nil, fmt.Errorf("failed to list cluster routes: %w", err)
	}

	return routes, nil
}

// SetLighthouse configures a node as a lighthouse or removes lighthouse status.
// Lighthouses must have a publicly accessible IP address and port.
//
// This operation requires cluster token authentication and is executed on the master instance.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - nodeID: The unique identifier of the node to configure
//   - enabled: True to enable lighthouse, false to disable
//   - publicIP: The publicly accessible IP address (required if enabled is true)
//   - port: The UDP port number (required if enabled is true, typically 4242)
//
// Returns:
//   - error: ErrUnauthorized if cluster token is invalid, ErrNotFound if node doesn't exist,
//     ErrRateLimited if rate limited, or other errors for validation failures or network issues
func (c *Client) SetLighthouse(ctx context.Context, nodeID string, enabled bool, publicIP string, port int) error {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/nodes/%s/lighthouse", c.TenantID, c.ClusterID, nodeID)

	reqBody := map[string]interface{}{
		"enabled": enabled,
	}

	if enabled {
		reqBody["public_ip"] = publicIP
		reqBody["port"] = port
	}

	if err := c.doJSONRequest(ctx, http.MethodPut, path, reqBody, nil, AuthTypeCluster, true); err != nil {
		return fmt.Errorf("failed to set lighthouse: %w", err)
	}

	return nil
}

// SetRelay configures a node as a relay or removes relay status.
// Relays help nodes behind restrictive NATs communicate with each other.
//
// This operation requires cluster token authentication and is executed on the master instance.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - nodeID: The unique identifier of the node to configure
//   - enabled: True to enable relay, false to disable
//
// Returns:
//   - error: ErrUnauthorized if cluster token is invalid, ErrNotFound if node doesn't exist,
//     ErrRateLimited if rate limited, or other errors for network issues
func (c *Client) SetRelay(ctx context.Context, nodeID string, enabled bool) error {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/nodes/%s/relay", c.TenantID, c.ClusterID, nodeID)

	reqBody := map[string]interface{}{
		"enabled": enabled,
	}

	if err := c.doJSONRequest(ctx, http.MethodPut, path, reqBody, nil, AuthTypeCluster, true); err != nil {
		return fmt.Errorf("failed to set relay: %w", err)
	}

	return nil
}

// GetTopology retrieves the complete cluster topology including all lighthouses,
// relays, and advertised routes. This provides a comprehensive view of the cluster
// configuration needed for generating Nebula config files.
//
// This operation requires node token authentication and can be executed on any
// control plane instance (master or replica).
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//
// Returns:
//   - *ClusterTopology: Complete cluster topology information
//   - error: ErrUnauthorized if node token is invalid, ErrRateLimited if rate limited,
//     or other errors for network issues
func (c *Client) GetTopology(ctx context.Context) (*ClusterTopology, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/topology", c.TenantID, c.ClusterID)

	var topology ClusterTopology
	if err := c.doJSONRequest(ctx, http.MethodGet, path, nil, &topology, AuthTypeNode, false); err != nil {
		return nil, fmt.Errorf("failed to get topology: %w", err)
	}

	return &topology, nil
}

// RotateClusterToken generates a new authentication token for the cluster.
// The old token is immediately invalidated. The new token is only returned once
// and must be distributed to all administrators.
//
// This operation requires cluster token authentication and is executed on the master instance.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//
// Returns:
//   - string: The new cluster authentication token (store securely, only returned once)
//   - error: ErrUnauthorized if cluster token is invalid, ErrRateLimited if rate limited,
//     or other errors for network issues
func (c *Client) RotateClusterToken(ctx context.Context) (string, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/rotate-token", c.TenantID, c.ClusterID)

	var response TokenRotationResponse
	if err := c.doJSONRequest(ctx, http.MethodPost, path, nil, &response, AuthTypeCluster, true); err != nil {
		return "", fmt.Errorf("failed to rotate cluster token: %w", err)
	}

	return response.Token, nil
}

// CheckMaster queries a specific control plane URL to determine if it is currently
// the master instance. This is useful for discovering the master in an HA cluster.
//
// This operation does not require authentication and uses the /health/master endpoint.
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//   - baseURL: The control plane URL to check (e.g., "https://control1.example.com")
//
// Returns:
//   - bool: True if the queried instance is the master, false otherwise
//   - error: Returns error if the instance is unreachable or returns an invalid response
func (c *Client) CheckMaster(ctx context.Context, baseURL string) (bool, error) {
	// Build request URL
	reqURL := fmt.Sprintf("%s/health/master", baseURL)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request (no authentication required for health check)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var masterStatus MasterStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&masterStatus); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	return masterStatus.IsMaster, nil
}

// GetClusterReplicas retrieves the list of control plane replica instances for the cluster.
// This is useful for discovering all available control plane instances and their health status.
//
// This operation requires cluster token authentication and can be executed on any control plane
// instance (master or replica).
//
// Parameters:
//   - ctx: Request context for cancellation and timeouts
//
// Returns:
//   - []ReplicaInfo: List of replica instances with their status
//   - error: ErrUnauthorized if cluster token is invalid, ErrRateLimited if rate limited,
//     or other errors for network issues
func (c *Client) GetClusterReplicas(ctx context.Context) ([]ReplicaInfo, error) {
	path := fmt.Sprintf("/api/v1/tenants/%s/clusters/%s/replicas", c.TenantID, c.ClusterID)

	var response struct {
		Replicas []ReplicaInfo `json:"replicas"`
	}
	if err := c.doJSONRequest(ctx, http.MethodGet, path, nil, &response, AuthTypeCluster, false); err != nil {
		return nil, fmt.Errorf("failed to get cluster replicas: %w", err)
	}

	return response.Replicas, nil
}
