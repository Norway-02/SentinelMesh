package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/contract"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/postgres"
	"github.com/sentinelmesh/sentinelmesh/internal/testutil"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestRunRepositoryContract(t *testing.T) {
	db := testutil.SetupTestDB(t)

	contract.RunRunRepositoryContractTests(t, func(t *testing.T) repository.RunRepository {
		_, err := db.Exec("DELETE FROM agent_run_state_history; DELETE FROM agent_runs; DELETE FROM agents;")
		if err != nil {
			t.Fatalf("failed to clean db: %v", err)
		}

		// Agent test requires an agent to exist for the foreign key.
		// The contract test creates a run with AgentID: "agent-1".
		// So we must create that agent first.
		agentRepo := postgres.NewAgentRepository(db)
		err = agentRepo.Create(context.Background(), domain.Agent{
			ID:       "agent-1",
			TenantID: "tenant-1",
			Name:     "contract-test-agent",
			State:    string(types.StateCreated),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
		if err != nil {
			t.Fatalf("failed to insert prerequisite agent: %v", err)
		}

		return postgres.NewRunRepository(db)
	})
}

func TestPostgresStateHistory(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx := context.Background()

	_, err := db.Exec("DELETE FROM agent_run_state_history; DELETE FROM agent_runs; DELETE FROM agents;")
	if err != nil {
		t.Fatalf("failed to clean db: %v", err)
	}

	agentRepo := postgres.NewAgentRepository(db)
	err = agentRepo.Create(ctx, domain.Agent{
		ID:        "agent-history",
		TenantID:  "tenant-1",
		Name:      "history-test",
		State:     string(types.StateCreated),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to insert agent: %v", err)
	}

	runRepo := postgres.NewRunRepository(db)
	now := time.Now()
	run := domain.AgentRun{
		ID:        "run-history",
		AgentID:   "agent-history",
		State:     types.StateCreated,
		Version:   1,
		StartedAt: &now,
	}

	err = runRepo.Create(ctx, run)
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	run.State = types.StateRunning
	err = runRepo.Update(ctx, run)
	if err != nil {
		t.Fatalf("failed to update run: %v", err)
	}

	// Verify the history table
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM agent_run_state_history WHERE run_id = $1", run.ID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count history: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 history record, got %d", count)
	}

	var from, to string
	err = db.QueryRow("SELECT from_state, to_state FROM agent_run_state_history WHERE run_id = $1", run.ID).Scan(&from, &to)
	if err != nil {
		t.Fatalf("failed to query history: %v", err)
	}
	if from != string(types.StateCreated) {
		t.Fatalf("expected from_state %s, got %s", types.StateCreated, from)
	}
	if to != string(types.StateRunning) {
		t.Fatalf("expected to_state %s, got %s", types.StateRunning, to)
	}
}
