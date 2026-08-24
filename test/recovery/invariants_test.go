package recovery_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// Invariant 1: Corrupted checkpoint must NEVER be restored
func TestInvariant_CorruptedCheckpointNeverRestored(t *testing.T) {
	ctx := context.Background()
	cpRepo := checkpoint.NewMemoryRepository()

	// Direct insertion of corrupted checkpoint (tampered state)
	corruptedCP := checkpoint.Checkpoint{
		ID:             "cp-corrupt-01",
		RunID:          "run-inv-01",
		AgentID:        "agent-inv",
		TenantID:       "tenant-test",
		SequenceNumber: 10,
		StateInline:    json.RawMessage(`{"status":"tampered"}`),
		StateChecksum:  "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		SizeBytes:      21,
		CreatedAt:      time.Now(),
	}
	_ = cpRepo.SaveRaw(ctx, corruptedCP)

	cpSvc := checkpoint.NewService(cpRepo, nil, nil)
	_, err := cpSvc.GetLatestCheckpoint(ctx, "run-inv-01")
	if !errors.Is(err, checkpoint.ErrCorruptedCheckpoint) {
		t.Fatalf("Expected ErrCorruptedCheckpoint, got %v", err)
	}
}

// Invariant 2: Idempotent checkpoint saving & conflict detection
func TestInvariant_IdempotentCheckpointSave(t *testing.T) {
	ctx := context.Background()
	cpRepo := checkpoint.NewMemoryRepository()
	cpSvc := checkpoint.NewService(cpRepo, nil, nil)

	req1 := checkpoint.SaveCheckpointRequest{
		RunID:          "run-inv-02",
		AgentID:        "agent-inv",
		TenantID:       "tenant-test",
		SequenceNumber: 5,
		StateInline:    json.RawMessage(`{"counter":500}`),
	}

	cp1, err := cpSvc.SaveCheckpoint(ctx, req1)
	if err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	// Identical payload and sequence -> Idempotent success
	cp2, err := cpSvc.SaveCheckpoint(ctx, req1)
	if err != nil {
		t.Fatalf("Idempotent save failed: %v", err)
	}
	if cp1.ID != cp2.ID || cp1.StateChecksum != cp2.StateChecksum {
		t.Errorf("Expected identical checkpoint returned on idempotent save")
	}

	// Different payload with same sequence -> Conflict rejection
	reqConflict := checkpoint.SaveCheckpointRequest{
		RunID:          "run-inv-02",
		AgentID:        "agent-inv",
		TenantID:       "tenant-test",
		SequenceNumber: 5,
		StateInline:    json.RawMessage(`{"counter":999,"conflict":true}`),
	}
	_, err = cpSvc.SaveCheckpoint(ctx, reqConflict)
	if !errors.Is(err, checkpoint.ErrSequenceConflict) {
		t.Fatalf("Expected ErrSequenceConflict, got %v", err)
	}
}

// Invariant 3: Monotonic checkpoint sequence progression
func TestInvariant_MonotonicSequenceProgression(t *testing.T) {
	ctx := context.Background()
	cpRepo := checkpoint.NewMemoryRepository()
	cpSvc := checkpoint.NewService(cpRepo, nil, nil)

	_, _ = cpSvc.SaveCheckpoint(ctx, checkpoint.SaveCheckpointRequest{
		RunID:          "run-inv-03",
		AgentID:        "agent-inv",
		TenantID:       "tenant-test",
		SequenceNumber: 10,
		StateInline:    json.RawMessage(`{"step":10}`),
	})

	// Attempt save with lower sequence number (step 8 < step 10)
	_, err := cpSvc.SaveCheckpoint(ctx, checkpoint.SaveCheckpointRequest{
		RunID:          "run-inv-03",
		AgentID:        "agent-inv",
		TenantID:       "tenant-test",
		SequenceNumber: 8,
		StateInline:    json.RawMessage(`{"step":8}`),
	})
	if !errors.Is(err, checkpoint.ErrNonMonotonicSeq) {
		t.Fatalf("Expected ErrNonMonotonicSeq, got %v", err)
	}
}

