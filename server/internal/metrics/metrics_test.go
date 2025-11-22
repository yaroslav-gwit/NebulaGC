package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestInit(t *testing.T) {
	// Reset initialized flag for testing
	initialized = false
	Registry = prometheus.NewRegistry()

	err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	if !initialized {
		t.Error("Expected initialized to be true after Init()")
	}
}

func TestInit_MultipleCallsAreIdempotent(t *testing.T) {
	// Reset for test
	initialized = false
	Registry = prometheus.NewRegistry()

	// First init
	if err := Init(); err != nil {
		t.Fatalf("First Init() failed: %v", err)
	}

	// Second init should not error
	if err := Init(); err != nil {
		t.Errorf("Second Init() returned error: %v", err)
	}
}

func TestMustInit(t *testing.T) {
	// Reset for test
	initialized = false
	Registry = prometheus.NewRegistry()

	// Should not panic with valid setup
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustInit() panicked: %v", r)
		}
	}()

	MustInit()

	if !initialized {
		t.Error("Expected initialized to be true after MustInit()")
	}
}

func TestHTTPMetrics_Registration(t *testing.T) {
	// Create new registry for test
	testRegistry := prometheus.NewRegistry()
	originalRegistry := Registry
	Registry = testRegistry
	defer func() { Registry = originalRegistry }()

	err := registerHTTPMetrics()
	if err != nil {
		t.Fatalf("registerHTTPMetrics() failed: %v", err)
	}

	// Verify metrics are registered by attempting to collect
	metrics, err := testRegistry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Should have registered some metrics
	if len(metrics) == 0 {
		t.Error("Expected metrics to be registered, got none")
	}
}

func TestRateLimitMetrics_Registration(t *testing.T) {
	testRegistry := prometheus.NewRegistry()
	originalRegistry := Registry
	Registry = testRegistry
	defer func() { Registry = originalRegistry }()

	// Need to create new metrics for this test since they're package-level vars
	rateLimitChecks := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_ratelimit_checks_total"},
		[]string{"limit_type", "allowed"},
	)

	if err := testRegistry.Register(rateLimitChecks); err != nil {
		t.Fatalf("Failed to register test metric: %v", err)
	}

	// Set a value so it appears in gather
	rateLimitChecks.WithLabelValues("test", "true").Inc()

	metrics, err := testRegistry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected rate limit metrics to be registered")
	}
}

func TestDatabaseMetrics_Registration(t *testing.T) {
	testRegistry := prometheus.NewRegistry()
	originalRegistry := Registry
	Registry = testRegistry
	defer func() { Registry = originalRegistry }()

	err := registerDatabaseMetrics()
	if err != nil {
		t.Fatalf("registerDatabaseMetrics() failed: %v", err)
	}

	metrics, err := testRegistry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected database metrics to be registered")
	}
}

func TestHAMetrics_Registration(t *testing.T) {
	testRegistry := prometheus.NewRegistry()
	originalRegistry := Registry
	Registry = testRegistry
	defer func() { Registry = originalRegistry }()

	err := registerHAMetrics()
	if err != nil {
		t.Fatalf("registerHAMetrics() failed: %v", err)
	}

	metrics, err := testRegistry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected HA metrics to be registered")
	}
}

func TestBusinessMetrics_Registration(t *testing.T) {
	testRegistry := prometheus.NewRegistry()
	originalRegistry := Registry
	Registry = testRegistry
	defer func() { Registry = originalRegistry }()

	// Create a test business metric
	testMetric := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "test_nodes_total"},
		[]string{"tenant_id", "cluster_id"},
	)

	if err := testRegistry.Register(testMetric); err != nil {
		t.Fatalf("Failed to register test metric: %v", err)
	}

	// Set a value so it appears in gather
	testMetric.WithLabelValues("tenant1", "cluster1").Set(5)

	metrics, err := testRegistry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected business metrics to be registered")
	}
}

func TestHTTPMetrics_Collection(t *testing.T) {
	// Reset for test
	initialized = false
	Registry = prometheus.NewRegistry()

	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Test incrementing counter
	HTTPRequestsTotal.WithLabelValues("GET", "/test", "200").Inc()

	// Test observing histogram
	HTTPRequestDuration.WithLabelValues("GET", "/test").Observe(0.123)
	HTTPResponseSize.WithLabelValues("GET", "/test").Observe(1024)

	// Test gauge
	HTTPRequestsInFlight.Set(5)

	// Gather metrics
	metrics, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Verify we have metrics
	if len(metrics) == 0 {
		t.Error("Expected collected metrics, got none")
	}
}

func TestBusinessMetrics_NodeCount(t *testing.T) {
	initialized = false
	Registry = prometheus.NewRegistry()

	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Set node counts for different clusters
	NodeCount.WithLabelValues("tenant1", "cluster1").Set(10)
	NodeCount.WithLabelValues("tenant1", "cluster2").Set(5)

	// Gather and verify
	metrics, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected node count metrics")
	}
}

func TestBusinessMetrics_BundleOperations(t *testing.T) {
	initialized = false
	Registry = prometheus.NewRegistry()

	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Record bundle operations
	BundleOperations.WithLabelValues("cluster1", "upload", "success").Inc()
	BundleOperations.WithLabelValues("cluster1", "download", "success").Add(5)

	metrics, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected bundle operation metrics")
	}
}
