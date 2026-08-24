package application

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

func TestRecoveryCoordinator_SuccessfulFailoverAndRestore(t *testing.T) {
	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	cpRepo := checkpoint.NewMemoryRepository()
	cpSvc := checkpoint.NewService(cpRepo, outboxRepo, txManager)

	multiNodeProv := &testClusterProvider{
		nodes: []domain.Node{
			{
				ID:        "worker-1",
				ClusterID: "local-cluster",
				Resources: domain.NodeResources{
					CPUCapacity:     4.0,
					CPUAvailable:    4.0,
					MemoryCapacity:  8192,
					MemoryAvailable: 8192,
				},
				Health:        domain.NodeHealthHealthy,
				SecurityClass: "standard",
			},
			{
				ID:        "worker-2",
				ClusterID: "local-cluster",
				Resources: domain.NodeResources{
					CPUCapacity:     8.0,
					CPUAvailable:    8.0,
					MemoryCapacity:  16384,
					MemoryAvailable: 16384,
				},
				Health:        domain.NodeHealthHealthy,
				SecurityClass: "standard",
			},
		},
	}
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, multiNodeProv)

	coordinator := NewRecoveryCoordinator(runRepo, cpSvc, schedSvc, outboxRepo, txManager)
	ctx := context.Background()

	// 1. Create Agent and initial Run
	agent := domain.Agent{
		ID:       "agent-heal-01",
		TenantID: "tenant-prod",
		Name:     "data-processor",
		Version:  "1.0.0",
		Resources: types.AgentResources{
			CPU:    "1",
			Memory: "1024Mi",
		},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Priority:       "normal",
		Image:          "sentinelmesh/agent:v1",
	}
	_ = agentRepo.Create(ctx, agent)

	now := time.Now()
	run := domain.AgentRun{
		ID:                 "run-heal-01",
		AgentID:            "agent-heal-01",
		TenantID:           "tenant-prod",
		State:              types.StateRunning,
		Node:               "worker-1",
		Cluster:            "local-cluster",
		StartedAt:          &now,
		RecoveryGeneration: 0,
		Version:            1,
	}
	_ = runRepo.Create(ctx, run)

	// 2. Save Checkpoint at Step 25
	stateData := json.RawMessage(`{"step":25,"offset":25000}`)
	cp, err := cpSvc.SaveCheckpoint(ctx, checkpoint.SaveCheckpointRequest{
		RunID:          "run-heal-01",
		AgentID:        "agent-heal-01",
		TenantID:       "tenant-prod",
		SequenceNumber: 25,
		StateInline:    stateData,
	})
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	// 3. Initiate Recovery (worker-1 failed)
	recReq := events.RunRecoveryRequestedPayload{
		RunID:              "run-heal-01",
		AgentID:            "agent-heal-01",
		TenantID:           "tenant-prod",
		FailedNodeID:       "worker-1",
		RecoveryGeneration: 1,
		SourceCheckpointID: cp.ID,
		SequenceNumber:     25,
		RequestedAt:        time.Now(),
	}

	err = coordinator.HandleRecovery(ctx, recReq)
	if err != nil {
		t.Fatalf("HandleRecovery failed: %v", err)
	}

	// 4. Verify updated Run state
	updatedRun, err := runRepo.Get(ctx, "run-heal-01")
	if err != nil {
		t.Fatalf("failed to fetch updated run: %v", err)
	}

	if updatedRun.State != types.StateScheduled {
		t.Errorf("expected state SCHEDULED, got %s", updatedRun.State)
	}
	if updatedRun.Node != "worker-2" {
		t.Errorf("expected failover to worker-2, got %s", updatedRun.Node)
	}
	if updatedRun.RecoveryGeneration != 1 {
		t.Errorf("expected recovery generation 1, got %d", updatedRun.RecoveryGeneration)
	}
	if updatedRun.RecoveredFromCheckpointID != cp.ID {
		t.Errorf("expected RecoveredFromCheckpointID %s, got %s", cp.ID, updatedRun.RecoveredFromCheckpointID)
	}

	// 5. Verify RunRecovered and RunScheduled Outbox Events
	var scheduledEvt *events.Event
	var recoveredEvt *events.Event

	for _, e := range outboxRepo.GetEvents() {
		if e.EventType == events.SubjectRunScheduled {
			scheduledEvt = &e
		}
		if e.EventType == events.SubjectRunRecovered {
			recoveredEvt = &e
		}
	}

	if scheduledEvt == nil {
		t.Fatalf("expected RunScheduled outbox event")
	}
	if recoveredEvt == nil {
		t.Fatalf("expected RunRecovered outbox event")
	}

	var schedPayload events.RunScheduledPayload
	_ = json.Unmarshal(scheduledEvt.Payload, &schedPayload)
	if schedPayload.Checkpoint == nil {
		t.Fatalf("expected Checkpoint metadata in RunScheduled payload")
	}
	if schedPayload.Checkpoint.ID != cp.ID {
		t.Errorf("expected checkpoint ID %s, got %s", cp.ID, schedPayload.Checkpoint.ID)
	}
	if schedPayload.Checkpoint.Sequence != 25 {
		t.Errorf("expected checkpoint sequence 25, got %d", schedPayload.Checkpoint.Sequence)
	}
}

