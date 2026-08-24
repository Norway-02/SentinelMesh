package verification

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestVerificationService_FullWorkflow(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	attRepo := NewMemoryRepository()
	agentRepo := memory.NewAgentRepository()
	runRepo := memory.NewRunRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	service := NewService(attRepo, agentRepo, runRepo, outboxRepo, txManager, nil, nil)

	// Create valid artifact
	outPath := filepath.Join(tmpDir, "report.json")
	_ = os.WriteFile(outPath, []byte(`{"status":"OK","processed":100}`), 0644)

	agent := domain.Agent{
		ID:       "agent-verif-01",
		TenantID: "tenant-analytics",
		Name:     "data-verifier",
		Version:  "1.0.0",
		VerificationPolicy: types.VerificationPolicy{
			Enabled: true,
			ArtifactRules: []types.ArtifactRule{
				{ID: "rule-art-1", Path: outPath, Required: true, MinSizeBytes: 5},
			},
			InvariantRules: []types.InvariantRule{
				{ID: "rule-inv-1", MetricName: "accuracy", Operator: "gte", ExpectedValue: "0.95"},
			},
		},
	}
	_ = agentRepo.Create(ctx, agent)

	now := time.Now()
	run := domain.AgentRun{
		ID:        "run-verif-01",
		AgentID:   "agent-verif-01",
		TenantID:  "tenant-analytics",
		State:     types.StateRunning,
		StartedAt: &now,
		Version:   1,
	}
	_ = runRepo.Create(ctx, run)

	// Case A: Successful Verification
	att, err := service.VerifyRun(ctx, VerifyRunRequest{
		RunID: "run-verif-01",
		ReportedMetrics: map[string]string{
			"accuracy": "0.98",
		},
	})
	if err != nil {
		t.Fatalf("VerifyRun returned error for passing case: %v", err)
	}

	if att.Status != StatusVerified {
		t.Errorf("expected StatusVerified, got %s", att.Status)
	}
	if !att.VerifyDigest() {
		t.Errorf("expected attestation evidence digest to be cryptographically valid")
	}

	updatedRun, _ := runRepo.Get(ctx, "run-verif-01")
	if updatedRun.State != types.StateCompleted {
		t.Errorf("expected run state COMPLETED, got %s", updatedRun.State)
	}
	if updatedRun.VerificationState != "VERIFIED" {
		t.Errorf("expected verification state VERIFIED, got %s", updatedRun.VerificationState)
	}

	eventsList := outboxRepo.GetEvents()
	var verifiedEvt *events.Event
	for _, e := range eventsList {
		if e.EventType == events.SubjectRunVerified {
			verifiedEvt = &e
		}
	}
	if verifiedEvt == nil {
		t.Errorf("expected SubjectRunVerified outbox event")
	}
}
