package operator

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"go.opentelemetry.io/otel/attribute"

	v1alpha1 "github.com/sentinelmesh/sentinelmesh/internal/kubernetes/api/v1alpha1"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
)

// AgentRunReconciler reconciles AgentRun objects.
type AgentRunReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func podNameForRun(runName string) string {
	return runName
}

// Reconcile drives the cluster state toward the desired state described by the AgentRun CR.
func (r *AgentRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, span := observability.StartSpan(ctx, "operator.reconcile")
	defer span.End()

	logger := log.FromContext(ctx)

	// Step 1: Fetch the AgentRun
	var agentRun v1alpha1.AgentRun
	if err := r.Get(ctx, req.NamespacedName, &agentRun); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		observability.RecordSpanError(span, err)
		logger.Error(err, "unable to fetch AgentRun")
		return ctrl.Result{}, err
	}

	span.SetAttributes(
		attribute.String("k8s.cr_name", agentRun.Name),
		attribute.String("k8s.namespace", agentRun.Namespace),
		attribute.String("sentinel.run_id", agentRun.Spec.RunID),
		attribute.String("sentinel.agent_id", agentRun.Spec.AgentID),
		attribute.String("sentinel.cluster_id", agentRun.Spec.ClusterID),
		attribute.String("sentinel.node_id", agentRun.Spec.NodeID),
		attribute.String("sentinel.fencing_token", agentRun.Spec.FencingToken),
		attribute.Int("sentinel.execution_generation", agentRun.Spec.ExecutionGeneration),
	)

	podName := podNameForRun(agentRun.Name)

	// Fencing check: If this run has been marked Fenced or Quarantined, ensure Pod is terminated
	if agentRun.Status.Phase == v1alpha1.AgentRunPhaseFenced || agentRun.Status.Phase == v1alpha1.AgentRunPhaseQuarantined {
		var existingPod corev1.Pod
		if err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: agentRun.Namespace}, &existingPod); err == nil {
			logger.Info("Deleting fenced/quarantined stale Pod", "Pod", podName, "Phase", agentRun.Status.Phase)
			_ = r.Delete(ctx, &existingPod)
		}
		return ctrl.Result{}, nil
	}

	// Step 2: Look up the owned Pod
	var pod corev1.Pod
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: agentRun.Namespace}, &pod)

	if err != nil && errors.IsNotFound(err) {
		// Step 3: Pod is absent — create it
		newPod, buildErr := r.buildPodForAgentRun(&agentRun)
		if buildErr != nil {
			logger.Error(buildErr, "failed to build Pod for AgentRun",
				"AgentRun", agentRun.Name)
			return r.setStatus(ctx, &agentRun, v1alpha1.AgentRunPhaseFailed,
				fmt.Sprintf("failed to build Pod: %v", buildErr), nil)
		}

		logger.Info("Creating Pod for AgentRun",
			"Pod", newPod.Name,
			"Namespace", newPod.Namespace,
			"Cluster", agentRun.Spec.ClusterID,
			"Node", newPod.Spec.NodeName,
			"Generation", agentRun.Spec.ExecutionGeneration,
			"FencingToken", agentRun.Spec.FencingToken,
		)

		if createErr := r.Create(ctx, newPod); createErr != nil {
			logger.Error(createErr, "failed to create Pod",
				"Pod", newPod.Name)
			return ctrl.Result{}, createErr
		}

		// Pod created — set phase to Creating and requeue to pick up Pod status
		return r.setStatus(ctx, &agentRun, v1alpha1.AgentRunPhaseCreating,
			"Pod created, waiting for it to start", &podStatus{
				name:                podName,
				nodeName:            agentRun.Spec.NodeID,
				executionGeneration: agentRun.Spec.ExecutionGeneration,
				fencingToken:        agentRun.Spec.FencingToken,
			})
	}

	if err != nil {
		logger.Error(err, "failed to get Pod", "Pod", podName)
		return ctrl.Result{}, err
	}

	// Step 4: Pod exists — sync status from Pod phase
	phase := mapPodPhase(pod.Status.Phase)
	ps := &podStatus{
		name:                pod.Name,
		nodeName:            pod.Spec.NodeName,
		executionGeneration: agentRun.Spec.ExecutionGeneration,
		fencingToken:        agentRun.Spec.FencingToken,
		startTime:           pod.Status.StartTime,
	}
	return r.setStatus(ctx, &agentRun, phase, "", ps)
}

