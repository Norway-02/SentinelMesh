package operator

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/sentinelmesh/sentinelmesh/internal/kubernetes/api/v1alpha1"
)

func setupTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

func TestReconciler_AgentRunExists_PodAbsent_CreatesPod(t *testing.T) {
	scheme := setupTestScheme()

	agentRun := &v1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-run-001",
			Namespace: "sentinelmesh",
		},
		Spec: v1alpha1.AgentRunSpec{
			RunID:     "run-001",
			AgentID:   "agent-001",
			ClusterID: "local",
			NodeID:    "worker-node-1",
			Image:     "sentinelmesh/test-agent:v1",
			Resources: v1alpha1.AgentRunResources{
				CPU:    "1000m",
				Memory: "2Gi",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(agentRun).
		WithStatusSubresource(&v1alpha1.AgentRun{}).
		Build()

	reconciler := &AgentRunReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "agentrun-run-001",
			Namespace: "sentinelmesh",
		},
	}

	res, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected reconcile error: %v", err)
	}
	if res.Requeue {
		t.Errorf("expected no requeue")
	}

	// Verify Pod was created
	var pod corev1.Pod
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "agentrun-run-001",
		Namespace: "sentinelmesh",
	}, &pod)
	if err != nil {
		t.Fatalf("expected Pod to be created, got error: %v", err)
	}

	// Verify hard placement via NodeName
	if pod.Spec.NodeName != "worker-node-1" {
		t.Errorf("expected Pod.Spec.NodeName to be 'worker-node-1', got '%s'", pod.Spec.NodeName)
	}

	// Verify OwnerReference
	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("expected 1 OwnerReference, got %d", len(pod.OwnerReferences))
	}
	if pod.OwnerReferences[0].Name != agentRun.Name {
		t.Errorf("expected OwnerReference name '%s', got '%s'", agentRun.Name, pod.OwnerReferences[0].Name)
	}

	// Verify AgentRun status updated to Creating
	var updatedRun v1alpha1.AgentRun
	_ = fakeClient.Get(context.Background(), req.NamespacedName, &updatedRun)
	if updatedRun.Status.Phase != v1alpha1.AgentRunPhaseCreating {
		t.Errorf("expected status Creating, got '%s'", updatedRun.Status.Phase)
	}
	if updatedRun.Status.PodName != "agentrun-run-001" {
		t.Errorf("expected status.podName 'agentrun-run-001', got '%s'", updatedRun.Status.PodName)
	}
}

func TestReconciler_Idempotent_DoesNotDuplicatePod(t *testing.T) {
	scheme := setupTestScheme()

	agentRun := &v1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-run-002",
			Namespace: "sentinelmesh",
		},
		Spec: v1alpha1.AgentRunSpec{
			RunID:   "run-002",
			AgentID: "agent-002",
			NodeID:  "worker-node-1",
			Image:   "sentinelmesh/test-agent:v1",
			Resources: v1alpha1.AgentRunResources{
				CPU:    "500m",
				Memory: "1Gi",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(agentRun).
		WithStatusSubresource(&v1alpha1.AgentRun{}).
		Build()

	reconciler := &AgentRunReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "agentrun-run-002",
			Namespace: "sentinelmesh",
		},
	}

	// Reconcile 1: creates Pod
	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("first reconcile failed: %v", err)
	}

	// Reconcile 2: Pod already exists, should not fail or duplicate
	_, err = reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}

	// Verify only 1 Pod exists in namespace
	var podList corev1.PodList
	err = fakeClient.List(context.Background(), &podList)
	if err != nil {
		t.Fatalf("failed to list pods: %v", err)
	}
	if len(podList.Items) != 1 {
		t.Fatalf("expected exactly 1 Pod, found %d", len(podList.Items))
	}
}

