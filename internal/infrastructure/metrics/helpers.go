package metrics

import "time"

func ResultLabel(err error) string {
	if err == nil {
		return "success"
	}
	return "error"
}

func RecordScanRun(start time.Time) {
	ScannerRunsTotal.Inc()
	ScannerDuration.Observe(time.Since(start).Seconds())
}
