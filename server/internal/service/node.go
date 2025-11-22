package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"nebulagc.io/models"
	"nebulagc.io/pkg/token"
)

// NodeService provides operations for managing cluster nodes.
//
// This service encapsulates node CRUD, token rotation, and MTU updates while
// enforcing tenant/cluster scoping and admin-only restrictions at the call site.
type NodeService struct {
	db     *sql.DB
	logger *zap.Logger
	secret string
}

// NewNodeService creates a new NodeService.
//
// Parameters:
//   - db: Database connection
//   - logger: Zap logger for structured logging
//   - secret: HMAC secret for token hashing/validation
func NewNodeService(db *sql.DB, logger *zap.Logger, secret string) *NodeService {
	return &NodeService{
		db:     db,
		logger: logger,
		secret: secret,
	}
}

// CreateNode creates a new node within the provided tenant and cluster.
//
// Parameters:
//   - ctx: Request context for cancellation
//   - tenantID: Owning tenant ID
//   - clusterID: Owning cluster ID
//   - clusterToken: Raw cluster token (echoed back for convenience)
//   - req: Node creation request payload
//
// Returns:
//   - *models.NodeCredentials containing the new node ID and token
//   - error if validation fails or database operations fail
func (s *NodeService) CreateNode(ctx context.Context, tenantID, clusterID, clusterToken string, req *models.NodeCreateRequest) (*models.NodeCredentials, error) {
	if err := validateNodeName(req.Name); err != nil {
		return nil, err
	}
	if err := validateMTU(req.MTU); err != nil {
		return nil, err
	}

	if err := s.ensureClusterExists(ctx, tenantID, clusterID); err != nil {
		return nil, err
	}

	nodeID := uuid.New().String()
	nodeToken, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate node token: %w", err)
	}
	tokenHash := token.Hash(nodeToken, s.secret)

	mtu := req.MTU
	if mtu == 0 {
		mtu = 1300
	}

	insertQuery := `
		INSERT INTO nodes (
			id, tenant_id, cluster_id, name, is_admin, token_hash, mtu
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, insertQuery,
		nodeID, tenantID, clusterID, req.Name, boolToInt(req.IsAdmin), tokenHash, mtu,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return nil, models.ErrDuplicateName
		}
		return nil, fmt.Errorf("failed to insert node: %w", err)
	}

	if err := s.bumpConfigVersion(ctx, tenantID, clusterID); err != nil {
		return nil, err
	}

	return &models.NodeCredentials{
		NodeID:       nodeID,
		NodeToken:    nodeToken,
		ClusterToken: clusterToken,
		CreatedAt:    time.Now(),
	}, nil
}

// ListNodes returns a paginated list of nodes for the given tenant and cluster.
//
// Parameters:
//   - ctx: Request context
//   - tenantID: Tenant scope
//   - clusterID: Cluster scope
//   - page: Page number (1-based)
//   - pageSize: Items per page (clamped to 1..500)
func (s *NodeService) ListNodes(ctx context.Context, tenantID, clusterID string, page, pageSize int) (*models.NodeListResponse, error) {
	if err := s.ensureClusterExists(ctx, tenantID, clusterID); err != nil {
		return nil, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 500 {
		pageSize = 500
	}

	offset := (page - 1) * pageSize

	countQuery := `
		SELECT COUNT(*)
		FROM nodes
		WHERE tenant_id = ? AND cluster_id = ?
	`

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, tenantID, clusterID).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count nodes: %w", err)
	}

	listQuery := `
		SELECT id, name, is_admin, mtu, is_lighthouse, is_relay, routes, created_at
		FROM nodes
		WHERE tenant_id = ? AND cluster_id = ?
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.QueryContext(ctx, listQuery, tenantID, clusterID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []models.NodeSummary
	for rows.Next() {
		var n models.NodeSummary
		var routes sql.NullString
		if err := rows.Scan(&n.NodeID, &n.Name, &n.IsAdmin, &n.MTU, &n.IsLighthouse, &n.IsRelay, &routes, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}

		if routes.Valid && strings.TrimSpace(routes.String) != "" {
			var parsed []string
			if err := json.Unmarshal([]byte(routes.String), &parsed); err == nil {
				n.Routes = parsed
			}
		}

		// UpdatedAt not stored; mirror created_at for now
		n.UpdatedAt = n.CreatedAt
		nodes = append(nodes, n)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate nodes: %w", err)
	}

	return &models.NodeListResponse{
		ClusterID: clusterID,
		Nodes:     nodes,
		Total:     total,
		Page:      page,
		PerPage:   pageSize,
	}, nil
}

