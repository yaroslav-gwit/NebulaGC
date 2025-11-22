// Package middleware provides HTTP middleware for the NebulaGC REST API.
//
// This package implements authentication, rate limiting, replica write guards,
// request logging, and CORS handling for all API requests.
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"nebulagc.io/server/internal/logging"
)

// RequestLogger creates a middleware that logs all HTTP requests using structured logging.
//
// This middleware:
// - Generates a unique request ID for tracing
// - Creates a request-scoped logger with standard fields
// - Stores logger in both Gin and request context
// - Logs request start and completion with duration
// - Includes tenant/cluster/node IDs if authenticated
// - Uses structured logging with consistent field names
//
// Parameters:
//   - logger: Zap logger instance
//
// Returns:
//   - Gin middleware handler function
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate unique request ID
		requestID := uuid.New().String()

		// Record start time
		start := time.Now()

		// Extract authentication context if available
		tenantID := extractTenantID(c)
		clusterID := extractClusterID(c)
		nodeID := extractNodeID(c)

		// Create request-scoped logger with standard fields
		requestLogger := logger.With(
			zap.String(logging.FieldRequestID, requestID),
			zap.String(logging.FieldMethod, c.Request.Method),
			zap.String(logging.FieldPath, c.Request.URL.Path),
			zap.String(logging.FieldRemoteAddr, c.ClientIP()),
			zap.String(logging.FieldUserAgent, c.Request.UserAgent()),
		)

		// Add optional authenticated fields
		if tenantID != "" {
			requestLogger = requestLogger.With(zap.String(logging.FieldTenantID, tenantID))
		}
		if clusterID != "" {
			requestLogger = requestLogger.With(zap.String(logging.FieldClusterID, clusterID))
		}
		if nodeID != "" {
			requestLogger = requestLogger.With(zap.String(logging.FieldNodeID, nodeID))
		}

		// Store logger and request ID in Gin context
		c.Set("logger", requestLogger)
		c.Set("request_id", requestID)

		// Store in request context for non-gin code
		ctx := logging.WithLogger(c.Request.Context(), requestLogger)
		c.Request = c.Request.WithContext(ctx)

		// Log request start
		requestLogger.Info("request started")

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Get response status
		status := c.Writer.Status()

		// Build completion log fields
		fields := []zap.Field{
			zap.Int(logging.FieldStatusCode, status),
			zap.Duration(logging.FieldDuration, duration),
			zap.Int64("duration_ms", duration.Milliseconds()),
			zap.Int("response_size", c.Writer.Size()),
		}

		// Add error information if present
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String(logging.FieldError, c.Errors.String()))
		}

		// Log at appropriate level based on status code
		if status >= 500 {
			requestLogger.Error("request completed with server error", fields...)
		} else if status >= 400 {
			requestLogger.Warn("request completed with client error", fields...)
		} else {
			requestLogger.Info("request completed", fields...)
		}
	}
}

// extractTenantID attempts to extract tenant ID from the request context.
func extractTenantID(c *gin.Context) string {
	if tenantID, exists := c.Get("tenant_id"); exists {
		if id, ok := tenantID.(string); ok {
			return id
		}
	}
	return ""
}

// extractClusterID attempts to extract cluster ID from the request context.
func extractClusterID(c *gin.Context) string {
	if clusterID, exists := c.Get("cluster_id"); exists {
		if id, ok := clusterID.(string); ok {
			return id
		}
	}
	return ""
}

// extractNodeID attempts to extract node ID from the request context.
func extractNodeID(c *gin.Context) string {
	if nodeID, exists := c.Get("node_id"); exists {
		if id, ok := nodeID.(string); ok {
			return id
		}
	}
	return ""
}

// GetLogger retrieves the request-scoped logger from Gin context.
// Returns a no-op logger if not found.
func GetLogger(c *gin.Context) *zap.Logger {
	if logger, exists := c.Get("logger"); exists {
		if l, ok := logger.(*zap.Logger); ok {
			return l
		}
	}
	return zap.NewNop()
}

// GetRequestID retrieves the request ID from Gin context.
// Returns empty string if not found.
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}
