package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func runMessagingBenchmarks() []BenchmarkResult {
	fmt.Printf("\n[2/6] RUNNING MESSAGING & TRANSACTIONAL OUTBOX BENCHMARKS...\n")
	var results []BenchmarkResult
	ctx := context.Background()

	// 1. Outbox Insert Throughput
	repo := outbox.NewMemoryRepository()
	rawPayload, _ := json.Marshal(events.RunCreatedPayload{
		RunID:     "run-bench",
		AgentID:   "agent-1",
		CreatedAt: time.Now(),
	})

	const insertIters = 1000
	insertDurations := make([]time.Duration, insertIters)
	for i := 0; i < insertIters; i++ {
		evt := events.Event{
			EventID:       fmt.Sprintf("evt-bench-%d", i),
			EventType:     events.SubjectRunCreated,
			SchemaVersion: 1,
			AggregateType: "Run",
			AggregateID:   "run-bench",
			TenantID:      "tenant-1",
			OccurredAt:    time.Now(),
			Payload:       rawPayload,
		}
		t0 := time.Now()
		_ = repo.Insert(ctx, evt)
		insertDurations[i] = time.Since(t0)
	}
	p50, p95, p99, mean := calculatePercentiles(insertDurations)
	throughput := float64(time.Second) / float64(mean)
	results = append(results, BenchmarkResult{
		Suite:        "Messaging",
		Scenario:     "Outbox-Insert",
		Scale:        "1000_events",
		Iterations:   insertIters,
		P50Duration:  p50,
		P95Duration:  p95,
		P99Duration:  p99,
		MeanDuration: mean,
		Throughput:   throughput,
	})
	fmt.Printf("  • Outbox Insert: P50=%v P95=%v | Throughput=%.0f events/s\n", p50, p95, throughput)

	// 2. Outbox Batch Claiming (10, 50, 100, 500)
	batchSizes := []int{10, 50, 100, 500}
	for _, size := range batchSizes {
		const claimIters = 100
		claimDurations := make([]time.Duration, claimIters)
		for i := 0; i < claimIters; i++ {
			t0 := time.Now()
			_, _ = repo.Claim(ctx, size, "worker-1", time.Minute)
			claimDurations[i] = time.Since(t0)
		}
		p50, p95, p99, mean := calculatePercentiles(claimDurations)
		throughput := float64(size) * (float64(time.Second) / float64(mean))
		results = append(results, BenchmarkResult{
			Suite:        "Messaging",
			Scenario:     "Outbox-BatchClaim",
			Scale:        fmt.Sprintf("batch_%d", size),
			Iterations:   claimIters,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   throughput,
		})
		fmt.Printf("  • Outbox Batch Claim (size=%3d): P50=%v P95=%v | Throughput=%.0f events/s\n", size, p50, p95, throughput)
	}

	// 3. End-to-End Placement Latency (RunCreated -> Scheduler -> RunScheduled)
	e2eResults := runMessagingEndToEndPlacementBenchmark()
	results = append(results, e2eResults...)

	return results
}

type staticSimpleNodeProvider struct {
	nodes []domain.Node
}

func (p *staticSimpleNodeProvider) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return p.nodes, nil
}

func runMessagingEndToEndPlacementBenchmark() []BenchmarkResult {
	ctx := context.Background()
	txManager := memory.NewTxManager()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()

	agent := domain.Agent{
		ID:             "agent-msg-bench",
		TenantID:       "tenant-1",
		Name:           "msg-bench-agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "1", Memory: "1024Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Image:          "sentinelmesh/worker:v1",
	}
	_ = agentRepo.Create(ctx, agent)

	nodes := []domain.Node{
		{
			ID:             "node-1",
			ClusterID:      "local",
			SecurityClass:  "public",
			Health:         domain.NodeHealthHealthy,
			Resources:      domain.NodeResources{CPUCapacity: 100, CPUAvailable: 80, MemoryCapacity: 100000, MemoryAvailable: 80000},
			CostPerHour:    1.0,
			NetworkLatency: 0.1,
		},
	}
	prov := &staticSimpleNodeProvider{nodes: nodes}
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, prov)

	const iters = 200
	durations := make([]time.Duration, iters)
	for i := 0; i < iters; i++ {
		runID := fmt.Sprintf("run-msg-e2e-%d", i)
		_ = runRepo.Create(ctx, domain.AgentRun{
			ID:        runID,
			AgentID:   agent.ID,
			TenantID:  agent.TenantID,
			State:     types.StateQueued,
			Version:   1,
			CreatedAt: time.Now(),
		})

		t0 := time.Now()
		_ = schedSvc.ScheduleRun(ctx, runID)
		durations[i] = time.Since(t0)
	}

	p50, p95, p99, mean := calculatePercentiles(durations)
	throughput := float64(time.Second) / float64(mean)
	fmt.Printf("  • End-to-End Placement (RunCreated -> Scheduled): P50=%v P95=%v | Throughput=%.0f ops/s\n", p50, p95, throughput)

	return []BenchmarkResult{
		{
			Suite:        "Messaging",
			Scenario:     "E2E-Placement-Latency",
			Scale:        "single_cluster",
			Iterations:   iters,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   throughput,
		},
	}
}
