// Package token provides cryptographically secure token generation and validation
// for the NebulaGC authentication system.
//
// All tokens are generated using crypto/rand for cryptographic security and are
// hashed using HMAC-SHA256 before storage. The package enforces a minimum token
// length of 41 characters to ensure sufficient entropy.
package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const (
	// MinTokenLength is the minimum required length for all tokens.
	// This ensures sufficient entropy for security (41 chars = ~246 bits when base64-encoded).
	MinTokenLength = 41

	// DefaultTokenBytes is the number of random bytes to generate for tokens.
	// 32 bytes = 256 bits of entropy, which base64-encodes to 44 characters.
	DefaultTokenBytes = 32
)

// Generate creates a cryptographically secure random token suitable for authentication.
// The token is base64-URL-encoded and will be at least MinTokenLength characters.
//
// Returns:
//   - string: A base64-URL-encoded token (44 characters when using DefaultTokenBytes)
//   - error: An error if random number generation fails
//
// Example:
//
//	token, err := token.Generate()
//	if err != nil {
//	    return fmt.Errorf("failed to generate token: %w", err)
//	}
//	// token is now a 44-character string suitable for authentication
func Generate() (string, error) {
	return GenerateWithLength(DefaultTokenBytes)
}

// GenerateWithLength creates a cryptographically secure random token of specified byte length.
// The resulting base64-encoded token will be longer than the input byte length.
//
// Parameters:
//   - numBytes: Number of random bytes to generate (minimum 32 for security)
//
// Returns:
//   - string: A base64-URL-encoded token
//   - error: An error if random number generation fails or numBytes is too small
//
// Example:
//
//	token, err := token.GenerateWithLength(48) // 48 bytes = 288 bits
//	if err != nil {
//	    return fmt.Errorf("failed to generate token: %w", err)
//	}
func GenerateWithLength(numBytes int) (string, error) {
	if numBytes < DefaultTokenBytes {
		return "", fmt.Errorf("token length must be at least %d bytes", DefaultTokenBytes)
	}

	// Generate cryptographically secure random bytes
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to base64-URL for safe string representation
	token := base64.URLEncoding.EncodeToString(b)

	// Verify length meets minimum requirement
	if len(token) < MinTokenLength {
		return "", fmt.Errorf("generated token too short: got %d, need %d", len(token), MinTokenLength)
	}

	return token, nil
}

// Hash produces an HMAC-SHA256 hash of the token using the provided secret.
// The hash is returned as a hex-encoded string suitable for database storage.
//
// This function is used to securely store tokens in the database. The original token
// is never stored - only its HMAC hash. This prevents token disclosure even if the
// database is compromised.
//
// Parameters:
//   - token: The plaintext token to hash
//   - secret: The server-side secret key used for HMAC (from NEBULAGC_HMAC_SECRET)
//
// Returns:
//   - string: Hex-encoded HMAC-SHA256 hash (64 characters)
//
// Example:
//
//	secret := os.Getenv("NEBULAGC_HMAC_SECRET")
//	token := "generated-token-value"
//	hash := token.Hash(token, secret)
//	// Store hash in database, never store token
func Hash(token, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// Validate compares a provided token against a stored hash using constant-time comparison.
// This prevents timing attacks that could be used to determine valid token values.
//
// The function first hashes the provided token with the secret, then uses hmac.Equal
// for constant-time comparison. This ensures that token validation always takes the
// same amount of time regardless of whether the token is correct or not.
//
// Parameters:
//   - provided: The plaintext token provided in the authentication request
//   - secret: The server-side secret key (same one used for Hash)
//   - storedHash: The hex-encoded hash stored in the database
//
// Returns:
//   - bool: true if the provided token matches the stored hash, false otherwise
//
// Security Notes:
//   - Uses constant-time comparison to prevent timing attacks
//   - Never returns early based on comparison results
//   - Safe against timing-based side-channel attacks
//
// Example:
//
//	secret := os.Getenv("NEBULAGC_HMAC_SECRET")
//	providedToken := r.Header.Get("X-Nebula-Node-Token")
//	storedHash := node.TokenHash // from database
//
//	if token.Validate(providedToken, secret, storedHash) {
//	    // Authentication successful
//	} else {
//	    // Authentication failed
//	}
func Validate(provided, secret, storedHash string) bool {
	providedHash := Hash(provided, secret)
	// Use constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(providedHash), []byte(storedHash))
}

// ValidateLength checks if a token meets the minimum length requirement.
// This is a quick check that can be performed before attempting authentication.
//
// Parameters:
//   - token: The token to validate
//
// Returns:
//   - error: An error if the token is too short, nil if valid
//
// Example:
//
//	if err := token.ValidateLength(providedToken); err != nil {
//	    return fmt.Errorf("invalid token: %w", err)
//	}
func ValidateLength(token string) error {
	if len(token) < MinTokenLength {
		return fmt.Errorf("token too short: got %d characters, need at least %d", len(token), MinTokenLength)
	}
	return nil
}
