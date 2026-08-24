package contract

import (
	"context"
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

func RunAgentRepositoryContractTests(t *testing.T, factory func(t *testing.T) repository.AgentRepository) {
	t.Run("CreateAndGet", func(t *testing.T) {
		repo := factory(t)
		ctx := context.Background()

		agent := domain.Agent{
			ID:       "agent-1",
			Name:     "Test Agent",
			TenantID: "tenant-A",
		}

		err := repo.Create(ctx, agent)
		if err != nil {
			t.Fatalf("failed to create agent: %v", err)
		}

		// Create duplicate
		err = repo.Create(ctx, agent)
		if err != repository.ErrAlreadyExists {
			t.Fatalf("expected ErrAlreadyExists, got %v", err)
		}

		retrieved, err := repo.Get(ctx, "agent-1")
		if err != nil {
			t.Fatalf("failed to get agent: %v", err)
		}
		if retrieved.ID != agent.ID {
			t.Fatalf("expected id %v, got %v", agent.ID, retrieved.ID)
		}
	})

	t.Run("List", func(t *testing.T) {
		repo := factory(t)
		ctx := context.Background()

		repo.Create(ctx, domain.Agent{ID: "agent-2", TenantID: "tenant-A"})
		repo.Create(ctx, domain.Agent{ID: "agent-3", TenantID: "tenant-B"})

		page, token, err := repo.List(ctx, repository.AgentFilter{TenantID: "tenant-A", PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list agents: %v", err)
		}
		if len(page) != 1 {
			t.Fatalf("expected 1 agents for tenant-A, got %v", len(page))
		}
		if token != "" {
			t.Fatalf("expected empty next token, got %v", token)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		repo := factory(t)
		ctx := context.Background()

		agent := domain.Agent{ID: "agent-del", TenantID: "tenant-A"}
		repo.Create(ctx, agent)

		err := repo.Delete(ctx, agent.ID)
		if err != nil {
			t.Fatalf("failed to delete agent: %v", err)
		}

		_, err = repo.Get(ctx, agent.ID)
		if err != repository.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}
