package operator

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PodPhaseChangedPredicate filters Pod update events so the reconciler is only
// triggered when the Pod phase actually changes. This reduces noisy reconcile
// loops caused by metadata-only updates (e.g. annotation changes, heartbeats).
//
// Without this predicate, every kubelet heartbeat touching the Pod would trigger
// a full reconcile even though the AgentRun status wouldn't change.
type PodPhaseChangedPredicate struct {
	predicate.Funcs
}

// Update returns true only when the Pod phase has changed between old and new.
func (PodPhaseChangedPredicate) Update(e event.UpdateEvent) bool {
	oldPod, ok := e.ObjectOld.(*corev1.Pod)
	if !ok {
		return true // not a Pod — let it through
	}
	newPod, ok := e.ObjectNew.(*corev1.Pod)
	if !ok {
		return true
	}
	return oldPod.Status.Phase != newPod.Status.Phase
}
