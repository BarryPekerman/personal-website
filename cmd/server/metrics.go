package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTP metrics per route, served on /metrics. The WebSocket isn't timed here:
// its "request" lasts the whole session (see portfolio_terminal_*).
var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "portfolio_http_requests_total",
		Help: "HTTP requests, by route and status code.",
	}, []string{"route", "code"})
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "portfolio_http_request_duration_seconds",
		Help:    "Time to serve an HTTP request, by route.",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"route"})
)

// instrument counts and times the requests h serves as route
func instrument(route string, h http.Handler) http.Handler {
	labels := prometheus.Labels{"route": route}
	return promhttp.InstrumentHandlerDuration(httpDuration.MustCurryWith(labels),
		promhttp.InstrumentHandlerCounter(httpRequests.MustCurryWith(labels), h))
}

// metricsHandler serves everything registered with the default registry:
// the metrics above, the terminal's and the status sources', and the Go
// runtime and process metrics. It listens on its own port (METRICS_ADDR), so
// the tunnel, which only reaches the site's port, never exposes it.
func metricsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	return mux
}
