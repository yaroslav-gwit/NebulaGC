package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"nebulagc.io/server/internal/service"
)

// TopologyHandler handles topology management endpoints.
type TopologyHandler struct {
	service *service.TopologyService
}

// NewTopologyHandler creates a new topology handler.
//
// Parameters:
//   - service: Topology service for business logic
//
// Returns:
//   - Configured TopologyHandler
func NewTopologyHandler(service *service.TopologyService) *TopologyHandler {
	return &TopologyHandler{
		service: service,
	}
}

// UpdateRoutes handles PUT /api/v1/routes
//
// Allows any authenticated node to update its advertised routes.
// An empty array clears all routes.
//
// Request body:
//
//	{
//	  "routes": ["10.0.1.0/24", "10.0.2.0/24"]
//	}
//
// Response:
//
//	{
//	  "message": "Routes updated successfully"
//	}
func (h *TopologyHandler) UpdateRoutes(c *gin.Context) {
	nodeID := getNodeID(c)
	if nodeID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse request
	var req struct {
		Routes []string `json:"routes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Update routes
	if err := h.service.UpdateRoutes(nodeID, req.Routes); err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccessWithMessage(c, http.StatusOK, "Routes updated successfully")
}

// GetRoutes handles GET /api/v1/routes
//
// Returns the routes advertised by the authenticated node.
//
// Response:
//
//	{
//	  "routes": ["10.0.1.0/24", "10.0.2.0/24"]
//	}
func (h *TopologyHandler) GetRoutes(c *gin.Context) {
	nodeID := getNodeID(c)
	if nodeID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	routes, err := h.service.GetNodeRoutes(nodeID)
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccess(c, http.StatusOK, gin.H{
		"routes": routes,
	})
}

// GetClusterRoutes handles GET /api/v1/routes/cluster
//
// Returns all routes advertised in the cluster.
//
// Response:
//
//	{
//	  "routes": {
//	    "node-id-1": ["10.0.1.0/24"],
//	    "node-id-2": ["10.0.2.0/24", "10.0.3.0/24"]
//	  }
//	}
func (h *TopologyHandler) GetClusterRoutes(c *gin.Context) {
	clusterID := getClusterID(c)
	if clusterID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	routes, err := h.service.GetClusterRoutes(clusterID)
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccess(c, http.StatusOK, gin.H{
		"routes": routes,
	})
}

// AssignLighthouse handles POST /api/v1/topology/lighthouse
//
// Assigns lighthouse status to a node. Requires cluster token authentication.
//
// Request body:
//
//	{
//	  "node_id": "uuid",
//	  "public_ip": "203.0.113.1",
//	  "port": 4242
//	}
//
// Response:
//
//	{
//	  "message": "Lighthouse status assigned"
//	}
func (h *TopologyHandler) AssignLighthouse(c *gin.Context) {
	clusterID := getClusterID(c)
	if clusterID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse request
	var req struct {
		NodeID   string `json:"node_id" binding:"required"`
		PublicIP string `json:"public_ip" binding:"required"`
		Port     int    `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Assign lighthouse
	if err := h.service.SetLighthouse(clusterID, req.NodeID, req.PublicIP, req.Port); err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccessWithMessage(c, http.StatusOK, "Lighthouse status assigned")
}

// UnassignLighthouse handles DELETE /api/v1/topology/lighthouse/:node_id
//
// Removes lighthouse status from a node. Requires cluster token authentication.
//
// Response:
//
//	{
//	  "message": "Lighthouse status removed"
//	}
func (h *TopologyHandler) UnassignLighthouse(c *gin.Context) {
	clusterID := getClusterID(c)
	if clusterID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		respondError(c, http.StatusBadRequest, "invalid_request", "Missing node_id parameter")
		return
	}

	if err := h.service.UnsetLighthouse(clusterID, nodeID); err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccessWithMessage(c, http.StatusOK, "Lighthouse status removed")
}

// AssignRelay handles POST /api/v1/topology/relay
//
// Assigns relay status to a node. Requires cluster token authentication.
//
// Request body:
//
//	{
//	  "node_id": "uuid"
//	}
//
// Response:
//
//	{
//	  "message": "Relay status assigned"
//	}
func (h *TopologyHandler) AssignRelay(c *gin.Context) {
	clusterID := getClusterID(c)
	if clusterID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse request
	var req struct {
		NodeID string `json:"node_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Assign relay
	if err := h.service.SetRelay(clusterID, req.NodeID); err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccessWithMessage(c, http.StatusOK, "Relay status assigned")
}

// UnassignRelay handles DELETE /api/v1/topology/relay/:node_id
//
// Removes relay status from a node. Requires cluster token authentication.
//
// Response:
//
//	{
//	  "message": "Relay status removed"
//	}
func (h *TopologyHandler) UnassignRelay(c *gin.Context) {
	clusterID := getClusterID(c)
	if clusterID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		respondError(c, http.StatusBadRequest, "invalid_request", "Missing node_id parameter")
		return
	}

	if err := h.service.UnsetRelay(clusterID, nodeID); err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccessWithMessage(c, http.StatusOK, "Relay status removed")
}

// GetTopology handles GET /api/v1/topology
//
// Returns the complete topology for the cluster including lighthouses, relays, and routes.
//
// Response:
//
//	{
//	  "lighthouses": [
//	    {
//	      "node_id": "uuid",
//	      "name": "lighthouse-1",
//	      "public_ip": "203.0.113.1",
//	      "port": 4242
//	    }
//	  ],
//	  "relays": [
//	    {
//	      "node_id": "uuid",
//	      "name": "relay-1"
//	    }
//	  ],
//	  "routes": {
//	    "node-id-1": ["10.0.1.0/24"]
//	  }
//	}
func (h *TopologyHandler) GetTopology(c *gin.Context) {
	clusterID := getClusterID(c)
	if clusterID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	topology, err := h.service.GetTopology(clusterID)
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccess(c, http.StatusOK, topology)
}

// RotateClusterToken handles POST /api/v1/tokens/cluster/rotate
//
// Rotates the cluster token. Requires cluster token authentication.
// Returns the new plaintext token (only time it's visible).
//
// Response:
//
//	{
//	  "token": "new-cluster-token-string",
//	  "message": "Cluster token rotated successfully"
//	}
func (h *TopologyHandler) RotateClusterToken(c *gin.Context) {
	clusterID := getClusterID(c)
	if clusterID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	newToken, err := h.service.RotateClusterToken(clusterID)
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccess(c, http.StatusOK, gin.H{
		"token":   newToken,
		"message": "Cluster token rotated successfully",
	})
}

// getNodeID retrieves the authenticated node ID from the request context.
// Returns an empty string if not authenticated or node ID not set.
func getNodeID(c *gin.Context) string {
	if val, exists := c.Get("node_id"); exists {
		if nodeID, ok := val.(string); ok {
			return nodeID
		}
	}
	return ""
}
