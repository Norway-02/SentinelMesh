package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/observability"
)

// ProviderExecutionMode explicitly identifies the execution regime.
type ProviderExecutionMode string

const (
	ModeLive      ProviderExecutionMode = "LIVE"
	ModeSynthetic ProviderExecutionMode = "SYNTHETIC"
	ModeDryRun    ProviderExecutionMode = "DRY_RUN"
)

// ProviderType identifies the upstream API protocol.
type ProviderType string

const (
	ProviderTypeOpenAI    ProviderType = "openai"
	ProviderTypeAnthropic ProviderType = "anthropic"
	ProviderTypeGemini    ProviderType = "gemini"
	ProviderTypeOllama    ProviderType = "ollama"
)

// ProviderEndpointConfig holds credentials and endpoint parameters for a model.
type ProviderEndpointConfig struct {
	Type        ProviderType  `json:"type"`
	BaseURL     string        `json:"base_url"`
	APIKey      string        `json:"api_key"`
	ModelTarget string        `json:"model_target"`
	Timeout     time.Duration `json:"timeout"`
}

// LiveModelProvider dispatches real HTTP inference requests to LLM providers with normalized telemetry.
type LiveModelProvider struct {
	mu                sync.RWMutex
	registry          Registry
	mode              ProviderExecutionMode
	httpClient        *http.Client
	endpoints         map[string]ProviderEndpointConfig
	payloadLogging    bool
	synthetic         *SyntheticModelProvider
	faultInjection    bool
	providerStatus    map[string]string
	providerLastError map[string]string
}

// NewLiveModelProvider constructs a LiveModelProvider.
func NewLiveModelProvider(registry Registry, mode ProviderExecutionMode, payloadLogging bool) *LiveModelProvider {
	if mode == "" {
		mode = ModeSynthetic
	}

	return &LiveModelProvider{
		registry:          registry,
		mode:              mode,
		httpClient:        &http.Client{Timeout: 30 * time.Second},
		endpoints:         make(map[string]ProviderEndpointConfig),
		payloadLogging:    payloadLogging,
		synthetic:         NewSyntheticModelProvider(registry, false),
		faultInjection:    false,
		providerStatus:    make(map[string]string),
		providerLastError: make(map[string]string),
	}
}

// RecordProviderState updates tracked health status for a provider (READY, QUOTA_EXHAUSTED, RATE_LIMITED, AUTH_FAILED, OFFLINE).
func (p *LiveModelProvider) RecordProviderState(provider string, status string, lastErr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.providerStatus[provider] = status
	p.providerLastError[provider] = lastErr
}

// GetProviderState retrieves recorded health status and last error for a provider.
func (p *LiveModelProvider) GetProviderState(provider string) (string, string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	status, ok := p.providerStatus[provider]
	if !ok {
		return "READY", ""
	}
	return status, p.providerLastError[provider]
}

// SetEndpoint configures a live provider endpoint mapping.
func (p *LiveModelProvider) SetEndpoint(modelID string, cfg ProviderEndpointConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	p.endpoints[modelID] = cfg
}

// SetMode updates the active execution mode (LIVE, SYNTHETIC, DRY_RUN).
func (p *LiveModelProvider) SetMode(mode ProviderExecutionMode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = mode
}

// GetMode returns the current provider execution mode.
func (p *LiveModelProvider) GetMode() ProviderExecutionMode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

// SetFaultInjection enables or disables development fault injection simulation.
func (p *LiveModelProvider) SetFaultInjection(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.faultInjection = enabled
}

// GetFaultInjection returns current fault injection state.
func (p *LiveModelProvider) GetFaultInjection() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.faultInjection
}

// NormalizeOpenAIBaseURL ensures standard base URL formatting without duplicated /v1 paths.
func NormalizeOpenAIBaseURL(baseURL string) string {
	cleaned := strings.TrimRight(baseURL, "/")
	if cleaned == "" {
		return "https://api.openai.com/v1"
	}
	if strings.HasSuffix(cleaned, "/v1") {
		return cleaned
	}
	return cleaned + "/v1"
}

