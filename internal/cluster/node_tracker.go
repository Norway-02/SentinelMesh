package cluster

import (
	"context"
	"sync"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
)

// NodeHealthState represents normalized infrastructure health.
type NodeHealthState string

const (
	NodeStateHealthy   NodeHealthState = "HEALTHY"
	NodeStateDegraded  NodeHealthState = "DEGRADED"
	NodeStateFailed    NodeHealthState = "FAILED"
	NodeStateUnknown   NodeHealthState = "UNKNOWN"
)

// NodeInfo contains tracked metadata and observed state for a cluster node.
type NodeInfo struct {
	ID             string          `json:"id"`
	ClusterID      string          `json:"cluster_id"`
	State          NodeHealthState `json:"state"`
	LastObservedAt time.Time       `json:"last_observed_at"`
	FailureReason  string          `json:"failure_reason,omitempty"`
	Resources      domain.NodeResources `json:"resources"`
}

// NodeTracker maintains normalized cluster node health across infrastructure backends.
type NodeTracker struct {
	mu           sync.RWMutex
	nodes        map[string]*NodeInfo
	provider     scheduler.ResourceProvider
	failedNodes  map[string]time.Time
}

// NewNodeTracker constructs a NodeTracker.
func NewNodeTracker(provider scheduler.ResourceProvider) *NodeTracker {
	return &NodeTracker{
		nodes:       make(map[string]*NodeInfo),
		provider:    provider,
		failedNodes: make(map[string]time.Time),
	}
}

// Sync updates node health states from the underlying ResourceProvider.
func (t *NodeTracker) Sync(ctx context.Context) ([]string, error) {
	if t.provider == nil {
		return nil, nil
	}

	domainNodes, err := t.provider.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	var newlyFailed []string

	for _, dn := range domainNodes {
		existing, exists := t.nodes[dn.ID]
		newState := NodeStateHealthy
		if dn.Health == domain.NodeHealthUnhealthy {
			newState = NodeStateFailed
		}

		// Check if node was manually marked failed
		if _, manuallyFailed := t.failedNodes[dn.ID]; manuallyFailed {
			newState = NodeStateFailed
		}

		if !exists {
			t.nodes[dn.ID] = &NodeInfo{
				ID:             dn.ID,
				ClusterID:      dn.ClusterID,
				State:          newState,
				LastObservedAt: now,
				Resources:      dn.Resources,
			}
			if newState == NodeStateFailed {
				newlyFailed = append(newlyFailed, dn.ID)
			}
		} else {
			if existing.State == NodeStateHealthy && newState == NodeStateFailed {
				newlyFailed = append(newlyFailed, dn.ID)
			}
			existing.State = newState
			existing.LastObservedAt = now
			existing.Resources = dn.Resources
		}
	}

	return newlyFailed, nil
}

// MarkNodeFailed manually injects or confirms a node failure.
func (t *NodeTracker) MarkNodeFailed(nodeID, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.failedNodes[nodeID] = now
	if info, exists := t.nodes[nodeID]; exists {
		info.State = NodeStateFailed
		info.FailureReason = reason
		info.LastObservedAt = now
	} else {
		t.nodes[nodeID] = &NodeInfo{
			ID:             nodeID,
			ClusterID:      "local",
			State:          NodeStateFailed,
			LastObservedAt: now,
			FailureReason:  reason,
		}
	}
}

// IsNodeHealthy returns true if the node is known and in healthy state.
func (t *NodeTracker) IsNodeHealthy(nodeID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if _, failed := t.failedNodes[nodeID]; failed {
		return false
	}

	info, exists := t.nodes[nodeID]
	if !exists {
		return false
	}
	return info.State == NodeStateHealthy
}

// ListFailedNodeIDs returns all currently failed node identifiers.
func (t *NodeTracker) ListFailedNodeIDs() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var failed []string
	for id, info := range t.nodes {
		if info.State == NodeStateFailed {
			failed = append(failed, id)
		}
	}
	for id := range t.failedNodes {
		found := false
		for _, f := range failed {
			if f == id {
				found = true
				break
			}
		}
		if !found {
			failed = append(failed, id)
		}
	}
	return failed
}
