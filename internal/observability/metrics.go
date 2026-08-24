package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the registered Prometheus metrics with strictly low-cardinality labels.
type Metrics struct {
	Registry *prometheus.Registry

	// Platform
	RunsCreatedTotal   *prometheus.CounterVec
	RunsCompletedTotal *prometheus.CounterVec
	RunsFailedTotal    *prometheus.CounterVec
	ActiveRuns         *prometheus.GaugeVec

	// Scheduler
	SchedulerDecisionsTotal        *prometheus.CounterVec
	SchedulerDecisionDurationSec   *prometheus.HistogramVec
	SchedulerCandidatesTotal       *prometheus.CounterVec
	SchedulerRejectionsTotal       *prometheus.CounterVec

	// Checkpoint
	CheckpointSavedTotal      *prometheus.CounterVec
	CheckpointDurationSec     *prometheus.HistogramVec
	CheckpointSizeBytes       *prometheus.HistogramVec
	CheckpointFailuresTotal   *prometheus.CounterVec

	// Recovery & Self-Healing
	RecoveryRequestsTotal     *prometheus.CounterVec
	RecoverySuccessTotal      *prometheus.CounterVec
	RecoveryFailureTotal      *prometheus.CounterVec
	RecoveryDurationSec       *prometheus.HistogramVec
	RecoveryGenerationTotal   *prometheus.CounterVec

	// Recovery Effectiveness (Thesis Metrics)
	RecoveryPointSteps     *prometheus.HistogramVec
	RecoveryCheckpointAge  *prometheus.HistogramVec
	RecoveryLostWorkSteps  *prometheus.HistogramVec

	// Verification
	VerificationTotal       *prometheus.CounterVec
	VerificationSuccessTotal *prometheus.CounterVec
	VerificationFailureTotal *prometheus.CounterVec
	VerificationDurationSec *prometheus.HistogramVec

	// Security
	PolicyEvaluationsTotal      *prometheus.CounterVec
	PolicyDenialsTotal          *prometheus.CounterVec
	PolicyEvaluationDurationSec *prometheus.HistogramVec
	SecurityViolationsTotal     *prometheus.CounterVec

	// Outbox & Messaging
	OutboxPendingEvents          *prometheus.GaugeVec
	OutboxPublishTotal           *prometheus.CounterVec
	OutboxPublishFailuresTotal   *prometheus.CounterVec
	OutboxOldestPendingAgeSec    *prometheus.GaugeVec
}

var (
	defaultMetrics *Metrics
)

