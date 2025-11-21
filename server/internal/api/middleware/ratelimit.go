package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter implements token bucket rate limiting.
//
// This struct manages rate limiters for different identifiers (IP addresses,
// node IDs, etc.) with automatic cleanup of expired limiters.
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	cleanup  time.Duration
}

// NewRateLimiter creates a new rate limiter.
//
// Parameters:
//   - rps: Requests per second allowed
//   - burst: Burst size (number of requests that can be made in quick succession)
//   - cleanup: How often to clean up expired limiters (e.g., 1 minute)
//
// Returns:
//   - Configured RateLimiter
func NewRateLimiter(rps float64, burst int, cleanup time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
		cleanup:  cleanup,
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// getLimiter gets or creates a rate limiter for the given identifier.
func (rl *RateLimiter) getLimiter(identifier string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[identifier]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[identifier] = limiter
	}

	return limiter
}

// cleanupLoop periodically removes limiters that haven't been used recently.
//
// This prevents memory leaks from accumulating limiters for one-time IPs.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()

		// Remove limiters that have full tokens (haven't been used)
		for identifier, limiter := range rl.limiters {
			if limiter.Tokens() == float64(rl.burst) {
				delete(rl.limiters, identifier)
			}
		}

		rl.mu.Unlock()
	}
}

// allow checks if a request from the given identifier should be allowed.
func (rl *RateLimiter) allow(identifier string) bool {
	limiter := rl.getLimiter(identifier)
	return limiter.Allow()
}

// RateLimitByIP creates middleware that rate limits requests by client IP address.
//
// This provides basic protection against abuse by limiting how many requests
// a single IP can make per second. Use this for public endpoints.
//
// Parameters:
//   - rps: Requests per second per IP
//   - burst: Burst size per IP
//
// Returns:
//   - Gin middleware handler function
//
// Example:
//
//	router.Use(RateLimitByIP(10.0, 20)) // 10 req/s, burst of 20
func RateLimitByIP(rps float64, burst int) gin.HandlerFunc {
	limiter := NewRateLimiter(rps, burst, 1*time.Minute)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !limiter.allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate_limit_exceeded",
				"message": "Rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitByNode creates middleware that rate limits requests by authenticated node.
//
// This provides protection against individual nodes making excessive requests.
// Use this after authentication middleware on node-authenticated endpoints.
//
// Parameters:
//   - rps: Requests per second per node
//   - burst: Burst size per node
//
// Returns:
//   - Gin middleware handler function
//
// Example:
//
//	router.Use(RequireNodeToken(config))
//	router.Use(RateLimitByNode(50.0, 100)) // 50 req/s per node
func RateLimitByNode(rps float64, burst int) gin.HandlerFunc {
	limiter := NewRateLimiter(rps, burst, 5*time.Minute)

	return func(c *gin.Context) {
		// Get node ID from authenticated context
		nodeID, exists := c.Get("node_id")
		if !exists {
			// No node ID - shouldn't happen if used after auth middleware
			// Allow the request but log this unusual situation
			c.Next()
			return
		}

		identifier := nodeID.(string)

		if !limiter.allow(identifier) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate_limit_exceeded",
				"message": "Rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitByCluster creates middleware that rate limits requests by cluster token.
//
// This provides protection against a single cluster making excessive requests.
// Use this after cluster authentication middleware.
//
// Parameters:
//   - rps: Requests per second per cluster
//   - burst: Burst size per cluster
//
// Returns:
//   - Gin middleware handler function
//
// Example:
//
//	router.Use(RequireClusterToken(config))
//	router.Use(RateLimitByCluster(100.0, 200)) // 100 req/s per cluster
func RateLimitByCluster(rps float64, burst int) gin.HandlerFunc {
	limiter := NewRateLimiter(rps, burst, 5*time.Minute)

	return func(c *gin.Context) {
		// Get cluster ID from authenticated context
		clusterID, exists := c.Get("cluster_id")
		if !exists {
			// No cluster ID - shouldn't happen if used after auth middleware
			c.Next()
			return
		}

		identifier := clusterID.(string)

		if !limiter.allow(identifier) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate_limit_exceeded",
				"message": "Rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
