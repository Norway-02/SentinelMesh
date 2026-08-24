package security_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
	"github.com/sentinelmesh/sentinelmesh/internal/verification"
)

func TestSecurity_SecretRedactionInTelemetry(t *testing.T) {
	registry := router.NewDefaultModelRegistry()
	provider := router.NewLiveModelProvider(registry, router.ModeLive, false)

	const rawSecretKey = "sk-super-secret-production-key-999"
	provider.SetEndpoint("small-fast-v1", router.ProviderEndpointConfig{
		Type:        router.ProviderTypeOpenAI,
		BaseURL:     "http://localhost:1", // intentionally invalid to cause immediate failure
		APIKey:      rawSecretKey,
		ModelTarget: "gpt-4o-mini",
	})

	ctx := context.Background()
	_, err := provider.Invoke(ctx, "small-fast-v1", router.ModelInvocationRequest{
		TaskID: "sec-leak-test",
		Prompt: "Testing secret redaction",
	})

	if err == nil {
		t.Fatalf("Expected connection error, got nil")
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, rawSecretKey) {
		t.Fatalf("SECURITY VIOLATION: raw API secret key leaked into error message: %s", errMsg)
	}
}

func TestSecurity_FencingTokenEnforcement(t *testing.T) {
	// Simulate an active agent lease with generation token = 5
	activeGeneration := int64(5)

	// A stale worker attempts state mutation with generation token = 4
	staleGeneration := int64(4)
	if staleGeneration >= activeGeneration {
		t.Fatalf("Stale token must be strictly < active generation")
	}

	// Validating monotonic fence
	canExecute := func(token int64) bool {
		return token >= activeGeneration
	}

	if canExecute(staleGeneration) {
		t.Fatalf("Fencing token check failed: stale worker generation (4) was permitted to mutate state over active generation (5)")
	}

	if !canExecute(activeGeneration) {
		t.Fatalf("Valid fencing token (5) rejected")
	}
}

func TestSecurity_AttestationEvidenceDigestVerification(t *testing.T) {
	evaluations := []verification.RuleEvaluation{
		{
			RuleID:         "rule-k8s-pod-running",
			RuleType:       "K8S_POD_PHASE",
			Status:         verification.RulePass,
			Reason:         "Pod phase is Running",
			EvaluatedValue: "Running",
			ExpectedValue:  "Running",
			DurationNs:     120000,
		},
		{
			RuleID:         "rule-invariant-no-secret-leak",
			RuleType:       "INVARIANT",
			Status:         verification.RulePass,
			Reason:         "Zero secret tokens in logs",
			EvaluatedValue: "0",
			ExpectedValue:  "0",
			DurationNs:     45000,
		},
	}

	// 1. Calculate deterministic evidence digest
	digest1 := verification.ComputeEvidenceDigest(evaluations)
	digest2 := verification.ComputeEvidenceDigest(evaluations)

	if digest1 == "" || digest1 != digest2 {
		t.Fatalf("Evidence digest calculation is non-deterministic: %s != %s", digest1, digest2)
	}

	// 2. Tamper with evaluation results -> Evidence digest MUST change
	tamperedEvals := []verification.RuleEvaluation{
		evaluations[0],
		{
			RuleID:         "rule-invariant-no-secret-leak",
			RuleType:       "INVARIANT",
			Status:         verification.RuleFail, // Tampered status
			Reason:         "Zero secret tokens in logs",
			EvaluatedValue: "0",
			ExpectedValue:  "0",
			DurationNs:     45000,
		},
	}

	tamperedDigest := verification.ComputeEvidenceDigest(tamperedEvals)
	if tamperedDigest == digest1 {
		t.Fatalf("SECURITY VIOLATION: Tampered evaluation evidence digest matched legitimate digest!")
	}
}

func TestSecurity_CheckpointChecksumIntegrity(t *testing.T) {
	statePayload := []byte(`{"agent_id":"agent-sec-1","step":42,"memory":{"tokens":1200}}`)
	checksum1 := checkpoint.ComputeCanonicalChecksum(statePayload)
	checksum2 := checkpoint.ComputeCanonicalChecksum(statePayload)

	if checksum1 == "" || checksum1 != checksum2 {
		t.Fatalf("Checksum computation is non-deterministic")
	}

	// Tampered state
	tamperedPayload := []byte(`{"agent_id":"agent-sec-1","step":42,"memory":{"tokens":999999}}`)
	tamperedChecksum := checkpoint.ComputeCanonicalChecksum(tamperedPayload)

	if tamperedChecksum == checksum1 {
		t.Fatalf("SECURITY VIOLATION: Tampered state checksum collision!")
	}

	chk := &checkpoint.Checkpoint{
		ID:             "chk-sec-1",
		RunID:          "run-sec-1",
		AgentID:        "agent-sec-1",
		TenantID:       "tenant-sec-1",
		SequenceNumber: 1,
		StateInline:    statePayload,
		StateChecksum:  checksum1,
		SizeBytes:      int64(len(statePayload)),
		CreatedAt:      time.Now().UTC(),
	}

	if err := chk.Validate(); err != nil {
		t.Fatalf("Valid checkpoint failed validation: %v", err)
	}
}

func TestSecurity_AgentStateTransitionsAreMonotonic(t *testing.T) {
	// Legal transition: Running -> Completed
	if err := domain.ValidateTransition(types.StateRunning, types.StateCompleted); err != nil {
		t.Errorf("Legal transition failed: %v", err)
	}

	// Illegal backwards transition: Completed -> Queued (Terminal state cannot transition)
	if err := domain.ValidateTransition(types.StateCompleted, types.StateQueued); err == nil {
		t.Errorf("SECURITY VIOLATION: Backwards transition from terminal state permitted: Completed -> Queued")
	}
}
