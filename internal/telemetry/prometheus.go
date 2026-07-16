package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Время обработки HTTP запросов",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status_code"},
	)
)

func InitAppMetrics(serviceName string, enabled bool) {
	if !enabled {
		return
	}
	reg := prometheus.WrapRegistererWith(
		prometheus.Labels{"service": serviceName},
		prometheus.DefaultRegisterer,
	)

	reg.MustRegister(
		HTTPRequestDuration,
		// ... в будущем тут будут метрики регистраций, логинов и т.д.
	)
}
