package token

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"generate token 1"},
		{"generate token 2"},
		{"generate token 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := Generate()
			if err != nil {
				t.Errorf("Generate() error = %v", err)
				return
			}

			if len(token) < MinTokenLength {
				t.Errorf("Generate() token length = %d, want >= %d", len(token), MinTokenLength)
			}

			// Verify token is base64-URL encoded (only valid characters)
			for _, c := range token {
				if !isBase64URLChar(c) {
					t.Errorf("Generate() token contains invalid character: %c", c)
				}
			}
		})
	}

	// Test uniqueness
	token1, _ := Generate()
	token2, _ := Generate()
	if token1 == token2 {
		t.Error("Generate() produced duplicate tokens")
	}
}

func TestGenerateWithLength(t *testing.T) {
	tests := []struct {
		name      string
		numBytes  int
		wantErr   bool
		minLength int
	}{
		{
			name:      "default length",
			numBytes:  DefaultTokenBytes,
			wantErr:   false,
			minLength: MinTokenLength,
		},
		{
			name:      "longer token",
			numBytes:  48,
			wantErr:   false,
			minLength: 60,
		},
		{
			name:     "too short",
			numBytes: 16,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateWithLength(tt.numBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateWithLength() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(token) < tt.minLength {
				t.Errorf("GenerateWithLength() token length = %d, want >= %d", len(token), tt.minLength)
			}
		})
	}
}

func TestHash(t *testing.T) {
	tests := []struct {
		name   string
		token  string
		secret string
	}{
		{
			name:   "basic hash",
			token:  "test-token-value-123456789012345678901",
			secret: "test-secret-key",
		},
		{
			name:   "longer token",
			token:  "very-long-token-value-with-lots-of-entropy-12345678901234567890",
			secret: "another-secret-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := Hash(tt.token, tt.secret)

			// HMAC-SHA256 produces 32 bytes, hex-encoded = 64 characters
			if len(hash) != 64 {
				t.Errorf("Hash() length = %d, want 64", len(hash))
			}

			// Verify hash is hex-encoded (only 0-9a-f)
			for _, c := range hash {
				if !isHexChar(c) {
					t.Errorf("Hash() contains invalid hex character: %c", c)
				}
			}

			// Verify same input produces same hash
			hash2 := Hash(tt.token, tt.secret)
			if hash != hash2 {
				t.Error("Hash() not deterministic")
			}

			// Verify different secret produces different hash
			hash3 := Hash(tt.token, "different-secret")
			if hash == hash3 {
				t.Error("Hash() same for different secrets")
			}
		})
	}
}

func TestValidate(t *testing.T) {
	secret := "test-secret-key-for-validation"
	token := "valid-token-value-123456789012345678901"
	hash := Hash(token, secret)

	tests := []struct {
		name       string
		provided   string
		secret     string
		storedHash string
		want       bool
	}{
		{
			name:       "valid token",
			provided:   token,
			secret:     secret,
			storedHash: hash,
			want:       true,
		},
		{
			name:       "wrong token",
			provided:   "wrong-token-value-123456789012345678901",
			secret:     secret,
			storedHash: hash,
			want:       false,
		},
		{
			name:       "wrong secret",
			provided:   token,
			secret:     "wrong-secret",
			storedHash: hash,
			want:       false,
		},
		{
			name:       "wrong hash",
			provided:   token,
			secret:     secret,
			storedHash: "0000000000000000000000000000000000000000000000000000000000000000",
			want:       false,
		},
		{
			name:       "empty token",
			provided:   "",
			secret:     secret,
			storedHash: hash,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Validate(tt.provided, tt.secret, tt.storedHash); got != tt.want {
				t.Errorf("Validate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateLength(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid length",
			token:   strings.Repeat("x", MinTokenLength),
			wantErr: false,
		},
		{
			name:    "longer than minimum",
			token:   "this-is-a-very-long-token-with-lots-of-characters-much-more-than-41",
			wantErr: false,
		},
		{
			name:    "too short",
			token:   "short",
			wantErr: true,
		},
		{
			name:    "exactly minimum",
			token:   strings.Repeat("x", MinTokenLength),
			wantErr: false,
		},
		{
			name:    "one character too short",
			token:   strings.Repeat("x", MinTokenLength-1),
			wantErr: true,
		},
		{
			name:    "empty",
			token:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLength(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Benchmark tests for performance monitoring
func BenchmarkGenerate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Generate()
	}
}

func BenchmarkHash(b *testing.B) {
	token := "benchmark-token-value-123456789012345678901"
	secret := "benchmark-secret-key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Hash(token, secret)
	}
}

func BenchmarkValidate(b *testing.B) {
	token := "benchmark-token-value-123456789012345678901"
	secret := "benchmark-secret-key"
	hash := Hash(token, secret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(token, secret, hash)
	}
}

// Helper functions for tests
func isBase64URLChar(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '='
}

func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'f')
}
