package api

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var (
	ActiveSessions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "velarix_active_sessions_total",
		Help: "Current number of sessions loaded in RAM. Recommended alert: > 5000 per instance.",
	})

	ExtractionLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "velarix_extraction_latency_ms",
		Help:    "End-to-end latency of the fact extraction call in milliseconds (success and failure). Recommended alert: P99 > 8000ms.",
		Buckets: []float64{10, 50, 100, 500, 1000, 3000, 8000, 15000},
	})

	VerifierFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "velarix_verifier_failures_total",
		Help: "Total OpenAI consistency-verifier call failures by reason. Reasons: timeout, api_error, parse_error.",
	}, []string{"reason"})

	AutoRetractionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "velarix_auto_retractions_total",
		Help: "Total facts automatically retracted due to detected contradictions.",
	})

	FactsExpiredTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "velarix_facts_expired_total",
		Help: "Total facts expired via ValidUntil sweeps (fact_expired).",
	})

	BadgerDiskUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "velarix_badger_disk_usage_bytes",
		Help: "Total bytes used by BadgerDB. Recommended alert: > 80% of volume capacity.",
	})

	FactAssertionLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "velarix_fact_assertion_latency_ms",
		Help:    "Latency of fact assertions in milliseconds. Recommended alert: P99 > 500ms.",
		Buckets: []float64{10, 50, 100, 250, 500, 1000},
	})

	APIRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "velarix_api_requests_total",
		Help: "Total API requests by endpoint and status. Recommended alert: status=5xx rate > 1%.",
	}, []string{"endpoint", "status"})

	CacheRatio = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "velarix_cache_ratio",
		Help: "GetSlice cache hit/miss count. Recommended alert: miss rate > 50% on warm sessions.",
	}, []string{"type"})

	PruneLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "velarix_prune_latency_ms",
		Help:    "Latency of causal collapse/pruning in milliseconds. Recommended alert: P99 > 100ms.",
		Buckets: []float64{1, 5, 10, 50, 100, 500},
	})

	SLOSuccessRate = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "velarix_slo_success_rate",
		Help: "Service Level Objective for overall request success (99.9% target)",
	}, []string{"result"})
)

func InitTracer() (*sdktrace.TracerProvider, error) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("velarix"),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}
