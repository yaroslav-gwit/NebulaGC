package models

import "time"

// ClusterTopology represents the network topology of a Nebula cluster.
// This includes information about lighthouses, relays, and control plane
// lighthouse functionality.
type ClusterTopology struct {
	// ClusterID is the UUID of the cluster
	ClusterID string `json:"cluster_id"`

	// ProvideLighthouse indicates if the control plane acts as a lighthouse
	ProvideLighthouse bool `json:"provide_lighthouse"`

	// ControlPlaneLighthouse contains information about control plane lighthouse
	// functionality if ProvideLighthouse is true
	ControlPlaneLighthouse *ControlPlaneLighthouseInfo `json:"control_plane_lighthouse,omitempty"`

	// Lighthouses is the list of node-based lighthouses in the cluster
	Lighthouses []LighthouseInfo `json:"lighthouses"`

	// Relays is the list of relay nodes in the cluster
	Relays []RelayInfo `json:"relays"`
}

// ControlPlaneLighthouseInfo represents control plane lighthouse configuration.
type ControlPlaneLighthouseInfo struct {
	// Enabled indicates whether control plane lighthouse is active
	Enabled bool `json:"enabled"`

	// Addresses is the list of all control plane instance addresses
	// These can be used as lighthouse addresses by Nebula nodes
	Addresses []string `json:"addresses"`

	// Port is the UDP port for lighthouse traffic
	Port int `json:"port"`
}

// LighthouseInfo represents a node configured as a lighthouse.
type LighthouseInfo struct {
	// NodeID is the UUID of the lighthouse node
	NodeID string `json:"node_id"`

	// Name is the human-readable node name
	Name string `json:"name"`

	// PublicIP is the public IP address for this lighthouse
	PublicIP string `json:"public_ip"`

	// Port is the UDP port this lighthouse listens on
	Port int `json:"port"`

	// IsRelay indicates if this node is also a relay
	IsRelay bool `json:"is_relay"`
}

// RelayInfo represents a node configured as a relay.
type RelayInfo struct {
	// NodeID is the UUID of the relay node
	NodeID string `json:"node_id"`

	// Name is the human-readable node name
	Name string `json:"name"`

	// IsLighthouse indicates if this node is also a lighthouse
	IsLighthouse bool `json:"is_lighthouse"`
}

// NodeRoutes represents the routes advertised by a single node.
type NodeRoutes struct {
	// NodeID is the UUID of the node
	NodeID string `json:"node_id"`

	// Name is the human-readable node name
	Name string `json:"name"`

	// Routes is the list of CIDR strings this node advertises
	Routes []string `json:"routes"`

	// UpdatedAt is the timestamp when routes were last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// ClusterRoutesResponse represents the response for listing all routes in a cluster.
type ClusterRoutesResponse struct {
	// ClusterID is the UUID of the cluster
	ClusterID string `json:"cluster_id"`

	// Nodes is the list of nodes with their advertised routes
	Nodes []NodeRoutes `json:"nodes"`
}

// ClusterTokenRotateResponse represents the response after rotating a cluster token.
type ClusterTokenRotateResponse struct {
	// ClusterID is the UUID of the cluster
	ClusterID string `json:"cluster_id"`

	// ClusterToken is the new shared secret for the cluster
	// The old token is immediately invalidated
	// All nodes must be updated with this new token out-of-band
	ClusterToken string `json:"cluster_token"`

	// RotatedAt is the timestamp when the token was rotated
	RotatedAt time.Time `json:"rotated_at"`
}
