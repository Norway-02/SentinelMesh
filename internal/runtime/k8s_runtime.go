package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

const DefaultNamespace = "sentinelmesh"

// KubernetesRuntime manages agent execution inside Kubernetes Pods.
type KubernetesRuntime struct {
	client    client.Client
	clientset kubernetes.Interface
	namespace string
}

// NewKubernetesRuntime constructs a KubernetesRuntime instance.
func NewKubernetesRuntime(c client.Client, cs kubernetes.Interface, namespace string) *KubernetesRuntime {
	if namespace == "" {
		namespace = DefaultNamespace
	}
	return &KubernetesRuntime{
		client:    c,
		clientset: cs,
		namespace: namespace,
	}
}

// podNameForRun returns the canonical Pod name for a run.
func podNameForRun(runID string) string {
	return "agentrun-" + runID
}

// Start checks the Pod readiness and returns an ExecutionHandle.
func (k *KubernetesRuntime) Start(ctx context.Context, req ExecutionRequest) (*ExecutionHandle, error) {
	podName := podNameForRun(req.RunID)
	var pod corev1.Pod
	err := k.client.Get(ctx, client.ObjectKey{Namespace: k.namespace, Name: podName}, &pod)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("%w: pod %s not yet created by operator", ErrRunNotFound, podName)
		}
		return nil, fmt.Errorf("failed to fetch pod %s: %w", podName, err)
	}

	now := time.Now()
	if pod.Status.StartTime != nil {
		now = pod.Status.StartTime.Time
	}

	return &ExecutionHandle{
		RunID:     req.RunID,
		PodName:   podName,
		StartTime: now,
	}, nil
}

// Status reads Pod lifecycle state and extracts detailed container exit codes.
func (k *KubernetesRuntime) Status(ctx context.Context, runID string) (*ExecutionStatus, error) {
	podName := podNameForRun(runID)
	var pod corev1.Pod
	err := k.client.Get(ctx, client.ObjectKey{Namespace: k.namespace, Name: podName}, &pod)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, ErrRunNotFound
		}
		return nil, fmt.Errorf("failed to fetch pod %s: %w", podName, err)
	}

	st := &ExecutionStatus{
		RunID: runID,
		State: types.StateStarting,
	}

	if pod.Status.StartTime != nil {
		st.StartedAt = &pod.Status.StartTime.Time
	}

	// Inspect container status
	var agentContainerStatus *corev1.ContainerStatus
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name == "agent" {
			agentContainerStatus = &pod.Status.ContainerStatuses[i]
			break
		}
	}

	if agentContainerStatus != nil {
		if agentContainerStatus.State.Running != nil {
			st.State = types.StateRunning
			startTime := agentContainerStatus.State.Running.StartedAt.Time
			st.StartedAt = &startTime
		} else if agentContainerStatus.State.Terminated != nil {
			term := agentContainerStatus.State.Terminated
			st.ExitCode = int(term.ExitCode)
			if term.FinishedAt.Time.IsZero() {
				now := time.Now()
				st.FinishedAt = &now
			} else {
				st.FinishedAt = &term.FinishedAt.Time
			}

			if term.ExitCode == 0 {
				st.State = types.StateCompleted
			} else {
				st.State = types.StateFailed
				st.ErrorReason = fmt.Sprintf("container terminated with code %d: %s (%s)",
					term.ExitCode, term.Reason, term.Message)
			}
		} else if agentContainerStatus.State.Waiting != nil {
			st.State = types.StateStarting
			st.ErrorReason = agentContainerStatus.State.Waiting.Reason
		}
	} else {
		// Fall back to pod phase
		switch pod.Status.Phase {
		case corev1.PodPending:
			st.State = types.StateStarting
		case corev1.PodRunning:
			st.State = types.StateRunning
		case corev1.PodSucceeded:
			st.State = types.StateCompleted
			st.ExitCode = 0
		case corev1.PodFailed:
			st.State = types.StateFailed
			st.ExitCode = 1
			st.ErrorReason = pod.Status.Message
		}
	}

	return st, nil
}

// Stop deletes or stops the target Pod.
func (k *KubernetesRuntime) Stop(ctx context.Context, runID string, gracePeriod time.Duration) error {
	podName := podNameForRun(runID)
	var gracePeriodSeconds *int64
	if gracePeriod > 0 {
		secs := int64(gracePeriod.Seconds())
		gracePeriodSeconds = &secs
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: k.namespace,
		},
	}

	err := k.client.Delete(ctx, pod, &client.DeleteOptions{
		GracePeriodSeconds: gracePeriodSeconds,
	})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete pod %s: %w", podName, err)
	}

	return nil
}

// Pause is not natively supported on standard K8s Pods without cgroups freezing.
func (k *KubernetesRuntime) Pause(ctx context.Context, runID string) error {
	return fmt.Errorf("pause operation requires container runtime pause plugins")
}

// Resume is not natively supported on standard K8s Pods.
func (k *KubernetesRuntime) Resume(ctx context.Context, runID string) error {
	return fmt.Errorf("resume operation requires container runtime resume plugins")
}

// Logs streams logs from the agent container via Kubernetes API.
func (k *KubernetesRuntime) Logs(ctx context.Context, runID string, opts LogOptions) (io.ReadCloser, error) {
	if k.clientset == nil {
		return io.NopCloser(bytes.NewBufferString("Kubernetes clientset not configured for live streaming")), nil
	}

	podName := podNameForRun(runID)
	reqOpts := &corev1.PodLogOptions{
		Container:  "agent",
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
	}
	if opts.TailLines > 0 {
		lines := int64(opts.TailLines)
		reqOpts.TailLines = &lines
	}
	if !opts.Since.IsZero() {
		since := metav1.NewTime(opts.Since)
		reqOpts.SinceTime = &since
	}

	req := k.clientset.CoreV1().Pods(k.namespace).GetLogs(podName, reqOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to stream pod logs for %s: %w", podName, err)
	}

	return stream, nil
}

// Delete cleans up the Pod.
func (k *KubernetesRuntime) Delete(ctx context.Context, runID string) error {
	return k.Stop(ctx, runID, 0)
}
