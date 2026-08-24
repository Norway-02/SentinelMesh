package memory

import (
	"context"
	"sync"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type RunRepository struct {
	mu   sync.RWMutex
	runs map[string]domain.AgentRun
}

func NewRunRepository() *RunRepository {
	return &RunRepository{
		runs: make(map[string]domain.AgentRun),
	}
}

func (r *RunRepository) Create(ctx context.Context, run domain.AgentRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.runs[run.ID]; exists {
		return repository.ErrAlreadyExists
	}
	
	if run.Version == 0 {
		run.Version = 1
	}

	r.runs[run.ID] = run
	return nil
}

func (r *RunRepository) Get(ctx context.Context, id string) (domain.AgentRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	run, exists := r.runs[id]
	if !exists {
		return domain.AgentRun{}, repository.ErrNotFound
	}
	return run, nil
}

func (r *RunRepository) Update(ctx context.Context, run domain.AgentRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.runs[run.ID]
	if !exists {
		return repository.ErrNotFound
	}
	if existing.Version != run.Version {
		return repository.ErrConflict
	}
	
	run.Version++
	r.runs[run.ID] = run
	return nil
}

func (r *RunRepository) ListByNode(ctx context.Context, nodeID string) ([]domain.AgentRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.AgentRun
	for _, run := range r.runs {
		if run.Node == nodeID {
			result = append(result, run)
		}
	}
	return result, nil
}

func (r *RunRepository) ListByCluster(ctx context.Context, clusterID string) ([]domain.AgentRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.AgentRun
	for _, run := range r.runs {
		if run.Cluster == clusterID {
			result = append(result, run)
		}
	}
	return result, nil
}

