package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	globalLogger *slog.Logger
	globalLogMu  sync.RWMutex
)

var sensitiveKeys = map[string]bool{
	"password":       true,
	"secret":         true,
	"token":          true,
	"authorization":  true,
	"api_key":        true,
	"credential":     true,
	"private_key":    true,
	"state_inline":   true,
	"state_data":     true,
	"checkpoint_raw": true,
}

// ContextHandler wraps slog.JSONHandler to automatically inject trace, run, and correlation metadata.
type ContextHandler struct {
	slog.Handler
	serviceName string
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	// Inject service
	if h.serviceName != "" {
		r.AddAttrs(slog.String("service", h.serviceName))
	}

	// Inject trace_id and span_id from OpenTelemetry
	traceID, spanID := GetTraceAndSpanID(ctx)
	if traceID != "" {
		r.AddAttrs(slog.String("trace_id", traceID))
	}
	if spanID != "" {
		r.AddAttrs(slog.String("span_id", spanID))
	}

	// Inject Sentinel context metadata
	if runID := GetRunID(ctx); runID != "" {
		r.AddAttrs(slog.String("run_id", runID))
	}
	if agentID := GetAgentID(ctx); agentID != "" {
		r.AddAttrs(slog.String("agent_id", agentID))
	}
	if tenantID := GetTenantID(ctx); tenantID != "" {
		r.AddAttrs(slog.String("tenant_id", tenantID))
	}
	if corrID := GetCorrelationID(ctx); corrID != "" {
		r.AddAttrs(slog.String("correlation_id", corrID))
	}
	if gen := GetExecutionGeneration(ctx); gen > 0 {
		r.AddAttrs(slog.Int("execution_generation", gen))
	}

	return h.Handler.Handle(ctx, r)
}

// SanitizeReplaceAttr sanitizes sensitive fields from log attributes.
func SanitizeReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	keyLower := strings.ToLower(a.Key)
	if sensitiveKeys[keyLower] || strings.Contains(keyLower, "secret") || strings.Contains(keyLower, "token") {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}

// InitLogging initializes the structured JSON logger.
func InitLogging(serviceName string, out io.Writer, level slog.Level) *slog.Logger {
	globalLogMu.Lock()
	defer globalLogMu.Unlock()

	if out == nil {
		out = os.Stdout
	}

	jsonHandler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: SanitizeReplaceAttr,
	})

	handler := &ContextHandler{
		Handler:     jsonHandler,
		serviceName: serviceName,
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	globalLogger = logger

	return logger
}

// Logger returns the global logger.
func Logger() *slog.Logger {
	globalLogMu.RLock()
	defer globalLogMu.RUnlock()

	if globalLogger == nil {
		return slog.Default()
	}
	return globalLogger
}
