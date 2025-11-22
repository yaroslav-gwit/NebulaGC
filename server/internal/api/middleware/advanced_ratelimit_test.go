package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"nebulagc.io/server/internal/ratelimit"
)

func TestAdvancedRateLimitMiddleware_RateLimitRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := ratelimit.Config{
		RequestsPerMin: 2, // Very low limit for testing
	}
	middleware := NewAdvancedRateLimitMiddleware(config)
	defer middleware.Stop()

	// Create router with middleware
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Simulate authenticated context
		c.Set("node_id", "test-node")
		c.Next()
	})
	router.Use(middleware.RateLimitRequest())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First request should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("First request status = %d, want %d", w1.Code, http.StatusOK)
	}

	// Second request should succeed
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Second request status = %d, want %d", w2.Code, http.StatusOK)
	}

	// Third request should be rate limited
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("Third request status = %d, want %d", w3.Code, http.StatusTooManyRequests)
	}

	// Check Retry-After header
	retryAfter := w3.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header should be present")
	}
}

func TestAdvancedRateLimitMiddleware_RateLimitBundleUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := ratelimit.Config{
		BundleUploadsPerMin: 1, // Very low limit for testing
	}
	middleware := NewAdvancedRateLimitMiddleware(config)
	defer middleware.Stop()

	// Create router with middleware
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Simulate authenticated context
		c.Set("cluster_id", "test-cluster")
		c.Next()
	})
	router.Use(middleware.RateLimitBundleUpload())
	router.POST("/upload", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First upload should succeed
	req1 := httptest.NewRequest(http.MethodPost, "/upload", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("First upload status = %d, want %d", w1.Code, http.StatusOK)
	}

	// Second upload should be rate limited
	req2 := httptest.NewRequest(http.MethodPost, "/upload", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Second upload status = %d, want %d", w2.Code, http.StatusTooManyRequests)
	}

	// Check Retry-After header
	retryAfter := w2.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header should be present")
	}
}

func TestAdvancedRateLimitMiddleware_RateLimitHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := ratelimit.Config{
		HealthChecksPerMin: 3, // Low limit for testing
	}
	middleware := NewAdvancedRateLimitMiddleware(config)
	defer middleware.Stop()

	// Create router with middleware
	router := gin.New()
	router.Use(middleware.RateLimitHealthCheck())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.RemoteAddr = "192.168.1.1:12345" // Same IP
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// Fourth request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.168.1.1:12345" // Same IP
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Fourth request status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// Check Retry-After header
	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header should be present")
	}
}

func TestAdvancedRateLimitMiddleware_IndependentLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := ratelimit.Config{
		RequestsPerMin: 1,
	}
	middleware := NewAdvancedRateLimitMiddleware(config)
	defer middleware.Stop()

	// Create router with middleware
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Set node ID based on header
		nodeID := c.GetHeader("X-Node-ID")
		if nodeID != "" {
			c.Set("node_id", nodeID)
		}
		c.Next()
	})
	router.Use(middleware.RateLimitRequest())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Request from node 1 should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("X-Node-ID", "node-1")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Node 1 first request status = %d, want %d", w1.Code, http.StatusOK)
	}

	// Second request from node 1 should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Node-ID", "node-1")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Node 1 second request status = %d, want %d", w2.Code, http.StatusTooManyRequests)
	}

	// Request from node 2 should succeed (independent limit)
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.Header.Set("X-Node-ID", "node-2")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("Node 2 first request status = %d, want %d", w3.Code, http.StatusOK)
	}
}

func TestAdvancedRateLimitMiddleware_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := ratelimit.Config{
		RequestsPerMin: 1,
	}
	middleware := NewAdvancedRateLimitMiddleware(config)
	defer middleware.Stop()

	// Create router WITHOUT auth context
	router := gin.New()
	router.Use(middleware.RateLimitRequest())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Request without node_id should be allowed (middleware should skip)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Request without auth status = %d, want %d (should be allowed)", w.Code, http.StatusOK)
	}
}

func TestAdvancedRateLimitMiddleware_RateLimitAuthFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := ratelimit.Config{
		AuthFailuresPerMin: 2, // Very low limit for testing
	}
	middleware := NewAdvancedRateLimitMiddleware(config)
	defer middleware.Stop()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "192.168.1.100:12345"

	// First auth failure should be allowed
	allowed1, _ := middleware.RateLimitAuthFailure(c)
	if !allowed1 {
		t.Error("First auth failure should be allowed")
	}

	// Second auth failure should be allowed
	allowed2, _ := middleware.RateLimitAuthFailure(c)
	if !allowed2 {
		t.Error("Second auth failure should be allowed")
	}

	// Third auth failure should be rate limited
	allowed3, retryAfter := middleware.RateLimitAuthFailure(c)
	if allowed3 {
		t.Error("Third auth failure should be rate limited")
	}
	if retryAfter == 0 {
		t.Error("retryAfter should be > 0 when rate limited")
	}
}
