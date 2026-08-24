package operator

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestPodPhaseChangedPredicate(t *testing.T) {
	pred := PodPhaseChangedPredicate{}

	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	samePhasePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
			Annotations: map[string]string{
				"heartbeat": "123",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	newPhasePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	// Case 1: Same phase (e.g. heartbeat) -> should NOT trigger reconcile
	updateSame := event.UpdateEvent{
		ObjectOld: oldPod,
		ObjectNew: samePhasePod,
	}
	if pred.Update(updateSame) {
		t.Errorf("expected predicate to return false for same Pod phase")
	}

	// Case 2: Changed phase -> should trigger reconcile
	updateNew := event.UpdateEvent{
		ObjectOld: oldPod,
		ObjectNew: newPhasePod,
	}
	if !pred.Update(updateNew) {
		t.Errorf("expected predicate to return true for changed Pod phase")
	}

	// Case 3: Non-pod object -> allow through safely
	updateNonPod := event.UpdateEvent{
		ObjectOld: &corev1.Node{},
		ObjectNew: &corev1.Node{},
	}
	if !pred.Update(updateNonPod) {
		t.Errorf("expected predicate to return true for non-Pod objects")
	}
}
