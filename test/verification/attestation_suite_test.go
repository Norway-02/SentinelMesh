package verification_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
	"github.com/sentinelmesh/sentinelmesh/internal/verification"
)

func setupK8sScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	return s
}

// Scenario 1: Legitimate Success across Artifact, K8s, HTTP, Invariant, and Command verification.
func TestAttestationSuite_LegitimateMultiDimensionalSuccess(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// 1. Prepare Artifact
	artPath := filepath.Join(tmpDir, "model_metrics.json")
	artData := []byte(`{"accuracy":0.97,"f1_score":0.96,"dataset_version":"2026.08"}`)
	_ = os.WriteFile(artPath, artData, 0644)
	artHash := sha256.Sum256(artData)
	artChecksum := hex.EncodeToString(artHash[:])

	// 2. Prepare Mock HTTP Service
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy","model_serving":true}`))
	}))
	defer server.Close()

	// 3. Prepare Mock K8s Infrastructure
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "serving-model-prod-abc",
			Namespace: "production",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "serving-container", RestartCount: 0},
			},
		},
	}
	fakeK8s := fake.NewClientBuilder().WithScheme(setupK8sScheme()).WithObjects(pod).Build()

	// 4. Initialize Verifier Subsystem
	attRepo := verification.NewMemoryRepository()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	verifier := verification.NewService(attRepo, agentRepo, runRepo, outboxRepo, txManager, fakeK8s, server.Client())

	agent := domain.Agent{
		ID:       "agent-model-deployer",
		TenantID: "tenant-ml",
		Name:     "model-deployer",
		Version:  "1.0.0",
		VerificationPolicy: types.VerificationPolicy{
			Enabled: true,
			ArtifactRules: []types.ArtifactRule{
				{ID: "rule-art-checksum", Path: artPath, Required: true, ExpectedChecksum: artChecksum, SchemaJSON: `["accuracy", "f1_score"]`},
			},
			KubernetesRules: []types.KubernetesStateRule{
				{ID: "rule-k8s-running", Namespace: "production", PodNamePrefix: "serving-model-prod", ExpectedPhase: "Running", MaxRestarts: 0},
			},
			HTTPRules: []types.HTTPHealthRule{
				{ID: "rule-http-probe", URL: server.URL, ExpectedStatus: 200, ExpectedBodySubstring: `"status":"healthy"`},
			},
			InvariantRules: []types.InvariantRule{
				{ID: "rule-inv-acc", MetricName: "accuracy", Operator: "gte", ExpectedValue: "0.95"},
			},
			CommandRules: []types.CommandRule{
				{ID: "rule-cmd-echo", Command: "echo", Args: []string{"attestation-ok"}, ExpectedExitCode: 0},
			},
		},
	}
	_ = agentRepo.Create(ctx, agent)

	now := time.Now()
	run := domain.AgentRun{
		ID:        "run-deploy-100",
		AgentID:   "agent-model-deployer",
		TenantID:  "tenant-ml",
		State:     types.StateRunning,
		StartedAt: &now,
		Version:   1,
	}
	_ = runRepo.Create(ctx, run)

	// Execute Verification
	att, err := verifier.VerifyRun(ctx, verification.VerifyRunRequest{
		RunID: "run-deploy-100",
		ReportedMetrics: map[string]string{
			"accuracy": "0.97",
		},
	})
	if err != nil {
		t.Fatalf("Expected verification to succeed, got error: %v", err)
	}

	if att.Status != verification.StatusVerified {
		t.Errorf("Expected StatusVerified, got %s", att.Status)
	}
	if len(att.Evaluations) != 5 {
		t.Errorf("Expected 5 rule evaluations, got %d", len(att.Evaluations))
	}
	if !att.VerifyDigest() {
		t.Errorf("Expected cryptographic evidence digest to be valid")
	}

	updatedRun, _ := runRepo.Get(ctx, "run-deploy-100")
	if updatedRun.State != types.StateCompleted {
		t.Errorf("Expected run state COMPLETED, got %s", updatedRun.State)
	}
	if updatedRun.VerificationState != "VERIFIED" {
		t.Errorf("Expected verification_state VERIFIED, got %s", updatedRun.VerificationState)
	}

	eventsList := outboxRepo.GetEvents()
	if len(eventsList) != 1 || eventsList[0].EventType != events.SubjectRunVerified {
		t.Errorf("Expected 1 SubjectRunVerified outbox event, got %v", eventsList)
	}
}

