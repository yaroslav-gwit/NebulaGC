package models

import "time"

// Node represents a machine enrolled in a Nebula cluster.
// Each node belongs to exactly one tenant and one cluster.
// Nodes are authenticated using both a node-specific token and the cluster's shared token.
type Node struct {
	// ID is the unique identifier for this node (UUID v4 format)
	ID string `json:"id" db:"id"`

	// TenantID is the UUID of the owning tenant
	TenantID string `json:"tenant_id" db:"tenant_id"`

	// ClusterID is the UUID of the cluster this node belongs to
	ClusterID string `json:"cluster_id" db:"cluster_id"`

	// Name is the human-readable node name (e.g., "web-server-01", "router-eu-west")
	// Maximum length: 255 characters
	// Must be unique within the cluster
	Name string `json:"name" db:"name"`

	// IsAdmin indicates whether this node has cluster administrator privileges
	// Admin nodes can create other nodes, upload config bundles, and manage topology
	// Regular nodes can only download configs and register their own routes
	IsAdmin bool `json:"is_admin" db:"is_admin"`

	// TokenHash is the HMAC-SHA256 hash of this node's authentication token
	// The actual token is returned only when the node is created or rotated
	// Never returned in API responses
	TokenHash string `json:"-" db:"token_hash"`

	// MTU is the Maximum Transmission Unit size in bytes for this node's Nebula interface
	// Default: 1300 bytes (safe for most networks)
	// Valid range: 1280-9000 bytes (IPv6 minimum to jumbo frames)
	MTU int `json:"mtu" db:"mtu"`

	// Routes is a JSON array of CIDR strings representing internal networks
	// this node can route to (e.g., ["10.0.0.0/8", "192.168.1.0/24"])
	// These routes are propagated to all other nodes via config bundles
	Routes string `json:"routes,omitempty" db:"routes"`

	// RoutesUpdatedAt is the timestamp when routes were last modified
	RoutesUpdatedAt *time.Time `json:"routes_updated_at,omitempty" db:"routes_updated_at"`

	// IsLighthouse indicates whether this node acts as a lighthouse
	// Lighthouses help other nodes discover each other through NAT
	IsLighthouse bool `json:"is_lighthouse" db:"is_lighthouse"`

	// LighthousePublicIP is the public IP address for this lighthouse
	// Required if IsLighthouse is true
	LighthousePublicIP string `json:"lighthouse_public_ip,omitempty" db:"lighthouse_public_ip"`

	// LighthousePort is the UDP port this lighthouse listens on
	// Defaults to the cluster's lighthouse_port or 4242
	LighthousePort int `json:"lighthouse_port,omitempty" db:"lighthouse_port"`

	// IsRelay indicates whether this node acts as a relay
	// Relays forward traffic for nodes that cannot establish direct connections
	IsRelay bool `json:"is_relay" db:"is_relay"`

	// LighthouseRelayUpdatedAt is the timestamp when lighthouse/relay status was last modified
	LighthouseRelayUpdatedAt *time.Time `json:"lighthouse_relay_updated_at,omitempty" db:"lighthouse_relay_updated_at"`

	// CreatedAt is the timestamp when this node was created
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// NodeCreateRequest represents the request body for creating a new node.
type NodeCreateRequest struct {
	// Name is the desired node name (required)
	// Must be 1-255 characters
	Name string `json:"name" binding:"required,min=1,max=255"`

	// IsAdmin indicates whether this node should have admin privileges
	// Default: false
	IsAdmin bool `json:"is_admin"`

	// MTU is the Maximum Transmission Unit size in bytes
	// Default: 1300
	// Valid range: 1280-9000
	MTU int `json:"mtu,omitempty"`
}

// NodeCredentials represents the response after creating a node.
// This is the only time the node_token is returned.
type NodeCredentials struct {
	// NodeID is the UUID of the created node
	NodeID string `json:"node_id"`

	// NodeToken is the authentication token for this specific node
	// Minimum 41 characters
	// Store this securely - it cannot be retrieved later
	NodeToken string `json:"node_token"`

	// ClusterToken is the shared secret for the cluster
	// Included for convenience when enrolling nodes
	// All nodes in the cluster use this same token
	ClusterToken string `json:"cluster_token"`

	// CreatedAt is the timestamp when this node was created
	CreatedAt time.Time `json:"created_at"`
}

// NodeSummary represents a node in list responses (without sensitive fields).
type NodeSummary struct {
	// NodeID is the UUID of the node
	NodeID string `json:"node_id"`

	// Name is the human-readable node name
	Name string `json:"name"`

	// IsAdmin indicates whether this node has admin privileges
	IsAdmin bool `json:"is_admin"`

	// MTU is the Maximum Transmission Unit size in bytes
	MTU int `json:"mtu"`

	// IsLighthouse indicates whether this node acts as a lighthouse
	IsLighthouse bool `json:"is_lighthouse"`

	// IsRelay indicates whether this node acts as a relay
	IsRelay bool `json:"is_relay"`

	// Routes is the list of CIDR strings this node advertises
	Routes []string `json:"routes,omitempty"`

	// CreatedAt is the timestamp when this node was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the timestamp when this node was last modified
	UpdatedAt time.Time `json:"updated_at"`
}

// NodeListResponse represents the response for listing nodes.
type NodeListResponse struct {
	// ClusterID is the UUID of the cluster these nodes belong to
	ClusterID string `json:"cluster_id"`

	// Nodes is the list of nodes in the cluster
	Nodes []NodeSummary `json:"nodes"`

	// Total is the total number of nodes
	Total int `json:"total"`

	// Page is the current page number (if pagination is used)
	Page int `json:"page,omitempty"`

	// PerPage is the number of nodes per page (if pagination is used)
	PerPage int `json:"per_page,omitempty"`
}

// NodeMTUUpdateRequest represents the request body for updating a node's MTU.
type NodeMTUUpdateRequest struct {
	// MTU is the new Maximum Transmission Unit size in bytes
	// Valid range: 1280-9000
	MTU int `json:"mtu" binding:"required,min=1280,max=9000"`
}

// NodeTokenRotateResponse represents the response after rotating a node's token.
type NodeTokenRotateResponse struct {
	// NodeID is the UUID of the node
	NodeID string `json:"node_id"`

	// NodeToken is the new authentication token
	// The old token is immediately invalidated
	NodeToken string `json:"node_token"`

	// RotatedAt is the timestamp when the token was rotated
	RotatedAt time.Time `json:"rotated_at"`
}

// NodeRoutesRequest represents the request body for registering internal routes.
type NodeRoutesRequest struct {
	// Routes is the list of CIDR strings this node can route to
	// Empty array clears all routes for this node
	// Each route must be valid CIDR notation (e.g., "10.0.0.0/8")
	Routes []string `json:"routes" binding:"required"`
}

// NodeRoutesResponse represents the response after registering routes.
type NodeRoutesResponse struct {
	// NodeID is the UUID of the node
	NodeID string `json:"node_id"`

	// Routes is the list of CIDR strings this node advertises
	Routes []string `json:"routes"`

	// UpdatedAt is the timestamp when routes were last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// NodeLighthouseRequest represents the request body for setting lighthouse status.
type NodeLighthouseRequest struct {
	// IsLighthouse indicates whether to enable or disable lighthouse status
	IsLighthouse bool `json:"is_lighthouse"`

	// PublicIP is the public IP address for the lighthouse
	// Required if IsLighthouse is true
	PublicIP string `json:"public_ip,omitempty"`

	// LighthousePort is the UDP port for lighthouse traffic
	// Defaults to cluster's lighthouse_port or 4242
	LighthousePort int `json:"lighthouse_port,omitempty"`
}

// NodeRelayRequest represents the request body for setting relay status.
type NodeRelayRequest struct {
	// IsRelay indicates whether to enable or disable relay status
	IsRelay bool `json:"is_relay"`
}
