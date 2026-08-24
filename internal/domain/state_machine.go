package domain

import (
	"fmt"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// ValidateTransition checks whether moving from `current` to `next` is a valid state transition.
func ValidateTransition(current, next types.AgentState) error {
	// If the state is not changing, we consider it a no-op / valid.
	if current == next {
		return nil
	}

	valid := false
	switch current {
	case types.StateCreated:
		valid = next == types.StateQueued
	case types.StateQueued:
		valid = next == types.StateScheduled || next == types.StateCancelled
	case types.StateScheduled:
		valid = next == types.StateStarting || next == types.StateRestoring || next == types.StateCancelled || next == types.StateFailed
	case types.StateStarting:
		valid = next == types.StateRunning || next == types.StateRestoring || next == types.StateFailed
	case types.StateRunning:
		valid = next == types.StatePaused || next == types.StateCheckpointing ||
			next == types.StateVerifying || next == types.StateCompleted || next == types.StateFailed || next == types.StateCancelled
	case types.StateVerifying:
		valid = next == types.StateCompleted || next == types.StateFailed || next == types.StateCancelled
	case types.StatePaused:
		valid = next == types.StateRunning || next == types.StateCancelled
	case types.StateCheckpointing:
		valid = next == types.StateRunning || next == types.StateFailed
	case types.StateFailed:
		valid = next == types.StateRecovering
	case types.StateRecovering:
		valid = next == types.StateScheduled || next == types.StateQueued || next == types.StateRestoring || next == types.StateFailed
	case types.StateRestoring:
		valid = next == types.StateRunning || next == types.StateFailed || next == types.StateCancelled
	case types.StateCompleted, types.StateCancelled:
		// Terminal states cannot transition to any other state
		valid = false
	default:
		return fmt.Errorf("%w: unknown state '%s'", ErrInvalidStateTransition, current)
	}

	if !valid {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStateTransition, current, next)
	}

	return nil
}
