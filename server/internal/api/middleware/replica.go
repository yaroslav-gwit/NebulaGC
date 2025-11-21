package middleware

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ReplicaConfig holds configuration for replica middleware.
type ReplicaConfig struct {
	// DB is the database connection for checking master status.
	DB *sql.DB

	// InstanceID is this control plane instance's UUID.
	InstanceID string

	// HeartbeatThreshold is how long before a replica is considered stale.
	// Default: 30 seconds (3x heartbeat interval)
	HeartbeatThreshold time.Duration
}

// IsMaster checks if this instance is the master.
//
// The master is determined by:
// 1. Finding all healthy replicas (heartbeat within threshold)
// 2. Sorting by created_at (oldest first)
// 3. The oldest healthy replica is the master
//
// Returns:
// - isMaster: true if this instance is the master
// - masterURL: URL of the master instance (empty if we are master)
// - error: any error that occurred
func (rc *ReplicaConfig) IsMaster() (bool, string, error) {
	cutoff := time.Now().Add(-rc.HeartbeatThreshold)

	query := `
		SELECT instance_id, url
		FROM replicas
		WHERE last_heartbeat > ?
		ORDER BY created_at ASC
		LIMIT 1
	`

	var masterID, masterURL string
	err := rc.DB.QueryRow(query, cutoff).Scan(&masterID, &masterURL)

	if err == sql.ErrNoRows {
		// No healthy replicas found - this shouldn't happen but we'll
		// assume we're master to allow operations to continue
		return true, "", nil
	} else if err != nil {
		return false, "", err
	}

	return masterID == rc.InstanceID, masterURL, nil
}

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
//   - config: Replica configuration
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
func WriteGuard(config *ReplicaConfig) gin.HandlerFunc {
	// Set default heartbeat threshold if not specified
	if config.HeartbeatThreshold == 0 {
		config.HeartbeatThreshold = 30 * time.Second
	}

	return func(c *gin.Context) {
		method := c.Request.Method

		// Allow read operations on all replicas
		if method == http.MethodGet || method == http.MethodHead {
			c.Next()
			return
		}

		// Check if we're the master for write operations
		isMaster, masterURL, err := config.IsMaster()

		if err != nil {
			// Error checking master status - fail safe by rejecting write
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "master_check_failed",
				"message": "Unable to determine master status",
			})
			c.Abort()
			return
		}

		if !isMaster {
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
