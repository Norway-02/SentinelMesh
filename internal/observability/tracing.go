package observability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	globalTracerProvider *sdktrace.TracerProvider
	globalTracerMu       sync.RWMutex
	globalTracer         trace.Tracer
)

// InitTracing initializes the OpenTelemetry TracerProvider with W3C TraceContext propagation.
func InitTracing(cfg Config, exporter sdktrace.SpanExporter) (*sdktrace.TracerProvider, error) {
	globalTracerMu.Lock()
	defer globalTracerMu.Unlock()

	// Ensure W3C propagator is set globally
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel resource: %w", err)
	}

	var bsp sdktrace.SpanProcessor
	if exporter != nil {
		bsp = sdktrace.NewSimpleSpanProcessor(exporter)
	} else if cfg.Enabled && cfg.OTLPEndpoint != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		otlpExp, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			// Fallback to noop exporter if OTLP connection cannot be established
			bsp = sdktrace.NewSimpleSpanProcessor(&noopExporter{})
		} else {
			bsp = sdktrace.NewBatchSpanProcessor(otlpExp)
		}
	} else {
		bsp = sdktrace.NewSimpleSpanProcessor(&noopExporter{})
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRatio))),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tp)

	globalTracerProvider = tp
	globalTracer = tp.Tracer("github.com/sentinelmesh/sentinelmesh")

	return tp, nil
}

// GetTracer returns the global Tracer instance.
func GetTracer() trace.Tracer {
	globalTracerMu.RLock()
	defer globalTracerMu.RUnlock()

	if globalTracer == nil {
		return otel.Tracer("github.com/sentinelmesh/sentinelmesh")
	}
	return globalTracer
}

// StartSpan creates a child span with standard attributes.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return GetTracer().Start(ctx, name, opts...)
}

// ShutdownTracing flushes and stops the global TracerProvider.
func ShutdownTracing(ctx context.Context) error {
	globalTracerMu.Lock()
	defer globalTracerMu.Unlock()

	if globalTracerProvider != nil {
		err := globalTracerProvider.Shutdown(ctx)
		globalTracerProvider = nil
		globalTracer = nil
		return err
	}
	return nil
}

type noopExporter struct{}

func (n *noopExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return nil
}

func (n *noopExporter) Shutdown(ctx context.Context) error {
	return nil
}
