package observability

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registerMetricsOnce sync.Once
	requestsTotal       = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "openmirror_requests_total",
			Help: "Total number of HTTP requests handled by OpenMirror.",
		},
		[]string{"method", "path", "status"},
	)
)

func Handler() http.Handler {
	registerMetricsOnce.Do(func() {
		prometheus.MustRegister(requestsTotal)
		requestsTotal.WithLabelValues("GET", "/metrics", "200").Add(0)
	})

	return promhttp.Handler()
}
