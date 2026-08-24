package router

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages the catalog of registered model endpoints.
type Registry interface {
	Version() string
	GetModel(ctx context.Context, id string) (ModelDefinition, error)
	ListModels(ctx context.Context) ([]ModelDefinition, error)
	Snapshot(ctx context.Context) ([]ModelDefinition, error)
	RegisterModel(ctx context.Context, model ModelDefinition) error
	UpdateHealth(ctx context.Context, id string, status ModelHealthStatus, metrics ObservedModelMetrics) error
}

// MemoryRegistry provides an in-memory thread-safe implementation of Registry.
type MemoryRegistry struct {
	mu      sync.RWMutex
	version string
	models  map[string]ModelDefinition
}

// NewMemoryRegistry creates an empty MemoryRegistry.
func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		version: "registry-v1.0",
		models:  make(map[string]ModelDefinition),
	}
}

// Version returns the catalog schema/version tag.
func (r *MemoryRegistry) Version() string {
	return r.version
}

// NewDefaultModelRegistry pre-populates standard model tiers for evaluation.
func NewDefaultModelRegistry() *MemoryRegistry {
	r := NewMemoryRegistry()
	ctx := context.Background()

	// 1. Small Tier (Fast, ultra-low cost, high throughput)
	_ = r.RegisterModel(ctx, ModelDefinition{
		ID:                    "small-fast-v1",
		Name:                  "Small Fast Model",
		Tier:                  TierSmall,
		Provider:              "synthetic-local",
		CostPer1kInputTokens:  0.00015,
		CostPer1kOutputTokens: 0.00060,
		NominalP50LatencyMs:   45.0,
		NominalP95LatencyMs:   80.0,
		BaseOverheadMs:        20.0,
		InputMsPer1kTokens:    10.0,
		OutputMsPer1kTokens:   25.0,
		ContextWindow:         8192,
		SecurityClasses:       []string{"public", "standard", "restricted", "airgapped"},
		HealthStatus:          HealthHealthy,
		TaskQualityMatrix: map[TaskComplexity]float64{
			ComplexitySimple:         0.93,
			ComplexityModerate:       0.74,
			ComplexityComplex:        0.42,
			ComplexityReasoningHeavy: 0.20,
		},
	})

	// 2. Medium Tier (Balanced reasoning, moderate cost & latency)
	_ = r.RegisterModel(ctx, ModelDefinition{
		ID:                    "medium-balanced-v1",
		Name:                  "Medium Balanced Model",
		Tier:                  TierMedium,
		Provider:              "synthetic-cloud",
		CostPer1kInputTokens:  0.00150,
		CostPer1kOutputTokens: 0.00600,
		NominalP50LatencyMs:   220.0,
		NominalP95LatencyMs:   380.0,
		BaseOverheadMs:        80.0,
		InputMsPer1kTokens:    30.0,
		OutputMsPer1kTokens:   120.0,
		ContextWindow:         32768,
		SecurityClasses:       []string{"public", "standard", "restricted"},
		HealthStatus:          HealthHealthy,
		TaskQualityMatrix: map[TaskComplexity]float64{
			ComplexitySimple:         0.97,
			ComplexityModerate:       0.91,
			ComplexityComplex:        0.78,
			ComplexityReasoningHeavy: 0.58,
		},
	})

	// 3. Large Tier (State-of-the-art deep reasoning, higher cost & latency)
	_ = r.RegisterModel(ctx, ModelDefinition{
		ID:                    "large-reasoning-v1",
		Name:                  "Large Reasoning Model",
		Tier:                  TierLarge,
		Provider:              "synthetic-cloud",
		CostPer1kInputTokens:  0.01500,
		CostPer1kOutputTokens: 0.06000,
		NominalP50LatencyMs:   950.0,
		NominalP95LatencyMs:   1600.0,
		BaseOverheadMs:        250.0,
		InputMsPer1kTokens:    80.0,
		OutputMsPer1kTokens:   650.0,
		ContextWindow:         131072,
		SecurityClasses:       []string{"public", "standard"},
		HealthStatus:          HealthHealthy,
		TaskQualityMatrix: map[TaskComplexity]float64{
			ComplexitySimple:         0.99,
			ComplexityModerate:       0.97,
			ComplexityComplex:        0.95,
			ComplexityReasoningHeavy: 0.93,
		},
	})

	return r
}

// GetModel retrieves a model definition by ID.
func (r *MemoryRegistry) GetModel(ctx context.Context, id string) (ModelDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, ok := r.models[id]
	if !ok {
		return ModelDefinition{}, fmt.Errorf("model %s not found in registry", id)
	}
	return model, nil
}

// ListModels returns all registered models.
func (r *MemoryRegistry) ListModels(ctx context.Context) ([]ModelDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]ModelDefinition, 0, len(r.models))
	for _, m := range r.models {
		models = append(models, m)
	}
	return models, nil
}

// Snapshot creates an immutable point-in-time snapshot of the catalog for replay auditing.
func (r *MemoryRegistry) Snapshot(ctx context.Context) ([]ModelDefinition, error) {
	return r.ListModels(ctx)
}

// RegisterModel registers or updates a model definition.
func (r *MemoryRegistry) RegisterModel(ctx context.Context, model ModelDefinition) error {
	if model.ID == "" {
		return fmt.Errorf("model ID cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.models[model.ID] = model
	return nil
}

// UpdateHealth updates health status and telemetry observations for a model.
func (r *MemoryRegistry) UpdateHealth(ctx context.Context, id string, status ModelHealthStatus, metrics ObservedModelMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, ok := r.models[id]
	if !ok {
		return fmt.Errorf("model %s not found", id)
	}
	model.HealthStatus = status
	model.ObservedMetrics = metrics
	r.models[id] = model
	return nil
}
