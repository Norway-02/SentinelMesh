package rest_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/config"
	"github.com/sentinelmesh/sentinelmesh/internal/onlinepolicy"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
	"github.com/sentinelmesh/sentinelmesh/internal/transport/rest"
)

func setupTestMux() (*http.ServeMux, router.Registry) {
	mux := http.NewServeMux()

	modelRegistry := router.NewDefaultModelRegistry()
	syntheticProvider := router.NewSyntheticModelProvider(modelRegistry, false)
	liveProvider := router.NewLiveModelProvider(modelRegistry, router.ModeSynthetic, false)
	decisionRepo := router.NewMemoryDecisionRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	routerSvc := router.NewService(modelRegistry, liveProvider, decisionRepo, outboxRepo, txManager)
	learningStore := adaptive.NewMemoryLearningStore()
	driftDetector := adaptive.NewDualWindowDriftDetector()
	prior := adaptive.DefaultBetaPrior()

	adaptiveSvc := adaptive.NewAdaptiveService(modelRegistry, syntheticProvider, learningStore, driftDetector, prior, decisionRepo, outboxRepo, txManager)
	policyMgr := onlinepolicy.NewPolicyManager(onlinepolicy.DefaultPolicyState())
	guardrails := onlinepolicy.NewGuardrailEnforcer(onlinepolicy.DefaultGuardrailConfig())
	policySvc := onlinepolicy.NewOnlinePolicyService(modelRegistry, syntheticProvider, learningStore, prior, policyMgr, guardrails, decisionRepo, outboxRepo, txManager)

	sseHub := rest.NewSSEHub()
	cfg := &config.Config{
		HTTPAddr:    "127.0.0.1:8787",
		LogLevel:    "info",
		Environment: "development",
	}

	rest.NewModelHandler(modelRegistry, liveProvider).RegisterRoutes(mux)
	rest.NewRouterHandler(routerSvc, decisionRepo).RegisterRoutes(mux)
	rest.NewAdaptiveHandler(adaptiveSvc).RegisterRoutes(mux)
	rest.NewPolicyHandler(policySvc, adaptiveSvc, routerSvc, policyMgr).RegisterRoutes(mux)
	rest.NewMetricsHandler(decisionRepo, modelRegistry).RegisterRoutes(mux)
	rest.NewEventsHandler(outboxRepo, sseHub).RegisterRoutes(mux)
	rest.NewSettingsHandler(cfg, modelRegistry, policyMgr, liveProvider).RegisterRoutes(mux)

	return mux, modelRegistry
}

func TestListModelsEndpoint(t *testing.T) {
	mux, _ := setupTestMux()
	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Models []rest.ModelResponse `json:"models"`
		Count  int                  `json:"count"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if resp.Count == 0 || len(resp.Models) == 0 {
		t.Errorf("expected non-empty models array, got %d", resp.Count)
	}
}

func TestPolicyExecuteEndpoint(t *testing.T) {
	mux, _ := setupTestMux()

	reqBody, _ := json.Marshal(map[string]interface{}{
		"prompt":              "Test prompt for pipeline execution",
		"task_complexity":     "moderate",
		"routing_policy":      "balanced",
		"quality_requirement": 0.70,
		"latency_sla_ms":     2000.0,
		"cost_budget_usd":    0.10,
	})

	req := httptest.NewRequest("POST", "/v1/policy/execute", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp rest.ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Stage17Decision == nil {
		t.Errorf("expected Stage17 decision in response")
	}
	if resp.Stage18Decision == nil {
		t.Errorf("expected Stage18 decision in response")
	}
	if resp.Stage19Decision == nil {
		t.Errorf("expected Stage19 decision in response")
	}
	if resp.InvocationResult == nil {
		t.Errorf("expected InvocationResult in response")
	}
}

func TestMetricsSummaryEndpoint(t *testing.T) {
	mux, _ := setupTestMux()
	req := httptest.NewRequest("GET", "/v1/metrics/summary", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var summary rest.MetricsSummary
	if err := json.NewDecoder(w.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode metrics summary: %v", err)
	}

	if summary.ActiveProviders == 0 {
		t.Errorf("expected active providers > 0")
	}
}

func TestSettingsEndpoint(t *testing.T) {
	mux, _ := setupTestMux()
	req := httptest.NewRequest("GET", "/v1/settings", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var settings map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&settings); err != nil {
		t.Fatalf("failed to decode settings: %v", err)
	}

	if settings["http_addr"] != "127.0.0.1:8787" {
		t.Errorf("expected http_addr 127.0.0.1:8787, got %v", settings["http_addr"])
	}
}

func TestCORSPreflight(t *testing.T) {
	mux, _ := setupTestMux()
	handler := rest.Middleware(mux, rest.CORSMiddleware)

	req := httptest.NewRequest("OPTIONS", "/v1/models", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8900")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 No Content for OPTIONS preflight, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin '*', got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

