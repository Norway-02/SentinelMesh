package cluster

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestNodeTracker_HealthTransitions(t *testing.T) {
	prov := scheduler.NewStaticResourceProvider()
	tracker := NewNodeTracker(prov)
	ctx := context.Background()

	// 1. Initial sync
	newlyFailed, err := tracker.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// node-unhealthy-1 is unhealthy in static provider
	if len(newlyFailed) != 1 || newlyFailed[0] != "node-unhealthy-1" {
		t.Errorf("expected newly failed [node-unhealthy-1], got %v", newlyFailed)
	}

	if !tracker.IsNodeHealthy("node-small-1") {
		t.Errorf("expected node-small-1 to be healthy")
	}
	if tracker.IsNodeHealthy("node-unhealthy-1") {
		t.Errorf("expected node-unhealthy-1 to be unhealthy")
	}

	// 2. Mark node failed manually
	tracker.MarkNodeFailed("node-small-1", "kubelet not responding")
	if tracker.IsNodeHealthy("node-small-1") {
		t.Errorf("expected node-small-1 to be marked failed")
	}

	failed := tracker.ListFailedNodeIDs()
	if len(failed) < 2 {
		t.Errorf("expected at least 2 failed nodes, got %v", failed)
	}
}

func TestFailureDetector_DetectsAffectedRunsAndEmitsEvents(t *testing.T) {
	prov := scheduler.NewStaticResourceProvider()
	tracker := NewNodeTracker(prov)
	runRepo := memory.NewRunRepository()
	cpRepo := checkpoint.NewMemoryRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	detector := NewFailureDetector(tracker, runRepo, cpRepo, outboxRepo, txManager)
	ctx := context.Background()

	// Setup an active run on node-small-1
	now := time.Now()
	run1 := domain.AgentRun{
		ID:                 "run-active-01",
		AgentID:            "agent-1",
		TenantID:           "tenant-prod",
		State:              types.StateRunning,
		Node:               "node-small-1",
		Cluster:            "local-cluster",
		StartedAt:          &now,
		RecoveryGeneration: 0,
		Version:            1,
	}
	_ = runRepo.Create(ctx, run1)

	// Save a checkpoint at step 25
	cp := checkpoint.Checkpoint{
		ID:             "cp-step-25",
		RunID:          "run-active-01",
		AgentID:        "agent-1",
		TenantID:       "tenant-prod",
		SequenceNumber: 25,
		StateInline:    json.RawMessage(`{"step":25,"processed_rows":25000}`),
		StateChecksum:  checkpoint.ComputeCanonicalChecksum([]byte(`{"step":25,"processed_rows":25000}`)),
		SizeBytes:      35,
		CreatedAt:      now,
	}
	_ = cpRepo.Save(ctx, cp)

	// Setup a completed run on node-small-1 (should NOT be recovered)
	run2 := domain.AgentRun{
		ID:                 "run-completed-02",
		AgentID:            "agent-2",
		TenantID:           "tenant-prod",
		State:              types.StateCompleted,
		Node:               "node-small-1",
		Cluster:            "local-cluster",
		StartedAt:          &now,
		FinishedAt:         &now,
		RecoveryGeneration: 0,
		Version:            1,
	}
	_ = runRepo.Create(ctx, run2)

	// Trigger node failure
	affected, err := detector.HandleNodeFailure(ctx, "node-small-1", "simulated node crash")
	if err != nil {
		t.Fatalf("HandleNodeFailure failed: %v", err)
	}

	if len(affected) != 1 || affected[0] != "run-active-01" {
		t.Errorf("expected affected runs [run-active-01], got %v", affected)
	}

	// 1. Verify run state updated to FAILED with recovery generation 1
	updatedRun, _ := runRepo.Get(ctx, "run-active-01")
	if updatedRun.State != types.StateFailed {
		t.Errorf("expected state FAILED, got %s", updatedRun.State)
	}
	if updatedRun.RecoveryGeneration != 1 {
		t.Errorf("expected recovery generation 1, got %d", updatedRun.RecoveryGeneration)
	}
	if updatedRun.LastCheckpointID != "cp-step-25" {
		t.Errorf("expected last checkpoint cp-step-25, got %s", updatedRun.LastCheckpointID)
	}

	// 2. Verify Outbox events emitted
	eventsList := outboxRepo.GetEvents()
	if len(eventsList) != 2 {
		t.Fatalf("expected 2 outbox events (NodeFailed, RecoveryRequested), got %d", len(eventsList))
	}

	// Check NodeFailed event
	if eventsList[0].EventType != events.SubjectClusterNodeFailed {
		t.Errorf("expected first event %s, got %s", events.SubjectClusterNodeFailed, eventsList[0].EventType)
	}

	// Check RecoveryRequested event
	if eventsList[1].EventType != events.SubjectRunRecoveryRequested {
		t.Errorf("expected second event %s, got %s", events.SubjectRunRecoveryRequested, eventsList[1].EventType)
	}

	var recPayload events.RunRecoveryRequestedPayload
	_ = json.Unmarshal(eventsList[1].Payload, &recPayload)
	if recPayload.RunID != "run-active-01" {
		t.Errorf("expected recovery run_id run-active-01, got %s", recPayload.RunID)
	}
	if recPayload.SourceCheckpointID != "cp-step-25" {
		t.Errorf("expected source checkpoint cp-step-25, got %s", recPayload.SourceCheckpointID)
	}
	if recPayload.SequenceNumber != 25 {
		t.Errorf("expected sequence number 25, got %d", recPayload.SequenceNumber)
	}
	if recPayload.RecoveryGeneration != 1 {
		t.Errorf("expected recovery generation 1, got %d", recPayload.RecoveryGeneration)
	}

	// 3. Test Idempotency: Trigger failure again, should NOT re-trigger recovery generation 1
	outboxLenBefore := len(outboxRepo.GetEvents())
	affected2, _ := detector.HandleNodeFailure(ctx, "node-small-1", "duplicate signal")
	if len(affected2) != 0 {
		t.Errorf("expected 0 active affected runs on duplicate failure, got %v", affected2)
	}
	outboxLenAfter := len(outboxRepo.GetEvents())
	// NodeFailed event is emitted, but NO duplicate RunRecoveryRequested event
	if outboxLenAfter != outboxLenBefore+1 {
		t.Errorf("expected only 1 additional event (NodeFailed), got %d new events", outboxLenAfter-outboxLenBefore)
	}
}
