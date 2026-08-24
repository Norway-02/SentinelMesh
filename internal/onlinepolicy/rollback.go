package onlinepolicy

import (
	"fmt"
	"sync"
	"time"
)

// PolicyManager manages version lineage, live policy state, and automated rollbacks.
type PolicyManager struct {
	mu             sync.RWMutex
	activeState    PolicyState
	history        map[string]PolicyState
	rollbackEvents []RollbackEvent
}

// NewPolicyManager initializes a PolicyManager with an initial policy state.
func NewPolicyManager(initial PolicyState) *PolicyManager {
	history := make(map[string]PolicyState)
	history[initial.Version] = initial

	return &PolicyManager{
		activeState:    initial,
		history:        history,
		rollbackEvents: make([]RollbackEvent, 0),
	}
}

// GetActiveState returns a copy of the current active policy state.
func (m *PolicyManager) GetActiveState() PolicyState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeState
}

// PromotePolicy promotes a new policy version to active state and records its parent in history.
func (m *PolicyManager) PromotePolicy(newState PolicyState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newState.ParentVersion = m.activeState.Version
	newState.CreatedAt = time.Now().UTC()
	m.history[newState.Version] = newState
	m.activeState = newState
}

// TriggerRollback rolls back the active policy to its parent version.
func (m *PolicyManager) TriggerRollback(event RollbackEvent) (PolicyState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	targetVersion := m.activeState.ParentVersion
	if targetVersion == "" {
		targetVersion = "policy-v1.0"
	}

	parentState, ok := m.history[targetVersion]
	if !ok {
		// Fallback to default safe policy
		parentState = DefaultPolicyState()
		parentState.Version = targetVersion
	}

	event.CurrentVersion = m.activeState.Version
	event.TargetVersion = targetVersion
	event.RolledBackAt = time.Now().UTC()
	m.rollbackEvents = append(m.rollbackEvents, event)

	parentState.IsRolledBack = true
	parentState.LastRollback = event.RolledBackAt
	m.activeState = parentState

	return m.activeState, nil
}

// ListRollbackEvents returns all recorded rollback events.
func (m *PolicyManager) ListRollbackEvents() []RollbackEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]RollbackEvent, len(m.rollbackEvents))
	copy(res, m.rollbackEvents)
	return res
}

// RestoreVersion manually restores an arbitrary historical version.
func (m *PolicyManager) RestoreVersion(version string) (PolicyState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.history[version]
	if !ok {
		return m.activeState, fmt.Errorf("policy version %s not found in history", version)
	}

	m.activeState = state
	return m.activeState, nil
}
