package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"nebulagc.io/server/internal/logging"
)

func TestRequestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger, err := logging.NewDevelopmentLogger()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	router := gin.New()
	router.Use(RequestLogger(logger))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRequestLogger_WithAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger, _ := logging.NewDevelopmentLogger()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("tenant_id", "tenant123")
		c.Set("cluster_id", "cluster456")
		c.Set("node_id", "node789")
		c.Next()
	})
	router.Use(RequestLogger(logger))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRequestLogger_LoggerInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger, _ := logging.NewDevelopmentLogger()

	var contextLogger *zap.Logger

	router := gin.New()
	router.Use(RequestLogger(logger))
	router.GET("/test", func(c *gin.Context) {
		contextLogger = GetLogger(c)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if contextLogger == nil {
		t.Error("Expected logger to be stored in context")
	}

	// Logger should be usable
	contextLogger.Info("test message")
}

func TestRequestLogger_RequestIDGenerated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger, _ := logging.NewDevelopmentLogger()

	var requestID string

	router := gin.New()
	router.Use(RequestLogger(logger))
	router.GET("/test", func(c *gin.Context) {
		requestID = GetRequestID(c)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if requestID == "" {
		t.Error("Expected request ID to be generated")
	}

	if len(requestID) != 36 { // UUID length
		t.Errorf("Expected UUID format (36 chars), got %d chars", len(requestID))
	}
}

func TestRequestLogger_ErrorLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger, _ := logging.NewDevelopmentLogger()

	router := gin.New()
	router.Use(RequestLogger(logger))
	router.GET("/error", func(c *gin.Context) {
		c.Error(gin.Error{
			Err:  http.ErrBodyReadAfterClose,
			Type: gin.ErrorTypePrivate,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestRequestLogger_ClientError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger, _ := logging.NewDevelopmentLogger()

	router := gin.New()
	router.Use(RequestLogger(logger))
	router.GET("/bad-request", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
	})

	req := httptest.NewRequest(http.MethodGet, "/bad-request", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetLogger_NoLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	logger := GetLogger(c)
	if logger == nil {
		t.Error("Expected no-op logger when none exists")
	}

	// Should not panic
	logger.Info("test message")
}

func TestGetRequestID_NoRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	requestID := GetRequestID(c)
	if requestID != "" {
		t.Errorf("Expected empty request ID, got %s", requestID)
	}
}

func TestExtractTenantID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		setValue interface{}
		expected string
	}{
		{"valid string", "tenant123", "tenant123"},
		{"empty string", "", ""},
		{"invalid type", 123, ""},
		{"no value", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tt.setValue != nil {
				c.Set("tenant_id", tt.setValue)
			}

			result := extractTenantID(c)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractClusterID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		setValue interface{}
		expected string
	}{
		{"valid string", "cluster456", "cluster456"},
		{"empty string", "", ""},
		{"invalid type", 456, ""},
		{"no value", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tt.setValue != nil {
				c.Set("cluster_id", tt.setValue)
			}

			result := extractClusterID(c)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractNodeID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		setValue interface{}
		expected string
	}{
		{"valid string", "node789", "node789"},
		{"empty string", "", ""},
		{"invalid type", 789, ""},
		{"no value", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tt.setValue != nil {
				c.Set("node_id", tt.setValue)
			}

			result := extractNodeID(c)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRequestLogger_StandardFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger, _ := logging.NewDevelopmentLogger()

	router := gin.New()
	router.Use(RequestLogger(logger))
	router.POST("/test", func(c *gin.Context) {
		// Verify logger has standard fields
		reqLogger := GetLogger(c)
		if reqLogger == nil {
			t.Error("Expected logger in context")
		}

		// Verify request ID exists
		requestID := GetRequestID(c)
		if requestID == "" {
			t.Error("Expected request ID")
		}

		c.JSON(http.StatusCreated, gin.H{"message": "created"})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("User-Agent", "test-agent/1.0")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}
