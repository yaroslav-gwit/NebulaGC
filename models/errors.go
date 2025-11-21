package models

import "errors"

// Common error types used throughout the NebulaGC application.
// These errors provide semantic meaning and enable consistent error handling
// across different layers (API, service, database).

var (
	// ErrNotFound indicates the requested resource does not exist.
	// HTTP equivalent: 404 Not Found
	ErrNotFound = errors.New("resource not found")

	// ErrClusterNotFound indicates the requested cluster does not exist.
	// HTTP equivalent: 404 Not Found
	ErrClusterNotFound = errors.New("cluster not found")

	// ErrTenantNotFound indicates the requested tenant does not exist.
	// HTTP equivalent: 404 Not Found
	ErrTenantNotFound = errors.New("tenant not found")

	// ErrNodeNotFound indicates the requested node does not exist.
	// HTTP equivalent: 404 Not Found
	ErrNodeNotFound = errors.New("node not found")

	// ErrBundleNotFound indicates the requested config bundle does not exist.
	// HTTP equivalent: 404 Not Found
	ErrBundleNotFound = errors.New("config bundle not found")

	// ErrReplicaNotFound indicates the requested replica does not exist.
	// HTTP equivalent: 404 Not Found
	ErrReplicaNotFound = errors.New("replica not found")

	// ErrUnauthorized indicates the request lacks valid authentication credentials.
	// HTTP equivalent: 401 Unauthorized
	ErrUnauthorized = errors.New("unauthorized")

	// ErrInvalidToken indicates the authentication token is malformed or invalid.
	// HTTP equivalent: 401 Unauthorized
	ErrInvalidToken = errors.New("invalid authentication token")

	// ErrInvalidNodeToken indicates the node token is incorrect.
	// HTTP equivalent: 401 Unauthorized
	ErrInvalidNodeToken = errors.New("invalid node token")

	// ErrInvalidClusterToken indicates the cluster token is incorrect.
	// HTTP equivalent: 401 Unauthorized
	ErrInvalidClusterToken = errors.New("invalid cluster token")

	// ErrForbidden indicates the authenticated user lacks permission for this operation.
	// HTTP equivalent: 403 Forbidden
	ErrForbidden = errors.New("forbidden")

	// ErrNotAdmin indicates the operation requires admin privileges.
	// HTTP equivalent: 403 Forbidden
	ErrNotAdmin = errors.New("operation requires admin privileges")

	// ErrInvalidRequest indicates the request body or parameters are invalid.
	// HTTP equivalent: 400 Bad Request
	ErrInvalidRequest = errors.New("invalid request")

	// ErrInvalidCIDR indicates a route CIDR string is malformed.
	// HTTP equivalent: 400 Bad Request
	ErrInvalidCIDR = errors.New("invalid CIDR notation")

	// ErrInvalidMTU indicates the MTU value is outside the valid range (1280-9000).
	// HTTP equivalent: 400 Bad Request
	ErrInvalidMTU = errors.New("MTU must be between 1280 and 9000 bytes")

	// ErrConflict indicates the resource already exists.
	// HTTP equivalent: 409 Conflict
	ErrConflict = errors.New("resource already exists")

	// ErrDuplicateName indicates a resource with this name already exists.
	// HTTP equivalent: 409 Conflict
	ErrDuplicateName = errors.New("resource with this name already exists")

	// ErrPayloadTooLarge indicates the request body exceeds size limits.
	// HTTP equivalent: 413 Payload Too Large
	ErrPayloadTooLarge = errors.New("payload too large")

	// ErrBundleTooLarge indicates the config bundle exceeds 10 MiB.
	// HTTP equivalent: 413 Payload Too Large
	ErrBundleTooLarge = errors.New("config bundle exceeds 10 MiB limit")

	// ErrRateLimitExceeded indicates too many requests from this client.
	// HTTP equivalent: 429 Too Many Requests
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrInternalError indicates an unexpected server-side error.
	// HTTP equivalent: 500 Internal Server Error
	ErrInternalError = errors.New("internal server error")

	// ErrDatabaseError indicates a database operation failed.
	// HTTP equivalent: 500 Internal Server Error
	ErrDatabaseError = errors.New("database error")

	// ErrReplicaReadOnly indicates a write operation was attempted on a read-only replica.
	// HTTP equivalent: 503 Service Unavailable
	ErrReplicaReadOnly = errors.New("this is a read-only replica, write to master")

	// ErrServiceUnavailable indicates the service is temporarily unavailable.
	// HTTP equivalent: 503 Service Unavailable
	ErrServiceUnavailable = errors.New("service unavailable")
)

// ErrorResponse represents a standardized API error response.
type ErrorResponse struct {
	// Error is the human-readable error message
	Error string `json:"error"`

	// Code is an optional error code for programmatic handling
	// Examples: "UNAUTHORIZED", "FORBIDDEN", "NOT_FOUND", "REPLICA_READ_ONLY"
	Code string `json:"code,omitempty"`
}

// SuccessResponse represents a generic success response for operations
// that don't return a specific resource.
type SuccessResponse struct {
	// Message is a human-readable success message
	Message string `json:"message"`
}

// HealthResponse represents the response for health check endpoints.
type HealthResponse struct {
	// Status indicates the service health ("ok" or "degraded")
	Status string `json:"status"`

	// Timestamp is the current server time
	Timestamp string `json:"timestamp,omitempty"`
}
