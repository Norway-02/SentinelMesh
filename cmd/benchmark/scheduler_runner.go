package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func runSchedulerBenchmarks() []BenchmarkResult {
	fmt.Printf("\n[1/6] RUNNING SCHEDULER SCALABILITY BENCHMARKS (100 to 1,000,000 Nodes)...\n")
	var results []BenchmarkResult

	scales := []int{100, 1000, 10000, 100000, 1000000}
	agent := domain.Agent{
		ID:             "agent-bench",
		TenantID:       "tenant-bench",
		Name:           "bench-agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "2", Memory: "4Gi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Image:          "sentinelmesh/worker:v1",
	}

	for _, scale := range scales {
		nodes := generateSyntheticNodes(scale, 42)

		// 1. First-Fit baseline
		ffDurations := make([]time.Duration, 20)
		for i := 0; i < 20; i++ {
			t0 := time.Now()
			_, _ = scheduler.FindFirstFitNode(&agent, nodes)
			ffDurations[i] = time.Since(t0)
		}
		p50, p95, p99, mean := calculatePercentiles(ffDurations)
		throughput := float64(time.Second) / float64(mean)
		results = append(results, BenchmarkResult{
			Suite:        "Scheduler",
			Scenario:     "First-Fit",
			Scale:        fmt.Sprintf("%d_nodes", scale),
			Iterations:   20,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   throughput,
		})
		fmt.Printf("  • First-Fit (%7d nodes): P50=%v P95=%v | Throughput=%.0f ops/s\n", scale, p50, p95, throughput)

		// 2. Deterministic v1
		// For 1M nodes run 5 iterations, else 20
		iters := 20
		if scale == 1000000 {
			iters = 5
		}
		v1Durations := make([]time.Duration, iters)
		for i := 0; i < iters; i++ {
			t0 := time.Now()
			_, _, _ = scheduler.ScoreValidNodes(&agent, nodes)
			v1Durations[i] = time.Since(t0)
		}
		p50, p95, p99, mean = calculatePercentiles(v1Durations)
		throughput = float64(time.Second) / float64(mean)
		results = append(results, BenchmarkResult{
			Suite:        "Scheduler",
			Scenario:     "Deterministic-v1",
			Scale:        fmt.Sprintf("%d_nodes", scale),
			Iterations:   iters,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   throughput,
		})
		fmt.Printf("  • Deterministic-v1 (%7d nodes): P50=%v P95=%v | Throughput=%.0f ops/s\n", scale, p50, p95, throughput)
	}

	// 3. Two-Tier Multi-Cluster (100 clusters, 1M total nodes)
	twoTierResults := runTwoTierSchedulerBenchmark(agent)
	results = append(results, twoTierResults...)

	return results
}

type syntheticMultiClusterProvider struct {
	clusters []domain.Cluster
	nodes    map[string][]domain.Node
}

func (p *syntheticMultiClusterProvider) ListClusters(ctx context.Context) ([]domain.Cluster, error) {
	return p.clusters, nil
}

func (p *syntheticMultiClusterProvider) ListNodes(ctx context.Context, clusterID string) ([]domain.Node, error) {
	return p.nodes[clusterID], nil
}

func runTwoTierSchedulerBenchmark(agent domain.Agent) []BenchmarkResult {
	ctx := context.Background()
	clusterCount := 100
	nodesPerCluster := 10000 // 1,000,000 total nodes

	clusters := make([]domain.Cluster, clusterCount)
	nodeMap := make(map[string][]domain.Node, clusterCount)
	allNodes := generateSyntheticNodes(clusterCount*nodesPerCluster, 42)

	for c := 0; c < clusterCount; c++ {
		cid := fmt.Sprintf("cluster-%03d", c)
		clusters[c] = domain.Cluster{
			ID:              cid,
			Name:            cid,
			Region:          "us-east-1",
			ProviderType:    domain.ProviderKubernetes,
			SecurityClasses: []string{"standard"},
			NetworkCost:     1.0,
			BaseLatencyMs:   20.0,
			Status: domain.ClusterStatus{
				Health:          domain.ClusterHealthHealthy,
				TotalCPU:        50000.0,
				TotalMemory:     100000000.0,
				AvailableCPU:    40000.0,
				AvailableMemory: 80000000.0,
				LastHeartbeatAt: time.Now(),
			},
		}
		start := c * nodesPerCluster
		end := start + nodesPerCluster
		nodeMap[cid] = allNodes[start:end]
	}

	prov := &syntheticMultiClusterProvider{clusters: clusters, nodes: nodeMap}
	txManager := memory.NewTxManager()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	_ = agentRepo.Create(ctx, agent)

	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, nil).
		WithClusterResourceProvider(prov, scheduler.DefaultClusterScoringPolicy())

	iters := 25
	durations := make([]time.Duration, iters)
	for i := 0; i < iters; i++ {
		runID := fmt.Sprintf("run-twotier-%d", i)
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
	fmt.Printf("  • Two-Tier Multi-Cluster (100 Clusters / 1M Nodes): P50=%v P95=%v | Throughput=%.0f ops/s\n", p50, p95, throughput)

	return []BenchmarkResult{
		{
			Suite:        "Scheduler",
			Scenario:     "Two-Tier-MultiCluster",
			Scale:        "100clusters_1Mnodes",
			Iterations:   iters,
			P50Duration:  p50,
			P95Duration:  p95,
			P99Duration:  p99,
			MeanDuration: mean,
			Throughput:   throughput,
		},
	}
}

func generateSyntheticNodes(count int, seed int64) []domain.Node {
	rng := rand.New(rand.NewSource(seed))
	nodes := make([]domain.Node, count)

	for i := 0; i < count; i++ {
		health := domain.NodeHealthHealthy
		if rng.Float64() < 0.05 {
			health = domain.NodeHealthDegraded
		}
		sec := "public"
		if rng.Float64() < 0.20 {
			sec = "restricted"
		}

		cpuCap := 100.0 + rng.Float64()*400.0
		memCap := 200000.0 + rng.Float64()*800000.0
		cpuAvail := cpuCap * (0.20 + rng.Float64()*0.70)
		memAvail := memCap * (0.20 + rng.Float64()*0.70)

		nodes[i] = domain.Node{
			ID:            fmt.Sprintf("node-%07d", i),
			ClusterID:     fmt.Sprintf("cluster-%03d", i%100),
			SecurityClass: sec,
			Health:        health,
			Resources: domain.NodeResources{
				CPUCapacity:     cpuCap,
				CPUAvailable:    cpuAvail,
				MemoryCapacity:  memCap,
				MemoryAvailable: memAvail,
			},
			CostPerHour:    1.0 + rng.Float64()*2.0,
			NetworkLatency: 0.1 + rng.Float64()*0.4,
		}
	}
	return nodes
}
