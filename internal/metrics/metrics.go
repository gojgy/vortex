package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// в пакете определены метрики в формате prometheus для всего приложения. где они берутся?
//
// метки для `location` собираются в `vortex.ServeRequest` на основе результата `resolveLocation`
// метки для `upstream` и `server` собираются в `vortex.serveProxy` в момент, когда балансировщик возвращает конкретный бэкенд

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "vortex",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests processed.",
		},
		[]string{"location", "method", "code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "vortex",
			Name:      "http_request_duration_seconds",
			Help:      "Histogram of HTTP request latencies.",
			Buckets:   []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
		},
		[]string{"location"},
	)

	RateLimitRequestsLimitedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "vortex",
			Name:      "ratelimit_limited_requests_total",
			Help:      "Total number of requests limited/denied.",
		},
		[]string{"location"},
	)

	UpstreamRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "vortex",
			Name:      "upstream_requests_total",
			Help:      "Total number of requests sent to upstream servers.",
		},
		[]string{"upstream", "server"},
	)
)
