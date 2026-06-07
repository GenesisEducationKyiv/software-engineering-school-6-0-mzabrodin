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

	GRPCRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests by method and status code.",
		},
		[]string{"method", "code"},
	)

	GRPCRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "gRPC request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
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

	SubscriptionOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "subscription_operations_total",
			Help: "Total number of subscription operations by type and result.",
		},
		[]string{"operation", "result"},
	)

	ScannerErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "scanner_errors_total",
			Help: "Total number of scanner errors by reason.",
		},
		[]string{"reason"},
	)

	DBQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries by operation and table.",
		},
		[]string{"operation", "table"},
	)

	DBQueryErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_query_errors_total",
			Help: "Total number of database query errors by operation and table.",
		},
		[]string{"operation", "table"},
	)

	DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation", "table"},
	)

	CacheOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_operations_total",
			Help: "Total number of cache operations by operation and result.",
		},
		[]string{"operation", "result"},
	)

	CacheOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cache_operation_duration_seconds",
			Help:    "Cache operation duration in seconds.",
			Buckets: []float64{.0005, .001, .005, .01, .025, .05, .1},
		},
		[]string{"operation"},
	)

	EmailSendsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "email_sends_total",
			Help: "Total number of email send operations by type and status.",
		},
		[]string{"type", "status"},
	)

	EmailSendDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "email_send_duration_seconds",
			Help:    "Email send duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"type"},
	)

	GitHubAPIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "github_api_requests_total",
			Help: "Total number of GitHub API requests by operation and result.",
		},
		[]string{"operation", "result"},
	)

	GitHubAPIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "github_api_request_duration_seconds",
			Help:    "GitHub API request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		GRPCRequestsTotal,
		GRPCRequestDuration,
		ScannerRunsTotal,
		ScannerDuration,
		NotificationsSentTotal,
		GitHubAPIErrorsTotal,
		SubscriptionOperationsTotal,
		ScannerErrorsTotal,
		DBQueriesTotal,
		DBQueryErrorsTotal,
		DBQueryDuration,
		CacheOperationsTotal,
		CacheOperationDuration,
		EmailSendsTotal,
		EmailSendDuration,
		GitHubAPIRequestsTotal,
		GitHubAPIRequestDuration,
	)
}
