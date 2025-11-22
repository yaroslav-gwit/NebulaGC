package sdk

import "errors"

// Common SDK errors that clients can check for specific error handling.
var (
	// ErrInvalidConfig indicates the client configuration is invalid or incomplete.
	ErrInvalidConfig = errors.New("invalid client configuration")

	// ErrNoBaseURLs indicates no control plane URLs were provided.
	ErrNoBaseURLs = errors.New("no base URLs provided for control plane")

	// ErrAllInstancesFailed indicates all control plane instances are unreachable.
	ErrAllInstancesFailed = errors.New("all control plane instances failed")

	// ErrNoMasterFound indicates no master instance could be discovered.
	ErrNoMasterFound = errors.New("no master instance found")

	// ErrUnauthorized indicates the provided credentials are invalid.
	ErrUnauthorized = errors.New("unauthorized: invalid credentials")

	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrRateLimited indicates the request was rate limited by the server.
	ErrRateLimited = errors.New("rate limit exceeded")

	// ErrServerError indicates an internal server error occurred.
	ErrServerError = errors.New("internal server error")

	// ErrBadRequest indicates the request was malformed or invalid.
	ErrBadRequest = errors.New("bad request")

	// ErrConflict indicates the request conflicts with existing state.
	ErrConflict = errors.New("conflict with existing resource")

	// ErrMissingAuth indicates required authentication credentials were not provided.
	ErrMissingAuth = errors.New("missing authentication credentials")
)
