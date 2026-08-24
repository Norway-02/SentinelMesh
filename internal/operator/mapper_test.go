package operator

import (
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
)

func TestMapRunScheduledToAgentRun(t *testing.T) {
	payload := &events.RunScheduledPayload{
		RunID:       "run-987",
		AgentID:     "agent-654",
		ClusterID:   "local",
		NodeID:      "worker-2",
		AgentImage:  "sentinelmesh/worker:v1",
		AgentCPU:    "1500m",
		AgentMemory: "2Gi",
	}

	cr := MapRunScheduledToAgentRun(payload, "sentinelmesh")

	if cr.Name != "agentrun-run-987" {
		t.Errorf("expected CR name 'agentrun-run-987', got '%s'", cr.Name)
	}
	if cr.Namespace != "sentinelmesh" {
		t.Errorf("expected namespace 'sentinelmesh', got '%s'", cr.Namespace)
	}
	if cr.Labels["sentinelmesh.io/run-id"] != "run-987" {
		t.Errorf("expected label run-id 'run-987', got '%s'", cr.Labels["sentinelmesh.io/run-id"])
	}
	if cr.Labels["sentinelmesh.io/agent-id"] != "agent-654" {
		t.Errorf("expected label agent-id 'agent-654', got '%s'", cr.Labels["sentinelmesh.io/agent-id"])
	}
	if cr.Spec.RunID != "run-987" {
		t.Errorf("expected Spec.RunID 'run-987', got '%s'", cr.Spec.RunID)
	}
	if cr.Spec.NodeID != "worker-2" {
		t.Errorf("expected Spec.NodeID 'worker-2', got '%s'", cr.Spec.NodeID)
	}
	if cr.Spec.Image != "sentinelmesh/worker:v1" {
		t.Errorf("expected Spec.Image 'sentinelmesh/worker:v1', got '%s'", cr.Spec.Image)
	}
	if cr.Spec.Resources.CPU != "1500m" || cr.Spec.Resources.Memory != "2Gi" {
		t.Errorf("expected CPU '1500m' and Memory '2Gi', got CPU '%s' and Memory '%s'",
			cr.Spec.Resources.CPU, cr.Spec.Resources.Memory)
	}
}