// QuarantineStaleRun forces an AgentRun into the Fenced state, deleting any associated Pod.
func (r *AgentRunReconciler) QuarantineStaleRun(ctx context.Context, namespacedName types.NamespacedName, reason string) error {
	var agentRun v1alpha1.AgentRun
	if err := r.Get(ctx, namespacedName, &agentRun); err != nil {
		return err
	}

	agentRun.Status.Phase = v1alpha1.AgentRunPhaseFenced
	agentRun.Status.Message = fmt.Sprintf("Fenced execution: %s", reason)
	if err := r.Status().Update(ctx, &agentRun); err != nil {
		return err
	}

	podName := podNameForRun(agentRun.Name)
	var pod corev1.Pod
	if err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: agentRun.Namespace}, &pod); err == nil {
		_ = r.Delete(ctx, &pod)
	}

	return nil
}

// podStatus carries observed Pod details used to populate AgentRun.status.
type podStatus struct {
	name                string
	nodeName            string
	executionGeneration int
	fencingToken        string
	startTime           *metav1.Time
}

func (r *AgentRunReconciler) setStatus(
	ctx context.Context,
	agentRun *v1alpha1.AgentRun,
	phase v1alpha1.AgentRunPhase,
	message string,
	ps *podStatus,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	updated := agentRun.DeepCopy()
	updated.Status.Phase = phase
	updated.Status.Message = message

	if ps != nil {
		if ps.name != "" {
			updated.Status.PodName = ps.name
		}
		if ps.nodeName != "" {
			updated.Status.NodeName = ps.nodeName
		}
		if ps.executionGeneration > 0 {
			updated.Status.ExecutionGeneration = ps.executionGeneration
		}
		if ps.fencingToken != "" {
			updated.Status.FencingToken = ps.fencingToken
		}
		if ps.startTime != nil && updated.Status.StartTime == nil {
			updated.Status.StartTime = ps.startTime
		}
	}

	now := metav1.Now()
	readyStatus := metav1.ConditionFalse
	reason := string(phase)
	if phase == v1alpha1.AgentRunPhaseRunning {
		readyStatus = metav1.ConditionTrue
		reason = "PodRunning"
	} else if phase == v1alpha1.AgentRunPhaseSucceeded {
		reason = "PodSucceeded"
	} else if phase == v1alpha1.AgentRunPhaseFailed {
		reason = "PodFailed"
	} else if phase == v1alpha1.AgentRunPhaseFenced {
		reason = "ExecutionFenced"
	} else if phase == v1alpha1.AgentRunPhaseCreating {
		reason = "PodCreating"
	} else if phase == v1alpha1.AgentRunPhasePending {
		reason = "PodPending"
	}

	condMsg := message
	if condMsg == "" {
		condMsg = fmt.Sprintf("AgentRun is in %s phase", phase)
	}

	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             readyStatus,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            condMsg,
	}
	updated.Status.Conditions = []metav1.Condition{readyCondition}

	if agentRun.Status.Phase == updated.Status.Phase &&
		agentRun.Status.PodName == updated.Status.PodName &&
		agentRun.Status.NodeName == updated.Status.NodeName &&
		agentRun.Status.ExecutionGeneration == updated.Status.ExecutionGeneration &&
		agentRun.Status.FencingToken == updated.Status.FencingToken &&
		agentRun.Status.Message == updated.Status.Message &&
		len(agentRun.Status.Conditions) > 0 &&
		agentRun.Status.Conditions[0].Status == readyStatus {
		return ctrl.Result{}, nil
	}

	if err := r.Status().Update(ctx, updated); err != nil {
		logger.Error(err, "failed to update AgentRun status",
			"AgentRun", agentRun.Name, "phase", phase)
		return ctrl.Result{}, err
	}

	logger.Info("AgentRun status updated",
		"AgentRun", agentRun.Name,
		"phase", phase,
		"podName", updated.Status.PodName,
		"nodeName", updated.Status.NodeName,
		"generation", updated.Status.ExecutionGeneration,
	)

	return ctrl.Result{}, nil
}

