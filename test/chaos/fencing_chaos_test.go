package chaos_test

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	v1alpha1 "github.com/sentinelmesh/sentinelmesh/internal/kubernetes/api/v1alpha1"
	"github.com/sentinelmesh/sentinelmesh/internal/operator"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// TestF12_StaleGenerationReconnectFence verifies that reconnecting clusters running superseded generations are quarantined.
func TestF12_StaleGenerationReconnectFence(t *testing.T) {
	ctx := context.Background()

	runID := "run-f12"
	tokenGen1 := "token-gen1-" + uuid.NewString()
	tokenGen2 := "token-gen2-" + uuid.NewString()

	// 1. Stale Cluster EU-West running Generation 1
	crEU := &v1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{Name: "agentrun-" + runID, Namespace: "sentinelmesh"},
		Spec: v1alpha1.AgentRunSpec{
			RunID:               runID,
			AgentID:             "agent-1",
			ClusterID:           "eu-west-k8s",
			NodeID:              "eu-worker-1",
			Image:               "sentinelmesh/worker:v1",
			ExecutionGeneration: 1,
			FencingToken:        tokenGen1,
		},
		Status: v1alpha1.AgentRunStatus{
			Phase:               v1alpha1.AgentRunPhaseRunning,
			PodName:             "agentrun-" + runID,
			ExecutionGeneration: 1,
			FencingToken:        tokenGen1,
		},
	}

	podEU := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "agentrun-" + runID, Namespace: "sentinelmesh"},
		Spec: corev1.PodSpec{
			NodeName: "eu-worker-1",
			Containers: []corev1.Container{
				{
					Name:  "agent",
					Image: "sentinelmesh/worker:v1",
					Env: []corev1.EnvVar{
						{Name: "SENTINEL_RUN_ID", Value: runID},
						{Name: "SENTINEL_EXECUTION_GENERATION", Value: "1"},
						{Name: "SENTINEL_FENCING_TOKEN", Value: tokenGen1},
					},
				},
			},
		},
	}

	k8sClientEU := fake.NewClientBuilder().
		WithScheme(setupScheme()).
		WithObjects(crEU, podEU).
		WithStatusSubresource(&v1alpha1.AgentRun{}).
		Build()

	reconcilerEU := &operator.AgentRunReconciler{Client: k8sClientEU, Scheme: setupScheme()}

	// 2. Authoritative Control Plane is at Generation 2 on US-East
	globalRun := domain.AgentRun{
		ID:                 runID,
		AgentID:            "agent-1",
		TenantID:           "tenant-1",
		State:              types.StateRunning,
		Cluster:            "us-east-k8s",
		Node:               "us-worker-1",
		RecoveryGeneration: 2,
		FencingToken:       tokenGen2,
	}

	// 3. EU-West reconnects, compares local gen (1) against authoritative gen (2)
	if crEU.Spec.ExecutionGeneration < globalRun.RecoveryGeneration {
		err := reconcilerEU.QuarantineStaleRun(ctx, k8stypes.NamespacedName{Name: crEU.Name, Namespace: crEU.Namespace},
			fmt.Sprintf("superseded by generation %d", globalRun.RecoveryGeneration))
		if err != nil {
			t.Fatalf("QuarantineStaleRun failed: %v", err)
		}
	}

	// 4. Assert: Stale Pod deleted, CR status is FENCED
	var leftoverPod corev1.Pod
	err := k8sClientEU.Get(ctx, k8stypes.NamespacedName{Name: "agentrun-" + runID, Namespace: "sentinelmesh"}, &leftoverPod)
	if err == nil {
		t.Fatal("Stale pod still exists in EU cluster after quarantine!")
	}

	var updatedCR v1alpha1.AgentRun
	_ = k8sClientEU.Get(ctx, k8stypes.NamespacedName{Name: crEU.Name, Namespace: crEU.Namespace}, &updatedCR)
	if updatedCR.Status.Phase != v1alpha1.AgentRunPhaseFenced {
		t.Fatalf("Expected CR Phase %s, got %s", v1alpha1.AgentRunPhaseFenced, updatedCR.Status.Phase)
	}

	t.Logf("Scenario F12 Passed: Stale generation workload successfully fenced and quarantined upon cluster reconnection.")
}
