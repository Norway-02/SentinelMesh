package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestSyncService_SyncStatus_RunningAndCompleted(t *testing.T) {
	txManager := memory.NewTxManager()
	runRepo := memory.NewRunRepository()
	outboxRepo := outbox.NewMemoryRepository()
	syncSvc := NewSyncService(txManager, runRepo, outboxRepo)
	ctx := context.Background()

	// Setup initial run in SCHEDULED state
	initialRun := domain.AgentRun{
		ID:        "run-sync-1",
		AgentID:   "agent-1",
		TenantID:  "tenant-1",
		State:     types.StateScheduled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	if err := runRepo.Create(ctx, initialRun); err != nil {
		t.Fatalf("failed to seed run: %v", err)
	}

	// 1. Transition to RUNNING
	now := time.Now()
	runningStatus := ExecutionStatus{
		RunID:     "run-sync-1",
		State:     types.StateRunning,
		StartedAt: &now,
	}

	if err := syncSvc.SyncStatus(ctx, runningStatus); err != nil {
		t.Fatalf("failed to sync RUNNING status: %v", err)
	}

	// Verify DB state updated to RUNNING
	updatedRun, err := runRepo.Get(ctx, "run-sync-1")
	if err != nil {
		t.Fatalf("failed to fetch updated run: %v", err)
	}
	if updatedRun.State != types.StateRunning {
		t.Errorf("expected state RUNNING, got %s", updatedRun.State)
	}
	if updatedRun.StartedAt == nil {
		t.Errorf("expected StartedAt to be set")
	}

	// Verify Outbox event created
	eventsList := outboxRepo.GetEvents()
	if len(eventsList) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(eventsList))
	}
	if eventsList[0].EventType != events.SubjectRunStateChanged {
		t.Errorf("expected event type '%s', got '%s'", events.SubjectRunStateChanged, eventsList[0].EventType)
	}

	var payload events.RunStateChangedPayload
	if err := json.Unmarshal(eventsList[0].Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if payload.ToState != string(types.StateRunning) {
		t.Errorf("expected ToState 'RUNNING', got '%s'", payload.ToState)
	}

	// 2. Transition to COMPLETED
	finishedTime := time.Now()
	completedStatus := ExecutionStatus{
		RunID:      "run-sync-1",
		State:      types.StateCompleted,
		ExitCode:   0,
		FinishedAt: &finishedTime,
	}

	if err := syncSvc.SyncStatus(ctx, completedStatus); err != nil {
		t.Fatalf("failed to sync COMPLETED status: %v", err)
	}

	finalRun, _ := runRepo.Get(ctx, "run-sync-1")
	if finalRun.State != types.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", finalRun.State)
	}
	if finalRun.FinishedAt == nil {
		t.Errorf("expected FinishedAt to be set")
	}

	eventsList2 := outboxRepo.GetEvents()
	if len(eventsList2) != 2 {
		t.Fatalf("expected 2 outbox events total, got %d", len(eventsList2))
	}
}

func TestSyncService_SyncStatus_FailureReason(t *testing.T) {
	txManager := memory.NewTxManager()
	runRepo := memory.NewRunRepository()
	outboxRepo := outbox.NewMemoryRepository()
	syncSvc := NewSyncService(txManager, runRepo, outboxRepo)
	ctx := context.Background()

	initialRun := domain.AgentRun{
		ID:        "run-sync-fail",
		AgentID:   "agent-1",
		TenantID:  "tenant-1",
		State:     types.StateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	_ = runRepo.Create(ctx, initialRun)

	failStatus := ExecutionStatus{
		RunID:       "run-sync-fail",
		State:       types.StateFailed,
		ExitCode:    137,
		ErrorReason: "process killed: out of memory",
	}

	if err := syncSvc.SyncStatus(ctx, failStatus); err != nil {
		t.Fatalf("failed to sync FAILED status: %v", err)
	}

	run, _ := runRepo.Get(ctx, "run-sync-fail")
	if run.State != types.StateFailed {
		t.Errorf("expected state FAILED, got %s", run.State)
	}
	if run.FailureReason != "process killed: out of memory" {
		t.Errorf("expected failure reason 'process killed: out of memory', got '%s'", run.FailureReason)
	}
}