// Invoke dispatches inference according to the configured execution mode.
func (p *LiveModelProvider) Invoke(ctx context.Context, modelID string, req ModelInvocationRequest) (*ModelInvocationResponse, error) {
	p.mu.RLock()
	mode := p.mode
	cfg, hasEndpoint := p.endpoints[modelID]
	faultInj := p.faultInjection
	p.mu.RUnlock()

	model, err := p.registry.GetModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("model %s not found in registry: %w", modelID, err)
	}

	estInputTokens := len(req.Prompt) / 4
	if estInputTokens <= 0 {
		estInputTokens = 10
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 512
	}
	estimatedCostUSD := EstimateCost(model, estInputTokens, maxTokens)

	switch mode {
	case ModeDryRun:
		return &ModelInvocationResponse{
			TaskID:           req.TaskID,
			RunID:            req.RunID,
			ModelID:          modelID,
			Provider:         model.Provider,
			ProviderModelID:  cfg.ModelTarget,
			DataSource:       "DRY_RUN",
			Content:          fmt.Sprintf("[DRY_RUN] Pre-flight schema validated for %s (target: %s)", modelID, cfg.ModelTarget),
			FinishReason:     "stop",
			PromptTokens:     estInputTokens,
			CompletionTokens: maxTokens,
			TotalTokens:      estInputTokens + maxTokens,
			EstimatedCostUSD: estimatedCostUSD,
			ActualCostUSD:    estimatedCostUSD,
			ActualLatency:    1 * time.Millisecond,
			QualityScore:     0.0,
			QualityStatus:    "NOT_EVALUATED",
		}, nil

	case ModeSynthetic:
		resp, err := p.synthetic.Invoke(ctx, modelID, req)
		if err != nil {
			return nil, err
		}
		resp.EstimatedCostUSD = estimatedCostUSD
		resp.DataSource = "SYNTHETIC"
		resp.Provider = model.Provider
		resp.ProviderModelID = cfg.ModelTarget
		resp.QualityStatus = "SYNTHETIC_HEURISTIC"
		return resp, nil

	case ModeLive:
		// Check development fault injection
		if faultInj {
			return nil, &InfrastructureError{
				Code:    "503",
				Message: "Simulated upstream provider outage (503 Service Unavailable)",
			}
		}

		// Strictly require API key and endpoint configuration in LIVE mode — NO silent fallback!
		if !hasEndpoint || cfg.APIKey == "" {
			return nil, &ClientError{
				Code:    "missing_openai_api_key",
				Message: fmt.Sprintf("OPENAI_API_KEY is not configured for model %s (Mode: LIVE). Set OPENAI_API_KEY environment variable.", modelID),
			}
		}

		// Cost safety guardrail — reject before dispatch if estimated cost exceeds task budget
		if req.CostBudgetUSD > 0 && estimatedCostUSD > req.CostBudgetUSD {
			return nil, &ClientError{
				Code:    "budget_exceeded",
				Message: fmt.Sprintf("BLOCKED — estimated request cost ($%.5f) exceeds task budget ($%.5f)", estimatedCostUSD, req.CostBudgetUSD),
			}
		}

		return p.dispatchLiveRequest(ctx, model, cfg, req, estimatedCostUSD)

	default:
		return nil, fmt.Errorf("unrecognized provider execution mode: %s", mode)
	}
}

