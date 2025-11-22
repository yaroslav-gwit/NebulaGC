// Package logging provides structured logging utilities for the NebulaGC server.
package logging

// Standard field names for consistent logging across the application.
const (
	// FieldTenantID is the unique identifier for a tenant.
	FieldTenantID = "tenant_id"

	// FieldClusterID is the unique identifier for a cluster.
	FieldClusterID = "cluster_id"

	// FieldNodeID is the unique identifier for a node.
	FieldNodeID = "node_id"

	// FieldRequestID is a unique identifier for each HTTP request.
	FieldRequestID = "request_id"

	// FieldDuration is the duration of an operation in milliseconds.
	FieldDuration = "duration_ms"

	// FieldStatusCode is the HTTP status code of a response.
	FieldStatusCode = "status_code"

	// FieldMethod is the HTTP method of a request.
	FieldMethod = "method"

	// FieldPath is the URL path of an HTTP request.
	FieldPath = "path"

	// FieldRemoteAddr is the client's remote address.
	FieldRemoteAddr = "remote_addr"

	// FieldUserAgent is the client's user agent string.
	FieldUserAgent = "user_agent"

	// FieldError is the error message or description.
	FieldError = "error"

	// FieldComponent identifies the component or service generating the log.
	FieldComponent = "component"

	// FieldOperation identifies the specific operation being performed.
	FieldOperation = "operation"
)
