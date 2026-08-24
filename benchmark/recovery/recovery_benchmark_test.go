package recovery_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/application"
	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/cluster"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

type recoveryBenchHarness struct {
	runRepo       *memory.RunRepository
	checkpointSvc *checkpoint.Service
	schedulerSvc  *scheduler.Service
	recoveryCoord *application.RecoveryCoordinator
	detector      *cluster.FailureDetector
}

func setupRecoveryHarness() *recoveryBenchHarness {
	ctx := context.Background()
	txManager := memory.NewTxManager()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	cpRepo := checkpoint.NewMemoryRepository()

	agent := domain.Agent{
		ID:             "agent-rec-bench",
		TenantID:       "tenant-1",
		Name:           "rec-agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "1", Memory: "1024Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Image:          "sentinelmesh/worker:v1",
	}
	_ = agentRepo.Create(ctx, agent)

	nodes := []domain.Node{
		{ID: "node-primary", ClusterID: "cluster-1", SecurityClass: "public", Health: domain.NodeHealthHealthy, Resources: domain.NodeResources{CPUCapacity: 1000, CPUAvailable: 800, MemoryCapacity: 1000000, MemoryAvailable: 800000}, CostPerHour: 1.0, NetworkLatency: 0.1},
		{ID: "node-backup", ClusterID: "cluster-1", SecurityClass: "public", Health: domain.NodeHealthHealthy, Resources: domain.NodeResources{CPUCapacity: 1000, CPUAvailable: 800, MemoryCapacity: 1000000, MemoryAvailable: 800000}, CostPerHour: 1.0, NetworkLatency: 0.1},
	}
	prov := &simpleNodeProvider{nodes: nodes}
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, prov)
	cpSvc := checkpoint.NewService(cpRepo, outboxRepo, txManager)
	recCoord := application.NewRecoveryCoordinator(runRepo, cpSvc, schedSvc, outboxRepo, txManager)

	tracker := cluster.NewNodeTracker(prov)
	detector := cluster.NewFailureDetector(tracker, runRepo, cpRepo, outboxRepo, txManager)

	return &recoveryBenchHarness{
		runRepo:       runRepo,
		checkpointSvc: cpSvc,
		schedulerSvc:  schedSvc,
		recoveryCoord: recCoord,
		detector:      detector,
	}
}

type simpleNodeProvider struct {
	nodes []domain.Node
}

func (p *simpleNodeProvider) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return p.nodes, nil
}

func BenchmarkRecovery_SingleRun(b *testing.B) {
	ctx := context.Background()
	h := setupRecoveryHarness()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runID := fmt.Sprintf("run-rec-%d", i)
		_ = h.runRepo.Create(ctx, domain.AgentRun{
			ID:        runID,
			AgentID:   "agent-rec-bench",
			TenantID:  "tenant-1",
			State:     types.StateRunning,
			Node:      "node-primary",
			Cluster:   "cluster-1",
			Version:   1,
			CreatedAt: time.Now(),
		})

		req := events.RunRecoveryRequestedPayload{
			RunID:              runID,
			AgentID:            "agent-rec-bench",
			TenantID:           "tenant-1",
			FailedNodeID:       "node-primary",
			RecoveryGeneration: 1,
			RequestedAt:        time.Now(),
		}

		if err := h.recoveryCoord.HandleRecovery(ctx, req); err != nil {
			b.Fatalf("HandleRecovery failed: %v", err)
		}
	}
}

func BenchmarkRecovery_ConcurrentFailureBurst(b *testing.B) {
	concurrencyLevels := []int{10, 50, 100}
	ctx := context.Background()

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			h := setupRecoveryHarness()
			b.ReportAllocs()
			b.ResetTimer()

			for n := 0; n < b.N; n++ {
				var wg sync.WaitGroup
				wg.Add(concurrency)

				for i := 0; i < concurrency; i++ {
					runID := fmt.Sprintf("run-burst-%d-%d", n, i)
					_ = h.runRepo.Create(ctx, domain.AgentRun{
						ID:        runID,
						AgentID:   "agent-rec-bench",
						TenantID:  "tenant-1",
						State:     types.StateRunning,
						Node:      "node-primary",
						Cluster:   "cluster-1",
						Version:   1,
						CreatedAt: time.Now(),
					})

					go func(id string) {
						defer wg.Done()
						req := events.RunRecoveryRequestedPayload{
							RunID:              id,
							AgentID:            "agent-rec-bench",
							TenantID:           "tenant-1",
							FailedNodeID:       "node-primary",
							RecoveryGeneration: 1,
							RequestedAt:        time.Now(),
						}
						_ = h.recoveryCoord.HandleRecovery(ctx, req)
					}(runID)
				}
				wg.Wait()
			}
		})
	}
}
