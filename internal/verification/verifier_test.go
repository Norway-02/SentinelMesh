package verification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func setupK8sScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	return s
}

func TestArtifactEvaluator(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	validFile := filepath.Join(tmpDir, "output.json")
	validContent := []byte(`{"status":"success","records_processed":500,"model_version":"v2.1"}`)
	_ = os.WriteFile(validFile, validContent, 0644)

	hash := sha256.Sum256(validContent)
	validChecksum := hex.EncodeToString(hash[:])

	// Case 1: Valid artifact with checksum and schema
	rule1 := types.ArtifactRule{
		ID:               "art-01",
		Path:             validFile,
		Required:         true,
		MinSizeBytes:     10,
		ExpectedChecksum: validChecksum,
		SchemaJSON:       `["status", "records_processed"]`,
	}
	eval1 := EvaluateArtifactRule(ctx, rule1)
	if eval1.Status != RulePass {
		t.Errorf("expected RulePass for valid artifact, got %s: %s", eval1.Status, eval1.Reason)
	}

	// Case 2: Missing artifact
	rule2 := types.ArtifactRule{
		ID:       "art-02",
		Path:     filepath.Join(tmpDir, "missing.parquet"),
		Required: true,
	}
	eval2 := EvaluateArtifactRule(ctx, rule2)
	if eval2.Status != RuleFail {
		t.Errorf("expected RuleFail for missing artifact, got %s", eval2.Status)
	}

	// Case 3: Checksum mismatch
	rule3 := types.ArtifactRule{
		ID:               "art-03",
		Path:             validFile,
		ExpectedChecksum: "0000000000000000000000000000000000000000000000000000000000000000",
	}
	eval3 := EvaluateArtifactRule(ctx, rule3)
	if eval3.Status != RuleFail {
		t.Errorf("expected RuleFail for checksum mismatch, got %s", eval3.Status)
	}

	// Case 4: Missing required schema key
	rule4 := types.ArtifactRule{
		ID:         "art-04",
		Path:       validFile,
		SchemaJSON: `["status", "non_existent_key"]`,
	}
	eval4 := EvaluateArtifactRule(ctx, rule4)
	if eval4.Status != RuleFail {
		t.Errorf("expected RuleFail for missing schema key, got %s", eval4.Status)
	}
}

func TestHTTPEvaluator(t *testing.T) {
	ctx := context.Background()

	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"health":"OK","replicas":3}`))
	}))
	defer healthyServer.Close()

	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"database unavailable"}`))
	}))
	defer errorServer.Close()

	eval1 := EvaluateHTTPHealthRule(ctx, healthyServer.Client(), types.HTTPHealthRule{
		ID:                    "http-01",
		URL:                   healthyServer.URL,
		ExpectedStatus:        200,
		ExpectedBodySubstring: `"health":"OK"`,
	})
	if eval1.Status != RulePass {
		t.Errorf("expected RulePass for healthy server, got %s: %s", eval1.Status, eval1.Reason)
	}

	eval2 := EvaluateHTTPHealthRule(ctx, errorServer.Client(), types.HTTPHealthRule{
		ID:             "http-02",
		URL:            errorServer.URL,
		ExpectedStatus: 200,
	})
	if eval2.Status != RuleFail {
		t.Errorf("expected RuleFail for 500 status, got %s", eval2.Status)
	}
}

func TestInvariantEvaluator(t *testing.T) {
	metrics := map[string]string{
		"processed_records": "15000",
		"error_rate":        "0.002",
		"environment":       "production",
	}

	eval1 := EvaluateInvariantRule(metrics, types.InvariantRule{
		ID:            "inv-01",
		MetricName:    "processed_records",
		Operator:      "gte",
		ExpectedValue: "10000",
	})
	if eval1.Status != RulePass {
		t.Errorf("expected RulePass for processed_records gte 10000, got %s: %s", eval1.Status, eval1.Reason)
	}

	eval2 := EvaluateInvariantRule(metrics, types.InvariantRule{
		ID:            "inv-02",
		MetricName:    "environment",
		Operator:      "eq",
		ExpectedValue: "production",
	})
	if eval2.Status != RulePass {
		t.Errorf("expected RulePass for environment == production, got %s: %s", eval2.Status, eval2.Reason)
	}

	eval3 := EvaluateInvariantRule(metrics, types.InvariantRule{
		ID:            "inv-03",
		MetricName:    "error_rate",
		Operator:      "lt",
		ExpectedValue: "0.001",
	})
	if eval3.Status != RuleFail {
		t.Errorf("expected RuleFail for error_rate < 0.001, got %s", eval3.Status)
	}
}

func TestKubernetesStateEvaluator(t *testing.T) {
	ctx := context.Background()

	podRunning := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "crawler-abc1", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Name: "app", RestartCount: 0}},
		},
	}

	podCrashLoop := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "crashed-xyz1", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "app",
					RestartCount: 5,
					State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(setupK8sScheme()).
		WithObjects(podRunning, podCrashLoop).
		Build()

	eval1 := EvaluateKubernetesStateRule(ctx, fakeClient, types.KubernetesStateRule{
		ID:            "k8s-01",
		Namespace:     "default",
		PodNamePrefix: "crawler",
		ExpectedPhase: "Running",
		MaxRestarts:   1,
	})
	if eval1.Status != RulePass {
		t.Errorf("expected RulePass for healthy pod, got %s: %s", eval1.Status, eval1.Reason)
	}

	eval2 := EvaluateKubernetesStateRule(ctx, fakeClient, types.KubernetesStateRule{
		ID:            "k8s-02",
		Namespace:     "default",
		PodNamePrefix: "crashed",
		ExpectedPhase: "Running",
		MaxRestarts:   1,
	})
	if eval2.Status != RuleFail {
		t.Errorf("expected RuleFail for CrashLoopBackOff pod, got %s", eval2.Status)
	}
}
