package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"nebulagc.io/server/internal/ratelimit"
)

// AdvancedRateLimitMiddleware provides enhanced rate limiting with Retry-After headers
// and different limit types.
type AdvancedRateLimitMiddleware struct {
	limiter *ratelimit.Limiter
}

// NewAdvancedRateLimitMiddleware creates a new advanced rate limit middleware.
func NewAdvancedRateLimitMiddleware(config ratelimit.Config) *AdvancedRateLimitMiddleware {
	return &AdvancedRateLimitMiddleware{
		limiter: ratelimit.NewLimiter(config),
	}
}

// RateLimitRequest applies rate limiting for general authenticated requests.
// This should be used after authentication middleware on node endpoints.
func (m *AdvancedRateLimitMiddleware) RateLimitRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get node ID from authenticated context
		nodeID, exists := c.Get("node_id")
		if !exists {
			// No node ID - allow request but this shouldn't happen after auth
			c.Next()
			return
		}

		identifier := nodeID.(string)
		key := ratelimit.BuildKey(identifier, ratelimit.LimitTypeRequest)

		allowed, retryAfter := m.limiter.Allow(key, ratelimit.LimitTypeRequest)
		if !allowed {
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Rate limit exceeded for requests",
				"retry_after": retryAfter,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitBundleUpload applies rate limiting for config bundle uploads.
// This should be used on bundle upload endpoints after cluster auth.
func (m *AdvancedRateLimitMiddleware) RateLimitBundleUpload() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get cluster ID from authenticated context
		clusterID, exists := c.Get("cluster_id")
		if !exists {
			// No cluster ID - allow request but this shouldn't happen after auth
			c.Next()
			return
		}

		identifier := clusterID.(string)
		key := ratelimit.BuildKey(identifier, ratelimit.LimitTypeBundleUpload)

		allowed, retryAfter := m.limiter.Allow(key, ratelimit.LimitTypeBundleUpload)
		if !allowed {
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Rate limit exceeded for bundle uploads",
				"retry_after": retryAfter,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitHealthCheck applies rate limiting for unauthenticated health check requests.
// This should be used on public health endpoints.
func (m *AdvancedRateLimitMiddleware) RateLimitHealthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := ratelimit.BuildKey(ip, ratelimit.LimitTypeHealthCheck)

		allowed, retryAfter := m.limiter.Allow(key, ratelimit.LimitTypeHealthCheck)
		if !allowed {
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Rate limit exceeded for health checks",
				"retry_after": retryAfter,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitAuthFailure applies rate limiting for authentication failures.
// This should be called manually in auth middleware when authentication fails.
func (m *AdvancedRateLimitMiddleware) RateLimitAuthFailure(c *gin.Context) (allowed bool, retryAfter int) {
	ip := c.ClientIP()
	key := ratelimit.BuildKey(ip, ratelimit.LimitTypeAuthFailure)

	return m.limiter.Allow(key, ratelimit.LimitTypeAuthFailure)
}

// Stop gracefully stops the rate limiter.
func (m *AdvancedRateLimitMiddleware) Stop() {
	m.limiter.Stop()
}
