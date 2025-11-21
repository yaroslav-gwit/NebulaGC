// Package handlers provides HTTP handlers for the NebulaGC REST API.
//
// This package implements request handlers for all API endpoints including
// health checks, node management, config distribution, topology management,
// route management, and token rotation.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"nebulagc.io/models"
)

// ErrorResponse represents a standardized error response.
//
// All API errors are returned in this format to provide consistent
// error handling for clients.
type ErrorResponse struct {
	// Error is the error code (e.g., "unauthorized", "not_found").
	Error string `json:"error"`

	// Message is a human-readable error message.
	Message string `json:"message"`

	// RequestID is the unique request ID for tracing.
	RequestID string `json:"request_id,omitempty"`
}

// SuccessResponse represents a standardized success response with data.
//
// This is used for successful API responses that return data to the client.
type SuccessResponse struct {
	// Data contains the response payload.
	Data interface{} `json:"data,omitempty"`

	// Message is an optional success message.
	Message string `json:"message,omitempty"`
}

// respondError sends a standardized error response.
//
// This function ensures all error responses follow the same format and
// use generic error messages to prevent information disclosure.
//
// Parameters:
//   - c: Gin context
//   - statusCode: HTTP status code
//   - errorCode: Error code string (e.g., "unauthorized")
//   - message: Human-readable error message
func respondError(c *gin.Context, statusCode int, errorCode string, message string) {
	requestID := ""
	if val, exists := c.Get("request_id"); exists {
		if id, ok := val.(string); ok {
			requestID = id
		}
	}

	c.JSON(statusCode, ErrorResponse{
		Error:     errorCode,
		Message:   message,
		RequestID: requestID,
	})
}

// respondSuccess sends a standardized success response with data.
//
// Parameters:
//   - c: Gin context
//   - statusCode: HTTP status code
//   - data: Response data
func respondSuccess(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, SuccessResponse{
		Data: data,
	})
}

// respondSuccessWithMessage sends a standardized success response with a message.
//
// Parameters:
//   - c: Gin context
//   - statusCode: HTTP status code
//   - message: Success message
func respondSuccessWithMessage(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, SuccessResponse{
		Message: message,
	})
}

// mapErrorToResponse converts a models package error to an HTTP response.
//
// This function maps domain errors (from the models package) to appropriate
// HTTP status codes and error responses. It uses generic error messages to
// prevent information disclosure that could aid attackers.
//
// Parameters:
//   - c: Gin context
//   - err: Error from models package or other source
func mapErrorToResponse(c *gin.Context, err error) {
	switch err {
	// 404 Not Found errors
	case models.ErrNotFound, models.ErrClusterNotFound, models.ErrTenantNotFound,
		models.ErrNodeNotFound, models.ErrBundleNotFound, models.ErrReplicaNotFound:
		respondError(c, http.StatusNotFound, "not_found", "Resource not found")

	// 401 Unauthorized errors
	case models.ErrUnauthorized, models.ErrInvalidToken,
		models.ErrInvalidNodeToken, models.ErrInvalidClusterToken:
		// Generic message to prevent token enumeration
		respondError(c, http.StatusUnauthorized, "unauthorized", "Authentication failed")

	// 403 Forbidden errors
	case models.ErrForbidden, models.ErrNotAdmin:
		respondError(c, http.StatusForbidden, "forbidden", "Access denied")

	// 400 Bad Request errors
	case models.ErrInvalidRequest, models.ErrInvalidCIDR, models.ErrInvalidMTU:
		respondError(c, http.StatusBadRequest, "invalid_request", "Invalid request parameters")

	// 409 Conflict errors
	case models.ErrConflict, models.ErrDuplicateName:
		respondError(c, http.StatusConflict, "conflict", "Resource already exists")

	// 413 Payload Too Large errors
	case models.ErrPayloadTooLarge, models.ErrBundleTooLarge:
		respondError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "Payload exceeds size limit")

	// 429 Rate Limit errors
	case models.ErrRateLimitExceeded:
		respondError(c, http.StatusTooManyRequests, "rate_limit_exceeded", "Rate limit exceeded")

	// 500 Internal Server Error
	case models.ErrInternalError, models.ErrDatabaseError:
		respondError(c, http.StatusInternalServerError, "internal_error", "An internal error occurred")

	// 503 Service Unavailable errors
	case models.ErrReplicaReadOnly, models.ErrServiceUnavailable:
		respondError(c, http.StatusServiceUnavailable, "service_unavailable", "Service temporarily unavailable")

	default:
		// Unknown error - log internally but return generic message
		respondError(c, http.StatusInternalServerError, "internal_error", "An internal error occurred")
	}
}
