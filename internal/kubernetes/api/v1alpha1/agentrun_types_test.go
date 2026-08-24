package v1alpha1

import (
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestAgentRun_SchemeRegistration(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1alpha1 to scheme: %v", err)
	}

	gvks, _, err := scheme.ObjectKinds(&AgentRun{})
	if err != nil {
		t.Fatalf("failed to get GVK for AgentRun: %v", err)
	}
	if len(gvks) == 0 {
		t.Fatalf("expected at least one GVK registered for AgentRun")
	}

	expectedGVK := GroupVersion.WithKind("AgentRun")
	found := false
	for _, gvk := range gvks {
		if gvk == expectedGVK {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GVK %v, got %v", expectedGVK, gvks)
	}
}

func TestAgentRun_JSONSerialization(t *testing.T) {
	now := metav1.NewTime(time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	run := &AgentRun{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "sentinelmesh.io/v1alpha1",
			Kind:       "AgentRun",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-test-123",
			Namespace: "sentinelmesh",
			UID:       types.UID("test-uid"),
		},
		Spec: AgentRunSpec{
			RunID:         "run-123",
			AgentID:       "agent-456",
			ClusterID:     "local",
			NodeID:        "node-worker-1",
			Image:         "sentinelmesh/example-agent:v1",
			SecurityClass: "confidential",
			Resources: AgentRunResources{
				CPU:    "2000m",
				Memory: "4Gi",
			},
		},
		Status: AgentRunStatus{
			Phase:     AgentRunPhaseRunning,
			PodName:   "agentrun-test-123",
			NodeName:  "node-worker-1",
			StartTime: &now,
			Message:   "Pod is running",
			Conditions: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: now,
					Reason:             "PodRunning",
					Message:            "AgentRun is in Running phase",
				},
			},
		},
	}

	data, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("failed to marshal AgentRun: %v", err)
	}

	var unmarshaled AgentRun
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal AgentRun: %v", err)
	}

	if unmarshaled.Spec.RunID != "run-123" {
		t.Errorf("expected RunID 'run-123', got '%s'", unmarshaled.Spec.RunID)
	}
	if unmarshaled.Spec.Resources.CPU != "2000m" {
		t.Errorf("expected CPU '2000m', got '%s'", unmarshaled.Spec.Resources.CPU)
	}
	if unmarshaled.Spec.Resources.Memory != "4Gi" {
		t.Errorf("expected Memory '4Gi', got '%s'", unmarshaled.Spec.Resources.Memory)
	}
	if unmarshaled.Status.Phase != AgentRunPhaseRunning {
		t.Errorf("expected status phase 'Running', got '%s'", unmarshaled.Status.Phase)
	}
	if unmarshaled.Status.PodName != "agentrun-test-123" {
		t.Errorf("expected PodName 'agentrun-test-123', got '%s'", unmarshaled.Status.PodName)
	}
	if len(unmarshaled.Status.Conditions) != 1 || unmarshaled.Status.Conditions[0].Status != metav1.ConditionTrue {
		t.Errorf("expected 1 Ready condition with True status, got %v", unmarshaled.Status.Conditions)
	}
}

func TestAgentRun_DeepCopy(t *testing.T) {
	orig := &AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "agentrun-original",
		},
		Spec: AgentRunSpec{
			RunID:  "run-orig",
			NodeID: "node-1",
			Resources: AgentRunResources{
				CPU:    "1",
				Memory: "2Gi",
			},
		},
		Status: AgentRunStatus{
			Phase: AgentRunPhasePending,
		},
	}

	copied := orig.DeepCopy()
	if copied == orig {
		t.Fatal("DeepCopy returned the same pointer")
	}
	if copied.Spec.RunID != orig.Spec.RunID {
		t.Errorf("DeepCopy failed to copy spec.RunID")
	}

	copied.Spec.RunID = "modified"
	if orig.Spec.RunID == "modified" {
		t.Error("Modifying copy mutated original")
	}
}
