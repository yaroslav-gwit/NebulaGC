package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter_Allow(t *testing.T) {
	config := Config{
		AuthFailuresPerMin:   10,
		AuthFailuresBlockMin: 60,
		RequestsPerMin:       100,
		BundleUploadsPerMin:  10,
		HealthChecksPerMin:   30,
	}

	limiter := NewLimiter(config)
	defer limiter.Stop()

	key := BuildKey("test-client", LimitTypeRequest)

	// First request should be allowed
	allowed, retryAfter := limiter.Allow(key, LimitTypeRequest)
	if !allowed {
		t.Error("First request should be allowed")
	}
	if retryAfter != 0 {
		t.Errorf("retryAfter = %d, want 0", retryAfter)
	}
}

func TestLimiter_RateLimitEnforcement(t *testing.T) {
	config := Config{
		RequestsPerMin: 5, // Very low limit for testing
	}

	limiter := NewLimiter(config)
	defer limiter.Stop()

	key := BuildKey("test-client", LimitTypeRequest)

	// Use up all tokens
	for i := 0; i < 5; i++ {
		allowed, _ := limiter.Allow(key, LimitTypeRequest)
		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// Next request should be rate limited
	allowed, retryAfter := limiter.Allow(key, LimitTypeRequest)
	if allowed {
		t.Error("Request should be rate limited after exhausting tokens")
	}
	if retryAfter == 0 {
		t.Error("retryAfter should be > 0 when rate limited")
	}
}

func TestLimiter_TokenRefill(t *testing.T) {
	config := Config{
		RequestsPerMin: 60, // 1 token per second
	}

	limiter := NewLimiter(config)
	defer limiter.Stop()

	key := BuildKey("test-client", LimitTypeRequest)

	// Use one token
	allowed, _ := limiter.Allow(key, LimitTypeRequest)
	if !allowed {
		t.Fatal("First request should be allowed")
	}

	// Exhaust all tokens
	for i := 0; i < 60; i++ {
		limiter.Allow(key, LimitTypeRequest)
	}

	// Should be rate limited now
	allowed, _ = limiter.Allow(key, LimitTypeRequest)
	if allowed {
		t.Error("Should be rate limited after exhausting tokens")
	}

	// Wait for token refill (1 second = 1 token at 60 per minute)
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed again after refill
	allowed, retryAfter := limiter.Allow(key, LimitTypeRequest)
	if !allowed {
		t.Errorf("Request should be allowed after token refill, retryAfter=%d", retryAfter)
	}
}

func TestLimiter_DifferentLimitTypes(t *testing.T) {
	config := Config{
		AuthFailuresPerMin:  5,
		RequestsPerMin:      10,
		BundleUploadsPerMin: 3,
		HealthChecksPerMin:  20,
	}

	limiter := NewLimiter(config)
	defer limiter.Stop()

	tests := []struct {
		name      string
		limitType LimitType
		limit     int
	}{
		{"auth failures", LimitTypeAuthFailure, 5},
		{"requests", LimitTypeRequest, 10},
		{"bundle uploads", LimitTypeBundleUpload, 3},
		{"health checks", LimitTypeHealthCheck, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := BuildKey("test-"+tt.name, tt.limitType)

			// Should allow up to limit
			for i := 0; i < tt.limit; i++ {
				allowed, _ := limiter.Allow(key, tt.limitType)
				if !allowed {
					t.Errorf("Request %d/%d should be allowed", i+1, tt.limit)
				}
			}

			// Next request should be rate limited
			allowed, retryAfter := limiter.Allow(key, tt.limitType)
			if allowed {
				t.Error("Request should be rate limited after exhausting tokens")
			}
			if retryAfter == 0 {
				t.Error("retryAfter should be > 0")
			}
		})
	}
}

func TestLimiter_IndependentKeys(t *testing.T) {
	config := Config{
		RequestsPerMin: 2, // Very low limit
	}

	limiter := NewLimiter(config)
	defer limiter.Stop()

	key1 := BuildKey("client-1", LimitTypeRequest)
	key2 := BuildKey("client-2", LimitTypeRequest)

	// Exhaust tokens for key1
	limiter.Allow(key1, LimitTypeRequest)
	limiter.Allow(key1, LimitTypeRequest)

	// key1 should be rate limited
	allowed, _ := limiter.Allow(key1, LimitTypeRequest)
	if allowed {
		t.Error("key1 should be rate limited")
	}

	// key2 should still be allowed (independent bucket)
	allowed, _ = limiter.Allow(key2, LimitTypeRequest)
	if !allowed {
		t.Error("key2 should be allowed (independent bucket)")
	}
}

func TestBuildKey(t *testing.T) {
	tests := []struct {
		identifier string
		limitType  LimitType
		want       string
	}{
		{"192.168.1.1", LimitTypeAuthFailure, "auth_failure:192.168.1.1"},
		{"node-123", LimitTypeRequest, "request:node-123"},
		{"cluster-456", LimitTypeBundleUpload, "bundle_upload:cluster-456"},
		{"10.0.0.1", LimitTypeHealthCheck, "health_check:10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := BuildKey(tt.identifier, tt.limitType)
			if got != tt.want {
				t.Errorf("BuildKey() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLimiter_BurstCapacity(t *testing.T) {
	config := Config{
		RequestsPerMin: 60, // 60 tokens capacity
	}

	limiter := NewLimiter(config)
	defer limiter.Stop()

	key := BuildKey("test-burst", LimitTypeRequest)

	// Should handle burst of requests up to capacity
	successCount := 0
	for i := 0; i < 70; i++ {
		allowed, _ := limiter.Allow(key, LimitTypeRequest)
		if allowed {
			successCount++
		}
	}

	// Should have allowed exactly 60 requests (capacity)
	if successCount != 60 {
		t.Errorf("Burst allowed %d requests, want 60", successCount)
	}
}

func TestLimiter_RetryAfterCalculation(t *testing.T) {
	config := Config{
		RequestsPerMin: 60, // 1 token per second
	}

	limiter := NewLimiter(config)
	defer limiter.Stop()

	key := BuildKey("test-retry", LimitTypeRequest)

	// Exhaust all tokens
	for i := 0; i < 60; i++ {
		limiter.Allow(key, LimitTypeRequest)
	}

	// Should be rate limited with retry-after
	allowed, retryAfter := limiter.Allow(key, LimitTypeRequest)
	if allowed {
		t.Error("Should be rate limited")
	}

	// Retry-after should be approximately 1 second (60 tokens/min = 1 token/sec)
	if retryAfter < 1 || retryAfter > 2 {
		t.Errorf("retryAfter = %d, want ~1 second", retryAfter)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.AuthFailuresPerMin != 10 {
		t.Errorf("AuthFailuresPerMin = %d, want 10", config.AuthFailuresPerMin)
	}
	if config.AuthFailuresBlockMin != 60 {
		t.Errorf("AuthFailuresBlockMin = %d, want 60", config.AuthFailuresBlockMin)
	}
	if config.RequestsPerMin != 100 {
		t.Errorf("RequestsPerMin = %d, want 100", config.RequestsPerMin)
	}
	if config.BundleUploadsPerMin != 10 {
		t.Errorf("BundleUploadsPerMin = %d, want 10", config.BundleUploadsPerMin)
	}
	if config.HealthChecksPerMin != 30 {
		t.Errorf("HealthChecksPerMin = %d, want 30", config.HealthChecksPerMin)
	}
}
