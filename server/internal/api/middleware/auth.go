package middleware

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"nebulagc.io/pkg/token"
)

const (
	// HeaderClusterToken is the header name for cluster token authentication.
	HeaderClusterToken = "X-NebulaGC-Cluster-Token"

	// HeaderNodeToken is the header name for node token authentication.
	HeaderNodeToken = "X-NebulaGC-Node-Token"
)

// AuthConfig holds configuration for authentication middleware.
type AuthConfig struct {
	// DB is the database connection for looking up tokens.
	DB *sql.DB

	// Secret is the HMAC secret for token validation.
	Secret string
}

// respondAuthError sends an authentication error response.
//
// This uses a generic error message to prevent information disclosure
// that could aid attackers in token enumeration.
func respondAuthError(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"error":   "unauthorized",
		"message": "Authentication failed",
	})
	c.Abort()
}

// RequireClusterToken creates middleware that requires cluster token authentication.
//
// This middleware:
// - Extracts cluster token from X-NebulaGC-Cluster-Token header
// - Validates token length (minimum 41 characters)
// - Queries database for cluster by token hash
// - Validates token using constant-time comparison
// - Sets tenant_id and cluster_id in context on success
//
// Usage: For endpoints that require cluster-level authentication
// (e.g., topology management, cluster-wide operations)
//
// Parameters:
//   - config: Authentication configuration
//
// Returns:
//   - Gin middleware handler function
func RequireClusterToken(config *AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from header
		providedToken := c.GetHeader(HeaderClusterToken)
		if providedToken == "" {
			respondAuthError(c)
			return
		}

		// Validate token length
		if err := token.ValidateLength(providedToken); err != nil {
			respondAuthError(c)
			return
		}

		// Query database for cluster with this token hash
		var cluster struct {
			ID             string
			TenantID       string
			ClusterTokenHash string
		}

		query := `
			SELECT id, tenant_id, cluster_token_hash
			FROM clusters
			WHERE cluster_token_hash = ?
			LIMIT 1
		`

		// Hash the provided token for lookup
		providedHash := token.Hash(providedToken, config.Secret)

		err := config.DB.QueryRow(query, providedHash).Scan(
			&cluster.ID,
			&cluster.TenantID,
			&cluster.ClusterTokenHash,
		)

		if err == sql.ErrNoRows {
			// No cluster found with this token hash
			respondAuthError(c)
			return
		} else if err != nil {
			// Database error
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "An internal error occurred",
			})
			c.Abort()
			return
		}

		// Validate token using constant-time comparison
		if !token.Validate(providedToken, config.Secret, cluster.ClusterTokenHash) {
			respondAuthError(c)
			return
		}

		// Set authenticated context
		c.Set("tenant_id", cluster.TenantID)
		c.Set("cluster_id", cluster.ID)

		c.Next()
	}
}

// RequireNodeToken creates middleware that requires node token authentication.
//
// This middleware:
// - Extracts node token from X-NebulaGC-Node-Token header
// - Validates token length (minimum 41 characters)
// - Queries database for node by token hash
// - Validates token using constant-time comparison
// - Sets tenant_id, cluster_id, node_id, and is_admin in context on success
//
// Usage: For endpoints that require node-level authentication
// (e.g., config download, route updates, node-specific operations)
//
// Parameters:
//   - config: Authentication configuration
//
// Returns:
//   - Gin middleware handler function
func RequireNodeToken(config *AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from header
		providedToken := c.GetHeader(HeaderNodeToken)
		if providedToken == "" {
			respondAuthError(c)
			return
		}

		// Validate token length
		if err := token.ValidateLength(providedToken); err != nil {
			respondAuthError(c)
			return
		}

		// Query database for node with this token hash
		var node struct {
			ID        string
			TenantID  string
			ClusterID string
			TokenHash string
			IsAdmin   bool
		}

		query := `
			SELECT id, tenant_id, cluster_id, token_hash, is_admin
			FROM nodes
			WHERE token_hash = ?
			LIMIT 1
		`

		// Hash the provided token for lookup
		providedHash := token.Hash(providedToken, config.Secret)

		err := config.DB.QueryRow(query, providedHash).Scan(
			&node.ID,
			&node.TenantID,
			&node.ClusterID,
			&node.TokenHash,
			&node.IsAdmin,
		)

		if err == sql.ErrNoRows {
			// No node found with this token hash
			respondAuthError(c)
			return
		} else if err != nil {
			// Database error
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "An internal error occurred",
			})
			c.Abort()
			return
		}

		// Validate token using constant-time comparison
		if !token.Validate(providedToken, config.Secret, node.TokenHash) {
			respondAuthError(c)
			return
		}

		// Set authenticated context
		c.Set("tenant_id", node.TenantID)
		c.Set("cluster_id", node.ClusterID)
		c.Set("node_id", node.ID)
		c.Set("is_admin", node.IsAdmin)

		c.Next()
	}
}

// RequireAdminNode creates middleware that requires admin node authentication.
//
// This middleware should be used after RequireNodeToken for endpoints that
// require admin privileges (e.g., creating/deleting other nodes).
//
// Returns:
//   - Gin middleware handler function
func RequireAdminNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("is_admin")
		if !exists || !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "Admin privileges required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
