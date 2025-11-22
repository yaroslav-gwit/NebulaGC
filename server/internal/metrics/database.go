package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// DBQueryDuration measures database query duration by operation.
	DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "nebulagc_db_query_duration_seconds",
			Help: "Database query duration in seconds",
			// Buckets optimized for database queries: 100µs to 10s
			Buckets: []float64{.0001, .0005, .001, .005, .01, .05, .1, .5, 1, 5, 10},
		},
		[]string{"operation"},
	)

	// DBQueriesTotal counts total database queries by operation and status.
	DBQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nebulagc_db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "status"},
	)

	// DBConnectionsOpen tracks currently open database connections.
	DBConnectionsOpen = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "nebulagc_db_connections_open",
			Help: "Number of currently open database connections",
		},
	)

	// DBConnectionsIdle tracks currently idle database connections.
	DBConnectionsIdle = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "nebulagc_db_connections_idle",
			Help: "Number of currently idle database connections",
		},
	)

	// DBConnectionsInUse tracks database connections currently in use.
	DBConnectionsInUse = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "nebulagc_db_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	// DBConnectionsMaxOpen tracks the maximum number of open connections.
	DBConnectionsMaxOpen = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "nebulagc_db_connections_max_open",
			Help: "Maximum number of open database connections allowed",
		},
	)
)

// registerDatabaseMetrics registers all database-related metrics.
func registerDatabaseMetrics() error {
	metrics := []prometheus.Collector{
		DBQueryDuration,
		DBQueriesTotal,
		DBConnectionsOpen,
		DBConnectionsIdle,
		DBConnectionsInUse,
		DBConnectionsMaxOpen,
	}

	for _, metric := range metrics {
		if err := Registry.Register(metric); err != nil {
			return err
		}
	}

	return nil
}
