package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
	"github.com/sentinelmesh/sentinelmesh/internal/verification"
)

type e2eHarness struct {
	txManager      *memory.TxManager
	agentRepo      *memory.AgentRepository
	runRepo        *memory.RunRepository
	assignmentRepo repository.AssignmentRepository
	outboxRepo     *outbox.MemoryRepository
	checkpointRepo *checkpoint.MemoryRepository
	attestRepo     *verification.MemoryRepository
	schedSvc       *scheduler.Service
	checkpointSvc  *checkpoint.Service
	verifySvc      *verification.Service
	agent          domain.Agent
}

type staticNodeProvider struct {
	nodes []domain.Node
}

func (p *staticNodeProvider) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return p.nodes, nil
}

func setupE2EHarness() *e2eHarness {
	ctx := context.Background()
	txManager := memory.NewTxManager()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	checkpointRepo := checkpoint.NewMemoryRepository()
	attestRepo := verification.NewMemoryRepository()

	agent := domain.Agent{
		ID:             "agent-e2e-bench",
		TenantID:       "tenant-1",
		Name:           "e2e-bench-agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "1", Memory: "1024Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		VerificationPolicy: types.VerificationPolicy{
			Enabled: true,
			InvariantRules: []types.InvariantRule{
				{ID: "rule-1", MetricName: "status", Operator: "eq", ExpectedValue: "completed"},
			},
		},
		Image: "sentinelmesh/worker:v1",
	}
	_ = agentRepo.Create(ctx, agent)

	nodes := []domain.Node{
		{
			ID:             "node-1",
			ClusterID:      "local-cluster",
			SecurityClass:  "public",
			Health:         domain.NodeHealthHealthy,
			Resources:      domain.NodeResources{CPUCapacity: 100, CPUAvailable: 80, MemoryCapacity: 100000, MemoryAvailable: 80000},
			CostPerHour:    1.0,
			NetworkLatency: 0.1,
		},
	}
	prov := &staticNodeProvider{nodes: nodes}
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, prov)
	cpSvc := checkpoint.NewService(checkpointRepo, outboxRepo, txManager)
	verifySvc := verification.NewService(attestRepo, agentRepo, runRepo, outboxRepo, txManager, nil, nil)

	return &e2eHarness{
		txManager:      txManager,
		agentRepo:      agentRepo,
		runRepo:        runRepo,
		assignmentRepo: assignmentRepo,
		outboxRepo:     outboxRepo,
		checkpointRepo: checkpointRepo,
		attestRepo:     attestRepo,
		schedSvc:       schedSvc,
		checkpointSvc:  cpSvc,
		verifySvc:      verifySvc,
		agent:          agent,
	}
}

func BenchmarkE2E_FullRunLifecycle(b *testing.B) {
	ctx := context.Background()
	h := setupE2EHarness()
	checkpointPayload := []byte(`{"step":50,"status":"completed"}`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runID := fmt.Sprintf("run-e2e-life-%d", i)

		// 1. Create Run
		_ = h.runRepo.Create(ctx, domain.AgentRun{
			ID:        runID,
			AgentID:   h.agent.ID,
			TenantID:  h.agent.TenantID,
			State:     types.StateQueued,
			Version:   1,
			CreatedAt: time.Now(),
		})

		// 2. Schedule
		if err := h.schedSvc.ScheduleRun(ctx, runID); err != nil {
			b.Fatalf("ScheduleRun failed: %v", err)
		}

		// 3. Running state + Checkpoint
		run, _ := h.runRepo.Get(ctx, runID)
		_ = run.TransitionTo(types.StateStarting)
		_ = run.TransitionTo(types.StateRunning)
		_ = h.runRepo.Update(ctx, run)

		_, err := h.checkpointSvc.SaveCheckpoint(ctx, checkpoint.SaveCheckpointRequest{
			RunID:          runID,
			AgentID:        h.agent.ID,
			TenantID:       h.agent.TenantID,
			SequenceNumber: 50,
			StateInline:    checkpointPayload,
		})
		if err != nil {
			b.Fatalf("SaveCheckpoint failed: %v", err)
		}

		// 4. Verify & Attest
		attestation, err := h.verifySvc.VerifyRun(ctx, verification.VerifyRunRequest{
			RunID: runID,
			ReportedMetrics: map[string]string{
				"status": "completed",
			},
		})
		if err != nil || !attestation.VerifyDigest() {
			b.Fatalf("Verification failed: %v", err)
		}

		// 5. Complete Run
		run, _ = h.runRepo.Get(ctx, runID)
		_ = run.TransitionTo(types.StateCompleted)
		_ = h.runRepo.Update(ctx, run)
	}
}
