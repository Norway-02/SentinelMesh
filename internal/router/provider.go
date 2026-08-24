package router

import (
	"context"
	"encoding/json"
	"time"
)

// ModelInvocationRequest encapsulates parameters for model inference.
type ModelInvocationRequest struct {
	TaskID         string         `json:"task_id"`
	RunID          string         `json:"run_id"`
	AgentID        string         `json:"agent_id"`
	TenantID       string         `json:"tenant_id"`
	Prompt         string         `json:"prompt"`
	TaskComplexity TaskComplexity `json:"task_complexity"`
	MaxTokens      int            `json:"max_tokens"`
	Temperature    float64        `json:"temperature"`
	CostBudgetUSD  float64        `json:"cost_budget_usd"`
	Timeout        time.Duration  `json:"timeout"`
}

// ModelInvocationResponse encapsulates the output and actual measured metrics of an inference.
type ModelInvocationResponse struct {
	TaskID           string        `json:"task_id"`
	RunID            string        `json:"run_id"`
	ModelID          string        `json:"model_id"`
	Provider         string        `json:"provider"`
	ProviderModelID  string        `json:"provider_model_id"`
	DataSource       string        `json:"data_source"`
	Content          string        `json:"content"`
	FinishReason     string        `json:"finish_reason"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalTokens      int           `json:"total_tokens"`
	EstimatedCostUSD float64       `json:"estimated_cost_usd"`
	ActualCostUSD    float64       `json:"actual_cost_usd"`
	ActualLatency    time.Duration `json:"actual_latency"`
	QualityScore     float64       `json:"quality_score"`
	QualityStatus    string        `json:"quality_status"`
	FallbackUsed     bool          `json:"fallback_used"`
	AttemptNumber    int           `json:"attempt_number"`
}

// MarshalJSON provides explicit actual_latency_ms in float64 milliseconds in serialized JSON payloads.
func (m *ModelInvocationResponse) MarshalJSON() ([]byte, error) {
	type Alias ModelInvocationResponse
	return json.Marshal(&struct {
		*Alias
		ActualLatencyMs float64 `json:"actual_latency_ms"`
	}{
		Alias:           (*Alias)(m),
		ActualLatencyMs: float64(m.ActualLatency) / float64(time.Millisecond),
	})
}

// ModelProvider defines the contract for dispatching inference to model backends.
type ModelProvider interface {
	Invoke(ctx context.Context, modelID string, req ModelInvocationRequest) (*ModelInvocationResponse, error)
}
