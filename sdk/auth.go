package sdk

import "net/http"

// Authentication header constants matching the server expectations.
const (
	// HeaderNodeToken is the header name for node authentication.
	HeaderNodeToken = "X-NebulaGC-Node-Token"

	// HeaderClusterToken is the header name for cluster authentication.
	HeaderClusterToken = "X-NebulaGC-Cluster-Token"
)

// AuthType represents the type of authentication to use for a request.
type AuthType int

const (
	// AuthTypeNone indicates no authentication headers should be added.
	AuthTypeNone AuthType = iota

	// AuthTypeNode indicates node token authentication should be used.
	AuthTypeNode

	// AuthTypeCluster indicates cluster token authentication should be used.
	AuthTypeCluster
)

// addAuthHeaders adds the appropriate authentication headers to the request based on the auth type.
// Returns an error if the required credentials are not available.
func (c *Client) addAuthHeaders(req *http.Request, authType AuthType) error {
	switch authType {
	case AuthTypeNode:
		if c.NodeToken == "" {
			return ErrMissingAuth
		}
		req.Header.Set(HeaderNodeToken, c.NodeToken)
	case AuthTypeCluster:
		if c.ClusterToken == "" {
			return ErrMissingAuth
		}
		req.Header.Set(HeaderClusterToken, c.ClusterToken)
	case AuthTypeNone:
		// No authentication required
	}

	return nil
}
