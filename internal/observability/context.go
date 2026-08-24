package observability

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const (
	runIDKey               contextKey = "sentinel_run_id"
	correlationIDKey       contextKey = "sentinel_correlation_id"
	executionGenerationKey contextKey = "sentinel_execution_generation"
	agentIDKey             contextKey = "sentinel_agent_id"
	tenantIDKey            contextKey = "sentinel_tenant_id"
)

// NATSHeaderCarrier adapts nats.Header to OpenTelemetry TextMapCarrier.
type NATSHeaderCarrier nats.Header

func (c NATSHeaderCarrier) Get(key string) string {
	return nats.Header(c).Get(key)
}

func (c NATSHeaderCarrier) Set(key string, val string) {
	nats.Header(c).Set(key, val)
}

func (c NATSHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectNATSHeaders writes W3C trace context into NATS message headers.
func InjectNATSHeaders(ctx context.Context, headers nats.Header) {
	if headers == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, NATSHeaderCarrier(headers))
}

// ExtractNATSHeaders reads W3C trace context from NATS message headers into context.
func ExtractNATSHeaders(ctx context.Context, headers nats.Header) context.Context {
	if headers == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, NATSHeaderCarrier(headers))
}

// InjectHTTPHeaders writes W3C trace context into HTTP headers.
func InjectHTTPHeaders(ctx context.Context, headers http.Header) {
	if headers == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
}

// ExtractHTTPHeaders reads W3C trace context from HTTP headers into context.
func ExtractHTTPHeaders(ctx context.Context, headers http.Header) context.Context {
	if headers == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(headers))
}

// GRPCMetadataCarrier adapts gRPC metadata.MD to OpenTelemetry TextMapCarrier.
type GRPCMetadataCarrier metadata.MD

func (c GRPCMetadataCarrier) Get(key string) string {
	vals := metadata.MD(c).Get(strings.ToLower(key))
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func (c GRPCMetadataCarrier) Set(key string, val string) {
	metadata.MD(c).Set(strings.ToLower(key), val)
}

func (c GRPCMetadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectGRPCMetadata writes W3C trace context into gRPC metadata.
func InjectGRPCMetadata(ctx context.Context, md metadata.MD) {
	if md == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, GRPCMetadataCarrier(md))
}

// ExtractGRPCMetadata reads W3C trace context from gRPC metadata into context.
func ExtractGRPCMetadata(ctx context.Context, md metadata.MD) context.Context {
	if md == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, GRPCMetadataCarrier(md))
}

// Context Metadata Setters & Getters
func WithRunID(ctx context.Context, runID string) context.Context {
	return context.WithValue(ctx, runIDKey, runID)
}

func GetRunID(ctx context.Context) string {
	if val, ok := ctx.Value(runIDKey).(string); ok {
		return val
	}
	return ""
}

func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

func GetCorrelationID(ctx context.Context) string {
	if val, ok := ctx.Value(correlationIDKey).(string); ok {
		return val
	}
	return ""
}

func WithExecutionGeneration(ctx context.Context, gen int) context.Context {
	return context.WithValue(ctx, executionGenerationKey, gen)
}

func GetExecutionGeneration(ctx context.Context) int {
	if val, ok := ctx.Value(executionGenerationKey).(int); ok {
		return val
	}
	return 0
}

func WithAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentIDKey, agentID)
}

func GetAgentID(ctx context.Context) string {
	if val, ok := ctx.Value(agentIDKey).(string); ok {
		return val
	}
	return ""
}

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

func GetTenantID(ctx context.Context) string {
	if val, ok := ctx.Value(tenantIDKey).(string); ok {
		return val
	}
	return ""
}

// GetTraceAndSpanID extracts hex-encoded trace_id and span_id from span in context.
func GetTraceAndSpanID(ctx context.Context) (traceID, spanID string) {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String(), spanCtx.SpanID().String()
	}
	return "", ""
}

// GetTraceParent returns the formatted W3C traceparent string (e.g. 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01).
func GetTraceParent(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		flags := spanCtx.TraceFlags()
		if flags == 0 {
			flags = trace.FlagsSampled
		}
		return fmt.Sprintf("00-%s-%s-%02x", spanCtx.TraceID().String(), spanCtx.SpanID().String(), byte(flags))
	}
	return ""
}

// ContextWithTraceParent extracts a remote span context from a W3C traceparent string and returns a new context.
func ContextWithTraceParent(ctx context.Context, traceParent string) context.Context {
	if traceParent == "" {
		return ctx
	}
	carrier := propagation.MapCarrier{
		"traceparent": traceParent,
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
