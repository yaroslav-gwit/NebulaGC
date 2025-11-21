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
)

// RequestLogger creates a middleware that logs all HTTP requests using Zap.
//
// This middleware:
// - Generates a unique request ID for tracing
// - Logs request method, path, status, duration, and client IP
// - Includes tenant/cluster/node IDs if authenticated
// - Uses structured logging for easy parsing and analysis
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
		c.Set("request_id", requestID)

		// Record start time
		start := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Build log fields
		fields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", duration),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		}

		// Add authenticated context if available
		if tenantID, exists := c.Get("tenant_id"); exists {
			if tid, ok := tenantID.(string); ok {
				fields = append(fields, zap.String("tenant_id", tid))
			}
		}
		if clusterID, exists := c.Get("cluster_id"); exists {
			if cid, ok := clusterID.(string); ok {
				fields = append(fields, zap.String("cluster_id", cid))
			}
		}
		if nodeID, exists := c.Get("node_id"); exists {
			if nid, ok := nodeID.(string); ok {
				fields = append(fields, zap.String("node_id", nid))
			}
		}

		// Add error if request failed
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", c.Errors.String()))
		}

		// Log based on status code
		statusCode := c.Writer.Status()
		if statusCode >= 500 {
			logger.Error("HTTP request", fields...)
		} else if statusCode >= 400 {
			logger.Warn("HTTP request", fields...)
		} else {
			logger.Info("HTTP request", fields...)
		}
	}
}
