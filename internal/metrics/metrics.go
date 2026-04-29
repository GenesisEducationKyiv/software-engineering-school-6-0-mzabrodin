package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, path and status code.",
		},
		[]string{"method", "path", "status_code"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	ScannerRunsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "scanner_runs_total",
			Help: "Total number of completed scanner runs.",
		},
	)

	ScannerDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "scanner_duration_seconds",
			Help:    "Time spent running a full repository scan.",
			Buckets: prometheus.DefBuckets,
		},
	)

	NotificationsSentTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "notifications_sent_total",
			Help: "Total number of release notification emails sent.",
		},
	)

	GitHubAPIErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "github_api_errors_total",
			Help: "Total number of GitHub API errors by type.",
		},
		[]string{"type"},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		ScannerRunsTotal,
		ScannerDuration,
		NotificationsSentTotal,
		GitHubAPIErrorsTotal,
	)
}
