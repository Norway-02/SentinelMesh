package scheduler

import (
	"fmt"
	"math/rand"
	"testing"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func generateNodes(count int) []domain.Node {
	rand.Seed(42) // deterministic benchmark
	nodes := make([]domain.Node, count)
	for i := 0; i < count; i++ {
		capCPU := float64(4 + rand.Intn(60)) // 4 to 64 cores
		capMem := float64(4096 + rand.Intn(250000)) // 4GB to 256GB
		nodes[i] = domain.Node{
			ID:        fmt.Sprintf("node-%d", i),
			ClusterID: "bench-cluster",
			Resources: domain.NodeResources{
				CPUCapacity:     capCPU,
				CPUAvailable:    capCPU * rand.Float64(),
				MemoryCapacity:  capMem,
				MemoryAvailable: capMem * rand.Float64(),
				GPUCapacity:     rand.Intn(2),
				GPUAvailable:    rand.Intn(2),
			},
			Health:         domain.NodeHealthHealthy,
			SecurityClass:  "public",
			NetworkLatency: rand.Float64(),
			CostPerHour:    rand.Float64() * 5.0,
		}
	}
	return nodes
}

func firstFitScore(agent *domain.Agent, validNodes []domain.Node) (domain.Node, domain.SchedulingDecision, error) {
	if len(validNodes) == 0 {
		return domain.Node{}, domain.SchedulingDecision{}, fmt.Errorf("no feasible nodes")
	}
	return validNodes[0], domain.SchedulingDecision{ResourceFit: 1.0}, nil
}

func benchmarkScheduler(b *testing.B, nodeCount int, useFirstFit bool) {
	nodes := generateNodes(nodeCount)
	agent := &domain.Agent{
		Resources: types.AgentResources{
			CPU:    "4",
			Memory: "8Gi",
			GPU:    0,
		},
		Priority: "normal",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid := filterNodes(agent, nodes)
		if useFirstFit {
			_, _, _ = firstFitScore(agent, valid)
		} else {
			_, _, _ = scoreNodes(agent, valid)
		}
	}
}

func BenchmarkScheduler_100_Nodes_FirstFit(b *testing.B) { benchmarkScheduler(b, 100, true) }
func BenchmarkScheduler_100_Nodes_DetV1(b *testing.B)    { benchmarkScheduler(b, 100, false) }

func BenchmarkScheduler_1000_Nodes_FirstFit(b *testing.B) { benchmarkScheduler(b, 1000, true) }
func BenchmarkScheduler_1000_Nodes_DetV1(b *testing.B)    { benchmarkScheduler(b, 1000, false) }

func BenchmarkScheduler_10000_Nodes_FirstFit(b *testing.B) { benchmarkScheduler(b, 10000, true) }
func BenchmarkScheduler_10000_Nodes_DetV1(b *testing.B)    { benchmarkScheduler(b, 10000, false) }

func BenchmarkScheduler_100000_Nodes_FirstFit(b *testing.B) { benchmarkScheduler(b, 100000, true) }
func BenchmarkScheduler_100000_Nodes_DetV1(b *testing.B)    { benchmarkScheduler(b, 100000, false) }
