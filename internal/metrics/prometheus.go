package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	DevicesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "control_plane_devices_total",
			Help: "Total number of registered devices",
		},
		[]string{"status", "type"},
	)
	
	HeartbeatsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "control_plane_heartbeats_total",
			Help: "Total number of heartbeats received",
		},
		[]string{"device_type"},
	)
	
	RegistrationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "control_plane_registrations_total",
			Help: "Total device registrations",
		},
	)
	
	GRPCRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "control_plane_grpc_duration_seconds",
			Help: "gRPC request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(DevicesTotal)
	prometheus.MustRegister(HeartbeatsTotal)
	prometheus.MustRegister(RegistrationsTotal)
	prometheus.MustRegister(GRPCRequestDuration)
}

func Handler() http.Handler {
	return promhttp.Handler()
}