// Package token provides secure token generation and validation for NebulaGC authentication.
//
// This package implements cryptographically secure token generation using crypto/rand,
// HMAC-SHA256 hashing for secure storage, and constant-time comparison for validation
// to prevent timing attacks.
//
// # Token Generation
//
// Tokens are generated using crypto/rand for cryptographic security:
//
//	token, err := token.Generate()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// token is a 44-character base64-URL-encoded string
//
// # Token Hashing
//
// Tokens are never stored in plaintext. Instead, they are hashed using HMAC-SHA256:
//
//	secret := os.Getenv("NEBULAGC_HMAC_SECRET")
//	hash := token.Hash(token, secret)
//	// Store hash in database, never store token
//
// # Token Validation
//
// Validation uses constant-time comparison to prevent timing attacks:
//
//	if token.Validate(providedToken, secret, storedHash) {
//	    // Authentication successful
//	} else {
//	    // Authentication failed
//	}
//
// # Security Properties
//
//   - Minimum 41 characters (enforced)
//   - 256 bits of entropy (default)
//   - Cryptographically secure random generation (crypto/rand)
//   - HMAC-SHA256 hashing with server secret
//   - Constant-time comparison (prevents timing attacks)
//   - Never logs token values (only hashes)
//
// # Usage in NebulaGC
//
// This package is used for two types of tokens:
//
//  1. Node tokens: Per-node authentication (unique to each node)
//  2. Cluster tokens: Shared secret for all nodes in a cluster
//
// Both token types use the same generation and validation mechanisms.
package token