func TestReconciler_PodRunning_UpdatesStatusToRunning(t *testing.T) {
	scheme := setupTestScheme()

	agentRun := &v1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-run-003",
			Namespace: "sentinelmesh",
		},
		Spec: v1alpha1.AgentRunSpec{
			RunID:   "run-003",
			AgentID: "agent-003",
			NodeID:  "worker-node-2",
			Image:   "sentinelmesh/test-agent:v1",
			Resources: v1alpha1.AgentRunResources{
				CPU:    "1000m",
				Memory: "2Gi",
			},
		},
		Status: v1alpha1.AgentRunStatus{
			Phase: v1alpha1.AgentRunPhaseCreating,
		},
	}

	startTime := metav1.NewTime(time.Now())
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-run-003",
			Namespace: "sentinelmesh",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-node-2",
		},
		Status: corev1.PodStatus{
			Phase:     corev1.PodRunning,
			StartTime: &startTime,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(agentRun, pod).
		WithStatusSubresource(&v1alpha1.AgentRun{}).
		Build()

	reconciler := &AgentRunReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "agentrun-run-003",
			Namespace: "sentinelmesh",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	var updatedRun v1alpha1.AgentRun
	_ = fakeClient.Get(context.Background(), req.NamespacedName, &updatedRun)

	if updatedRun.Status.Phase != v1alpha1.AgentRunPhaseRunning {
		t.Errorf("expected phase Running, got '%s'", updatedRun.Status.Phase)
	}
	if updatedRun.Status.NodeName != "worker-node-2" {
		t.Errorf("expected nodeName 'worker-node-2', got '%s'", updatedRun.Status.NodeName)
	}
	if len(updatedRun.Status.Conditions) == 0 || updatedRun.Status.Conditions[0].Status != metav1.ConditionTrue {
		t.Errorf("expected Ready condition True on Running phase, got %v", updatedRun.Status.Conditions)
	}
}

func TestReconciler_PodFailed_UpdatesStatusToFailed(t *testing.T) {
	scheme := setupTestScheme()

	agentRun := &v1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-run-004",
			Namespace: "sentinelmesh",
		},
		Spec: v1alpha1.AgentRunSpec{
			RunID:   "run-004",
			AgentID: "agent-004",
			NodeID:  "worker-node-1",
			Image:   "sentinelmesh/test-agent:v1",
			Resources: v1alpha1.AgentRunResources{
				CPU:    "500m",
				Memory: "1Gi",
			},
		},
		Status: v1alpha1.AgentRunStatus{
			Phase: v1alpha1.AgentRunPhaseRunning,
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-run-004",
			Namespace: "sentinelmesh",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-node-1",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(agentRun, pod).
		WithStatusSubresource(&v1alpha1.AgentRun{}).
		Build()

	reconciler := &AgentRunReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "agentrun-run-004",
			Namespace: "sentinelmesh",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	var updatedRun v1alpha1.AgentRun
	_ = fakeClient.Get(context.Background(), req.NamespacedName, &updatedRun)

	if updatedRun.Status.Phase != v1alpha1.AgentRunPhaseFailed {
		t.Errorf("expected phase Failed, got '%s'", updatedRun.Status.Phase)
	}
	if len(updatedRun.Status.Conditions) == 0 || updatedRun.Status.Conditions[0].Status != metav1.ConditionFalse {
		t.Errorf("expected Ready condition False on Failed phase, got %v", updatedRun.Status.Conditions)
	}
}

func TestReconciler_InvalidAgentRun_HandlesGracefully(t *testing.T) {
	scheme := setupTestScheme()

	agentRun := &v1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-run-invalid",
			Namespace: "sentinelmesh",
		},
		Spec: v1alpha1.AgentRunSpec{
			RunID:   "run-invalid",
			AgentID: "agent-001",
			NodeID:  "worker-node-1",
			Image:   "sentinelmesh/test-agent:v1",
			Resources: v1alpha1.AgentRunResources{
				CPU:    "not-a-valid-cpu-qty",
				Memory: "2Gi",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(agentRun).
		WithStatusSubresource(&v1alpha1.AgentRun{}).
		Build()

	reconciler := &AgentRunReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "agentrun-run-invalid",
			Namespace: "sentinelmesh",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile should not return fatal error on bad resource quantity, got: %v", err)
	}

	var updatedRun v1alpha1.AgentRun
	_ = fakeClient.Get(context.Background(), req.NamespacedName, &updatedRun)

	if updatedRun.Status.Phase != v1alpha1.AgentRunPhaseFailed {
		t.Errorf("expected phase Failed for invalid resources, got '%s'", updatedRun.Status.Phase)
	}
}

