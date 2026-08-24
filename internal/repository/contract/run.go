package contract

import (
	"context"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func RunRunRepositoryContractTests(t *testing.T, factory func(t *testing.T) repository.RunRepository) {
	t.Run("CreateAndGet", func(t *testing.T) {
		repo := factory(t)
		ctx := context.Background()

		now := time.Now()
		run := domain.AgentRun{
			ID:        "run-1",
			AgentID:   "agent-1",
			State:     types.StateCreated,
			StartedAt: &now,
			Version:   1,
		}

		err := repo.Create(ctx, run)
		if err != nil {
			t.Fatalf("failed to create run: %v", err)
		}

		retrieved, err := repo.Get(ctx, "run-1")
		if err != nil {
			t.Fatalf("failed to get run: %v", err)
		}
		if retrieved.ID != run.ID {
			t.Fatalf("expected id %v, got %v", run.ID, retrieved.ID)
		}
	})

	t.Run("OptimisticConcurrencyUpdate", func(t *testing.T) {
		repo := factory(t)
		ctx := context.Background()

		run := domain.AgentRun{ID: "run-opt", AgentID: "agent-1", Version: 1, State: types.StateCreated}
		repo.Create(ctx, run)

		// First update should succeed
		run.State = types.StateRunning
		err := repo.Update(ctx, run)
		if err != nil {
			t.Fatalf("failed to update run: %v", err)
		}

		// Second update with old version should fail
		run.State = types.StatePaused
		err = repo.Update(ctx, run) // run.Version is still 1 here, but DB is at 2
		if err != repository.ErrConflict {
			t.Fatalf("expected ErrConflict, got %v", err)
		}
		
		// Getting the latest should allow update
		updated, _ := repo.Get(ctx, "run-opt")
		if updated.Version != 2 {
			t.Fatalf("expected version 2, got %v", updated.Version)
		}
		
		updated.State = types.StatePaused
		err = repo.Update(ctx, updated)
		if err != nil {
			t.Fatalf("expected successful update with latest version, got %v", err)
		}
	})
}
