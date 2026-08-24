package verification_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
	"github.com/sentinelmesh/sentinelmesh/internal/verification"
)

// Scenario 4: Data Quality & Schema Invariant Violation.
func TestAttestationSuite_SchemaAndInvariantViolation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Incomplete JSON output
	jsonPath := filepath.Join(tmpDir, "report.json")
	_ = os.WriteFile(jsonPath, []byte(`{"error":"timeout during ingestion"}`), 0644)

	attRepo := verification.NewMemoryRepository()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	verifier := verification.NewService(attRepo, agentRepo, runRepo, outboxRepo, txManager, nil, nil)

	agent := domain.Agent{
		ID:       "agent-etl-01",
		TenantID: "tenant-data",
		Name:     "etl-pipeline",
		Version:  "1.0.0",
		VerificationPolicy: types.VerificationPolicy{
			Enabled: true,
			ArtifactRules: []types.ArtifactRule{
				{ID: "rule-schema-check", Path: jsonPath, SchemaJSON: `["status", "rows_ingested", "checksum"]`},
			},
		},
	}
	_ = agentRepo.Create(ctx, agent)

	now := time.Now()
	run := domain.AgentRun{
		ID:        "run-schema-01",
		AgentID:   "agent-etl-01",
		TenantID:  "tenant-data",
		State:     types.StateRunning,
		StartedAt: &now,
		Version:   1,
	}
	_ = runRepo.Create(ctx, run)

	att, err := verifier.VerifyRun(ctx, verification.VerifyRunRequest{RunID: "run-schema-01"})
	if err == nil {
		t.Errorf("Expected verification to fail on missing schema keys")
	}

	if att.Status != verification.StatusFailed {
		t.Errorf("Expected StatusFailed, got %s", att.Status)
	}
}

// Scenario 5: Cryptographic Digest Tamper Proof.
func TestAttestationSuite_EvidenceDigestTamperDetection(t *testing.T) {
	evals := []verification.RuleEvaluation{
		{RuleID: "art-1", RuleType: "artifact", Status: verification.RulePass, Reason: "ok", DurationNs: 1000},
		{RuleID: "http-1", RuleType: "http_health", Status: verification.RulePass, Reason: "status 200", DurationNs: 2000},
	}

	digest := verification.ComputeEvidenceDigest(evals)
	record := verification.AttestationRecord{
		ID:             "att-001",
		RunID:          "run-001",
		Status:         verification.StatusVerified,
		EvidenceDigest: digest,
		Evaluations:    evals,
	}

	if !record.VerifyDigest() {
		t.Errorf("Expected digest verification to pass on authentic record")
	}

	// Tamper with an evaluation field
	record.Evaluations[0].Status = verification.RuleFail
	if record.VerifyDigest() {
		t.Errorf("Expected digest verification to fail on tampered record")
	}
}
