package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"nebulagc.io/models"
	"nebulagc.io/server/internal/service"
)

// NodeHandler handles node management endpoints.
type NodeHandler struct {
	service *service.NodeService
}

// NewNodeHandler creates a new NodeHandler.
func NewNodeHandler(service *service.NodeService) *NodeHandler {
	return &NodeHandler{service: service}
}

// CreateNode handles POST /api/v1/nodes to create a new node (admin only).
func (h *NodeHandler) CreateNode(c *gin.Context) {
	tenantID := getTenantID(c)
	clusterID := getClusterID(c)
	clusterToken := c.GetHeader("X-NebulaGC-Cluster-Token")

	var req models.NodeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapErrorToResponse(c, models.ErrInvalidRequest)
		return
	}

	creds, err := h.service.CreateNode(c.Request.Context(), tenantID, clusterID, clusterToken, &req)
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccess(c, http.StatusCreated, creds)
}

// ListNodes handles GET /api/v1/nodes to list cluster nodes (admin only).
func (h *NodeHandler) ListNodes(c *gin.Context) {
	tenantID := getTenantID(c)
	clusterID := getClusterID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	resp, err := h.service.ListNodes(c.Request.Context(), tenantID, clusterID, page, perPage)
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccess(c, http.StatusOK, resp)
}

// UpdateMTU handles PATCH /api/v1/nodes/:id/mtu to update MTU (admin only).
func (h *NodeHandler) UpdateMTU(c *gin.Context) {
	tenantID := getTenantID(c)
	clusterID := getClusterID(c)
	nodeID := c.Param("id")

	var req models.NodeMTUUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mapErrorToResponse(c, models.ErrInvalidRequest)
		return
	}

	summary, err := h.service.UpdateMTU(c.Request.Context(), tenantID, clusterID, nodeID, req.MTU)
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccess(c, http.StatusOK, summary)
}

// RotateNodeToken handles POST /api/v1/nodes/:id/token to rotate a node token (admin only).
func (h *NodeHandler) RotateNodeToken(c *gin.Context) {
	tenantID := getTenantID(c)
	clusterID := getClusterID(c)
	nodeID := c.Param("id")

	resp, err := h.service.RotateNodeToken(c.Request.Context(), tenantID, clusterID, nodeID)
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccess(c, http.StatusOK, resp)
}

// DeleteNode handles DELETE /api/v1/nodes/:id to remove a node (admin only).
func (h *NodeHandler) DeleteNode(c *gin.Context) {
	tenantID := getTenantID(c)
	clusterID := getClusterID(c)
	nodeID := c.Param("id")

	if err := h.service.DeleteNode(c.Request.Context(), tenantID, clusterID, nodeID); err != nil {
		mapErrorToResponse(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func getTenantID(c *gin.Context) string {
	if val, exists := c.Get("tenant_id"); exists {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}

func getClusterID(c *gin.Context) string {
	if val, exists := c.Get("cluster_id"); exists {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}