// Invariant 4: Monotonic recovery generation protection
func TestInvariant_MonotonicRecoveryGeneration(t *testing.T) {
	ctx := context.Background()
	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	resProv := &clusterProvider{nodes: []domain.Node{
		{ID: "node-1", Health: domain.NodeHealthHealthy, SecurityClass: "standard", Resources: domain.NodeResources{CPUCapacity: 4, CPUAvailable: 4, MemoryCapacity: 4096, MemoryAvailable: 4096}},
		{ID: "node-2", Health: domain.NodeHealthHealthy, SecurityClass: "standard", Resources: domain.NodeResources{CPUCapacity: 4, CPUAvailable: 4, MemoryCapacity: 4096, MemoryAvailable: 4096}},
	}}
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, resProv)

	agent := domain.Agent{
		ID:             "agent-gen-01",
		TenantID:       "tenant-test",
		Name:           "agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "1", Memory: "512Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Priority:       "normal",
	}
	_ = agentRepo.Create(ctx, agent)

	now := time.Now()
	run := domain.AgentRun{
		ID:                 "run-gen-01",
		AgentID:            "agent-gen-01",
		TenantID:           "tenant-test",
		State:              types.StateRunning,
		Node:               "node-1",
		StartedAt:          &now,
		RecoveryGeneration: 2, // Current generation is 2
		Version:            1,
	}
	_ = runRepo.Create(ctx, run)

	// Reschedule request with older generation (1 < 2) must be safely dropped
	err := schedSvc.RescheduleRun(ctx, scheduler.RescheduleRequest{
		RunID:              "run-gen-01",
		ExcludeNodeIDs:     []string{"node-1"},
		RecoveryGeneration: 1, // Stale
	})
	if err != nil {
		t.Fatalf("RescheduleRun returned unexpected error on stale generation: %v", err)
	}

	updatedRun, _ := runRepo.Get(ctx, "run-gen-01")
	if updatedRun.RecoveryGeneration != 2 {
		t.Errorf("Expected generation 2 preserved, got %d", updatedRun.RecoveryGeneration)
	}
	if updatedRun.Node != "node-1" {
		t.Errorf("Expected node assignment preserved without mutation, got %s", updatedRun.Node)
	}
}

// Invariant 5: Failed node exclusion during rescheduling
func TestInvariant_FailedNodeExclusion(t *testing.T) {
	ctx := context.Background()
	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	nodes := []domain.Node{
		{ID: "failed-node-A", Health: domain.NodeHealthHealthy, SecurityClass: "standard", Resources: domain.NodeResources{CPUCapacity: 16, CPUAvailable: 16, MemoryCapacity: 32768, MemoryAvailable: 32768}},
		{ID: "surviving-node-B", Health: domain.NodeHealthHealthy, SecurityClass: "standard", Resources: domain.NodeResources{CPUCapacity: 4, CPUAvailable: 4, MemoryCapacity: 8192, MemoryAvailable: 8192}},
	}
	resProv := &clusterProvider{nodes: nodes}
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, resProv)

	agent := domain.Agent{
		ID:             "agent-excl-01",
		TenantID:       "tenant-test",
		Name:           "agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "1", Memory: "1024Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Priority:       "normal",
	}
	_ = agentRepo.Create(ctx, agent)

	now := time.Now()
	run := domain.AgentRun{
		ID:                 "run-excl-01",
		AgentID:            "agent-excl-01",
		TenantID:           "tenant-test",
		State:              types.StateRunning,
		Node:               "failed-node-A",
		StartedAt:          &now,
		RecoveryGeneration: 0,
		Version:            1,
	}
	_ = runRepo.Create(ctx, run)

	// failed-node-A has more capacity/higher score, but MUST BE EXCLUDED
	err := schedSvc.RescheduleRun(ctx, scheduler.RescheduleRequest{
		RunID:              "run-excl-01",
		ExcludeNodeIDs:     []string{"failed-node-A"},
		RecoveryGeneration: 1,
	})
	if err != nil {
		t.Fatalf("RescheduleRun failed: %v", err)
	}

	updatedRun, _ := runRepo.Get(ctx, "run-excl-01")
	if updatedRun.Node != "surviving-node-B" {
		t.Errorf("Expected surviving-node-B chosen, got %s", updatedRun.Node)
	}
}

// Invariant 6: Storage Abstraction with URI Reference (state_uri)
func TestInvariant_StateURISupport(t *testing.T) {
	ctx := context.Background()
	cpRepo := checkpoint.NewMemoryRepository()
	cpSvc := checkpoint.NewService(cpRepo, nil, nil)

	gcsURI := "s3://sentinelmesh-checkpoints/run-100/seq-50.tar.gz"
	checksum := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	cp, err := cpSvc.SaveCheckpoint(ctx, checkpoint.SaveCheckpointRequest{
		RunID:          "run-100",
		AgentID:        "agent-uri",
		TenantID:       "tenant-test",
		SequenceNumber: 50,
		StateURI:       gcsURI,
		StateChecksum:  checksum,
		SizeBytes:      1048576, // 1MB blob
	})
	if err != nil {
		t.Fatalf("Failed to save URI checkpoint: %v", err)
	}

	if cp.StateURI != gcsURI {
		t.Errorf("Expected StateURI %s, got %s", gcsURI, cp.StateURI)
	}
	if cp.StateChecksum != checksum {
		t.Errorf("Expected StateChecksum %s, got %s", checksum, cp.StateChecksum)
	}
	if !cp.VerifyIntegrity() {
		t.Errorf("Expected URI checkpoint with valid checksum to verify integrity")
	}
}
