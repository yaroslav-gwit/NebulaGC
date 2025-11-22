package sdk

import "time"

// NodeCredentials contains the credentials returned after creating a node.
// These credentials must be stored securely and provided to the node daemon.
type NodeCredentials struct {
	// NodeID is the unique identifier for the created node.
	NodeID string `json:"node_id"`

	// NodeToken is the authentication token for the node (only returned once).
	NodeToken string `json:"node_token"`

	// NebulaIP is the Nebula overlay IP address assigned to the node.
	NebulaIP string `json:"nebula_ip"`
}

// NodeSummary represents a node in list responses.
type NodeSummary struct {
	// ID is the unique identifier for the node.
	ID string `json:"id"`

	// Name is the human-readable node name.
	Name string `json:"name"`

	// NebulaIP is the Nebula overlay IP address.
	NebulaIP string `json:"nebula_ip"`

	// IsAdmin indicates if this node has administrative privileges.
	IsAdmin bool `json:"is_admin"`

	// MTU is the Maximum Transmission Unit for the node.
	MTU int `json:"mtu"`

	// CreatedAt is the node creation timestamp.
	CreatedAt time.Time `json:"created_at"`
}

// NodeRoutes represents routes advertised by a node.
type NodeRoutes struct {
	// NodeID is the unique identifier for the node.
	NodeID string `json:"node_id"`

	// Routes is the list of CIDR routes advertised by this node.
	Routes []string `json:"routes"`
}

// LighthouseInfo contains information about a lighthouse node.
type LighthouseInfo struct {
	// NodeID is the unique identifier for the lighthouse node.
	NodeID string `json:"node_id"`

	// Name is the lighthouse node's human-readable name.
	Name string `json:"name"`

	// PublicIP is the publicly accessible IP address.
	PublicIP string `json:"public_ip"`

	// Port is the UDP port the lighthouse listens on.
	Port int `json:"port"`
}

// RelayInfo contains information about a relay node.
type RelayInfo struct {
	// NodeID is the unique identifier for the relay node.
	NodeID string `json:"node_id"`

	// Name is the relay node's human-readable name.
	Name string `json:"name"`
}

// ClusterTopology represents the complete topology of a cluster.
type ClusterTopology struct {
	// Lighthouses is the list of all lighthouse nodes in the cluster.
	Lighthouses []LighthouseInfo `json:"lighthouses"`

	// Relays is the list of all relay nodes in the cluster.
	Relays []RelayInfo `json:"relays"`

	// Routes maps node IDs to their advertised routes.
	Routes map[string][]string `json:"routes"`
}

// ReplicaInfo represents a control plane replica instance.
type ReplicaInfo struct {
	// InstanceID is the unique identifier for this replica.
	InstanceID string `json:"instance_id"`

	// URL is the public URL for this replica.
	URL string `json:"url"`

	// IsMaster indicates if this instance is currently the master.
	IsMaster bool `json:"is_master"`

	// LastHeartbeat is the last time this replica sent a heartbeat.
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// MasterStatusResponse represents the response from /health/master endpoint.
type MasterStatusResponse struct {
	// IsMaster indicates if the queried instance is currently the master.
	IsMaster bool `json:"is_master"`

	// InstanceID is the unique identifier for the queried instance.
	InstanceID string `json:"instance_id"`

	// MasterURL is the URL of the master instance (if this instance is not master).
	MasterURL string `json:"master_url,omitempty"`
}

// TokenRotationResponse contains the new token after rotation.
type TokenRotationResponse struct {
	// Token is the new authentication token (only returned once).
	Token string `json:"token"`

	// Message is a human-readable confirmation message.
	Message string `json:"message"`
}

// APIResponse is a generic wrapper for API responses with data.
type APIResponse struct {
	// Data contains the response payload.
	Data interface{} `json:"data,omitempty"`

	// Message contains a human-readable message.
	Message string `json:"message,omitempty"`

	// Error contains an error message if the request failed.
	Error string `json:"error,omitempty"`
}

// VersionResponse contains the current config version.
type VersionResponse struct {
	// Version is the current config bundle version number.
	Version int64 `json:"version"`
}