// InitMetrics initializes the Prometheus metrics registry and instruments.
func InitMetrics(reg *prometheus.Registry) *Metrics {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}
	factory := promauto.With(reg)

	m := &Metrics{
		Registry: reg,

		// Platform Metrics
		RunsCreatedTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_runs_created_total",
				Help: "Total number of agent runs created.",
			},
			[]string{"tenant_type"},
		),
		RunsCompletedTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_runs_completed_total",
				Help: "Total number of agent runs completed successfully.",
			},
			[]string{"verification_status"},
		),
		RunsFailedTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_runs_failed_total",
				Help: "Total number of agent runs that reached failed state.",
			},
			[]string{"failure_reason_category"},
		),
		ActiveRuns: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "sentinel_active_runs",
				Help: "Current count of active agent runs by state.",
			},
			[]string{"state"},
		),

		// Scheduler Metrics
		SchedulerDecisionsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_scheduler_decisions_total",
				Help: "Total number of scheduling placement decisions made.",
			},
			[]string{"status", "algorithm_version"},
		),
		SchedulerDecisionDurationSec: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sentinel_scheduler_decision_duration_seconds",
				Help:    "Latency of scheduling decision in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"algorithm_version"},
		),
		SchedulerCandidatesTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_scheduler_candidates_total",
				Help: "Total candidates considered for scheduling placement.",
			},
			[]string{"filter_result"},
		),
		SchedulerRejectionsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_scheduler_rejections_total",
				Help: "Total candidate nodes rejected during scheduling filter phase.",
			},
			[]string{"rejection_reason"},
		),

		// Checkpoint Metrics
		CheckpointSavedTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_checkpoint_saved_total",
				Help: "Total durable checkpoints persisted.",
			},
			[]string{"storage_tier"},
		),
		CheckpointDurationSec: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sentinel_checkpoint_duration_seconds",
				Help:    "Time taken to persist checkpoint in seconds.",
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
			},
			[]string{"storage_tier"},
		),
		CheckpointSizeBytes: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sentinel_checkpoint_size_bytes",
				Help:    "Size of persisted checkpoint payloads in bytes.",
				Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
			},
			[]string{"storage_tier"},
		),
		CheckpointFailuresTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_checkpoint_failures_total",
				Help: "Total failed checkpoint persistence attempts.",
			},
			[]string{"error_category"},
		),

		// Recovery & Self-Healing
		RecoveryRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_recovery_requests_total",
				Help: "Total node failure recovery requests initiated.",
			},
			[]string{"trigger_source"},
		),
		RecoverySuccessTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_recovery_success_total",
				Help: "Total runs successfully recovered onto healthy nodes.",
			},
			[]string{"target_cluster"},
		),
		RecoveryFailureTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_recovery_failure_total",
				Help: "Total recovery attempts that failed.",
			},
			[]string{"failure_reason"},
		),
		RecoveryDurationSec: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sentinel_recovery_duration_seconds",
				Help:    "Total time taken to recover a failed workload in seconds.",
				Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
			},
			[]string{"status"},
		),
		RecoveryGenerationTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_recovery_generation_total",
				Help: "Distribution of recovery execution generations.",
			},
			[]string{"generation_bucket"},
		),

		// Recovery Effectiveness (Thesis Metrics)
		RecoveryPointSteps: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sentinel_recovery_recovery_point_steps",
				Help:    "Step index from which execution resumed during recovery.",
				Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500},
			},
			[]string{"workload_type"},
		),
		RecoveryCheckpointAge: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sentinel_recovery_checkpoint_age_seconds",
				Help:    "Age of the restored checkpoint when recovery triggered.",
				Buckets: []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0},
			},
			[]string{"workload_type"},
		),
		RecoveryLostWorkSteps: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sentinel_recovery_lost_work_steps",
				Help:    "Estimated execution steps lost between latest checkpoint and failure.",
				Buckets: []float64{0, 1, 2, 5, 10, 20, 50},
			},
			[]string{"workload_type"},
		),

		// Verification Metrics
		VerificationTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_verification_total",
				Help: "Total outcome verification attempts.",
			},
			[]string{"verifier_type"},
		),
		VerificationSuccessTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_verification_success_total",
				Help: "Total outcome verifications that passed.",
			},
			[]string{"verifier_type"},
		),
		VerificationFailureTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_verification_failure_total",
				Help: "Total outcome verifications that failed.",
			},
			[]string{"verifier_type", "rule_category"},
		),
		VerificationDurationSec: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sentinel_verification_duration_seconds",
				Help:    "Time taken to evaluate outcome verification rules in seconds.",
				Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 2.0},
			},
			[]string{"verifier_type"},
		),

		// Security & Policy Metrics
		PolicyEvaluationsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_policy_evaluations_total",
				Help: "Total security policy evaluation operations performed.",
			},
			[]string{"profile", "operation"},
		),
		PolicyDenialsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_policy_denials_total",
				Help: "Total policy violations denied by policy engine.",
			},
			[]string{"profile", "operation", "rule_category"},
		),
		PolicyEvaluationDurationSec: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sentinel_policy_evaluation_duration_seconds",
				Help:    "Latency of policy evaluation engine in seconds.",
				Buckets: []float64{0.000001, 0.000005, 0.00001, 0.00005, 0.0001, 0.001},
			},
			[]string{"profile"},
		),
		SecurityViolationsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_security_violations_total",
				Help: "Total security sandbox containment violation events.",
			},
			[]string{"profile", "violation_type"},
		),

		// Outbox & Messaging Metrics
		OutboxPendingEvents: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "sentinel_outbox_pending_events",
				Help: "Current count of uncommitted outbox events.",
			},
			[]string{"aggregate_type"},
		),
		OutboxPublishTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_outbox_publish_total",
				Help: "Total outbox events published to message bus.",
			},
			[]string{"event_type"},
		),
		OutboxPublishFailuresTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sentinel_outbox_publish_failures_total",
				Help: "Total outbox event publishing failures.",
			},
			[]string{"event_type"},
		),
		OutboxOldestPendingAgeSec: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "sentinel_outbox_oldest_pending_age_seconds",
				Help: "Age in seconds of the oldest pending event in the outbox.",
			},
			[]string{"aggregate_type"},
		),
	}

	defaultMetrics = m
	return m
}

// GetMetrics returns the default global Metrics instance.
func GetMetrics() *Metrics {
	if defaultMetrics == nil {
		defaultMetrics = InitMetrics(prometheus.DefaultRegisterer.(*prometheus.Registry))
	}
	return defaultMetrics
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics from the active registry.
func MetricsHandler() http.Handler {
	m := GetMetrics()
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// StartMetricsServer starts a dedicated HTTP server on the given port to serve /metrics.
func StartMetricsServer(ctx context.Context, port int) *http.Server {
	if port <= 0 {
		port = 9090
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", MetricsHandler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		slog.Info("Starting Prometheus metrics server", slog.Int("port", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Prometheus metrics server failed", slog.Any("error", err))
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	return server
}
