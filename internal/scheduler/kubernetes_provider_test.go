package scheduler

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
)

func TestKubernetesResourceProvider_ListNodes_Mapping(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	k8sNodeReady := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-node-1",
			Labels: map[string]string{
				"kubernetes.io/hostname":         "worker-node-1",
				"sentinelmesh.io/security-class": "confidential",
				"topology.kubernetes.io/zone":    "us-east-1a",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    "dedicated",
					Value:  "ai-workloads",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2000m"), // 2.0 cores
				corev1.ResourceMemory: resource.MustParse("4Gi"),   // 4096 MB
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	k8sNodeUnready := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-node-2",
			Labels: map[string]string{
				"kubernetes.io/hostname": "worker-node-2",
			},
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("7500m"),
				corev1.ResourceMemory: resource.MustParse("14Gi"),
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(k8sNodeReady, k8sNodeUnready).
		Build()

	provider := NewKubernetesResourceProvider(fakeClient)

	nodes, err := provider.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("failed to list nodes: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	// Verify worker-node-1 mapping
	var n1 domain.Node
	for _, n := range nodes {
		if n.ID == "worker-node-1" {
			n1 = n
			break
		}
	}

	if n1.ID != "worker-node-1" {
		t.Fatalf("node worker-node-1 not found in mapped list")
	}

	// 2000m CPU allocatable -> 2.0 float64
	if n1.Resources.CPUAvailable != 2.0 {
		t.Errorf("expected CPUAvailable 2.0, got %f", n1.Resources.CPUAvailable)
	}
	// 4 cores total capacity -> 4.0 float64
	if n1.Resources.CPUCapacity != 4.0 {
		t.Errorf("expected CPUCapacity 4.0, got %f", n1.Resources.CPUCapacity)
	}
	// 4Gi memory allocatable -> 4096 MB
	if n1.Resources.MemoryAvailable != 4096.0 {
		t.Errorf("expected MemoryAvailable 4096 MB, got %f", n1.Resources.MemoryAvailable)
	}
	// 8Gi memory total capacity -> 8192 MB
	if n1.Resources.MemoryCapacity != 8192.0 {
		t.Errorf("expected MemoryCapacity 8192 MB, got %f", n1.Resources.MemoryCapacity)
	}
	// Ready condition -> Healthy
	if n1.Health != domain.NodeHealthHealthy {
		t.Errorf("expected NodeHealthHealthy, got %s", n1.Health)
	}
	// Security class from label
	if n1.SecurityClass != "confidential" {
		t.Errorf("expected securityClass 'confidential', got '%s'", n1.SecurityClass)
	}
	// Labels preserved
	if n1.Labels["topology.kubernetes.io/zone"] != "us-east-1a" {
		t.Errorf("expected zone label 'us-east-1a', got '%s'", n1.Labels["topology.kubernetes.io/zone"])
	}
	// Taints mapped
	if len(n1.Taints) != 1 || n1.Taints[0] != "dedicated=ai-workloads:NoSchedule" {
		t.Errorf("expected taint 'dedicated=ai-workloads:NoSchedule', got %v", n1.Taints)
	}

	// Verify worker-node-2 mapping (Unready)
	var n2 domain.Node
	for _, n := range nodes {
		if n.ID == "worker-node-2" {
			n2 = n
			break
		}
	}
	if n2.Health != domain.NodeHealthUnhealthy {
		t.Errorf("expected unready node to have NodeHealthUnhealthy, got %s", n2.Health)
	}
	if n2.SecurityClass != "standard" {
		t.Errorf("expected default securityClass 'standard', got '%s'", n2.SecurityClass)
	}
}
