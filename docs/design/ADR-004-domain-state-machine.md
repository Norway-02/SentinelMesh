# ADR 004: Explicit Domain State Machine

## Status
Accepted

## Context
SentinelMesh agent runs undergo a complex lifecycle (e.g., scheduled, starting, running, checkpointing, failing, recovering, completed). If state transitions are scattered across the codebase and embedded in repository logic, scheduler logic, or API handlers, it leads to race conditions, invalid states (e.g., transitioning from COMPLETED to RUNNING), and difficult-to-test business rules.

## Decision
We model the AgentRun state machine explicitly in the core domain layer (`internal/domain/state_machine.go` and `internal/domain/run.go`).

- **Single Source of Truth:** All valid transitions are explicitly enumerated in the domain layer.
- **Typed Errors:** Any invalid transition immediately returns a strongly typed error (e.g., `ErrInvalidStateTransition`) before any mutation occurs.
- **No Side Effects:** The transition logic is pure. It only updates the in-memory aggregate and timestamps. 
- **Persistence & Events Deferred:** The repository layer will later be responsible for saving this state to the database, and the event bus layer will publish the transition event. The domain layer doesn't know about NATS or Postgres.

## Consequences
**Positive:**
- 100% testable business rules without requiring databases or external services.
- Impossible to persist an invalid state transition (assuming the application logic uses the domain transition methods).
- Future components (like the Scheduler and Checkpoint Manager) have a strict contract for how they interact with agent state.

**Negative:**
- Requires careful mapping between the domain layer, gRPC layer, and database entities.
- Event generation (e.g., `AgentStarted` event) must be carefully synchronized by the application service coordinating the domain mutation and the event publisher.
