package verification

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// EvaluateKubernetesStateRule inspects live Pod and container state in Kubernetes.
func EvaluateKubernetesStateRule(ctx context.Context, k8sClient client.Client, rule types.KubernetesStateRule) RuleEvaluation {
	start := time.Now()
	eval := RuleEvaluation{
		RuleID:        rule.ID,
		RuleType:      "kubernetes_state",
		ExpectedValue: fmt.Sprintf("phase=%s, max_restarts=%d", rule.ExpectedPhase, rule.MaxRestarts),
	}

	if k8sClient == nil {
		eval.Status = RuleSkipped
		eval.Reason = "kubernetes client not configured"
		eval.DurationNs = time.Since(start).Nanoseconds()
		return eval
	}

	var podList corev1.PodList
	opts := []client.ListOption{client.InNamespace(rule.Namespace)}
	if err := k8sClient.List(ctx, &podList, opts...); err != nil {
		eval.Status = RuleError
		eval.Reason = fmt.Sprintf("failed to list pods in namespace %s: %v", rule.Namespace, err)
		eval.DurationNs = time.Since(start).Nanoseconds()
		return eval
	}

	var matchedPods []corev1.Pod
	for _, p := range podList.Items {
		if strings.HasPrefix(p.Name, rule.PodNamePrefix) {
			matchedPods = append(matchedPods, p)
		}
	}

	if len(matchedPods) == 0 {
		eval.Status = RuleFail
		eval.Reason = fmt.Sprintf("no pods found with prefix '%s' in namespace '%s'", rule.PodNamePrefix, rule.Namespace)
		eval.DurationNs = time.Since(start).Nanoseconds()
		return eval
	}

	for _, pod := range matchedPods {
		eval.EvaluatedValue = fmt.Sprintf("pod=%s, phase=%s", pod.Name, pod.Status.Phase)
		if rule.ExpectedPhase != "" && string(pod.Status.Phase) != rule.ExpectedPhase {
			eval.Status = RuleFail
			eval.Reason = fmt.Sprintf("pod %s in phase %s, expected %s", pod.Name, pod.Status.Phase, rule.ExpectedPhase)
			eval.DurationNs = time.Since(start).Nanoseconds()
			return eval
		}

		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				eval.Status = RuleFail
				eval.Reason = fmt.Sprintf("pod %s container %s in CrashLoopBackOff", pod.Name, cs.Name)
				eval.DurationNs = time.Since(start).Nanoseconds()
				return eval
			}
			if rule.MaxRestarts >= 0 && cs.RestartCount > rule.MaxRestarts {
				eval.Status = RuleFail
				eval.Reason = fmt.Sprintf("pod %s container %s restart count %d exceeded max %d", pod.Name, cs.Name, cs.RestartCount, rule.MaxRestarts)
				eval.DurationNs = time.Since(start).Nanoseconds()
				return eval
			}
		}
	}

	eval.Status = RulePass
	eval.Reason = fmt.Sprintf("matched %d pods in namespace %s with expected healthy status", len(matchedPods), rule.Namespace)
	eval.DurationNs = time.Since(start).Nanoseconds()
	return eval
}
