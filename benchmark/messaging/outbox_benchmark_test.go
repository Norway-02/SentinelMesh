package messaging_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func BenchmarkOutbox_Insert(b *testing.B) {
	ctx := context.Background()
	repo := outbox.NewMemoryRepository()
	rawPayload, _ := json.Marshal(events.RunCreatedPayload{
		RunID:     "run-bench-1",
		AgentID:   "agent-1",
		CreatedAt: time.Now(),
	})

	event := events.Event{
		EventID:       uuid.NewString(),
		EventType:     events.SubjectRunCreated,
		SchemaVersion: 1,
		AggregateType: "Run",
		AggregateID:   "run-bench-1",
		TenantID:      "tenant-1",
		OccurredAt:    time.Now(),
		Payload:       rawPayload,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event.EventID = fmt.Sprintf("evt-%d", i)
		if err := repo.Insert(ctx, event); err != nil {
			b.Fatalf("Insert failed: %v", err)
		}
	}
}

func BenchmarkOutbox_BatchProcessing(b *testing.B) {
	batchSizes := []int{10, 50, 100, 500}
	ctx := context.Background()

	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", size), func(b *testing.B) {
			repo := outbox.NewMemoryRepository()
			rawPayload, _ := json.Marshal(events.RunCreatedPayload{
				RunID:     "run-bench",
				AgentID:   "agent-1",
				CreatedAt: time.Now(),
			})

			// Pre-populate 10,000 events
			for i := 0; i < 10000; i++ {
				_ = repo.Insert(ctx, events.Event{
					EventID:       fmt.Sprintf("evt-batch-%d", i),
					EventType:     events.SubjectRunCreated,
					SchemaVersion: 1,
					AggregateType: "Run",
					AggregateID:   "run-bench",
					TenantID:      "tenant-1",
					OccurredAt:    time.Now(),
					Payload:       rawPayload,
				})
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				evts, err := repo.Claim(ctx, size, "worker-1", time.Minute)
				if err != nil {
					b.Fatalf("Claim failed: %v", err)
				}
				_ = evts
			}
		})
	}
}

func BenchmarkMessaging_EndToEndPlacementLatency(b *testing.B) {
	ctx := context.Background()
	txManager := memory.NewTxManager()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()

	agent := domain.Agent{
		ID:             "agent-e2e-bench",
		TenantID:       "tenant-1",
		Name:           "e2e-agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "1", Memory: "1024Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Image:          "sentinelmesh/worker:v1",
	}
	_ = agentRepo.Create(ctx, agent)

	nodes := []domain.Node{
		{
			ID:             "worker-node-1",
			ClusterID:      "local",
			SecurityClass:  "public",
			Health:         domain.NodeHealthHealthy,
			Resources:      domain.NodeResources{CPUCapacity: 100, CPUAvailable: 80, MemoryCapacity: 100000, MemoryAvailable: 80000},
			CostPerHour:    1.0,
			NetworkLatency: 0.1,
		},
	}
	prov := &simpleNodeProvider{nodes: nodes}
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, prov)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runID := fmt.Sprintf("run-e2e-%d", i)
		_ = runRepo.Create(ctx, domain.AgentRun{
			ID:        runID,
			AgentID:   agent.ID,
			TenantID:  agent.TenantID,
			State:     types.StateQueued,
			Version:   1,
			CreatedAt: time.Now(),
		})

		// Event pipeline trigger: RunCreated -> Scheduler -> Outbox RunScheduled
		if err := schedSvc.ScheduleRun(ctx, runID); err != nil {
			b.Fatalf("ScheduleRun failed: %v", err)
		}
	}
}

type simpleNodeProvider struct {
	nodes []domain.Node
}

func (p *simpleNodeProvider) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return p.nodes, nil
}