// UpdateMTU updates the MTU for a specific node (admin only).
//
// Parameters:
//   - ctx: Request context
//   - tenantID: Tenant scope
//   - clusterID: Cluster scope
//   - nodeID: Target node ID
//   - mtu: Desired MTU (validated 1280-9000)
func (s *NodeService) UpdateMTU(ctx context.Context, tenantID, clusterID, nodeID string, mtu int) (*models.NodeSummary, error) {
	if err := validateMTU(mtu); err != nil {
		return nil, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE nodes
		SET mtu = ?
		WHERE id = ? AND tenant_id = ? AND cluster_id = ?
	`, mtu, nodeID, tenantID, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to update MTU: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to check MTU update result: %w", err)
	}
	if rows == 0 {
		return nil, models.ErrNodeNotFound
	}

	if err := s.bumpConfigVersion(ctx, tenantID, clusterID); err != nil {
		return nil, err
	}

	return s.getNodeSummary(ctx, tenantID, clusterID, nodeID)
}

// RotateNodeToken generates a new token for the specified node (admin only).
//
// Parameters:
//   - ctx: Request context
//   - tenantID: Tenant scope
//   - clusterID: Cluster scope
//   - nodeID: Target node ID
func (s *NodeService) RotateNodeToken(ctx context.Context, tenantID, clusterID, nodeID string) (*models.NodeTokenRotateResponse, error) {
	newToken, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate node token: %w", err)
	}

	hash := token.Hash(newToken, s.secret)

	result, err := s.db.ExecContext(ctx, `
		UPDATE nodes
		SET token_hash = ?
		WHERE id = ? AND tenant_id = ? AND cluster_id = ?
	`, hash, nodeID, tenantID, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to rotate token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to check token rotation result: %w", err)
	}
	if rows == 0 {
		return nil, models.ErrNodeNotFound
	}

	if err := s.bumpConfigVersion(ctx, tenantID, clusterID); err != nil {
		return nil, err
	}

	now := time.Now()
	return &models.NodeTokenRotateResponse{
		NodeID:    nodeID,
		NodeToken: newToken,
		RotatedAt: now,
	}, nil
}

// DeleteNode removes a node (admin only).
//
// Parameters:
//   - ctx: Request context
//   - tenantID: Tenant scope
//   - clusterID: Cluster scope
//   - nodeID: Target node ID
func (s *NodeService) DeleteNode(ctx context.Context, tenantID, clusterID, nodeID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM nodes
		WHERE id = ? AND tenant_id = ? AND cluster_id = ?
	`, nodeID, tenantID, clusterID)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check delete result: %w", err)
	}
	if rows == 0 {
		return models.ErrNodeNotFound
	}

	return s.bumpConfigVersion(ctx, tenantID, clusterID)
}

func (s *NodeService) ensureClusterExists(ctx context.Context, tenantID, clusterID string) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM clusters WHERE id = ? AND tenant_id = ?
	`, clusterID, tenantID).Scan(&count); err != nil {
		return fmt.Errorf("failed to verify cluster: %w", err)
	}
	if count == 0 {
		return models.ErrClusterNotFound
	}
	return nil
}

func (s *NodeService) bumpConfigVersion(ctx context.Context, tenantID, clusterID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE clusters
		SET config_version = config_version + 1
		WHERE id = ? AND tenant_id = ?
	`, clusterID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to bump config version: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check version bump: %w", err)
	}
	if rows == 0 {
		return models.ErrClusterNotFound
	}
	return nil
}

func (s *NodeService) getNodeSummary(ctx context.Context, tenantID, clusterID, nodeID string) (*models.NodeSummary, error) {
	query := `
		SELECT id, name, is_admin, mtu, is_lighthouse, is_relay, routes, created_at
		FROM nodes
		WHERE id = ? AND tenant_id = ? AND cluster_id = ?
		LIMIT 1
	`

	var summary models.NodeSummary
	var routes sql.NullString
	if err := s.db.QueryRowContext(ctx, query, nodeID, tenantID, clusterID).Scan(
		&summary.NodeID,
		&summary.Name,
		&summary.IsAdmin,
		&summary.MTU,
		&summary.IsLighthouse,
		&summary.IsRelay,
		&routes,
		&summary.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrNodeNotFound
		}
		return nil, fmt.Errorf("failed to load node summary: %w", err)
	}

	if routes.Valid && strings.TrimSpace(routes.String) != "" {
		var parsed []string
		if err := json.Unmarshal([]byte(routes.String), &parsed); err == nil {
			summary.Routes = parsed
		}
	}

	summary.UpdatedAt = summary.CreatedAt
	return &summary, nil
}

func validateMTU(mtu int) error {
	if mtu == 0 {
		return nil
	}
	if mtu < 1280 || mtu > 9000 {
		return models.ErrInvalidMTU
	}
	return nil
}

func validateNodeName(name string) error {
	if len(strings.TrimSpace(name)) == 0 || len(name) > 255 {
		return models.ErrInvalidRequest
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	// SQLite constraint errors include "UNIQUE constraint failed"
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
