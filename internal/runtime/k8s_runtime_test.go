package runtime

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestKubernetesRuntime_Lifecycle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	startTime := metav1.NewTime(time.Now().Add(-5 * time.Second))
	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-k8s-run-1",
			Namespace: "sentinelmesh",
		},
		Status: corev1.PodStatus{
			Phase:     corev1.PodRunning,
			StartTime: &startTime,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "agent",
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: startTime,
						},
					},
				},
			},
		},
	}

	finishedTime := metav1.NewTime(time.Now())
	completedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-k8s-run-completed",
			Namespace: "sentinelmesh",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "agent",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode:   0,
							FinishedAt: finishedTime,
							Reason:     "Completed",
						},
					},
				},
			},
		},
	}

	failedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-k8s-run-failed",
			Namespace: "sentinelmesh",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "agent",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode:   137,
							FinishedAt: finishedTime,
							Reason:     "OOMKilled",
							Message:    "Out of memory",
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(runningPod, completedPod, failedPod).
		Build()

	k8sRuntime := NewKubernetesRuntime(fakeClient, nil, "sentinelmesh")
	ctx := context.Background()

	// 1. Test Start on running pod
	handle, err := k8sRuntime.Start(ctx, ExecutionRequest{
		RunID: "k8s-run-1",
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if handle.PodName != "agentrun-k8s-run-1" {
		t.Errorf("expected pod name 'agentrun-k8s-run-1', got '%s'", handle.PodName)
	}

	// 2. Test Status on running pod
	status, err := k8sRuntime.Status(ctx, "k8s-run-1")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.State != types.StateRunning {
		t.Errorf("expected state RUNNING, got %s", status.State)
	}

	// 3. Test Status on completed pod
	compStatus, err := k8sRuntime.Status(ctx, "k8s-run-completed")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if compStatus.State != types.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", compStatus.State)
	}
	if compStatus.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", compStatus.ExitCode)
	}

	// 4. Test Status on failed/OOM pod
	failStatus, err := k8sRuntime.Status(ctx, "k8s-run-failed")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if failStatus.State != types.StateFailed {
		t.Errorf("expected state FAILED, got %s", failStatus.State)
	}
	if failStatus.ExitCode != 137 {
		t.Errorf("expected exit code 137, got %d", failStatus.ExitCode)
	}

	// 5. Test Stop / Delete
	err = k8sRuntime.Stop(ctx, "k8s-run-1", 1*time.Second)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	var deletedPod corev1.Pod
	err = fakeClient.Get(ctx, client.ObjectKey{Namespace: "sentinelmesh", Name: "agentrun-k8s-run-1"}, &deletedPod)
	if err == nil {
		t.Errorf("expected pod to be deleted")
	}
}