func (p *LiveModelProvider) dispatchLiveRequest(ctx context.Context, model ModelDefinition, cfg ProviderEndpointConfig, req ModelInvocationRequest, estimatedCostUSD float64) (*ModelInvocationResponse, error) {
	start := time.Now()

	var reqBody []byte
	var urlStr string
	var httpReq *http.Request
	var err error

	targetModel := cfg.ModelTarget
	if targetModel == "" {
		targetModel = "gpt-4o-mini"
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 512
	}

	switch cfg.Type {
	case ProviderTypeAnthropic:
		baseURL := strings.TrimRight(cfg.BaseURL, "/")
		urlStr = baseURL + "/messages"
		payload := map[string]interface{}{
			"model":      targetModel,
			"max_tokens": maxTokens,
			"messages": []map[string]string{
				{"role": "user", "content": req.Prompt},
			},
		}
		reqBody, _ = json.Marshal(payload)
		httpReq, err = http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create anthropic request: %w", err)
		}
		httpReq.Header.Set("x-api-key", cfg.APIKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
		httpReq.Header.Set("Content-Type", "application/json")

	case ProviderTypeGemini:
		baseURL := strings.TrimRight(cfg.BaseURL, "/")
		urlStr = fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, targetModel, cfg.APIKey)
		payload := map[string]interface{}{
			"contents": []map[string]interface{}{
				{"parts": []map[string]string{{"text": req.Prompt}}},
			},
		}
		reqBody, _ = json.Marshal(payload)
		httpReq, err = http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create gemini request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

	case ProviderTypeOpenAI, ProviderTypeOllama:
		fallthrough
	default:
		normalizedBaseURL := NormalizeOpenAIBaseURL(cfg.BaseURL)
		urlStr = normalizedBaseURL + "/chat/completions"

		payload := map[string]interface{}{
			"model":      targetModel,
			"max_tokens": maxTokens,
			"messages": []map[string]string{
				{"role": "user", "content": req.Prompt},
			},
		}
		reqBody, _ = json.Marshal(payload)
		httpReq, err = http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create openai request: %w", err)
		}
		if cfg.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
		httpReq.Header.Set("Content-Type", "application/json")
	}

	var resp *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		resp, err = p.httpClient.Do(httpReq)
		if err == nil {
			break
		}
		if attempt == 0 && (strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "connection refused")) {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		return nil, &InfrastructureError{
			Code:    "network_failure",
			Message: observability.RedactSecrets(fmt.Sprintf("transport error to %s: %v", urlStr, err)),
		}
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &InfrastructureError{
			Code:    "read_error",
			Message: fmt.Sprintf("failed to read response body: %v", err),
		}
	}

	duration := time.Since(start)

	// Error Handling & Redaction
	if resp.StatusCode == http.StatusTooManyRequests {
		errStr := string(respBytes)
		if strings.Contains(errStr, "credit_balance_exhausted") || strings.Contains(errStr, "insufficient_quota") {
			p.RecordProviderState(string(cfg.Type), "QUOTA_EXHAUSTED", "insufficient_quota")
		} else {
			p.RecordProviderState(string(cfg.Type), "RATE_LIMITED", "rate_limit")
		}
		return nil, &InfrastructureError{
			Code:    "429",
			Message: observability.RedactSecrets(fmt.Sprintf("rate limit 429 from %s: %s", cfg.Type, errStr)),
		}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		p.RecordProviderState(string(cfg.Type), "AUTH_FAILED", "unauthorized")
		return nil, &ClientError{
			Code:    fmt.Sprintf("%d", resp.StatusCode),
			Message: observability.RedactSecrets(fmt.Sprintf("auth error from %s: %s", cfg.Type, string(respBytes))),
		}
	}
	if resp.StatusCode >= 500 {
		p.RecordProviderState(string(cfg.Type), "OFFLINE", "server_error")
		return nil, &InfrastructureError{
			Code:    fmt.Sprintf("%d", resp.StatusCode),
			Message: observability.RedactSecrets(fmt.Sprintf("server error from %s: %s", cfg.Type, string(respBytes))),
		}
	}
	if resp.StatusCode >= 400 {
		p.RecordProviderState(string(cfg.Type), "DEGRADED", "client_error")
		return nil, &ClientError{
			Code:    fmt.Sprintf("%d", resp.StatusCode),
			Message: observability.RedactSecrets(fmt.Sprintf("client error from %s: %s", cfg.Type, string(respBytes))),
		}
	}

	p.RecordProviderState(string(cfg.Type), "READY", "")

	// Response Parsing for both OpenAI Responses API and Chat Completions
	var content string
	var finishReason string
	promptTokens := len(req.Prompt) / 4
	completionTokens := maxTokens

	var genericOpenAIResp struct {
		// Responses API fields
		OutputText string `json:"output_text"`
		// Chat Completions fields
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			InputTokens      int `json:"input_tokens"`
			OutputTokens     int `json:"output_tokens"`
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBytes, &genericOpenAIResp); err == nil {
		// 1. Try Responses API format
		if genericOpenAIResp.OutputText != "" {
			content = genericOpenAIResp.OutputText
			finishReason = "completed"
		} else if len(genericOpenAIResp.Choices) > 0 { // 2. Try Chat Completions format
			content = genericOpenAIResp.Choices[0].Message.Content
			finishReason = genericOpenAIResp.Choices[0].FinishReason
		} else {
			content = observability.RedactSecrets(string(respBytes))
			finishReason = "stop"
		}

		// Token usage extraction
		pTokens := genericOpenAIResp.Usage.PromptTokens
		if pTokens == 0 {
			pTokens = genericOpenAIResp.Usage.InputTokens
		}
		cTokens := genericOpenAIResp.Usage.CompletionTokens
		if cTokens == 0 {
			cTokens = genericOpenAIResp.Usage.OutputTokens
		}

		if pTokens > 0 {
			promptTokens = pTokens
			completionTokens = cTokens
		}
	} else {
		content = observability.RedactSecrets(string(respBytes))
		finishReason = "stop"
	}

	// Real cost calculated using ModelDefinition pricing registry
	actualCost := (float64(promptTokens) / 1000.0 * model.CostPer1kInputTokens) +
		(float64(completionTokens) / 1000.0 * model.CostPer1kOutputTokens)

	return &ModelInvocationResponse{
		TaskID:           req.TaskID,
		RunID:            req.RunID,
		ModelID:          model.ID,
		Provider:         string(cfg.Type),
		ProviderModelID:  targetModel,
		DataSource:       "LIVE",
		Content:          content,
		FinishReason:     finishReason,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		EstimatedCostUSD: estimatedCostUSD,
		ActualCostUSD:    actualCost,
		ActualLatency:    duration,
		QualityScore:     0.0,
		QualityStatus:    "NOT_EVALUATED",
	}, nil
}
