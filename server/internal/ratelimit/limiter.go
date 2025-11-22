package ratelimit

import (
	"fmt"
	"sync"
	"time"
)

// LimitType represents the type of rate limit to apply.
type LimitType string

const (
	// LimitTypeAuthFailure is for authentication failure attempts per IP.
	LimitTypeAuthFailure LimitType = "auth_failure"

	// LimitTypeRequest is for general authenticated requests per node.
	LimitTypeRequest LimitType = "request"

	// LimitTypeBundleUpload is for config bundle uploads per cluster.
	LimitTypeBundleUpload LimitType = "bundle_upload"

	// LimitTypeHealthCheck is for unauthenticated health check requests per IP.
	LimitTypeHealthCheck LimitType = "health_check"
)

// Config holds the rate limiting configuration.
type Config struct {
	// AuthFailuresPerMin is the number of auth failures allowed per minute per IP.
	AuthFailuresPerMin int

	// AuthFailuresBlockMin is the block duration in minutes after exceeding auth failure limit.
	AuthFailuresBlockMin int

	// RequestsPerMin is the number of authenticated requests allowed per minute per node.
	RequestsPerMin int

	// BundleUploadsPerMin is the number of bundle uploads allowed per minute per cluster.
	BundleUploadsPerMin int

	// HealthChecksPerMin is the number of health checks allowed per minute per IP.
	HealthChecksPerMin int
}

// DefaultConfig returns the default rate limiting configuration.
func DefaultConfig() Config {
	return Config{
		AuthFailuresPerMin:   10,
		AuthFailuresBlockMin: 60,
		RequestsPerMin:       100,
		BundleUploadsPerMin:  10,
		HealthChecksPerMin:   30,
	}
}

// Limiter implements token bucket rate limiting with support for multiple limit types.
type Limiter struct {
	storage *Storage
	config  Config
	mu      sync.RWMutex
}

// NewLimiter creates a new rate limiter with the given configuration.
func NewLimiter(config Config) *Limiter {
	return &Limiter{
		storage: NewStorage(),
		config:  config,
	}
}

// Allow checks if a request should be allowed based on the rate limit.
// It returns true if allowed, false if rate limited, and the number of seconds to wait.
func (l *Limiter) Allow(key string, limitType LimitType) (allowed bool, retryAfter int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Get or create bucket
	bucket := l.storage.Get(key)
	if bucket == nil {
		bucket = l.createBucket(limitType)
		l.storage.Set(key, bucket)
	}

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(bucket.LastRefill).Seconds()
	bucket.Tokens += elapsed * bucket.RefillRate

	// Cap at capacity
	if bucket.Tokens > bucket.Capacity {
		bucket.Tokens = bucket.Capacity
	}

	bucket.LastRefill = now

	// Check if we have tokens available
	if bucket.Tokens >= 1.0 {
		bucket.Tokens -= 1.0
		l.storage.Set(key, bucket)
		return true, 0
	}

	// Calculate retry-after based on refill rate
	tokensNeeded := 1.0 - bucket.Tokens
	retrySeconds := int(tokensNeeded / bucket.RefillRate)
	if retrySeconds < 1 {
		retrySeconds = 1
	}

	return false, retrySeconds
}

// createBucket creates a new token bucket based on the limit type.
func (l *Limiter) createBucket(limitType LimitType) *Bucket {
	var capacity float64
	var refillRate float64

	switch limitType {
	case LimitTypeAuthFailure:
		// Auth failures: X per minute with burst capacity
		capacity = float64(l.config.AuthFailuresPerMin)
		refillRate = capacity / 60.0 // tokens per second

	case LimitTypeRequest:
		// General requests: X per minute with burst capacity
		capacity = float64(l.config.RequestsPerMin)
		refillRate = capacity / 60.0

	case LimitTypeBundleUpload:
		// Bundle uploads: X per minute with burst capacity
		capacity = float64(l.config.BundleUploadsPerMin)
		refillRate = capacity / 60.0

	case LimitTypeHealthCheck:
		// Health checks: X per minute with burst capacity
		capacity = float64(l.config.HealthChecksPerMin)
		refillRate = capacity / 60.0

	default:
		// Default to general request limits
		capacity = float64(l.config.RequestsPerMin)
		refillRate = capacity / 60.0
	}

	return &Bucket{
		Tokens:     capacity, // Start with full capacity
		LastRefill: time.Now(),
		Capacity:   capacity,
		RefillRate: refillRate,
	}
}

// BuildKey creates a rate limit key from identifier and limit type.
func BuildKey(identifier string, limitType LimitType) string {
	return fmt.Sprintf("%s:%s", limitType, identifier)
}

// Stop gracefully stops the limiter and cleans up resources.
func (l *Limiter) Stop() {
	l.storage.Stop()
}

// GetStorage returns the underlying storage (for testing).
func (l *Limiter) GetStorage() *Storage {
	return l.storage
}
