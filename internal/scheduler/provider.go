package scheduler

import (
	"context"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
)

// ResourceProvider abstracts the source of cluster node state.
type ResourceProvider interface {
	ListNodes(ctx context.Context) ([]domain.Node, error)
}

// StaticResourceProvider is a stub implementation for Stage 7.
type StaticResourceProvider struct {
	nodes []domain.Node
}

func NewStaticResourceProvider() *StaticResourceProvider {
	return &StaticResourceProvider{
		nodes: []domain.Node{
			{
				ID:        "node-small-1",
				ClusterID: "local-cluster",
				Resources: domain.NodeResources{
					CPUCapacity:     2.0,
					CPUAvailable:    2.0,
					MemoryCapacity:  4096, // MB
					MemoryAvailable: 4096,
					GPUCapacity:     0,
					GPUAvailable:    0,
				},
				Health:         domain.NodeHealthHealthy,
				SecurityClass:  "public",
				NetworkLatency: 0.8,
				CostPerHour:    0.05,
			},
			{
				ID:        "node-large-1",
				ClusterID: "local-cluster",
				Resources: domain.NodeResources{
					CPUCapacity:     16.0,
					CPUAvailable:    12.0,
					MemoryCapacity:  32768, // MB
					MemoryAvailable: 24000,
					GPUCapacity:     1,
					GPUAvailable:    1,
				},
				Health:         domain.NodeHealthHealthy,
				SecurityClass:  "confidential",
				NetworkLatency: 0.5,
				CostPerHour:    1.50,
			},
			{
				ID:        "node-unhealthy-1",
				ClusterID: "local-cluster",
				Resources: domain.NodeResources{
					CPUCapacity:     8.0,
					CPUAvailable:    8.0,
					MemoryCapacity:  16384,
					MemoryAvailable: 16384,
					GPUCapacity:     0,
					GPUAvailable:    0,
				},
				Health:         domain.NodeHealthUnhealthy,
				SecurityClass:  "public",
				NetworkLatency: 0.9,
				CostPerHour:    0.10,
			},
		},
	}
}

func (p *StaticResourceProvider) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return p.nodes, nil
}
