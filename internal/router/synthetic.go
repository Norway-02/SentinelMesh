package router

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// SyntheticFaultType defines controllable fault injection for testing.
type SyntheticFaultType string

const (
	FaultNone        SyntheticFaultType = "none"
	FaultRateLimit   SyntheticFaultType = "rate_limit_429"
	FaultTimeout     SyntheticFaultType = "timeout"
	FaultServerError SyntheticFaultType = "server_error_503"
)

// SyntheticModelProvider provides deterministic simulated model inference.
type SyntheticModelProvider struct {
	mu                sync.RWMutex
	registry          Registry
	faults            map[string]SyntheticFaultType
	faultCounts       map[string]int
	degradedQuality   map[string]float64
	latencyMultiplier map[string]float64
	emulateSleep      bool
}

// NewSyntheticModelProvider constructs a synthetic provider.
func NewSyntheticModelProvider(registry Registry, emulateSleep bool) *SyntheticModelProvider {
	return &SyntheticModelProvider{
		registry:          registry,
		faults:            make(map[string]SyntheticFaultType),
		faultCounts:       make(map[string]int),
		degradedQuality:   make(map[string]float64),
		latencyMultiplier: make(map[string]float64),
		emulateSleep:      emulateSleep,
	}
}

// SetDegradedMode injects a performance regression (quality drop and/or latency multiplier).
func (p *SyntheticModelProvider) SetDegradedMode(modelID string, qualityOverride float64, latencyMultiplier float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.degradedQuality[modelID] = qualityOverride
	p.latencyMultiplier[modelID] = latencyMultiplier
}

// SetFault configures fault injection for a specific model ID.
func (p *SyntheticModelProvider) SetFault(modelID string, fault SyntheticFaultType, count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.faults[modelID] = fault
	p.faultCounts[modelID] = count
}

// ClearFaults clears all configured fault injections.
func (p *SyntheticModelProvider) ClearFaults() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.faults = make(map[string]SyntheticFaultType)
	p.faultCounts = make(map[string]int)
}

// Invoke executes deterministic synthetic inference.
func (p *SyntheticModelProvider) Invoke(ctx context.Context, modelID string, req ModelInvocationRequest) (*ModelInvocationResponse, error) {
	// Check Fault Injection
	p.mu.Lock()
	if fault, ok := p.faults[modelID]; ok && p.faultCounts[modelID] > 0 {
		p.faultCounts[modelID]--
		p.mu.Unlock()
		switch fault {
		case FaultRateLimit:
			return nil, &InfrastructureError{Code: "429", Message: "rate_limit_429"}
		case FaultTimeout:
			return nil, &InfrastructureError{Code: "timeout", Message: "timeout"}
		case FaultServerError:
			return nil, &InfrastructureError{Code: "503", Message: "server_error_503"}
		}
	} else {
		p.mu.Unlock()
	}

	model, err := p.registry.GetModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("model lookup failed: %w", err)
	}

	// Deterministic token calculations based on prompt & task
	promptTokens := len(req.Prompt)/4 + 20
	if promptTokens < 10 {
		promptTokens = 10
	}
	completionTokens := req.MaxTokens
	if completionTokens <= 0 {
		completionTokens = 150
	}

	// Cost Calculation
	actualCost := EstimateCost(model, promptTokens, completionTokens)

	// Simulated Latency
	estimatedLatMs := EstimateLatency(model, promptTokens, completionTokens)
	p.mu.RLock()
	if mult, ok := p.latencyMultiplier[modelID]; ok && mult > 0 {
		estimatedLatMs *= mult
	}
	p.mu.RUnlock()
	actualLat := time.Duration(estimatedLatMs * float64(time.Millisecond))

	if p.emulateSleep && actualLat > 0 {
		// Cap sleep in test environments to 10ms for responsiveness
		sleepDur := actualLat
		if sleepDur > 10*time.Millisecond {
			sleepDur = 10 * time.Millisecond
		}
		time.Sleep(sleepDur)
	}

	// Deterministic Quality Score with slight prompt-based variance
	baseQuality := GetTaskQuality(model, req.TaskComplexity)
	p.mu.RLock()
	if override, ok := p.degradedQuality[modelID]; ok && override >= 0 {
		baseQuality = override
	}
	p.mu.RUnlock()

	h := sha256.Sum256([]byte(req.TaskID + modelID))
	seed := int64(binary.BigEndian.Uint64(h[:8]))
	rng := rand.New(rand.NewSource(seed))
	variance := (rng.Float64() - 0.5) * 0.04 // +/- 2%
	actualQuality := baseQuality + variance
	if actualQuality > 1.0 {
		actualQuality = 1.0
	} else if actualQuality < 0.0 {
		actualQuality = 0.0
	}

	return &ModelInvocationResponse{
		TaskID:           req.TaskID,
		RunID:            req.RunID,
		ModelID:          modelID,
		Content:          fmt.Sprintf("Deterministic output from %s for task %s [quality=%.2f]", modelID, req.TaskID, actualQuality),
		FinishReason:     "stop",
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		ActualCostUSD:    actualCost,
		ActualLatency:    actualLat,
		QualityScore:     actualQuality,
	}, nil
}
