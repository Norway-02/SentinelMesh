package scheduler

import (
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestFilterNodes(t *testing.T) {
	agent := &domain.Agent{
		Resources: types.AgentResources{
			CPU:    "2",
			Memory: "4Gi",
			GPU:    0,
		},
		SecurityPolicy: types.SecurityPolicy{
			Profile: "public",
		},
	}

	nodes := []domain.Node{
		{
			ID: "n1", Health: domain.NodeHealthHealthy, SecurityClass: "public",
			Resources: domain.NodeResources{CPUAvailable: 4.0, MemoryAvailable: 8192},
		}, // should pass
		{
			ID: "n2", Health: domain.NodeHealthHealthy, SecurityClass: "public",
			Resources: domain.NodeResources{CPUAvailable: 1.0, MemoryAvailable: 8192},
		}, // fail: cpu
		{
			ID: "n3", Health: domain.NodeHealthHealthy, SecurityClass: "public",
			Resources: domain.NodeResources{CPUAvailable: 4.0, MemoryAvailable: 2048},
		}, // fail: mem
		{
			ID: "n4", Health: domain.NodeHealthHealthy, SecurityClass: "confidential",
			Resources: domain.NodeResources{CPUAvailable: 4.0, MemoryAvailable: 8192},
		}, // fail: security
		{
			ID: "n5", Health: domain.NodeHealthUnhealthy, SecurityClass: "public",
			Resources: domain.NodeResources{CPUAvailable: 4.0, MemoryAvailable: 8192},
		}, // fail: health
	}

	valid := filterNodes(agent, nodes)
	assert.Len(t, valid, 1)
	assert.Equal(t, "n1", valid[0].ID)
}

func TestScoreNodes(t *testing.T) {
	agent := &domain.Agent{
		Resources: types.AgentResources{
			CPU:    "4",
			Memory: "8Gi",
			GPU:    0,
		},
		Priority: "normal",
	}

	nodes := []domain.Node{
		{
			ID: "node-small-fit", 
			Resources: domain.NodeResources{CPUCapacity: 6.0, CPUAvailable: 6.0, MemoryCapacity: 12288, MemoryAvailable: 12288}, // 12Gi
			NetworkLatency: 0.1, CostPerHour: 0.1,
		},
		{
			ID: "node-huge-waste", 
			Resources: domain.NodeResources{CPUCapacity: 128.0, CPUAvailable: 128.0, MemoryCapacity: 262144, MemoryAvailable: 262144},
			NetworkLatency: 0.1, CostPerHour: 5.0,
		},
	}

	best, dec, err := scoreNodes(agent, nodes)
	assert.NoError(t, err)
	assert.Equal(t, "node-small-fit", best.ID) // small node should win due to better headroom/cost
	assert.True(t, dec.ResourceFit > 0)
}
