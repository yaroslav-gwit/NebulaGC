package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// HAStateTransitions counts HA state transitions by from/to state.
	HAStateTransitions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nebulagc_ha_state_transitions_total",
			Help: "Total number of HA state transitions",
		},
		[]string{"from_state", "to_state"},
	)

	// HAIsMaster indicates whether this instance is currently the master (1) or not (0).
	HAIsMaster = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "nebulagc_ha_is_master",
			Help: "Whether this instance is currently the master (1=master, 0=replica)",
		},
	)

	// HAHeartbeatDuration measures HA heartbeat processing duration.
	HAHeartbeatDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "nebulagc_ha_heartbeat_duration_seconds",
			Help: "HA heartbeat processing duration in seconds",
			// Buckets optimized for heartbeat operations: 1ms to 1s
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	// HAHeartbeatErrors counts HA heartbeat errors.
	HAHeartbeatErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "nebulagc_ha_heartbeat_errors_total",
			Help: "Total number of HA heartbeat errors",
		},
	)

	// HAReplicasTotal tracks the number of registered replicas.
	HAReplicasTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "nebulagc_ha_replicas_total",
			Help: "Total number of registered replica instances",
		},
	)

	// HALastHeartbeat tracks the timestamp of the last successful heartbeat.
	HALastHeartbeat = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "nebulagc_ha_last_heartbeat_timestamp_seconds",
			Help: "Unix timestamp of the last successful heartbeat",
		},
	)
)

// registerHAMetrics registers all HA-related metrics.
func registerHAMetrics() error {
	metrics := []prometheus.Collector{
		HAStateTransitions,
		HAIsMaster,
		HAHeartbeatDuration,
		HAHeartbeatErrors,
		HAReplicasTotal,
		HALastHeartbeat,
	}

	for _, metric := range metrics {
		if err := Registry.Register(metric); err != nil {
			return err
		}
	}

	return nil
}
