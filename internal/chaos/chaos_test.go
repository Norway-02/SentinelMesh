package chaos_test

import (
	"context"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/chaos"
	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestFaultController_DeterministicSeed(t *testing.T) {
	spec := []chaos.FaultSpec{
		{
			Type:        chaos.FaultError,
			TargetOp:    "Update",
			Probability: 0.5,
			ErrorMsg:    "db error",
		},
	}

	c1 := chaos.NewFaultController(42, spec)
	c2 := chaos.NewFaultController(42, spec)

	for i := 0; i < 20; i++ {
		t1, _ := c1.ShouldTrigger("Update")
		t2, _ := c2.ShouldTrigger("Update")
		if t1 != t2 {
			t.Fatalf("Iteration %d: Non-deterministic trigger behavior between identical seeds!", i)
		}
	}
}

func TestFaultyRunRepository_WriteFailure(t *testing.T) {
	ctx := context.Background()
	underlying := memory.NewRunRepository()
	controller := chaos.NewFaultController(100, []chaos.FaultSpec{
		{
			Type:        chaos.FaultError,
			TargetOp:    "Update",
			Probability: 1.0,
			ErrorMsg:    "simulated disk failure",
		},
	})

	repo := chaos.NewFaultyRunRepository(underlying, controller)

	run := domain.AgentRun{
		ID:       "run-1",
		AgentID:  "agent-1",
		TenantID: "tenant-1",
		State:    types.StateQueued,
		Version:  1,
	}
	if err := repo.Create(ctx, run); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	run.State = types.StateRunning
	err := repo.Update(ctx, run)
	if err == nil {
		t.Fatal("Expected error on Update, got nil")
	}

	// Verify underlying state was NOT mutated
	fetched, err := underlying.Get(ctx, "run-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if fetched.State != types.StateQueued {
		t.Fatalf("Expected StateQueued after aborted update, got %s", fetched.State)
	}
}

func TestFaultyOutboxRepository_DropAndDuplicate(t *testing.T) {
	ctx := context.Background()
	underlying := outbox.NewMemoryRepository()
	controller := chaos.NewFaultController(100, []chaos.FaultSpec{
		{
			Type:        chaos.FaultDuplicate,
			TargetOp:    "Insert",
			Probability: 1.0,
		},
	})

	outboxRepo := chaos.NewFaultyOutboxRepository(underlying, controller)

	event := events.Event{
		EventID:   "evt-1",
		EventType: events.SubjectRunCreated,
	}

	if err := outboxRepo.Insert(ctx, event); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	evts := underlying.GetEvents()
	if len(evts) != 2 {
		t.Fatalf("Expected 2 events due to FaultDuplicate, got %d", len(evts))
	}
}

func TestFaultyCheckpointRepository_CorruptAndTruncate(t *testing.T) {
	ctx := context.Background()
	underlying := checkpoint.NewMemoryRepository()

	// 1. Save valid checkpoint
	rawState := []byte(`{"step":10,"state":"valid"}`)
	cp := checkpoint.Checkpoint{
		ID:             "cp-10",
		RunID:          "run-cp",
		AgentID:        "agent-1",
		TenantID:       "tenant-1",
		SequenceNumber: 10,
		StateInline:    rawState,
		StateChecksum:  checkpoint.ComputeCanonicalChecksum(rawState),
		SizeBytes:      int64(len(rawState)),
		CreatedAt:      time.Now(),
	}
	if err := underlying.Save(ctx, cp); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 2. Wrap with FaultCorrupt
	controller := chaos.NewFaultController(100, []chaos.FaultSpec{
		{
			Type:        chaos.FaultCorrupt,
			TargetOp:    "GetLatest",
			Probability: 1.0,
		},
	})
	repo := chaos.NewFaultyCheckpointRepository(underlying, controller)

	corrupted, err := repo.GetLatest(ctx, "run-cp")
	if err != nil {
		t.Fatalf("GetLatest failed: %v", err)
	}
	if corrupted.VerifyIntegrity() {
		t.Fatal("Expected corrupted checkpoint to fail VerifyIntegrity()")
	}
}

func TestMetrics_ReportGeneration(t *testing.T) {
	m := []chaos.ExperimentMetrics{
		{
			ScenarioID:          "F03",
			Seed:                42,
			FaultType:           chaos.FaultError,
			DetectionLatency:    50 * time.Millisecond,
			RecoveryLatency:     120 * time.Millisecond,
			Outcome:             "PASS",
			FinalGeneration:     1,
			RestoredCheckpoint:  true,
		},
	}

	agg := []chaos.AggregateMetrics{
		chaos.ComputeAggregateMetrics("F03", m),
	}

	report := chaos.GenerateChaosValidationReport(m, agg)
	if len(report) == 0 {
		t.Fatal("Generated report is empty")
	}
}
