package repository

import (
	"errors"
	"time"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/metrics"
)

func trackDBQuery(start time.Time, operation, table string, err error) {
	metrics.DBQueriesTotal.WithLabelValues(operation, table).Inc()
	metrics.DBQueryDuration.WithLabelValues(operation, table).Observe(time.Since(start).Seconds())
	if err != nil && !isDomainError(err) {
		metrics.DBQueryErrorsTotal.WithLabelValues(operation, table).Inc()
	}
}

func isDomainError(err error) bool {
	return errors.Is(err, entity.ErrNotFound) || errors.Is(err, entity.ErrAlreadyExists)
}