func mapPodPhase(podPhase corev1.PodPhase) v1alpha1.AgentRunPhase {
	switch podPhase {
	case corev1.PodPending:
		return v1alpha1.AgentRunPhasePending
	case corev1.PodRunning:
		return v1alpha1.AgentRunPhaseRunning
	case corev1.PodSucceeded:
		return v1alpha1.AgentRunPhaseSucceeded
	case corev1.PodFailed:
		return v1alpha1.AgentRunPhaseFailed
	default:
		return v1alpha1.AgentRunPhaseUnknown
	}
}

func (r *AgentRunReconciler) buildPodForAgentRun(run *v1alpha1.AgentRun) (*corev1.Pod, error) {
	image := run.Spec.Image
	if image == "" {
		return nil, fmt.Errorf("AgentRun %s has no image specified", run.Name)
	}

	reqCPU, err := resource.ParseQuantity(run.Spec.Resources.CPU)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU quantity %q: %w", run.Spec.Resources.CPU, err)
	}
	reqMem, err := resource.ParseQuantity(run.Spec.Resources.Memory)
	if err != nil {
		return nil, fmt.Errorf("invalid memory quantity %q: %w", run.Spec.Resources.Memory, err)
	}

	limitCPU := reqCPU.DeepCopy()
	limitMem := reqMem.DeepCopy()

	switch run.Spec.SecurityClass {
	case "confidential":
		limitCPU = reqCPU.DeepCopy()
		limitMem = reqMem.DeepCopy()
	case "restricted":
		limitCPU = reqCPU.DeepCopy()
		limitCPU.Add(reqCPU)
		limitMem = reqMem.DeepCopy()
		limitMem.Add(reqMem)
	default:
		limitCPU = reqCPU.DeepCopy()
		limitCPU.Add(reqCPU)
		limitMem = reqMem.DeepCopy()
		limitMem.Add(reqMem)
	}

	automountToken := false
	runAsNonRoot := true
	allowPrivEsc := false
	readOnlyRootFS := true
	uid := int64(10001)
	gid := int64(10001)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podNameForRun(run.Name),
			Namespace: run.Namespace,
			Labels: map[string]string{
				"app":                        "sentinelmesh-agent",
				"sentinelmesh.io/run-id":     run.Spec.RunID,
				"sentinelmesh.io/agent-id":   run.Spec.AgentID,
				"sentinelmesh.io/cluster-id": run.Spec.ClusterID,
				"sentinelmesh.io/profile":    run.Spec.SecurityClass,
			},
		},
		Spec: corev1.PodSpec{
			NodeName:                      run.Spec.NodeID,
			AutomountServiceAccountToken: &automountToken,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
				RunAsUser:    &uid,
				RunAsGroup:   &gid,
				FSGroup:      &gid,
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "workspace",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "tmp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "agent",
					Image:   image,
					Command: []string{"sleep", "3600"},
					Env: func() []corev1.EnvVar {
						envs := []corev1.EnvVar{
							{
								Name:  "SENTINEL_RUN_ID",
								Value: run.Spec.RunID,
							},
							{
								Name:  "SENTINEL_CLUSTER_ID",
								Value: run.Spec.ClusterID,
							},
							{
								Name:  "SENTINEL_EXECUTION_GENERATION",
								Value: fmt.Sprintf("%d", run.Spec.ExecutionGeneration),
							},
							{
								Name:  "SENTINEL_FENCING_TOKEN",
								Value: run.Spec.FencingToken,
							},
						}
						if run.Spec.RestoreCheckpointID != "" {
							envs = append(envs,
								corev1.EnvVar{
									Name:  "SENTINEL_RESTORE_CHECKPOINT_ID",
									Value: run.Spec.RestoreCheckpointID,
								},
								corev1.EnvVar{
									Name:  "SENTINEL_RESTORE_STEP",
									Value: fmt.Sprintf("%d", run.Spec.RestoreStep),
								},
							)
						}
						return envs
					}(),
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: &allowPrivEsc,
						ReadOnlyRootFilesystem:   &readOnlyRootFS,
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "workspace",
							MountPath: "/workspace",
						},
						{
							Name:      "tmp",
							MountPath: "/tmp",
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    reqCPU,
							corev1.ResourceMemory: reqMem,
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    limitCPU,
							corev1.ResourceMemory: limitMem,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	if err := ctrl.SetControllerReference(run, pod, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set OwnerReference: %w", err)
	}

	return pod, nil
}

func (r *AgentRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.AgentRun{}).
		Owns(&corev1.Pod{}, builder.WithPredicates(PodPhaseChangedPredicate{})).
		Complete(r)
}
