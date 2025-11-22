package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// RateLimitChecks counts rate limit checks by type and result.
	RateLimitChecks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nebulagc_ratelimit_checks_total",
			Help: "Total number of rate limit checks",
		},
		[]string{"limit_type", "allowed"},
	)

	// RateLimitBlocks counts rate limit blocks by type.
	RateLimitBlocks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nebulagc_ratelimit_blocks_total",
			Help: "Total number of rate limit blocks",
		},
		[]string{"limit_type", "identifier"},
	)

	// RateLimitTokensAvailable tracks available tokens in buckets.
	RateLimitTokensAvailable = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nebulagc_ratelimit_tokens_available",
			Help: "Number of tokens currently available in rate limit bucket",
		},
		[]string{"limit_type", "identifier"},
	)

	// RateLimitBucketCapacity tracks the maximum capacity of rate limit buckets.
	RateLimitBucketCapacity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nebulagc_ratelimit_bucket_capacity",
			Help: "Maximum capacity of rate limit buckets",
		},
		[]string{"limit_type"},
	)
)

// registerRateLimitMetrics registers all rate limiting metrics.
func registerRateLimitMetrics() error {
	metrics := []prometheus.Collector{
		RateLimitChecks,
		RateLimitBlocks,
		RateLimitTokensAvailable,
		RateLimitBucketCapacity,
	}

	for _, metric := range metrics {
		if err := Registry.Register(metric); err != nil {
			return err
		}
	}

	return nil
}
