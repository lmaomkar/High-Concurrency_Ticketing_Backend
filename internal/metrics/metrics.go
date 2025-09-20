package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "evently_http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "route", "status"})

	BookingRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "evently_booking_requests_total",
		Help: "Booking outcomes",
	}, []string{"outcome"})

	BookingFinalizeDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "evently_booking_finalize_duration_seconds",
		Help:    "Finalize worker duration",
		Buckets: prometheus.DefBuckets,
	})

	ReconciliationRunsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "evently_reconciliation_runs_total",
		Help: "Total reconciliation runs",
	})

	ReconciliationFixesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "evently_reconciliation_fixes_total",
		Help: "Total reconciliation fixes applied",
	})
)
