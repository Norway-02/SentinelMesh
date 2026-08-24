package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
	"github.com/sentinelmesh/sentinelmesh/internal/verification"
)

func runVerificationBenchmarks() []BenchmarkResult {
	fmt.Printf("\n[5/6] RUNNING OUTCOME VERIFICATION & ATTESTATION OVERHEAD BENCHMARKS...\n")
	var results []BenchmarkResult

	// 1. Attestation Digest Hashing (5, 20, 50, 100 rules)
	ruleCounts := []int{5, 20, 50, 100}
	for _, count := range ruleCounts {
		evals := make([]verification.RuleEvaluation, count)
		for i := 0; i < count; i++ {
			evals[i] = verification.RuleEvaluation{
				RuleID:         fmt.Sprintf("rule-invariant-%03d", i),
				RuleType:       "invariant",
				Status:         verification.RulePass,
				Reason:         "assertion passed",
				EvaluatedValue: "200",
				ExpectedValue:  "200",
				DurationNs:     1500,
			}
		}

		const digestIters = 500
		digestDurations := make([]time.Duration, digestIters)
		for i := 0; i < digestIters; i++ {
			t0 := time.Now()
			_ = verification.ComputeEvidenceDigest(evals)
			digestDurations[i] = time.Since(t0)
		}
		p50, p95, p99, mean := calculatePercentiles(digestDurations)
		throughput := float64(time.Second) / float64(mean)
		results = append(results, BenchmarkResult{
			Suite:        "Verification",
			Scenario:     "Attestation-Digest-SHA256",
			Scale:        fmt.Sprintf("%d_rules", count),
			Iterations:   digestIters,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   throughput,
		})
		fmt.Printf("  • Attestation SHA-256 Digest (%3d rules): P50=%v P95=%v | Throughput=%.0f digests/s\n", count, p50, p95, throughput)
	}

	// 2. Lifecycle Verification Overhead Comparison
	overheadResults := runVerificationLifecycleOverheadBenchmark()
	results = append(results, overheadResults...)

	return results
}

func runVerificationLifecycleOverheadBenchmark() []BenchmarkResult {
	ctx := context.Background()
	attestRepo := verification.NewMemoryRepository()
	outboxRepo := outbox.NewMemoryRepository()
	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	txManager := memory.NewTxManager()

	agent := domain.Agent{
		ID:             "agent-overhead-bench",
		TenantID:       "tenant-1",
		Name:           "overhead-agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "1", Memory: "1024Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		VerificationPolicy: types.VerificationPolicy{
			Enabled: true,
			InvariantRules: []types.InvariantRule{
				{ID: "rule-1", MetricName: "status", Operator: "eq", ExpectedValue: "success"},
				{ID: "rule-2", MetricName: "exit_code", Operator: "eq", ExpectedValue: "0"},
			},
		},
		Image: "sentinelmesh/worker:v1",
	}
	_ = agentRepo.Create(ctx, agent)
	svc := verification.NewService(attestRepo, agentRepo, runRepo, outboxRepo, txManager, nil, nil)

	const iters = 200
	verifyDurations := make([]time.Duration, iters)
	for i := 0; i < iters; i++ {
		runID := fmt.Sprintf("run-verify-over-%d", i)
		_ = runRepo.Create(ctx, domain.AgentRun{
			ID:        runID,
			AgentID:   agent.ID,
			TenantID:  agent.TenantID,
			State:     types.StateRunning,
			Version:   1,
			CreatedAt: time.Now(),
		})

		req := verification.VerifyRunRequest{
			RunID: runID,
			ReportedMetrics: map[string]string{
				"status":    "success",
				"exit_code": "0",
			},
		}

		t0 := time.Now()
		_, _ = svc.VerifyRun(ctx, req)
		verifyDurations[i] = time.Since(t0)
	}

	p50, p95, p99, mean := calculatePercentiles(verifyDurations)
	throughput := float64(time.Second) / float64(mean)
	fmt.Printf("  • Full Verification Service: P50=%v P95=%v | Throughput=%.0f verifications/s\n", p50, p95, throughput)

	return []BenchmarkResult{
		{
			Suite:        "Verification",
			Scenario:     "Service-VerifyRun-Overhead",
			Scale:        "standard_policy",
			Iterations:   iters,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   throughput,
		},
	}
}

func runE2EBenchmarks() []BenchmarkResult {
	fmt.Printf("\n[6/6] RUNNING FULL LIFECYCLE END-TO-END BENCHMARK (Create -> Schedule -> Checkpoint -> Verify -> Attest -> Complete)...\n")
	ctx := context.Background()

	txManager := memory.NewTxManager()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	checkpointRepo := checkpoint.NewMemoryRepository()
	attestRepo := verification.NewMemoryRepository()

	agent := domain.Agent{
		ID:             "agent-e2e-cli",
		TenantID:       "tenant-1",
		Name:           "e2e-cli-agent",
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
	prov := &staticSimpleNodeProvider{nodes: nodes}
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, prov)
	cpSvc := checkpoint.NewService(checkpointRepo, outboxRepo, txManager)
	verifySvc := verification.NewService(attestRepo, agentRepo, runRepo, outboxRepo, txManager, nil, nil)

	checkpointPayload := []byte(`{"step":50,"status":"completed"}`)
	const iters = 100
	e2eDurations := make([]time.Duration, iters)

	for i := 0; i < iters; i++ {
		runID := fmt.Sprintf("run-e2e-cli-%d", i)
		t0 := time.Now()

		_ = runRepo.Create(ctx, domain.AgentRun{
			ID:        runID,
			AgentID:   agent.ID,
			TenantID:  agent.TenantID,
			State:     types.StateQueued,
			Version:   1,
			CreatedAt: time.Now(),
		})

		_ = schedSvc.ScheduleRun(ctx, runID)

		run, _ := runRepo.Get(ctx, runID)
		_ = run.TransitionTo(types.StateStarting)
		_ = run.TransitionTo(types.StateRunning)
		_ = runRepo.Update(ctx, run)

		_, _ = cpSvc.SaveCheckpoint(ctx, checkpoint.SaveCheckpointRequest{
			RunID:          runID,
			AgentID:        agent.ID,
			TenantID:       agent.TenantID,
			SequenceNumber: 50,
			StateInline:    checkpointPayload,
		})

		_, _ = verifySvc.VerifyRun(ctx, verification.VerifyRunRequest{
			RunID: runID,
			ReportedMetrics: map[string]string{
				"status": "completed",
			},
		})

		run, _ = runRepo.Get(ctx, runID)
		_ = run.TransitionTo(types.StateCompleted)
		_ = runRepo.Update(ctx, run)

		e2eDurations[i] = time.Since(t0)
	}

	p50, p95, p99, mean := calculatePercentiles(e2eDurations)
	throughput := float64(time.Second) / float64(mean)
	fmt.Printf("  • Complete Agent Run Lifecycle: P50=%v P95=%v | Throughput=%.0f full runs/s\n", p50, p95, throughput)

	return []BenchmarkResult{
		{
			Suite:        "EndToEnd",
			Scenario:     "Complete-Agent-Run-Lifecycle",
			Scale:        "full_pipeline",
			Iterations:   iters,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   throughput,
		},
	}
}
