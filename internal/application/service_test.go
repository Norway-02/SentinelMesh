package application

import (
	"context"
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestAgentService_CRUD(t *testing.T) {
	txManager := memory.NewTxManager()
	repo := memory.NewAgentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	svc := NewAgentService(txManager, repo, outboxRepo)
	ctx := context.Background()

	// Create
	agent, err := svc.CreateAgent(ctx, domain.Agent{
		Name:     "test-agent",
		Version:  "v1",
		TenantID: "tenant-1",
		Priority: "normal",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.ID == "" {
		t.Fatalf("expected ID to be generated")
	}
	if agent.CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt to be set")
	}

	// Get
	retrieved, err := svc.GetAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrieved.Name != "test-agent" {
		t.Fatalf("expected name to match")
	}

	// List
	_, err = svc.CreateAgent(ctx, domain.Agent{
		Name:     "test-agent-2",
		Version:  "v1",
		TenantID: "tenant-1",
		Priority: "high",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list, _, err := svc.ListAgents(ctx, repository.AgentFilter{TenantID: "tenant-1", PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(list))
	}

	// Delete
	err = svc.DeleteAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.GetAgent(ctx, agent.ID)
	if err != repository.ErrNotFound {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestRunService_CRUD(t *testing.T) {
	txManager := memory.NewTxManager()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	outboxRepo := outbox.NewMemoryRepository()
	agentSvc := NewAgentService(txManager, agentRepo, outboxRepo)
	runSvc := NewRunService(txManager, agentRepo, runRepo, outboxRepo)
	ctx := context.Background()

	// Create agent
	agent, err := agentSvc.CreateAgent(ctx, domain.Agent{
		Name:     "test-agent",
		Version:  "v1",
		TenantID: "tenant-1",
		Priority: "normal",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create run
	run, err := runSvc.CreateRun(ctx, agent.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ID == "" {
		t.Fatalf("expected run ID to be generated")
	}
	if run.State != types.StateCreated {
		t.Fatalf("expected initial state CREATED, got %s", run.State)
	}

	// Create run for non-existent agent
	_, err = runSvc.CreateRun(ctx, "non-existent")
	if err == nil {
		t.Fatalf("expected error creating run for non-existent agent")
	}

	// Get run state
	state, err := runSvc.GetRunState(ctx, run.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != types.StateCreated {
		t.Fatalf("expected state CREATED, got %s", state)
	}

	// Transition to QUEUED before cancelling
	run.TransitionTo(types.StateQueued)
	if err := runRepo.Update(ctx, run); err != nil {
		t.Fatalf("failed to update run: %v", err)
	}

	// Cancel run
	_, err = runSvc.CancelRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state, _ = runSvc.GetRunState(ctx, run.ID)
	if state != types.StateCancelled {
		t.Fatalf("expected state CANCELLED, got %s", state)
	}

	// Try to cancel already cancelled run (should succeed as no-op)
	_, err = runSvc.CancelRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("expected no error cancelling already cancelled run, got %v", err)
	}
}
