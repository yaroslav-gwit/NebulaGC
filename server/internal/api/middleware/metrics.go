package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"nebulagc.io/server/internal/metrics"
)

// MetricsMiddleware creates a middleware that collects Prometheus metrics for HTTP requests.
//
// This middleware:
// - Tracks request count by method, path, and status code
// - Measures request duration in seconds
// - Measures response size in bytes
// - Tracks in-flight requests
//
// The middleware should be added early in the middleware chain to capture
// accurate timing and ensure all requests are counted.
//
// Returns:
//   - Gin middleware handler function
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Increment in-flight requests
		metrics.HTTPRequestsInFlight.Inc()
		defer metrics.HTTPRequestsInFlight.Dec()

		// Record start time
		start := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start).Seconds()

		// Get request details
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path // Fallback for unmatched routes
		}
		status := strconv.Itoa(c.Writer.Status())
		responseSize := float64(c.Writer.Size())

		// Record metrics
		metrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)

		// Only record response size if we have a valid size
		if responseSize >= 0 {
			metrics.HTTPResponseSize.WithLabelValues(method, path).Observe(responseSize)
		}
	}
}