func TestRecoveryCoordinator_CorruptedCheckpointRejection(t *testing.T) {
	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	cpRepo := checkpoint.NewMemoryRepository()
	cpSvc := checkpoint.NewService(cpRepo, outboxRepo, txManager)
	resProv := scheduler.NewStaticResourceProvider()
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, resProv)

	coordinator := NewRecoveryCoordinator(runRepo, cpSvc, schedSvc, outboxRepo, txManager)
	ctx := context.Background()

	now := time.Now()
	run := domain.AgentRun{
		ID:                 "run-corrupt-01",
		AgentID:            "agent-corrupt",
		TenantID:           "tenant-prod",
		State:              types.StateRunning,
		Node:               "node-small-1",
		StartedAt:          &now,
		RecoveryGeneration: 0,
		Version:            1,
	}
	_ = runRepo.Create(ctx, run)

	// Save corrupted checkpoint directly in repository (tampered payload)
	tamperedCP := checkpoint.Checkpoint{
		ID:             "cp-tampered",
		RunID:          "run-corrupt-01",
		AgentID:        "agent-corrupt",
		TenantID:       "tenant-prod",
		SequenceNumber: 10,
		StateInline:    json.RawMessage(`{"tampered":true}`),
		StateChecksum:  "sha256:invalid-corrupted-hash",
		SizeBytes:      17,
		CreatedAt:      now,
	}
	// Bypass validation on direct store insertion to simulate storage bit-rot
	_ = cpRepo.Save(ctx, tamperedCP)

	recReq := events.RunRecoveryRequestedPayload{
		RunID:              "run-corrupt-01",
		AgentID:            "agent-corrupt",
		TenantID:           "tenant-prod",
		FailedNodeID:       "node-small-1",
		RecoveryGeneration: 1,
		SourceCheckpointID: "cp-tampered",
		SequenceNumber:     10,
		RequestedAt:        time.Now(),
	}

	err := coordinator.HandleRecovery(ctx, recReq)
	if err == nil {
		t.Errorf("expected recovery to fail on corrupted checkpoint, got nil")
	}

	updatedRun, _ := runRepo.Get(ctx, "run-corrupt-01")
	if updatedRun.State != types.StateFailed {
		t.Errorf("expected run state FAILED, got %s", updatedRun.State)
	}

	// Verify RunRecoveryFailed event
	var failedEvt *events.Event
	for _, e := range outboxRepo.GetEvents() {
		if e.EventType == events.SubjectRunRecoveryFailed {
			failedEvt = &e
			break
		}
	}
	if failedEvt == nil {
		t.Errorf("expected RunRecoveryFailed outbox event")
	}
}

func TestRecoveryCoordinator_StaleGenerationIgnored(t *testing.T) {
	runRepo := memory.NewRunRepository()
	coordinator := NewRecoveryCoordinator(runRepo, nil, nil, nil, nil)
	ctx := context.Background()

	now := time.Now()
	run := domain.AgentRun{
		ID:                 "run-gen-01",
		AgentID:            "agent-gen",
		TenantID:           "tenant-prod",
		State:              types.StateRunning,
		Node:               "node-large-1",
		StartedAt:          &now,
		RecoveryGeneration: 3, // Already at generation 3
		Version:            1,
	}
	_ = runRepo.Create(ctx, run)

	// Old stale recovery request for generation 2
	staleReq := events.RunRecoveryRequestedPayload{
		RunID:              "run-gen-01",
		AgentID:            "agent-gen",
		TenantID:           "tenant-prod",
		FailedNodeID:       "node-small-1",
		RecoveryGeneration: 2, // Stale
		RequestedAt:        time.Now(),
	}

	err := coordinator.HandleRecovery(ctx, staleReq)
	if err != nil {
		t.Fatalf("unexpected error on stale request: %v", err)
	}

	updatedRun, _ := runRepo.Get(ctx, "run-gen-01")
	if updatedRun.RecoveryGeneration != 3 {
		t.Errorf("expected generation to remain 3, got %d", updatedRun.RecoveryGeneration)
	}
	if updatedRun.State != types.StateRunning {
		t.Errorf("expected state to remain RUNNING, got %s", updatedRun.State)
	}
}

type testClusterProvider struct {
	nodes []domain.Node
}

func (p *testClusterProvider) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return p.nodes, nil
}
