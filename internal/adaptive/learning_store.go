package adaptive

import (
	"context"
	"sync"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// LearningStore defines the append-only event store and empirical profile lookup.
type LearningStore interface {
	RecordOutcome(ctx context.Context, outcome router.RoutingOutcomeRecord, taskReq router.RoutingRequest) error
	GetProfile(key FeatureKey) (PerformanceProfile, bool)
	GetHierarchicalProfile(key FeatureKey) (PerformanceProfile, string)
	ListEvents() []router.RoutingOutcomeRecord
	RebuildFromEvents(events []router.RoutingOutcomeRecord, requests []router.RoutingRequest) error
}

// MemoryLearningStore provides thread-safe in-memory event-sourced learning.
type MemoryLearningStore struct {
	mu       sync.RWMutex
	events   []router.RoutingOutcomeRecord
	requests map[string]router.RoutingRequest
	profiles map[FeatureKey]*PerformanceProfile
}

// NewMemoryLearningStore initializes an empty MemoryLearningStore.
func NewMemoryLearningStore() *MemoryLearningStore {
	return &MemoryLearningStore{
		events:   make([]router.RoutingOutcomeRecord, 0),
		requests: make(map[string]router.RoutingRequest),
		profiles: make(map[FeatureKey]*PerformanceProfile),
	}
}

// RecordOutcome appends an outcome event and updates the performance profile incrementally.
func (s *MemoryLearningStore) RecordOutcome(ctx context.Context, outcome router.RoutingOutcomeRecord, taskReq router.RoutingRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, outcome)
	s.requests[outcome.TaskID] = taskReq

	key := ComputeFeatureKey(taskReq, outcome.ModelID)
	s.updateProfileLocked(key, outcome, taskReq)

	// Also update intermediate key for hierarchical fallback
	interKey := key.IntermediateKey()
	s.updateProfileLocked(interKey, outcome, taskReq)

	return nil
}

// GetProfile retrieves exact profile for a specific FeatureKey.
func (s *MemoryLearningStore) GetProfile(key FeatureKey) (PerformanceProfile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.profiles[key]
	if !ok {
		return PerformanceProfile{Key: key}, false
	}
	return *p, true
}

// GetHierarchicalProfile retrieves profile with fallback from Specific -> Intermediate -> Empty.
func (s *MemoryLearningStore) GetHierarchicalProfile(key FeatureKey) (PerformanceProfile, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Level 1: Specific Key (N >= 10)
	if p, ok := s.profiles[key]; ok && p.TotalAttempts >= 10 {
		return *p, "specific"
	}

	// Level 2: Intermediate Key (N >= 5)
	interKey := key.IntermediateKey()
	if p, ok := s.profiles[interKey]; ok && p.TotalAttempts >= 5 {
		return *p, "intermediate"
	}

	// Level 3: Prior fallback
	if p, ok := s.profiles[key]; ok && p.TotalAttempts > 0 {
		return *p, "sparse_specific"
	}

	return PerformanceProfile{Key: key}, "nominal_prior"
}

// ListEvents returns all recorded outcome events.
func (s *MemoryLearningStore) ListEvents() []router.RoutingOutcomeRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]router.RoutingOutcomeRecord, len(s.events))
	copy(res, s.events)
	return res
}

// RebuildFromEvents reconstructs all performance profiles from raw historical event logs.
func (s *MemoryLearningStore) RebuildFromEvents(events []router.RoutingOutcomeRecord, requests []router.RoutingRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = make([]router.RoutingOutcomeRecord, 0, len(events))
	s.requests = make(map[string]router.RoutingRequest)
	s.profiles = make(map[FeatureKey]*PerformanceProfile)

	reqMap := make(map[string]router.RoutingRequest)
	for _, r := range requests {
		reqMap[r.TaskID] = r
	}

	for _, e := range events {
		s.events = append(s.events, e)
		req, ok := reqMap[e.TaskID]
		if !ok {
			continue
		}
		s.requests[e.TaskID] = req

		key := ComputeFeatureKey(req, e.ModelID)
		s.updateProfileLocked(key, e, req)
		s.updateProfileLocked(key.IntermediateKey(), e, req)
	}

	return nil
}

func (s *MemoryLearningStore) updateProfileLocked(key FeatureKey, outcome router.RoutingOutcomeRecord, req router.RoutingRequest) {
	p, ok := s.profiles[key]
	if !ok {
		p = &PerformanceProfile{
			Key: key,
			LatencyRegression: LatencyRegressionState{
				Theta0: 20.0,
				Theta1: 0.02,
				Theta2: 0.05,
			},
			RecentLatencies: make([]float64, 0, 50),
			RecentQualities: make([]float64, 0, 50),
			RecentSuccesses: make([]bool, 0, 50),
		}
		s.profiles[key] = p
	}

	p.TotalAttempts++
	if outcome.Success {
		p.SuccessCount++
	} else {
		p.FailureCount++
	}

	p.QualitySum += outcome.ActualQualityScore
	p.QualitySqSum += outcome.ActualQualityScore * outcome.ActualQualityScore

	latMs := float64(outcome.ActualLatency) / float64(time.Millisecond)
	p.RecentLatencies = append(p.RecentLatencies, latMs)
	if len(p.RecentLatencies) > 50 {
		p.RecentLatencies = p.RecentLatencies[1:]
	}

	p.RecentQualities = append(p.RecentQualities, outcome.ActualQualityScore)
	if len(p.RecentQualities) > 50 {
		p.RecentQualities = p.RecentQualities[1:]
	}

	p.RecentSuccesses = append(p.RecentSuccesses, outcome.Success)
	if len(p.RecentSuccesses) > 50 {
		p.RecentSuccesses = p.RecentSuccesses[1:]
	}

	// Online Regression Update for Latency
	predLat := p.LatencyRegression.Theta0 +
		p.LatencyRegression.Theta1*float64(req.EstimatedInputTokens) +
		p.LatencyRegression.Theta2*float64(req.EstimatedOutputTokens)
	latErr := latMs - predLat
	const lr = 0.02
	p.LatencyRegression.Theta0 += lr * latErr * 0.1
	p.LatencyRegression.Theta1 += lr * latErr * (float64(req.EstimatedInputTokens) / 10000.0)
	p.LatencyRegression.Theta2 += lr * latErr * (float64(req.EstimatedOutputTokens) / 10000.0)

	// Cost tracking
	p.CostActualSum += outcome.ActualCostUSD
	nomCost := 0.0001
	p.CostNominalSum += nomCost

	p.LastUpdated = time.Now().UTC()
}
