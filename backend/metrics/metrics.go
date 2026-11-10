// Package metrics holds every prulights Prometheus metric in one registry.
// Labels are fixed sets, never a user, device or address.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry is served at /metrics on METRICS_ADDR.
var Registry = prometheus.NewRegistry()

var (
	ScrapeRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "prulights_scrape_runs_total", Help: "Scraper runs, by result.",
	}, []string{"result"})
	ScrapeFetches = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "prulights_scrape_fetches_total", Help: "Source pages checked, by outcome.",
	}, []string{"outcome"})
	ScrapeWarnings = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prulights_scrape_warnings_total", Help: "Items or pages the scraper couldn't read cleanly.",
	})
	ScrapeLastSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prulights_scrape_last_success_timestamp_seconds", Help: "When the scraper last finished without an error.",
	})
	ScheduleNights = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prulights_schedule_published_days_ahead", Help: "How many nights ahead the source has published.",
	})
	DeviceCheckins = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prulights_device_checkins_total", Help: "Times a frame checked in.",
	})
	DeviceBundles = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "prulights_device_bundles_total", Help: "Frame bundles asked for, by whether they had to be drawn.",
	}, []string{"result"})
)

func init() {
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		ScrapeRuns, ScrapeFetches, ScrapeWarnings, ScrapeLastSuccess, ScheduleNights,
		DeviceCheckins, DeviceBundles,
	)
	// start every series at 0 so rate() works before the first event
	for _, r := range []string{"ok", "error"} {
		ScrapeRuns.WithLabelValues(r)
	}
	for _, o := range []string{"changed", "unchanged", "skipped", "error", "rejected"} {
		ScrapeFetches.WithLabelValues(o)
	}
	for _, r := range []string{"cached", "rendered"} {
		DeviceBundles.WithLabelValues(r)
	}
}
