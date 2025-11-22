// Package metrics provides Prometheus metrics for the NebulaGC server.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	// Registry is the global Prometheus registry for all metrics.
	Registry = prometheus.NewRegistry()

	// initialized tracks whether metrics have been initialized.
	initialized = false
)

// Init initializes the metrics registry with all collectors.
// This should be called once during application startup.
func Init() error {
	if initialized {
		return nil
	}

	// Register Go runtime collectors
	if err := Registry.Register(collectors.NewGoCollector()); err != nil {
		return err
	}
	if err := Registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return err
	}

	// Register HTTP metrics
	if err := registerHTTPMetrics(); err != nil {
		return err
	}

	// Register rate limit metrics
	if err := registerRateLimitMetrics(); err != nil {
		return err
	}

	// Register database metrics
	if err := registerDatabaseMetrics(); err != nil {
		return err
	}

	// Register HA metrics
	if err := registerHAMetrics(); err != nil {
		return err
	}

	// Register business metrics
	if err := registerBusinessMetrics(); err != nil {
		return err
	}

	initialized = true
	return nil
}

// MustInit initializes metrics and panics on error.
// Use this for application startup where metrics are required.
func MustInit() {
	if err := Init(); err != nil {
		panic("failed to initialize metrics: " + err.Error())
	}
}

// registerBusinessMetrics registers business-level metrics.
func registerBusinessMetrics() error {
	metrics := []prometheus.Collector{
		NodeCount,
		ClusterCount,
		BundleOperations,
	}

	for _, metric := range metrics {
		if err := Registry.Register(metric); err != nil {
			return err
		}
	}

	return nil
}

var (
	// NodeCount tracks the number of nodes per cluster.
	NodeCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nebulagc_nodes_total",
			Help: "Total number of nodes in each cluster",
		},
		[]string{"tenant_id", "cluster_id"},
	)

	// ClusterCount tracks the number of clusters per tenant.
	ClusterCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nebulagc_clusters_total",
			Help: "Total number of clusters for each tenant",
		},
		[]string{"tenant_id"},
	)

	// BundleOperations tracks config bundle operations.
	BundleOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nebulagc_bundle_operations_total",
			Help: "Total number of config bundle operations",
		},
		[]string{"cluster_id", "operation", "status"},
	)
)
