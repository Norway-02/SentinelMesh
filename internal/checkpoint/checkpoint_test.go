package checkpoint

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
)

func TestCheckpointService_SaveAndIntegrity(t *testing.T) {
	repo := NewMemoryRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	svc := NewService(repo, outboxRepo, txManager)
	ctx := context.Background()

	state1 := json.RawMessage(`{"step":10,"processed_records":100,"status":"OK"}`)
	req1 := SaveCheckpointRequest{
		RunID:          "run-100",
		AgentID:        "agent-1",
		TenantID:       "tenant-1",
		SequenceNumber: 10,
		StateInline:    state1,
		Metadata:       map[string]string{"phase": "processing"},
	}

	cp1, err := svc.SaveCheckpoint(ctx, req1)
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	if cp1.StateChecksum == "" {
		t.Errorf("expected non-empty state checksum")
	}
	if cp1.SequenceNumber != 10 {
		t.Errorf("expected sequence 10, got %d", cp1.SequenceNumber)
	}

	// 1. Verify retrieval and integrity check
	latest, err := svc.GetLatestCheckpoint(ctx, "run-100")
	if err != nil {
		t.Fatalf("GetLatestCheckpoint failed: %v", err)
	}
	if latest.ID != cp1.ID {
		t.Errorf("expected checkpoint ID %s, got %s", cp1.ID, latest.ID)
	}
	if string(latest.StateInline) != string(state1) {
		t.Errorf("expected state data %s, got %s", string(state1), string(latest.StateInline))
	}

	// 2. Verify Outbox event
	eventsList := outboxRepo.GetEvents()
	if len(eventsList) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(eventsList))
	}
	if eventsList[0].EventType != events.SubjectCheckpointSaved {
		t.Errorf("expected event %s, got %s", events.SubjectCheckpointSaved, eventsList[0].EventType)
	}

	// 3. Save second checkpoint with higher sequence
	state2 := json.RawMessage(`{"step":20,"processed_records":200,"status":"OK"}`)
	req2 := SaveCheckpointRequest{
		RunID:          "run-100",
		AgentID:        "agent-1",
		TenantID:       "tenant-1",
		SequenceNumber: 20,
		StateInline:    state2,
	}

	cp2, err := svc.SaveCheckpoint(ctx, req2)
	if err != nil {
		t.Fatalf("SaveCheckpoint second failed: %v", err)
	}

	latest2, err := svc.GetLatestCheckpoint(ctx, "run-100")
	if err != nil {
		t.Fatalf("GetLatestCheckpoint failed: %v", err)
	}
	if latest2.SequenceNumber != 20 {
		t.Errorf("expected latest sequence 20, got %d", latest2.SequenceNumber)
	}
	if latest2.ID != cp2.ID {
		t.Errorf("expected latest ID %s, got %s", cp2.ID, latest2.ID)
	}
}

func TestCheckpointService_MonotonicityAndConflict(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, nil, nil)
	ctx := context.Background()

	state1 := json.RawMessage(`{"step":15,"progress":"halfway"}`)
	_, err := svc.SaveCheckpoint(ctx, SaveCheckpointRequest{
		RunID:          "run-200",
		AgentID:        "agent-1",
		TenantID:       "tenant-1",
		SequenceNumber: 15,
		StateInline:    state1,
	})
	if err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	// Case A: Monotonicity violation (trying to save sequence 10 when latest is 15)
	_, err = svc.SaveCheckpoint(ctx, SaveCheckpointRequest{
		RunID:          "run-200",
		AgentID:        "agent-1",
		TenantID:       "tenant-1",
		SequenceNumber: 10,
		StateInline:    json.RawMessage(`{"step":10}`),
	})
	if err == nil {
		t.Errorf("expected error for non-monotonic sequence, got nil")
	}

	// Case B: Idempotency (saving exact same sequence + same state)
	_, err = svc.SaveCheckpoint(ctx, SaveCheckpointRequest{
		RunID:          "run-200",
		AgentID:        "agent-1",
		TenantID:       "tenant-1",
		SequenceNumber: 15,
		StateInline:    state1,
	})
	if err != nil {
		t.Errorf("expected idempotent success for identical checkpoint, got %v", err)
	}

	// Case C: Conflict (same sequence + different state payload)
	err = repo.Save(ctx, Checkpoint{
		ID:             "cp-conflict",
		RunID:          "run-200",
		AgentID:        "agent-1",
		TenantID:       "tenant-1",
		SequenceNumber: 15,
		StateInline:    json.RawMessage(`{"step":15,"different_payload":true}`),
		StateChecksum:  ComputeCanonicalChecksum([]byte(`{"step":15,"different_payload":true}`)),
		SizeBytes:      35,
	})
	if err != ErrSequenceConflict {
		t.Errorf("expected ErrSequenceConflict, got %v", err)
	}
}

func TestCheckpoint_CorruptionRejection(t *testing.T) {
	state := json.RawMessage(`{"counter":42}`)
	validChecksum := ComputeCanonicalChecksum(state)

	cp := Checkpoint{
		ID:             "cp-corrupt",
		RunID:          "run-corrupt",
		AgentID:        "agent-1",
		TenantID:       "tenant-1",
		SequenceNumber: 1,
		StateInline:    state,
		StateChecksum:  validChecksum,
	}

	if !cp.VerifyIntegrity() {
		t.Errorf("expected valid integrity check")
	}

	// Tamper with state data
	cp.StateInline = json.RawMessage(`{"counter":999}`) // altered without updating checksum
	if cp.VerifyIntegrity() {
		t.Errorf("expected tampered checkpoint to fail integrity verification")
	}
}
