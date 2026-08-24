package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestTracing_InitAndSpanGeneration(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := DefaultConfig("test-service")

	_, err := InitTracing(cfg, exp)
	if err != nil {
		t.Fatalf("InitTracing failed: %v", err)
	}

	ctx := context.Background()
	ctx, rootSpan := StartSpan(ctx, "root-operation")
	_, childSpan := StartSpan(ctx, "child-operation")
	childSpan.End()
	rootSpan.End()

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 exported spans, got %d", len(spans))
	}

	if spans[0].Name != "child-operation" && spans[1].Name != "child-operation" {
		t.Errorf("child span not found in exported spans")
	}

	if err := ShutdownTracing(context.Background()); err != nil {
		t.Errorf("ShutdownTracing failed: %v", err)
	}
}

func TestW3CContextCarrier_NATSAndHTTP(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := DefaultConfig("carrier-test")
	_, _ = InitTracing(cfg, exp)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "nats-producer-span")
	defer span.End()

	// Test NATS carrier
	natsHeaders := make(nats.Header)
	InjectNATSHeaders(ctx, natsHeaders)

	traceparent := natsHeaders.Get("traceparent")
	if traceparent == "" {
		t.Fatalf("expected traceparent header in NATS message, got empty")
	}

	extractedCtx := ExtractNATSHeaders(context.Background(), natsHeaders)
	extractedSpanCtx := trace.SpanContextFromContext(extractedCtx)
	if !extractedSpanCtx.IsValid() {
		t.Fatalf("extracted span context is invalid")
	}
	if extractedSpanCtx.TraceID() != span.SpanContext().TraceID() {
		t.Errorf("trace ID mismatch: expected %s, got %s", span.SpanContext().TraceID(), extractedSpanCtx.TraceID())
	}

	// Test HTTP carrier
	httpHeaders := make(http.Header)
	InjectHTTPHeaders(ctx, httpHeaders)
	if httpHeaders.Get("traceparent") == "" {
		t.Fatalf("expected traceparent header in HTTP request, got empty")
	}

	// Test TraceParent formatting helpers
	tp := GetTraceParent(ctx)
	if tp == "" {
		t.Fatalf("expected non-empty traceparent string")
	}
	fromTPCtx := ContextWithTraceParent(context.Background(), tp)
	fromTPSpanCtx := trace.SpanContextFromContext(fromTPCtx)
	if !fromTPSpanCtx.IsValid() || fromTPSpanCtx.TraceID() != span.SpanContext().TraceID() {
		t.Errorf("ContextWithTraceParent failed to restore valid span context")
	}
}

func TestGRPCMetadataCarrier(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := DefaultConfig("grpc-carrier-test")
	_, _ = InitTracing(cfg, exp)

	ctx := context.Background()
	ctx = WithRunID(ctx, "run-grpc-01")
	ctx = WithCorrelationID(ctx, "corr-grpc-01")
	ctx, span := StartSpan(ctx, "grpc-client-call")
	defer span.End()

	md := metadata.New(nil)
	InjectGRPCMetadata(ctx, md)

	if len(md.Get("traceparent")) == 0 {
		t.Fatalf("expected traceparent in gRPC metadata")
	}

	extractedCtx := ExtractGRPCMetadata(context.Background(), md)
	extractedSpanCtx := trace.SpanContextFromContext(extractedCtx)
	if !extractedSpanCtx.IsValid() || extractedSpanCtx.TraceID() != span.SpanContext().TraceID() {
		t.Errorf("extracted gRPC span context invalid or mismatched trace ID")
	}
}

func TestGRPCInterceptors(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := DefaultConfig("grpc-interceptor-test")
	_, _ = InitTracing(cfg, exp)

	serverInterceptor := UnaryServerInterceptor()

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		traceID, spanID := GetTraceAndSpanID(ctx)
		if traceID == "" || spanID == "" {
			t.Errorf("expected traceID and spanID in handler context")
		}
		if GetCorrelationID(ctx) != "test-corr-id" {
			t.Errorf("expected correlation ID 'test-corr-id', got '%s'", GetCorrelationID(ctx))
		}
		return "ok", nil
	}

	incomingMD := metadata.Pairs(
		"x-correlation-id", "test-corr-id",
		"x-run-id", "test-run-id",
	)
	ctx := metadata.NewIncomingContext(context.Background(), incomingMD)

	info := &grpc.UnaryServerInfo{FullMethod: "/sentinelmesh.v1.RunService/CreateRun"}
	resp, err := serverInterceptor(ctx, "req", info, handler)
	if err != nil {
		t.Fatalf("unexpected error from unary server interceptor: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected 'ok', got %v", resp)
	}
}

