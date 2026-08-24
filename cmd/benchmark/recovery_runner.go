package main

import (
	"context"
	"fmt"
	"sync"
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

type recoveryRunnerHarness struct {
	runRepo       *memory.RunRepository
	checkpointSvc *checkpoint.Service
	schedulerSvc  *scheduler.Service
	recoveryCoord *application.RecoveryCoordinator
	detector      *cluster.FailureDetector
}

func setupRecoveryRunnerHarness() *recoveryRunnerHarness {
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
	prov := &staticSimpleNodeProvider{nodes: nodes}
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, prov)
	cpSvc := checkpoint.NewService(cpRepo, outboxRepo, txManager)
	recCoord := application.NewRecoveryCoordinator(runRepo, cpSvc, schedSvc, outboxRepo, txManager)

	tracker := cluster.NewNodeTracker(prov)
	detector := cluster.NewFailureDetector(tracker, runRepo, cpRepo, outboxRepo, txManager)

	return &recoveryRunnerHarness{
		runRepo:       runRepo,
		checkpointSvc: cpSvc,
		schedulerSvc:  schedSvc,
		recoveryCoord: recCoord,
		detector:      detector,
	}
}

func runRecoveryBenchmarks() []BenchmarkResult {
	fmt.Printf("\n[4/6] RUNNING CONCURRENT RECOVERY & BURST BENCHMARKS...\n")
	var results []BenchmarkResult
	ctx := context.Background()

	// 1. Single Run Recovery
	h := setupRecoveryRunnerHarness()
	const singleIters = 100
	singleDurations := make([]time.Duration, singleIters)
	for i := 0; i < singleIters; i++ {
		runID := fmt.Sprintf("run-rec-cli-%d", i)
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

		t0 := time.Now()
		_ = h.recoveryCoord.HandleRecovery(ctx, req)
		singleDurations[i] = time.Since(t0)
	}

	p50, p95, p99, mean := calculatePercentiles(singleDurations)
	throughput := float64(time.Second) / float64(mean)
	results = append(results, BenchmarkResult{
		Suite:        "Recovery",
		Scenario:     "Single-Run-Recovery",
		Scale:        "1_failure",
		Iterations:   singleIters,
		P50Duration:  p50,
		P95Duration:  p95,
		P99Duration:  p99,
		MeanDuration: mean,
		Throughput:   throughput,
	})
	fmt.Printf("  • Single Run Recovery: P50=%v P95=%v | Throughput=%.0f recoveries/s\n", p50, p95, throughput)

	// 2. Concurrent Failures (10, 50, 100)
	concurrencyLevels := []int{10, 50, 100}
	for _, concurrency := range concurrencyLevels {
		const burstIters = 20
		burstDurations := make([]time.Duration, burstIters)

		for bIdx := 0; bIdx < burstIters; bIdx++ {
			var wg sync.WaitGroup
			wg.Add(concurrency)
			t0 := time.Now()

			for c := 0; c < concurrency; c++ {
				runID := fmt.Sprintf("run-burst-%d-%d", bIdx, c)
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
			burstDurations[bIdx] = time.Since(t0)
		}

		p50, p95, p99, mean = calculatePercentiles(burstDurations)
		throughput = (float64(concurrency) * float64(time.Second)) / float64(mean)
		results = append(results, BenchmarkResult{
			Suite:        "Recovery",
			Scenario:     "Concurrent-Failure-Burst",
			Scale:        fmt.Sprintf("%d_concurrent", concurrency),
			Iterations:   burstIters,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   throughput,
		})
		fmt.Printf("  • Concurrent Failures (%3d simultaneous): P50=%v P95=%v | Throughput=%.0f recoveries/s\n", concurrency, p50, p95, throughput)
	}

	return results
}
