package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// WriteGuard creates middleware that blocks write operations on replica instances.
//
// This middleware:
// - Allows all GET and HEAD requests (read-only)
// - Blocks POST, PUT, DELETE on replicas (returns 503)
// - Provides master URL in response for client failover
// - Only allows writes on the master instance
//
// This implements the master/replica HA architecture where only one instance
// (the master) can perform write operations at a time.
//
// Parameters:
//   - isMaster: Function to determine if this instance is master and provide master address
//
// Returns:
//   - Gin middleware handler function
//
// Response on replica write attempt:
//
//	503 Service Unavailable
//	{
//	  "error": "not_master",
//	  "message": "This replica is not the master",
//	  "master_url": "https://master.example.com"
//	}
func WriteGuard(isMaster func() (bool, string, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method

		// Allow read operations on all replicas
		if method == http.MethodGet || method == http.MethodHead {
			c.Next()
			return
		}

		// Check if we're the master for write operations
		master, masterURL, err := isMaster()

		if err != nil {
			// Error checking master status - fail safe by rejecting write
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "master_check_failed",
				"message": "Unable to determine master status",
			})
			c.Abort()
			return
		}

		if !master {
			// We're not the master - reject write and provide master URL
			c.Header("X-NebulaGC-Master-URL", masterURL)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":      "not_master",
				"message":    "This replica is not the master",
				"master_url": masterURL,
			})
			c.Abort()
			return
		}

		// We're the master - allow the write
		c.Next()
	}
}
