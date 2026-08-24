package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/config"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
	resthandler "github.com/sentinelmesh/sentinelmesh/internal/transport/rest"
)

func TestLiveOpenAIProvider(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	liveTestsEnabled := os.Getenv("LIVE_PROVIDER_TESTS") == "true"
	execMode := os.Getenv("SENTINEL_EXECUTION_MODE")

	if !liveTestsEnabled || apiKey == "" || strings.ToUpper(execMode) != "LIVE" {
		t.Skip("LIVE_PROVIDER_TESTS=true, OPENAI_API_KEY, and SENTINEL_EXECUTION_MODE=LIVE required for live OpenAI E2E test - skipping")
	}

	modelRegistry := router.NewDefaultModelRegistry()
	liveProvider := router.NewLiveModelProvider(modelRegistry, router.ModeLive, false)

	modelTarget := os.Getenv("OPENAI_MODEL")
	if modelTarget == "" {
		modelTarget = "gpt-4o-mini"
	}

	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	endpointCfg := router.ProviderEndpointConfig{
		Type:        router.ProviderTypeOpenAI,
		BaseURL:     baseURL,
		APIKey:      apiKey,
		ModelTarget: modelTarget,
		Timeout:     30 * time.Second,
	}
	liveProvider.SetEndpoint("medium-balanced-v1", endpointCfg)

	cfg := &config.Config{
		Environment:         "development",
		ExecutionMode:       "LIVE",
		OpenAIAPIKey:        apiKey,
		OpenAIModel:         modelTarget,
		OpenAIBaseURL:       baseURL,
		EnableTestEndpoints: true,
	}

	handler := resthandler.NewProviderHandler(cfg, liveProvider)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	// 1. Health Check Test
	t.Run("OpenAI Provider Health Check", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/v1/providers/openai/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200 OK from health check, got %d", resp.StatusCode)
		}

		var healthResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&healthResp)

		status, _ := healthResp["status"].(string)
		if healthResp["provider"] != "openai" || (status != "healthy" && status != "READY") {
			t.Fatalf("Unexpected health response: %v", healthResp)
		}
	})

	// 2. Live Tiny Test Prompt Invocation
	t.Run("Live Tiny Prompt Request", func(t *testing.T) {
		payload := map[string]string{
			"prompt": "Reply with exactly: SENTINELMESH_LIVE_TEST",
		}
		bodyBytes, _ := json.Marshal(payload)

		resp, err := http.Post(server.URL+"/v1/providers/openai/test", "application/json", bytes.NewBuffer(bodyBytes))
		if err != nil {
			t.Fatalf("Post test prompt failed: %v", err)
		}
		defer resp.Body.Close()

		var testResp struct {
			Provider       string  `json:"provider"`
			Output         string  `json:"output"`
			Error          string  `json:"error"`
			LatencyMS      int64   `json:"latency_ms"`
			InputTokens    int     `json:"input_tokens"`
			OutputTokens   int     `json:"output_tokens"`
			ActualCostUSD  float64 `json:"actual_cost_usd"`
			Success        bool    `json:"success"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&testResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.StatusCode == http.StatusOK {
			if !testResp.Success {
				t.Fatalf("Expected success=true for 200 OK, got false")
			}
			if testResp.InputTokens <= 0 || testResp.OutputTokens <= 0 {
				t.Fatalf("Expected non-zero token counts, got input=%d output=%d", testResp.InputTokens, testResp.OutputTokens)
			}
			if strings.Contains(testResp.Output, apiKey) {
				t.Fatalf("SECURITY VIOLATION: API key leaked in response output!")
			}
			t.Logf("LIVE OPENAI CALL SUCCESSFUL: output=%s, tokens=%d, latency=%dms", testResp.Output, testResp.InputTokens+testResp.OutputTokens, testResp.LatencyMS)
		} else {
			// Verify quota/rate limit 429 response from OpenAI
			if !strings.Contains(testResp.Error, "openai") {
				t.Fatalf("Expected OpenAI provider error payload, got: %s", testResp.Error)
			}
			if strings.Contains(testResp.Error, apiKey) {
				t.Fatalf("SECURITY VIOLATION: API key leaked in error output!")
			}
			t.Logf("LIVE OPENAI TRANSPORT VERIFIED: provider=%s, response error=%s", testResp.Provider, testResp.Error)
		}
	})
}
