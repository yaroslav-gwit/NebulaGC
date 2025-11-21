// Package api provides the REST API implementation for the NebulaGC control plane.
//
// This package implements the HTTP layer including routing, middleware, and handlers
// for all API endpoints. It uses Gin for HTTP handling and integrates with the
// authentication, database, and service layers.
package api

import (
	"github.com/gin-gonic/gin"
)

// Context keys for storing authenticated request information.
const (
	// ContextKeyTenantID stores the authenticated tenant ID.
	ContextKeyTenantID = "tenant_id"

	// ContextKeyClusterID stores the authenticated cluster ID.
	ContextKeyClusterID = "cluster_id"

	// ContextKeyNodeID stores the authenticated node ID.
	ContextKeyNodeID = "node_id"

	// ContextKeyRequestID stores the unique request ID for tracing.
	ContextKeyRequestID = "request_id"

	// ContextKeyIsAdmin indicates if the authenticated node has admin privileges.
	ContextKeyIsAdmin = "is_admin"
)

// GetTenantID retrieves the authenticated tenant ID from the request context.
// Returns an empty string if not authenticated or tenant ID not set.
func GetTenantID(c *gin.Context) string {
	if val, exists := c.Get(ContextKeyTenantID); exists {
		if tenantID, ok := val.(string); ok {
			return tenantID
		}
	}
	return ""
}

// GetClusterID retrieves the authenticated cluster ID from the request context.
// Returns an empty string if not authenticated or cluster ID not set.
func GetClusterID(c *gin.Context) string {
	if val, exists := c.Get(ContextKeyClusterID); exists {
		if clusterID, ok := val.(string); ok {
			return clusterID
		}
	}
	return ""
}

// GetNodeID retrieves the authenticated node ID from the request context.
// Returns an empty string if not authenticated or node ID not set.
func GetNodeID(c *gin.Context) string {
	if val, exists := c.Get(ContextKeyNodeID); exists {
		if nodeID, ok := val.(string); ok {
			return nodeID
		}
	}
	return ""
}

// GetRequestID retrieves the unique request ID from the request context.
// Returns an empty string if request ID not set.
func GetRequestID(c *gin.Context) string {
	if val, exists := c.Get(ContextKeyRequestID); exists {
		if requestID, ok := val.(string); ok {
			return requestID
		}
	}
	return ""
}

// IsAdmin checks if the authenticated node has admin privileges.
// Returns false if not authenticated or not an admin node.
func IsAdmin(c *gin.Context) bool {
	if val, exists := c.Get(ContextKeyIsAdmin); exists {
		if isAdmin, ok := val.(bool); ok {
			return isAdmin
		}
	}
	return false
}

// SetTenantID sets the authenticated tenant ID in the request context.
func SetTenantID(c *gin.Context, tenantID string) {
	c.Set(ContextKeyTenantID, tenantID)
}

// SetClusterID sets the authenticated cluster ID in the request context.
func SetClusterID(c *gin.Context, clusterID string) {
	c.Set(ContextKeyClusterID, clusterID)
}

// SetNodeID sets the authenticated node ID in the request context.
func SetNodeID(c *gin.Context, nodeID string) {
	c.Set(ContextKeyNodeID, nodeID)
}

// SetRequestID sets the unique request ID in the request context.
func SetRequestID(c *gin.Context, requestID string) {
	c.Set(ContextKeyRequestID, requestID)
}

// SetIsAdmin sets whether the authenticated node has admin privileges.
func SetIsAdmin(c *gin.Context, isAdmin bool) {
	c.Set(ContextKeyIsAdmin, isAdmin)
}
