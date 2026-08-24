package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type AgentService struct {
	txManager repository.TxManager
	repo      repository.AgentRepository
	outbox    outbox.Repository
}

func NewAgentService(
	txManager repository.TxManager,
	repo repository.AgentRepository,
	outboxRepo outbox.Repository,
) *AgentService {
	return &AgentService{
		txManager: txManager,
		repo:      repo,
		outbox:    outboxRepo,
	}
}

func (s *AgentService) CreateAgent(ctx context.Context, agent domain.Agent) (domain.Agent, error) {
	if agent.ID == "" {
		agent.ID = uuid.NewString()
	}
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = agent.CreatedAt
	
	if agent.Resources.CPU == "" {
		agent.Resources.CPU = "500m"
	}
	if agent.Resources.Memory == "" {
		agent.Resources.Memory = "512Mi"
	}

	if err := agent.Validate(); err != nil {
		return domain.Agent{}, fmt.Errorf("invalid agent: %w", err)
	}

	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, agent); err != nil {
			return err
		}

		payload, _ := json.Marshal(events.AgentCreatedPayload{
			AgentID:   agent.ID,
			Name:      agent.Name,
			CreatedAt: agent.CreatedAt,
		})

		evt := events.Event{
			EventID:       uuid.NewString(),
			EventType:     events.SubjectAgentCreated, // using subject as type for simplicity, or we can use "AgentCreated"
			SchemaVersion: 1,
			AggregateType: "Agent",
			AggregateID:   agent.ID,
			TenantID:      agent.TenantID,
			OccurredAt:    time.Now(),
			Payload:       payload,
		}

		return s.outbox.Insert(txCtx, evt)
	})

	if err != nil {
		return domain.Agent{}, err
	}

	return agent, nil
}

func (s *AgentService) GetAgent(ctx context.Context, id string) (domain.Agent, error) {
	if id == "" {
		return domain.Agent{}, fmt.Errorf("invalid agent id: empty")
	}

	return s.repo.Get(ctx, id)
}

func (s *AgentService) ListAgents(ctx context.Context, filter repository.AgentFilter) ([]domain.Agent, string, error) {
	if filter.PageSize < 0 {
		return nil, "", fmt.Errorf("invalid page size")
	}
	return s.repo.List(ctx, filter)
}

func (s *AgentService) DeleteAgent(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("invalid agent id: empty")
	}
	
	// Apply any business rules before deletion here (e.g., check for active runs).
	// For now, just delete.
	return s.repo.Delete(ctx, id)
}
