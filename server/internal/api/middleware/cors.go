package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS creates a middleware that handles Cross-Origin Resource Sharing.
//
// This middleware sets appropriate CORS headers to allow web clients to
// access the API from different origins. In production, you should configure
// this to only allow specific trusted origins.
//
// Parameters:
//   - allowOrigins: List of allowed origins (e.g., ["https://app.example.com"])
//     Use ["*"] to allow all origins (not recommended for production)
//
// Returns:
//   - Gin middleware handler function
func CORS(allowOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the origin from the request
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			// Set CORS headers
			if origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
			} else if len(allowOrigins) == 1 && allowOrigins[0] == "*" {
				c.Header("Access-Control-Allow-Origin", "*")
			}

			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-NebulaGC-Cluster-Token, X-NebulaGC-Node-Token")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400") // 24 hours

			// Handle preflight requests
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}
		}

		c.Next()
	}
}
