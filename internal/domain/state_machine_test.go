package domain

import (
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    types.AgentState
		to      types.AgentState
		wantErr bool
	}{
		// Valid transitions
		{"CREATED to QUEUED", types.StateCreated, types.StateQueued, false},
		{"QUEUED to SCHEDULED", types.StateQueued, types.StateScheduled, false},
		{"QUEUED to CANCELLED", types.StateQueued, types.StateCancelled, false},
		{"SCHEDULED to STARTING", types.StateScheduled, types.StateStarting, false},
		{"SCHEDULED to CANCELLED", types.StateScheduled, types.StateCancelled, false},
		{"STARTING to RUNNING", types.StateStarting, types.StateRunning, false},
		{"STARTING to FAILED", types.StateStarting, types.StateFailed, false},
		{"RUNNING to PAUSED", types.StateRunning, types.StatePaused, false},
		{"RUNNING to CHECKPOINTING", types.StateRunning, types.StateCheckpointing, false},
		{"RUNNING to COMPLETED", types.StateRunning, types.StateCompleted, false},
		{"RUNNING to FAILED", types.StateRunning, types.StateFailed, false},
		{"RUNNING to CANCELLED", types.StateRunning, types.StateCancelled, false},
		{"PAUSED to RUNNING", types.StatePaused, types.StateRunning, false},
		{"PAUSED to CANCELLED", types.StatePaused, types.StateCancelled, false},
		{"CHECKPOINTING to RUNNING", types.StateCheckpointing, types.StateRunning, false},
		{"CHECKPOINTING to FAILED", types.StateCheckpointing, types.StateFailed, false},
		{"FAILED to RECOVERING", types.StateFailed, types.StateRecovering, false},
		{"RECOVERING to RESTORING", types.StateRecovering, types.StateRestoring, false},
		{"RECOVERING to FAILED", types.StateRecovering, types.StateFailed, false},
		{"RESTORING to RUNNING", types.StateRestoring, types.StateRunning, false},
		{"RESTORING to FAILED", types.StateRestoring, types.StateFailed, false},

		// Same state (no-op)
		{"RUNNING to RUNNING", types.StateRunning, types.StateRunning, false},

		// Invalid transitions
		{"CREATED to RUNNING", types.StateCreated, types.StateRunning, true},
		{"COMPLETED to RUNNING", types.StateCompleted, types.StateRunning, true},
		{"CANCELLED to RUNNING", types.StateCancelled, types.StateRunning, true},
		{"FAILED to RUNNING", types.StateFailed, types.StateRunning, true},
		{"UNKNOWN to QUEUED", types.AgentState("UNKNOWN"), types.StateQueued, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTransition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
