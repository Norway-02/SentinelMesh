package observability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// WithSpan executes an operation within a traced OpenTelemetry span.
func WithSpan(ctx context.Context, spanName string, fn func(ctx context.Context) error, opts ...trace.SpanStartOption) error {
	ctx, span := StartSpan(ctx, spanName, opts...)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return err
}

// RecordSpanError marks a span with error status and records the exception.
func RecordSpanError(span trace.Span, err error) {
	if span != nil && err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// InjectSpanAttributes sets standard SentinelMesh attributes onto an active span.
func InjectSpanAttributes(span trace.Span, runID, agentID, tenantID, correlationID string, gen int) {
	if span == nil {
		return
	}
	if runID != "" {
		span.SetAttributes(attribute.String("sentinel.run_id", runID))
	}
	if agentID != "" {
		span.SetAttributes(attribute.String("sentinel.agent_id", agentID))
	}
	if tenantID != "" {
		span.SetAttributes(attribute.String("sentinel.tenant_id", tenantID))
	}
	if correlationID != "" {
		span.SetAttributes(attribute.String("sentinel.correlation_id", correlationID))
	}
	if gen > 0 {
		span.SetAttributes(attribute.Int("sentinel.execution_generation", gen))
	}
}

// StartConsumerSpan creates a span for an asynchronous message consumer, preserving the remote parent context.
func StartConsumerSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	defaultOpts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
	}
	opts = append(defaultOpts, opts...)
	return StartSpan(ctx, spanName, opts...)
}

// StartProducerSpan creates a span for an asynchronous message producer.
func StartProducerSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	defaultOpts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
	}
	opts = append(defaultOpts, opts...)
	return StartSpan(ctx, spanName, opts...)
}

// MeasureDuration measures the execution time of a block and records it into a Prometheus histogram.
func MeasureDuration(hist *prometheus.HistogramVec, labelValues ...string) func() {
	start := time.Now()
	return func() {
		duration := time.Since(start).Seconds()
		if hist != nil {
			hist.WithLabelValues(labelValues...).Observe(duration)
		}
	}
}
