package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
	"nebulagc.io/models"
	"nebulagc.io/pkg/token"
)

// TopologyService handles topology management including routes, lighthouses, and relays.
type TopologyService struct {
	db     *sql.DB
	logger *zap.Logger
	secret string // HMAC secret for token rotation
}

// NewTopologyService creates a new topology service.
//
// Parameters:
//   - db: Database connection
//   - logger: Zap logger
//   - secret: HMAC secret for cluster token generation
//
// Returns:
//   - Configured TopologyService
func NewTopologyService(db *sql.DB, logger *zap.Logger, secret string) *TopologyService {
	return &TopologyService{
		db:     db,
		logger: logger,
		secret: secret,
	}
}

// UpdateRoutes updates the advertised routes for a node.
//
// Routes are validated as CIDR notation. An empty array clears all routes.
// Updates bump the cluster config version.
//
// Parameters:
//   - nodeID: Node UUID
//   - routes: Array of CIDR strings (e.g., ["10.0.1.0/24"])
//
// Returns:
//   - Error if validation fails or update fails
func (s *TopologyService) UpdateRoutes(nodeID string, routes []string) error {
	// Validate all routes
	for _, route := range routes {
		if err := validateCIDR(route); err != nil {
			s.logger.Warn("Invalid CIDR in route update",
				zap.String("node_id", nodeID),
				zap.String("route", route),
				zap.Error(err))
			return fmt.Errorf("%w: %s", models.ErrInvalidCIDR, route)
		}
	}

	// Marshal routes to JSON
	var routesJSON string
	if len(routes) > 0 {
		data, err := json.Marshal(routes)
		if err != nil {
			return fmt.Errorf("failed to marshal routes: %w", err)
		}
		routesJSON = string(data)
	}

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Get cluster ID for version bump
	var clusterID string
	err = tx.QueryRow(`SELECT cluster_id FROM nodes WHERE id = ?`, nodeID).Scan(&clusterID)
	if err == sql.ErrNoRows {
		return models.ErrNodeNotFound
	} else if err != nil {
		return fmt.Errorf("failed to get cluster ID: %w", err)
	}

	// Update routes
	now := time.Now().Unix()
	result, err := tx.Exec(`
		UPDATE nodes
		SET routes = ?, routes_updated_at = ?
		WHERE id = ?
	`, routesJSON, now, nodeID)
	if err != nil {
		return fmt.Errorf("failed to update routes: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return models.ErrNodeNotFound
	}

	// Bump cluster config version
	_, err = tx.Exec(`
		UPDATE clusters
		SET config_version = config_version + 1
		WHERE id = ?
	`, clusterID)
	if err != nil {
		return fmt.Errorf("failed to bump config version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Updated node routes",
		zap.String("node_id", nodeID),
		zap.Int("route_count", len(routes)))

	return nil
}

// GetNodeRoutes returns the advertised routes for a specific node.
//
// Parameters:
//   - nodeID: Node UUID
//
// Returns:
//   - Array of CIDR strings
//   - Error if node not found
func (s *TopologyService) GetNodeRoutes(nodeID string) ([]string, error) {
	var routesJSON sql.NullString
	err := s.db.QueryRow(`SELECT routes FROM nodes WHERE id = ?`, nodeID).Scan(&routesJSON)
	if err == sql.ErrNoRows {
		return nil, models.ErrNodeNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to get routes: %w", err)
	}

	if !routesJSON.Valid || routesJSON.String == "" {
		return []string{}, nil
	}

	var routes []string
	if err := json.Unmarshal([]byte(routesJSON.String), &routes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal routes: %w", err)
	}

	return routes, nil
}

// GetClusterRoutes returns all routes advertised in a cluster.
//
// Parameters:
//   - clusterID: Cluster UUID
//
// Returns:
//   - Map of node ID to routes array
//   - Error if query fails
func (s *TopologyService) GetClusterRoutes(clusterID string) (map[string][]string, error) {
	rows, err := s.db.Query(`
		SELECT id, routes
		FROM nodes
		WHERE cluster_id = ? AND routes IS NOT NULL AND routes != ''
	`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to query routes: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var nodeID string
		var routesJSON string
		if err := rows.Scan(&nodeID, &routesJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var routes []string
		if err := json.Unmarshal([]byte(routesJSON), &routes); err != nil {
			s.logger.Warn("Failed to unmarshal routes for node",
				zap.String("node_id", nodeID),
				zap.Error(err))
			continue
		}

		result[nodeID] = routes
	}

	return result, nil
}

// SetLighthouse assigns lighthouse status to a node.
//
// Parameters:
//   - clusterID: Cluster UUID
//   - nodeID: Node UUID
//   - publicIP: Public IP address (required)
//   - port: UDP port (0 = use cluster default)
//
// Returns:
//   - Error if node not found or update fails
func (s *TopologyService) SetLighthouse(clusterID, nodeID, publicIP string, port int) error {
	// Validate public IP
	if net.ParseIP(publicIP) == nil {
		return fmt.Errorf("%w: invalid IP address", models.ErrInvalidRequest)
	}

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Update node
	now := time.Now().Unix()
	result, err := tx.Exec(`
		UPDATE nodes
		SET is_lighthouse = 1,
		    lighthouse_public_ip = ?,
		    lighthouse_port = ?,
		    lighthouse_relay_updated_at = ?
		WHERE id = ? AND cluster_id = ?
	`, publicIP, port, now, nodeID, clusterID)
	if err != nil {
		return fmt.Errorf("failed to set lighthouse: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return models.ErrNodeNotFound
	}

	// Bump cluster config version
	_, err = tx.Exec(`
		UPDATE clusters
		SET config_version = config_version + 1
		WHERE id = ?
	`, clusterID)
	if err != nil {
		return fmt.Errorf("failed to bump config version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Set lighthouse status",
		zap.String("node_id", nodeID),
		zap.String("public_ip", publicIP),
		zap.Int("port", port))

	return nil
}

// UnsetLighthouse removes lighthouse status from a node.
//
// Parameters:
//   - clusterID: Cluster UUID
//   - nodeID: Node UUID
//
// Returns:
//   - Error if node not found or update fails
func (s *TopologyService) UnsetLighthouse(clusterID, nodeID string) error {
	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Update node
	now := time.Now().Unix()
	result, err := tx.Exec(`
		UPDATE nodes
		SET is_lighthouse = 0,
		    lighthouse_public_ip = NULL,
		    lighthouse_port = NULL,
		    lighthouse_relay_updated_at = ?
		WHERE id = ? AND cluster_id = ?
	`, now, nodeID, clusterID)
	if err != nil {
		return fmt.Errorf("failed to unset lighthouse: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return models.ErrNodeNotFound
	}

	// Bump cluster config version
	_, err = tx.Exec(`
		UPDATE clusters
		SET config_version = config_version + 1
		WHERE id = ?
	`, clusterID)
	if err != nil {
		return fmt.Errorf("failed to bump config version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Unset lighthouse status", zap.String("node_id", nodeID))

	return nil
}

// SetRelay assigns relay status to a node.
//
// Parameters:
//   - clusterID: Cluster UUID
//   - nodeID: Node UUID
//
// Returns:
//   - Error if node not found or update fails
func (s *TopologyService) SetRelay(clusterID, nodeID string) error {
	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Update node
	now := time.Now().Unix()
	result, err := tx.Exec(`
		UPDATE nodes
		SET is_relay = 1,
		    lighthouse_relay_updated_at = ?
		WHERE id = ? AND cluster_id = ?
	`, now, nodeID, clusterID)
	if err != nil {
		return fmt.Errorf("failed to set relay: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return models.ErrNodeNotFound
	}

	// Bump cluster config version
	_, err = tx.Exec(`
		UPDATE clusters
		SET config_version = config_version + 1
		WHERE id = ?
	`, clusterID)
	if err != nil {
		return fmt.Errorf("failed to bump config version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Set relay status", zap.String("node_id", nodeID))

	return nil
}

// UnsetRelay removes relay status from a node.
//
// Parameters:
//   - clusterID: Cluster UUID
//   - nodeID: Node UUID
//
// Returns:
//   - Error if node not found or update fails
func (s *TopologyService) UnsetRelay(clusterID, nodeID string) error {
	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Update node
	now := time.Now().Unix()
	result, err := tx.Exec(`
		UPDATE nodes
		SET is_relay = 0,
		    lighthouse_relay_updated_at = ?
		WHERE id = ? AND cluster_id = ?
	`, now, nodeID, clusterID)
	if err != nil {
		return fmt.Errorf("failed to unset relay: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return models.ErrNodeNotFound
	}

	// Bump cluster config version
	_, err = tx.Exec(`
		UPDATE clusters
		SET config_version = config_version + 1
		WHERE id = ?
	`, clusterID)
	if err != nil {
		return fmt.Errorf("failed to bump config version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Unset relay status", zap.String("node_id", nodeID))

	return nil
}

// TopologyInfo holds information about cluster topology.
type TopologyInfo struct {
	// Lighthouses is the list of lighthouse nodes.
	Lighthouses []LighthouseInfo `json:"lighthouses"`

	// Relays is the list of relay nodes.
	Relays []RelayInfo `json:"relays"`

	// Routes is a map of node ID to advertised routes.
	Routes map[string][]string `json:"routes"`
}

// LighthouseInfo holds information about a lighthouse node.
type LighthouseInfo struct {
	// NodeID is the node's UUID.
	NodeID string `json:"node_id"`

	// Name is the node's name.
	Name string `json:"name"`

	// PublicIP is the lighthouse's public IP address.
	PublicIP string `json:"public_ip"`

	// Port is the UDP port.
	Port int `json:"port"`
}

// RelayInfo holds information about a relay node.
type RelayInfo struct {
	// NodeID is the node's UUID.
	NodeID string `json:"node_id"`

	// Name is the node's name.
	Name string `json:"name"`
}

// GetTopology returns the complete topology for a cluster.
//
// Parameters:
//   - clusterID: Cluster UUID
//
// Returns:
//   - TopologyInfo with lighthouses, relays, and routes
//   - Error if query fails
func (s *TopologyService) GetTopology(clusterID string) (*TopologyInfo, error) {
	topology := &TopologyInfo{
		Lighthouses: []LighthouseInfo{},
		Relays:      []RelayInfo{},
		Routes:      make(map[string][]string),
	}

	// Query all nodes
	rows, err := s.db.Query(`
		SELECT id, name, is_lighthouse, lighthouse_public_ip, lighthouse_port,
		       is_relay, routes
		FROM nodes
		WHERE cluster_id = ?
	`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var nodeID, name string
		var isLighthouse, isRelay int
		var publicIP sql.NullString
		var port sql.NullInt64
		var routesJSON sql.NullString

		if err := rows.Scan(&nodeID, &name, &isLighthouse, &publicIP, &port, &isRelay, &routesJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Add lighthouse
		if isLighthouse == 1 {
			topology.Lighthouses = append(topology.Lighthouses, LighthouseInfo{
				NodeID:   nodeID,
				Name:     name,
				PublicIP: publicIP.String,
				Port:     int(port.Int64),
			})
		}

		// Add relay
		if isRelay == 1 {
			topology.Relays = append(topology.Relays, RelayInfo{
				NodeID: nodeID,
				Name:   name,
			})
		}

		// Add routes
		if routesJSON.Valid && routesJSON.String != "" {
			var routes []string
			if err := json.Unmarshal([]byte(routesJSON.String), &routes); err != nil {
				s.logger.Warn("Failed to unmarshal routes",
					zap.String("node_id", nodeID),
					zap.Error(err))
			} else {
				topology.Routes[nodeID] = routes
			}
		}
	}

	return topology, nil
}

// RotateClusterToken generates a new cluster token and updates the hash.
//
// Parameters:
//   - clusterID: Cluster UUID
//
// Returns:
//   - New plaintext token (only time it's visible)
//   - Error if generation or update fails
func (s *TopologyService) RotateClusterToken(clusterID string) (string, error) {
	// Generate new token
	newToken, err := token.Generate()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Hash token
	hash := token.Hash(newToken, s.secret)

	// Update database
	result, err := s.db.Exec(`
		UPDATE clusters
		SET cluster_token_hash = ?
		WHERE id = ?
	`, hash, clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to update token: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return "", models.ErrClusterNotFound
	}

	s.logger.Info("Rotated cluster token", zap.String("cluster_id", clusterID))

	return newToken, nil
}

// validateCIDR checks if a string is valid CIDR notation.
func validateCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	return err
}
