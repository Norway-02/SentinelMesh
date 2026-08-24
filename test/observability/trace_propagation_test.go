package observability_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
)

// TestW3CTracePropagation_AsyncNATS verifies W3C TraceContext injection and extraction
// across simulated asynchronous NATS messaging boundaries.
func TestW3CTracePropagation_AsyncNATS(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := observability.DefaultConfig("nats-propagation-test")
	_, err := observability.InitTracing(cfg, exp)
	if err != nil {
		t.Fatalf("Failed to init tracing: %v", err)
	}
	defer observability.ShutdownTracing(context.Background())

	// 1. Producer Context
	prodCtx := context.Background()
	prodCtx = observability.WithCorrelationID(prodCtx, "corr-async-nats-001")
	prodCtx = observability.WithRunID(prodCtx, "run-async-nats-001")
	prodCtx = observability.WithTenantID(prodCtx, "tenant-alpha")

	prodCtx, prodSpan := observability.StartProducerSpan(prodCtx, "nats.produce_run_event")
	rootTraceID := prodSpan.SpanContext().TraceID().String()

	// Inject W3C Headers into NATS Msg Header
	headers := make(nats.Header)
	observability.InjectNATSHeaders(prodCtx, headers)

	traceparent := headers.Get("traceparent")
	if traceparent == "" {
		t.Fatalf("Expected traceparent header in NATS message")
	}

	event := events.Event{
		EventID:       "evt-12345",
		EventType:     "sentinel.run.v1.created",
		AggregateID:   "run-async-nats-001",
		TenantID:      "tenant-alpha",
		CorrelationID: "corr-async-nats-001",
		TraceParent:   traceparent,
		OccurredAt:    time.Now(),
	}

	prodSpan.End()

	// 2. Consumer Context (Simulating worker receiving event from NATS)
	consCtx := context.Background()
	consCtx = observability.ExtractNATSHeaders(consCtx, headers)
	consCtx = observability.WithRunID(consCtx, event.AggregateID)
	consCtx = observability.WithCorrelationID(consCtx, event.CorrelationID)
	consCtx = observability.WithTenantID(consCtx, event.TenantID)

	consCtx, consSpan := observability.StartConsumerSpan(consCtx, "nats.consume_run_event")
	consTraceID := consSpan.SpanContext().TraceID().String()

	if consTraceID != rootTraceID {
		t.Errorf("Consumer span trace_id mismatch: expected %s, got %s", rootTraceID, consTraceID)
	}

	// 3. Child Processing Span
	_, procSpan := observability.StartSpan(consCtx, "worker.process_workload")
	procTraceID := procSpan.SpanContext().TraceID().String()

	if procTraceID != rootTraceID {
		t.Errorf("Child processing span trace_id mismatch: expected %s, got %s", rootTraceID, procTraceID)
	}

	procSpan.End()
	consSpan.End()

	spans := exp.GetSpans()
	if len(spans) != 3 {
		t.Fatalf("Expected 3 spans, got %d", len(spans))
	}

	for _, s := range spans {
		if s.SpanContext.TraceID().String() != rootTraceID {
			t.Errorf("Span %s trace ID mismatch", s.Name)
		}
	}
}

// TestConcurrentTracing_RaceSafety verifies that high-concurrency tracing
// and metrics recordings are completely race-free under parallel execution.
func TestConcurrentTracing_RaceSafety(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := observability.DefaultConfig("concurrent-test")
	_, err := observability.InitTracing(cfg, exp)
	if err != nil {
		t.Fatalf("Failed to init tracing: %v", err)
	}
	defer observability.ShutdownTracing(context.Background())

	reg := prometheus.NewRegistry()
	m := observability.InitMetrics(reg)

	const concurrency = 50
	const operationsPerWorker = 20

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerWorker; j++ {
				runID := fmt.Sprintf("run-concurrent-%d-%d", workerID, j)
				corrID := fmt.Sprintf("corr-concurrent-%d", workerID)

				ctx := context.Background()
				ctx = observability.WithRunID(ctx, runID)
				ctx = observability.WithCorrelationID(ctx, corrID)
				ctx = observability.WithExecutionGeneration(ctx, j)

				ctx, span := observability.StartSpan(ctx, "concurrent.operation")
				observability.InjectSpanAttributes(span, runID, "agent-conc", "tenant-conc", corrID, j)

				m.RunsCreatedTotal.WithLabelValues("standard").Inc()
				m.SchedulerDecisionsTotal.WithLabelValues("SUCCESS", "v1").Inc()
				m.CheckpointSavedTotal.WithLabelValues("inline").Inc()
				m.RecoveryPointSteps.WithLabelValues("agent").Observe(float64(j * 5))

				traceID, spanID := observability.GetTraceAndSpanID(ctx)
				if traceID == "" || spanID == "" {
					t.Errorf("Invalid trace/span ID in worker %d", workerID)
				}

				time.Sleep(100 * time.Microsecond)
				span.End()
			}
		}(i)
	}

	wg.Wait()

	spans := exp.GetSpans()
	expectedTotal := concurrency * operationsPerWorker
	if len(spans) != expectedTotal {
		t.Errorf("Expected %d spans, got %d", expectedTotal, len(spans))
	}
}

// TestSeparationOfIdentifiers verifies that run_id, execution_generation,
// correlation_id, trace_id, and span_id remain strictly distinct and non-conflated.
func TestSeparationOfIdentifiers(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := observability.DefaultConfig("id-separation-test")
	_, _ = observability.InitTracing(cfg, exp)
	defer observability.ShutdownTracing(context.Background())

	ctx := context.Background()
	ctx = observability.WithRunID(ctx, "run-unique-001")
	ctx = observability.WithCorrelationID(ctx, "corr-unique-ABC")
	ctx = observability.WithExecutionGeneration(ctx, 4)

	ctx, span := observability.StartSpan(ctx, "id_separation.test")
	defer span.End()

	traceID, spanID := observability.GetTraceAndSpanID(ctx)
	runID := observability.GetRunID(ctx)
	corrID := observability.GetCorrelationID(ctx)
	gen := observability.GetExecutionGeneration(ctx)

	// All 5 identifiers must have different types/values
	if runID == corrID || runID == traceID || runID == spanID {
		t.Errorf("Identifier collision with run_id: %s", runID)
	}
	if corrID == traceID || corrID == spanID {
		t.Errorf("Identifier collision with correlation_id: %s", corrID)
	}
	if traceID == spanID {
		t.Errorf("Trace ID and Span ID collision: %s", traceID)
	}
	if gen != 4 {
		t.Errorf("Expected generation 4, got %d", gen)
	}

	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		t.Errorf("Span context is invalid")
	}
}
