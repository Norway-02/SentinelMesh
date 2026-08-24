package domain

import (
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestAgentRun_Validate(t *testing.T) {
	tests := []struct {
		name    string
		run     AgentRun
		wantErr bool
	}{
		{
			name: "valid run",
			run: AgentRun{
				ID:       "run-1",
				AgentID:  "agent-1",
				TenantID: "tenant-A",
				State:    types.StateCreated,
			},
			wantErr: false,
		},
		{
			name: "invalid run ID",
			run: AgentRun{
				AgentID:  "agent-1",
				TenantID: "tenant-A",
				State:    types.StateCreated,
			},
			wantErr: true,
		},
		{
			name: "invalid agent ID",
			run: AgentRun{
				ID:       "run-1",
				TenantID: "tenant-A",
				State:    types.StateCreated,
			},
			wantErr: true,
		},
		{
			name: "missing tenant ID",
			run: AgentRun{
				ID:      "run-1",
				AgentID: "agent-1",
				State:   types.StateCreated,
			},
			wantErr: true,
		},
		{
			name: "missing state",
			run: AgentRun{
				ID:       "run-1",
				AgentID:  "agent-1",
				TenantID: "tenant-A",
			},
			wantErr: true,
		},
		{
			name: "negative retry count",
			run: AgentRun{
				ID:         "run-1",
				AgentID:    "agent-1",
				TenantID:   "tenant-A",
				State:      types.StateCreated,
				RetryCount: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("AgentRun.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgentRun_TransitionTo(t *testing.T) {
	run := &AgentRun{
		ID:       "run-1",
		AgentID:  "agent-1",
		TenantID: "tenant-A",
		State:    types.StateCreated,
	}

	// Test valid transition and timestamp updates
	err := run.TransitionTo(types.StateQueued)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if run.State != types.StateQueued {
		t.Errorf("expected state QUEUED, got %s", run.State)
	}
	if run.StartedAt != nil {
		t.Error("StartedAt should be nil when in QUEUED")
	}

	// Transition to running
	err = run.TransitionTo(types.StateScheduled)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = run.TransitionTo(types.StateStarting)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if run.StartedAt == nil {
		t.Error("StartedAt should be set when in STARTING")
	}

	err = run.TransitionTo(types.StateRunning)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Test invalid transition (preserves state)
	err = run.TransitionTo(types.StateCreated)
	if err == nil {
		t.Fatal("expected error transitioning from RUNNING to CREATED")
	}
	if run.State != types.StateRunning {
		t.Errorf("expected state to remain RUNNING, got %s", run.State)
	}

	// Terminal transition
	err = run.TransitionTo(types.StateCompleted)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if run.FinishedAt == nil {
		t.Error("FinishedAt should be set when in COMPLETED")
	}
}

func TestAgentRun_CanRetry(t *testing.T) {
	run := &AgentRun{
		RetryCount: 1,
	}

	if run.CanRetry(-1) {
		t.Error("CanRetry should return false for negative max limits")
	}
	if !run.CanRetry(3) {
		t.Error("CanRetry should return true when RetryCount < maxRetries")
	}
	if run.CanRetry(1) {
		t.Error("CanRetry should return false when RetryCount == maxRetries")
	}
}
