package scheduler_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// generateSyntheticNodes creates a slice of synthetic node records for scale testing.
func generateSyntheticNodes(count int, seed int64) []domain.Node {
	rng := rand.New(rand.NewSource(seed))
	nodes := make([]domain.Node, count)

	for i := 0; i < count; i++ {
		// 95% healthy, 5% degraded/failed
		health := domain.NodeHealthHealthy
		if rng.Float64() < 0.05 {
			health = domain.NodeHealthDegraded
		}

		// Security profile distribution
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

func benchmarkAgent() domain.Agent {
	return domain.Agent{
		ID:             "agent-bench",
		TenantID:       "tenant-bench",
		Name:           "bench-agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "2", Memory: "4Gi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Image:          "sentinelmesh/worker:v1",
	}
}

func BenchmarkScheduler_DeterministicV1(b *testing.B) {
	scales := []int{100, 1000, 10000, 100000, 1000000}
	agent := benchmarkAgent()

	for _, scale := range scales {
		nodes := generateSyntheticNodes(scale, 42)
		b.Run(fmt.Sprintf("Nodes_%d", scale), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Score nodes
				_, _, err := scheduler.ScoreValidNodes(&agent, nodes)
				if err != nil && scale > 0 {
					b.Fatalf("Unexpected scheduling error: %v", err)
				}
			}
		})
	}
}

func BenchmarkScheduler_FirstFit(b *testing.B) {
	scales := []int{100, 1000, 10000, 100000, 1000000}
	agent := benchmarkAgent()

	for _, scale := range scales {
		nodes := generateSyntheticNodes(scale, 42)
		b.Run(fmt.Sprintf("Nodes_%d", scale), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := scheduler.FindFirstFitNode(&agent, nodes)
				if err != nil && scale > 0 {
					b.Fatalf("First-fit failed: %v", err)
				}
			}
		})
	}
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

func BenchmarkScheduler_TwoTierMultiCluster(b *testing.B) {
	ctx := context.Background()
	clusterCount := 100
	nodesPerCluster := 10000 // 1,000,000 total nodes
	agent := benchmarkAgent()

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

	b.Run("TwoTier_100Clusters_1MNodes", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runID := fmt.Sprintf("run-bench-%d", i)
			_ = runRepo.Create(ctx, domain.AgentRun{
				ID:        runID,
				AgentID:   agent.ID,
				TenantID:  agent.TenantID,
				State:     types.StateQueued,
				Version:   1,
				CreatedAt: time.Now(),
			})
			if err := schedSvc.ScheduleRun(ctx, runID); err != nil {
				b.Fatalf("Two-tier schedule failed: %v", err)
			}
		}
	})
}