func TestContextMetadata(t *testing.T) {
	ctx := context.Background()
	ctx = WithRunID(ctx, "run-12345")
	ctx = WithCorrelationID(ctx, "corr-abcde")
	ctx = WithExecutionGeneration(ctx, 3)
	ctx = WithAgentID(ctx, "agent-007")
	ctx = WithTenantID(ctx, "tenant-acme")

	if GetRunID(ctx) != "run-12345" {
		t.Errorf("expected run-12345, got %s", GetRunID(ctx))
	}
	if GetCorrelationID(ctx) != "corr-abcde" {
		t.Errorf("expected corr-abcde, got %s", GetCorrelationID(ctx))
	}
	if GetExecutionGeneration(ctx) != 3 {
		t.Errorf("expected generation 3, got %d", GetExecutionGeneration(ctx))
	}
	if GetAgentID(ctx) != "agent-007" {
		t.Errorf("expected agent-007, got %s", GetAgentID(ctx))
	}
	if GetTenantID(ctx) != "tenant-acme" {
		t.Errorf("expected tenant-acme, got %s", GetTenantID(ctx))
	}
}

func TestPrometheusMetrics_LowCardinality(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := InitMetrics(reg)

	m.RunsCreatedTotal.WithLabelValues("standard").Inc()
	m.SchedulerDecisionsTotal.WithLabelValues("SUCCESS", "v1").Inc()
	m.CheckpointSavedTotal.WithLabelValues("inline").Inc()
	m.RecoverySuccessTotal.WithLabelValues("cluster-us-east").Inc()
	m.RecoveryPointSteps.WithLabelValues("crawler").Observe(25)
	m.VerificationTotal.WithLabelValues("attestation_engine").Inc()
	m.PolicyEvaluationsTotal.WithLabelValues("restricted", "filesystem").Inc()

	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather prometheus metrics: %v", err)
	}

	foundMetrics := make(map[string]bool)
	for _, mf := range metricFamilies {
		foundMetrics[mf.GetName()] = true
	}

	expected := []string{
		"sentinel_runs_created_total",
		"sentinel_scheduler_decisions_total",
		"sentinel_checkpoint_saved_total",
		"sentinel_recovery_success_total",
		"sentinel_recovery_recovery_point_steps",
		"sentinel_verification_total",
		"sentinel_policy_evaluations_total",
	}

	for _, name := range expected {
		if !foundMetrics[name] {
			t.Errorf("metric %s not found in gathered registry", name)
		}
	}
}

func TestMetricsHandler_HTTPScrape(t *testing.T) {
	handler := MetricsHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from /metrics, got %d", rec.Code)
	}

	body := rec.Body.String()
	if len(body) == 0 {
		t.Errorf("expected non-empty metrics output")
	}
}

func TestLogging_RedactionAndTraceInjection(t *testing.T) {
	var buf bytes.Buffer
	logger := InitLogging("test-logger", &buf, slog.LevelInfo)

	ctx := context.Background()
	ctx = WithRunID(ctx, "run-secret-01")

	logger.InfoContext(ctx, "processing step",
		slog.String("token", "super_secret_token_123"),
		slog.String("password", "p@ssword!"),
		slog.String("step", "fetch_data"),
	)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}

	if logEntry["token"] != "[REDACTED]" {
		t.Errorf("expected token to be [REDACTED], got %v", logEntry["token"])
	}
	if logEntry["password"] != "[REDACTED]" {
		t.Errorf("expected password to be [REDACTED], got %v", logEntry["password"])
	}
	if logEntry["run_id"] != "run-secret-01" {
		t.Errorf("expected run_id run-secret-01, got %v", logEntry["run_id"])
	}
	if logEntry["service"] != "test-logger" {
		t.Errorf("expected service test-logger, got %v", logEntry["service"])
	}
}