func TestReconciler_PodSecurityHardening_ProfileMatrix(t *testing.T) {
	scheme := setupTestScheme()

	profiles := []struct {
		name         string
		secClass     string
		reqCPU       string
		reqMem       string
		wantLimitCPU string
		wantLimitMem string
	}{
		{
			name:         "Standard Profile",
			secClass:     "standard",
			reqCPU:       "500m",
			reqMem:       "512Mi",
			wantLimitCPU: "1",
			wantLimitMem: "1Gi",
		},
		{
			name:         "Restricted Profile",
			secClass:     "restricted",
			reqCPU:       "1",
			reqMem:       "1Gi",
			wantLimitCPU: "2",
			wantLimitMem: "2Gi",
		},
		{
			name:         "Confidential Profile",
			secClass:     "confidential",
			reqCPU:       "500m",
			reqMem:       "512Mi",
			wantLimitCPU: "500m",
			wantLimitMem: "512Mi",
		},
	}

	for _, tt := range profiles {
		t.Run(tt.name, func(t *testing.T) {
			runName := "agentrun-" + tt.secClass
			agentRun := &v1alpha1.AgentRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      runName,
					Namespace: "sentinelmesh",
				},
				Spec: v1alpha1.AgentRunSpec{
					RunID:         tt.secClass,
					AgentID:       "agent-sec",
					NodeID:        "worker-node-1",
					Image:         "sentinelmesh/agent:latest",
					SecurityClass: tt.secClass,
					Resources: v1alpha1.AgentRunResources{
						CPU:    tt.reqCPU,
						Memory: tt.reqMem,
					},
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(agentRun).
				WithStatusSubresource(&v1alpha1.AgentRun{}).
				Build()

			reconciler := &AgentRunReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      runName,
					Namespace: "sentinelmesh",
				},
			}

			_, err := reconciler.Reconcile(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected reconcile error: %v", err)
			}

			var pod corev1.Pod
			err = fakeClient.Get(context.Background(), types.NamespacedName{
				Name:      runName,
				Namespace: "sentinelmesh",
			}, &pod)
			if err != nil {
				t.Fatalf("failed to get created pod: %v", err)
			}

			// 1. Service Account token disabled
			if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken != false {
				t.Errorf("expected automountServiceAccountToken=false, got %v", pod.Spec.AutomountServiceAccountToken)
			}

			// 2. Pod Security Context: RunAsNonRoot, UID 10001, Seccomp RuntimeDefault
			if pod.Spec.SecurityContext == nil {
				t.Fatal("expected PodSecurityContext to be set")
			}
			if pod.Spec.SecurityContext.RunAsNonRoot == nil || !*pod.Spec.SecurityContext.RunAsNonRoot {
				t.Errorf("expected RunAsNonRoot=true")
			}
			if pod.Spec.SecurityContext.RunAsUser == nil || *pod.Spec.SecurityContext.RunAsUser == 0 {
				t.Errorf("expected non-root UID, got %v", pod.Spec.SecurityContext.RunAsUser)
			}
			if pod.Spec.SecurityContext.SeccompProfile == nil || pod.Spec.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
				t.Errorf("expected SeccompProfile RuntimeDefault, got %v", pod.Spec.SecurityContext.SeccompProfile)
			}

			// 3. Workspace volume
			hasWorkspace := false
			for _, v := range pod.Spec.Volumes {
				if v.Name == "workspace" && v.EmptyDir != nil {
					hasWorkspace = true
					break
				}
			}
			if !hasWorkspace {
				t.Errorf("expected /workspace emptyDir volume")
			}

			// 4. Container Security Context
			if len(pod.Spec.Containers) == 0 {
				t.Fatal("expected at least 1 container")
			}
			c := pod.Spec.Containers[0]
			if c.SecurityContext == nil {
				t.Fatal("expected container SecurityContext")
			}
			if c.SecurityContext.AllowPrivilegeEscalation == nil || *c.SecurityContext.AllowPrivilegeEscalation != false {
				t.Errorf("expected AllowPrivilegeEscalation=false")
			}
			if c.SecurityContext.ReadOnlyRootFilesystem == nil || !*c.SecurityContext.ReadOnlyRootFilesystem {
				t.Errorf("expected ReadOnlyRootFilesystem=true")
			}
			if c.SecurityContext.Capabilities == nil || len(c.SecurityContext.Capabilities.Drop) == 0 || string(c.SecurityContext.Capabilities.Drop[0]) != "ALL" {
				t.Errorf("expected Capabilities.Drop ALL, got %v", c.SecurityContext.Capabilities)
			}

			// 5. Container Limits and Requests
			reqCPU := c.Resources.Requests[corev1.ResourceCPU]
			reqMem := c.Resources.Requests[corev1.ResourceMemory]
			limitCPU := c.Resources.Limits[corev1.ResourceCPU]
			limitMem := c.Resources.Limits[corev1.ResourceMemory]

			if reqCPU.String() != tt.reqCPU {
				t.Errorf("expected req CPU %s, got %s", tt.reqCPU, reqCPU.String())
			}
			if reqMem.String() != tt.reqMem {
				t.Errorf("expected req Mem %s, got %s", tt.reqMem, reqMem.String())
			}
			if limitCPU.String() != tt.wantLimitCPU {
				t.Errorf("expected limit CPU %s, got %s", tt.wantLimitCPU, limitCPU.String())
			}
			if limitMem.String() != tt.wantLimitMem {
				t.Errorf("expected limit Mem %s, got %s", tt.wantLimitMem, limitMem.String())
			}
		})
	}
}

