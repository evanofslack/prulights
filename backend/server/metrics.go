package server

import (
	"net/http"

	"github.com/evanofslack/prulights/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func metricsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))
	return mux
}
