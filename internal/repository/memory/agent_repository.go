package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type AgentRepository struct {
	mu     sync.RWMutex
	agents map[string]domain.Agent
}

func NewAgentRepository() *AgentRepository {
	return &AgentRepository{
		agents: make(map[string]domain.Agent),
	}
}

func (r *AgentRepository) Create(ctx context.Context, agent domain.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agent.ID]; exists {
		return repository.ErrAlreadyExists
	}

	r.agents[agent.ID] = agent
	return nil
}

func (r *AgentRepository) Get(ctx context.Context, id string) (domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[id]
	if !exists {
		return domain.Agent{}, repository.ErrNotFound
	}
	return agent, nil
}

func (r *AgentRepository) List(ctx context.Context, filter repository.AgentFilter) ([]domain.Agent, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []domain.Agent
	for _, agent := range r.agents {
		if filter.TenantID == "" || agent.TenantID == filter.TenantID {
			matched = append(matched, agent)
		}
	}

	// Deterministic sorting by ID
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].ID < matched[j].ID
	})

	// Pagination
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 50 // Default
	}
	if pageSize > 1000 {
		pageSize = 1000 // Max cap
	}

	startIdx := 0
	if filter.PageToken != "" {
		for i, agent := range matched {
			if agent.ID == filter.PageToken {
				startIdx = i // PageToken itself should be returned or the one after? Usually the one after, but wait, usually page token is exclusive. Let's make it exclusive.
				break
			}
		}
	}

	endIdx := startIdx + pageSize
	if endIdx > len(matched) {
		endIdx = len(matched)
	}

	page := matched[startIdx:endIdx]
	var nextToken string
	if endIdx < len(matched) {
		nextToken = matched[endIdx].ID // The token for the next page is the ID of the first item on the next page
	}

	return page, nextToken, nil
}

func (r *AgentRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[id]; !exists {
		return repository.ErrNotFound
	}

	delete(r.agents, id)
	return nil
}
