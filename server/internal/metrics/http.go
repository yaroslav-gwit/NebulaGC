package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// HTTPRequestsTotal counts total HTTP requests by method, path, and status.
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nebulagc_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration measures HTTP request duration in seconds.
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "nebulagc_http_request_duration_seconds",
			Help: "HTTP request duration in seconds",
			// Buckets optimized for API latencies: 1ms to 10s
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// HTTPResponseSize measures HTTP response size in bytes.
	HTTPResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "nebulagc_http_response_size_bytes",
			Help: "HTTP response size in bytes",
			// Buckets from 100 bytes to 10MB
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"method", "path"},
	)

	// HTTPRequestsInFlight tracks currently processing requests.
	HTTPRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "nebulagc_http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)
)

// registerHTTPMetrics registers all HTTP-related metrics.
func registerHTTPMetrics() error {
	metrics := []prometheus.Collector{
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPResponseSize,
		HTTPRequestsInFlight,
	}

	for _, metric := range metrics {
		if err := Registry.Register(metric); err != nil {
			return err
		}
	}

	return nil
}
