package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

func TestLiveProvider_ExplicitExecutionModes(t *testing.T) {
	registry := router.NewDefaultModelRegistry()
	ctx := context.Background()

	req := router.ModelInvocationRequest{
		TaskID:         "mode-test-task",
		RunID:          "mode-test-run",
		Prompt:         "Validate execution modes",
		TaskComplexity: router.ComplexitySimple,
		MaxTokens:      100,
	}

	// 1. Dry Run Mode
	pDry := router.NewLiveModelProvider(registry, router.ModeDryRun, false)
	respDry, err := pDry.Invoke(ctx, "small-fast-v1", req)
	if err != nil {
		t.Fatalf("Dry run failed: %v", err)
	}
	if respDry.ActualCostUSD <= 0 {
		t.Errorf("Dry run must estimate positive cost")
	}

	// 2. Synthetic Mode
	pSynth := router.NewLiveModelProvider(registry, router.ModeSynthetic, false)
	respSynth, err := pSynth.Invoke(ctx, "small-fast-v1", req)
	if err != nil {
		t.Fatalf("Synthetic mode failed: %v", err)
	}
	if respSynth.Content == "" {
		t.Errorf("Synthetic invocation returned empty content")
	}

	// 3. Live Mode without endpoint must NOT silently fall back to synthetic
	pLive := router.NewLiveModelProvider(registry, router.ModeLive, false)
	_, err = pLive.Invoke(ctx, "small-fast-v1", req)
	if err == nil {
		t.Fatalf("Expected error in Live mode when endpoint is unconfigured; got success (proves no silent fallback)")
	}
}

func TestLiveProvider_OpenAIProtocolFormattingAndResponseParsing(t *testing.T) {
	registry := router.NewDefaultModelRegistry()
	ctx := context.Background()

	// Mock OpenAI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-test-key-123" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": "Formal proof verified for consensus module.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     45,
				"completion_tokens": 12,
				"total_tokens":      57,
			},
		})
	}))
	defer server.Close()

	provider := router.NewLiveModelProvider(registry, router.ModeLive, false)
	provider.SetEndpoint("small-fast-v1", router.ProviderEndpointConfig{
		Type:        router.ProviderTypeOpenAI,
		BaseURL:     server.URL,
		APIKey:      "sk-test-key-123",
		ModelTarget: "gpt-4o-mini",
		Timeout:     5 * time.Second,
	})

	resp, err := provider.Invoke(ctx, "small-fast-v1", router.ModelInvocationRequest{
		TaskID:         "test-openai",
		Prompt:         "Verify consensus proof",
		TaskComplexity: router.ComplexitySimple,
		MaxTokens:      50,
	})

	if err != nil {
		t.Fatalf("Live invocation failed: %v", err)
	}
	if resp.Content != "Formal proof verified for consensus module." {
		t.Errorf("Unexpected content: %s", resp.Content)
	}
	if resp.PromptTokens != 45 || resp.CompletionTokens != 12 {
		t.Errorf("Unexpected tokens: prompt=%d completion=%d", resp.PromptTokens, resp.CompletionTokens)
	}
	if resp.ActualLatency <= 0 {
		t.Errorf("Actual latency must be measured and positive")
	}
}

func TestLiveProvider_AnthropicProtocolFormatting(t *testing.T) {
	registry := router.NewDefaultModelRegistry()
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("x-api-key")
		version := r.Header.Get("anthropic-version")
		if apiKey != "ant-key-789" || version != "2023-06-01" {
			http.Error(w, "Invalid headers", http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "msg_test",
			"type":    "message",
			"role":    "assistant",
			"content": []map[string]string{{"type": "text", "text": "Anthropic response OK"}},
			"usage":   map[string]int{"input_tokens": 30, "output_tokens": 10},
		})
	}))
	defer server.Close()

	provider := router.NewLiveModelProvider(registry, router.ModeLive, false)
	provider.SetEndpoint("medium-balanced-v1", router.ProviderEndpointConfig{
		Type:        router.ProviderTypeAnthropic,
		BaseURL:     server.URL,
		APIKey:      "ant-key-789",
		ModelTarget: "claude-3-5-sonnet-20241022",
	})

	resp, err := provider.Invoke(ctx, "medium-balanced-v1", router.ModelInvocationRequest{
		TaskID:         "test-anthropic",
		Prompt:         "Refactor actor system",
		TaskComplexity: router.ComplexityModerate,
		MaxTokens:      50,
	})

	if err != nil {
		t.Fatalf("Anthropic invocation failed: %v", err)
	}
	if resp.ModelID != "medium-balanced-v1" {
		t.Errorf("Expected model ID medium-balanced-v1, got %s", resp.ModelID)
	}
}

func TestLiveProvider_ErrorClassificationAnd429Handling(t *testing.T) {
	registry := router.NewDefaultModelRegistry()
	ctx := context.Background()

	// Server that returns 429 Too Many Requests
	server429 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
	}))
	defer server429.Close()

	provider := router.NewLiveModelProvider(registry, router.ModeLive, false)
	provider.SetEndpoint("small-fast-v1", router.ProviderEndpointConfig{
		Type:        router.ProviderTypeOpenAI,
		BaseURL:     server429.URL,
		APIKey:      "sk-test-key",
		ModelTarget: "gpt-4o-mini",
	})

	_, err := provider.Invoke(ctx, "small-fast-v1", router.ModelInvocationRequest{
		TaskID: "test-429",
		Prompt: "Test rate limit",
	})

	if err == nil {
		t.Fatalf("Expected 429 error, got nil")
	}

	infraErr, ok := err.(*router.InfrastructureError)
	if !ok || infraErr.Code != "429" {
		t.Errorf("Expected InfrastructureError with code 429, got %T: %v", err, err)
	}
}