// Scenario 2: Anti-Hallucination — Agent claims dataset generated, but artifact was never created.
func TestAttestationSuite_HallucinatedSuccessMissingArtifact(t *testing.T) {
	ctx := context.Background()
	attRepo := verification.NewMemoryRepository()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	verifier := verification.NewService(attRepo, agentRepo, runRepo, outboxRepo, txManager, nil, nil)

	agent := domain.Agent{
		ID:       "agent-hallucinating-01",
		TenantID: "tenant-data",
		Name:     "data-extractor",
		Version:  "1.0.0",
		VerificationPolicy: types.VerificationPolicy{
			Enabled: true,
			ArtifactRules: []types.ArtifactRule{
				{ID: "art-mandatory-dataset", Path: "/workspace/non_existent_dataset.parquet", Required: true},
			},
		},
	}
	_ = agentRepo.Create(ctx, agent)

	now := time.Now()
	run := domain.AgentRun{
		ID:        "run-hallucinate-01",
		AgentID:   "agent-hallucinating-01",
		TenantID:  "tenant-data",
		State:     types.StateRunning,
		StartedAt: &now,
		Version:   1,
	}
	_ = runRepo.Create(ctx, run)

	// Execute Verification (Agent claimed completion)
	att, err := verifier.VerifyRun(ctx, verification.VerifyRunRequest{RunID: "run-hallucinate-01"})
	if err == nil {
		t.Errorf("Expected verification to fail on missing artifact, got nil")
	}

	if att.Status != verification.StatusFailed {
		t.Errorf("Expected StatusFailed, got %s", att.Status)
	}

	updatedRun, _ := runRepo.Get(ctx, "run-hallucinate-01")
	if updatedRun.State != types.StateFailed {
		t.Errorf("Expected run state FAILED, got %s", updatedRun.State)
	}
	if updatedRun.VerificationState != "FAILED" {
		t.Errorf("Expected verification_state FAILED, got %s", updatedRun.VerificationState)
	}

	eventsList := outboxRepo.GetEvents()
	if len(eventsList) != 1 || eventsList[0].EventType != events.SubjectRunVerificationFailed {
		t.Errorf("Expected SubjectRunVerificationFailed outbox event, got %v", eventsList)
	}
}

// Scenario 3: Silent Infrastructure Failure — Kubernetes Pod in CrashLoopBackOff.
func TestAttestationSuite_SilentKubernetesFailureDetection(t *testing.T) {
	ctx := context.Background()

	podCrash := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-app-crashed-1",
			Namespace: "staging",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "app",
					RestartCount: 7,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	}
	fakeK8s := fake.NewClientBuilder().WithScheme(setupK8sScheme()).WithObjects(podCrash).Build()

	attRepo := verification.NewMemoryRepository()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	verifier := verification.NewService(attRepo, agentRepo, runRepo, outboxRepo, txManager, fakeK8s, nil)

	agent := domain.Agent{
		ID:       "agent-k8s-deployer",
		TenantID: "tenant-infra",
		Name:     "app-deployer",
		Version:  "1.0.0",
		VerificationPolicy: types.VerificationPolicy{
			Enabled: true,
			KubernetesRules: []types.KubernetesStateRule{
				{ID: "rule-k8s-healthy", Namespace: "staging", PodNamePrefix: "web-app", ExpectedPhase: "Running", MaxRestarts: 2},
			},
		},
	}
	_ = agentRepo.Create(ctx, agent)

	now := time.Now()
	run := domain.AgentRun{
		ID:        "run-k8s-fail-01",
		AgentID:   "agent-k8s-deployer",
		TenantID:  "tenant-infra",
		State:     types.StateRunning,
		StartedAt: &now,
		Version:   1,
	}
	_ = runRepo.Create(ctx, run)

	att, err := verifier.VerifyRun(ctx, verification.VerifyRunRequest{RunID: "run-k8s-fail-01"})
	if err == nil {
		t.Errorf("Expected verification to fail on CrashLoopBackOff pod")
	}

	if att.Status != verification.StatusFailed {
		t.Errorf("Expected StatusFailed, got %s", att.Status)
	}

	updatedRun, _ := runRepo.Get(ctx, "run-k8s-fail-01")
	if updatedRun.State != types.StateFailed {
		t.Errorf("Expected run state FAILED, got %s", updatedRun.State)
	}
}
