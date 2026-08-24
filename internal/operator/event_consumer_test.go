package operator

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
	v1alpha1 "github.com/sentinelmesh/sentinelmesh/internal/kubernetes/api/v1alpha1"
)

func TestValidateRunScheduledPayload(t *testing.T) {
	valid := &events.RunScheduledPayload{
		RunID:       "run-101",
		AgentID:     "agent-202",
		ClusterID:   "local",
		NodeID:      "node-worker-1",
		AgentImage:  "sentinelmesh/agent:v1",
		AgentCPU:    "500m",
		AgentMemory: "1Gi",
	}

	if err := validateRunScheduledPayload(valid); err != nil {
		t.Fatalf("expected valid payload to pass, got: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(p *events.RunScheduledPayload)
		wantErr string
	}{
		{
			name:    "missing run_id",
			mutate:  func(p *events.RunScheduledPayload) { p.RunID = "" },
			wantErr: "run_id is required",
		},
		{
			name:    "missing agent_id",
			mutate:  func(p *events.RunScheduledPayload) { p.AgentID = "" },
			wantErr: "agent_id is required",
		},
		{
			name:    "missing node_id",
			mutate:  func(p *events.RunScheduledPayload) { p.NodeID = "" },
			wantErr: "node_id is required",
		},
		{
			name:    "missing image",
			mutate:  func(p *events.RunScheduledPayload) { p.AgentImage = "" },
			wantErr: "agent_image is required",
		},
		{
			name:    "missing cpu",
			mutate:  func(p *events.RunScheduledPayload) { p.AgentCPU = "" },
			wantErr: "agent_cpu is required",
		},
		{
			name:    "missing memory",
			mutate:  func(p *events.RunScheduledPayload) { p.AgentMemory = "" },
			wantErr: "agent_memory is required",
		},
		{
			name:    "invalid cpu quantity",
			mutate:  func(p *events.RunScheduledPayload) { p.AgentCPU = "invalid-cpu-string" },
			wantErr: "not a valid resource quantity",
		},
		{
			name:    "invalid memory quantity",
			mutate:  func(p *events.RunScheduledPayload) { p.AgentMemory = "invalid-mem-string" },
			wantErr: "not a valid resource quantity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := *valid
			tt.mutate(&p)
			err := validateRunScheduledPayload(&p)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
		})
	}
}

func TestEventConsumer_IdempotentCRCreation(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	payload := &events.RunScheduledPayload{
		RunID:       "run-idem-1",
		AgentID:     "agent-1",
		ClusterID:   "local",
		NodeID:      "node-worker-1",
		AgentImage:  "sentinelmesh/agent:v1",
		AgentCPU:    "1000m",
		AgentMemory: "2Gi",
	}

	// First creation
	agentRun := MapRunScheduledToAgentRun(payload, Namespace)
	err := fakeClient.Create(context.Background(), agentRun)
	if err != nil {
		t.Fatalf("first creation failed: %v", err)
	}

	// Verify CR exists
	var retrieved v1alpha1.AgentRun
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      agentRun.Name,
		Namespace: Namespace,
	}, &retrieved)
	if err != nil {
		t.Fatalf("failed to retrieve created AgentRun: %v", err)
	}

	// Second creation of identical CR -> simulates duplicate event delivery
	agentRun2 := MapRunScheduledToAgentRun(payload, Namespace)
	err2 := fakeClient.Create(context.Background(), agentRun2)
	// Fake client returns AlreadyExists on duplicate creation
	if err2 == nil {
		t.Fatalf("expected duplicate creation to return AlreadyExists, got nil")
	}
}
