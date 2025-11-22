package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"nebulagc.io/pkg/bundle"
	"nebulagc.io/server/internal/service"
)

// BundleHandler handles config bundle endpoints.
type BundleHandler struct {
	service *service.BundleService
}

// NewBundleHandler creates a new bundle handler.
//
// Parameters:
//   - service: Bundle service for business logic
//
// Returns:
//   - Configured BundleHandler
func NewBundleHandler(service *service.BundleService) *BundleHandler {
	return &BundleHandler{
		service: service,
	}
}

// GetVersion handles GET /api/v1/config/version
//
// Returns the current config version for the authenticated cluster.
//
// Response:
//
//	{
//	  "version": 42
//	}
func (h *BundleHandler) GetVersion(c *gin.Context) {
	clusterID := getClusterID(c)
	if clusterID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	version, err := h.service.GetCurrentVersion(clusterID)
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	respondSuccess(c, http.StatusOK, gin.H{
		"version": version,
	})
}

// DownloadBundle handles GET /api/v1/config/bundle
//
// Downloads the config bundle for the authenticated cluster.
// Supports conditional requests via If-None-Match header.
//
// Query Parameters:
//   - current_version: Client's current version (optional)
//
// Headers:
//   - If-None-Match: "v{version}" for conditional requests
//
// Returns:
//   - 200 with bundle data if update available
//   - 304 Not Modified if client has current version
func (h *BundleHandler) DownloadBundle(c *gin.Context) {
	clusterID := getClusterID(c)
	if clusterID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Check if client provided their current version
	var clientVersion int64
	if versionStr := c.Query("current_version"); versionStr != "" {
		v, err := strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid_version", "Invalid version parameter")
			return
		}
		clientVersion = v
	}

	// Also check If-None-Match header (format: "v123")
	if etag := c.GetHeader("If-None-Match"); etag != "" {
		var v int64
		if _, err := fmt.Sscanf(etag, "\"v%d\"", &v); err == nil {
			if v > clientVersion {
				clientVersion = v
			}
		}
	}

	// Check if client is up-to-date
	isCurrent, currentVersion, err := h.service.CheckVersion(clusterID, clientVersion)
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	if isCurrent && clientVersion > 0 {
		// Client has current version - return 304 Not Modified
		c.Header("ETag", fmt.Sprintf("\"v%d\"", currentVersion))
		c.Status(http.StatusNotModified)
		return
	}

	// Download bundle
	data, version, err := h.service.Download(clusterID, 0) // 0 = latest
	if err != nil {
		mapErrorToResponse(c, err)
		return
	}

	// Set headers
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"config-v%d.tar.gz\"", version))
	c.Header("ETag", fmt.Sprintf("\"v%d\"", version))
	c.Header("X-Config-Version", fmt.Sprintf("%d", version))

	// Send bundle
	c.Data(http.StatusOK, "application/gzip", data)
}

// UploadBundle handles POST /api/v1/config/bundle
//
// Uploads a new config bundle for the authenticated cluster.
// Requires admin node authentication.
//
// Request body: application/gzip (tar.gz bundle)
//
// Response:
//
//	{
//	  "version": 43,
//	  "message": "Bundle uploaded successfully"
//	}
func (h *BundleHandler) UploadBundle(c *gin.Context) {
	clusterID := getClusterID(c)
	if clusterID == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Check Content-Type
	contentType := c.GetHeader("Content-Type")
	if contentType != "application/gzip" && contentType != "application/x-gzip" {
		respondError(c, http.StatusBadRequest, "invalid_content_type",
			"Content-Type must be application/gzip")
		return
	}

	// Read request body with size limit
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, bundle.MaxBundleSize+1))
	if err != nil {
		respondError(c, http.StatusBadRequest, "read_error", "Failed to read request body")
		return
	}

	// Check size
	if len(data) > bundle.MaxBundleSize {
		respondError(c, http.StatusRequestEntityTooLarge, "bundle_too_large",
			"Bundle exceeds 10 MiB size limit")
		return
	}

	// Upload bundle
	version, err := h.service.Upload(clusterID, data)
	if err != nil {
		// Map bundle validation errors to appropriate HTTP responses
		switch err {
		case bundle.ErrBundleTooLarge:
			respondError(c, http.StatusRequestEntityTooLarge, "bundle_too_large", err.Error())
		case bundle.ErrInvalidFormat, bundle.ErrEmptyBundle:
			respondError(c, http.StatusBadRequest, "invalid_format", err.Error())
		case bundle.ErrMissingRequiredFile:
			respondError(c, http.StatusBadRequest, "missing_required_file", err.Error())
		case bundle.ErrInvalidYAML:
			respondError(c, http.StatusBadRequest, "invalid_yaml", err.Error())
		default:
			mapErrorToResponse(c, err)
		}
		return
	}

	respondSuccess(c, http.StatusOK, gin.H{
		"version": version,
		"message": "Bundle uploaded successfully",
	})
}
