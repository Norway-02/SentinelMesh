package operator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/sentinelmesh/sentinelmesh/internal/kubernetes/api/v1alpha1"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
)

// MapRunScheduledToAgentRun translates a validated RunScheduledPayload into an
// AgentRun CR representing the desired state.
func MapRunScheduledToAgentRun(p *events.RunScheduledPayload, namespace string) *v1alpha1.AgentRun {
	agentRun := &v1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-" + p.RunID,
			Namespace: namespace,
			Labels: map[string]string{
				"sentinelmesh.io/run-id":     p.RunID,
				"sentinelmesh.io/agent-id":   p.AgentID,
				"sentinelmesh.io/cluster-id": p.ClusterID,
			},
		},
		Spec: v1alpha1.AgentRunSpec{
			RunID:               p.RunID,
			AgentID:             p.AgentID,
			ClusterID:           p.ClusterID,
			NodeID:              p.NodeID,
			Image:               p.AgentImage,
			ExecutionGeneration: p.ExecutionGeneration,
			FencingToken:        p.FencingToken,
			Resources: v1alpha1.AgentRunResources{
				CPU:    p.AgentCPU,
				Memory: p.AgentMemory,
			},
		},
	}

	if p.Checkpoint != nil {
		agentRun.Spec.RestoreCheckpointID = p.Checkpoint.ID
		agentRun.Spec.RestoreStep = p.Checkpoint.Sequence
	}

	return agentRun
}
