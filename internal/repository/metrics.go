package repository

import (
	"time"

	"github-release-notifier/internal/metrics"
)

func trackDBQuery(start time.Time, operation, table string, err error) {
	metrics.DBQueriesTotal.WithLabelValues(operation, table).Inc()
	metrics.DBQueryDuration.WithLabelValues(operation, table).Observe(time.Since(start).Seconds())
	if err != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues(operation, table).Inc()
	}
}
