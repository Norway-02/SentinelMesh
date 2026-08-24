# ADR-008: Deterministic Chaos Testing and Failure Validation Matrix

## Status
Accepted

## Context
Through Stages 6 to 14, SentinelMesh developed sophisticated distributed fault-tolerance mechanisms, including transactional outbox messaging, multi-cluster two-tier scheduling, self-healing checkpoint recovery, and monotonic execution fencing. 

However, asserting that a distributed system is resilient based solely on architecture diagrams and happy-path unit tests is insufficient for production systems and peer-reviewed systems research. The system must be subjected to active, adversarial fault injection across its entire operational envelope.

## Decision
We establish a native, in-process deterministic chaos testing framework and mandatory verification policy:

1. **Deterministic Fault Injection via Interface Wrapping**:
   Rather than relying on non-deterministic external network monkey tools or live Kubernetes clusters for core CI testing, SentinelMesh provides pure-Go fault wrappers in `internal/chaos/` that decorate the `repository.RunRepository`, `outbox.Repository`, `checkpoint.Repository`, and cluster failure detection interfaces.

2. **Reproducible Experiment Seeds**:
   All chaos scenarios are parameterized with an `ExperimentConfig` containing an explicit pseudo-random number generator `Seed`. Given the same seed, fault configuration, and workload, the exact sequence and timing of faults is 100% reproducible.

3. **13-Scenario Controlled Fault Matrix**:
   The test harness evaluates the system across 13 distinct failure classes:
   - **F01**: Pod exit / worker failure
   - **F02**: Container crash / runtime panic
   - **F03**: Compute node unreachability / lease expiration
   - **F04**: Total cluster loss-of-control / cross-region WAN network partition
   - **F05**: NATS publication transient loss / broker retry durability
   - **F06**: NATS message delay & out-of-order delivery
   - **F07**: Duplicate NATS message delivery across all domain event consumers
   - **F08**: PostgreSQL transaction write failure & OCC atomicity rollback
   - **F09**: Adversarial checkpoint bit-rot / CRC32 checksum corruption
   - **F10**: Incomplete / truncated checkpoint payload bytes
   - **F11**: Scheduler process crash before and after assignment commit
   - **F12**: Split-brain stale generation reconnection and fencing quarantine
   - **F13**: Compound cascading failure (WAN partition + corrupted state storage)

4. **Quantitative Invariant Telemetry & Separation of Phases**:
   Each chaos experiment captures high-resolution timestamps:
   - `fault_injected_at`
   - `fault_observed_at`
   - `recovery_started_at`
   - `replacement_active_at`
   - `recovery_completed_at`

   And computes:
   - **Detection Latency** = `fault_observed_at - fault_injected_at`
   - **Recovery Latency** = `replacement_active_at - fault_observed_at`
   - **Authority Violations** (must be 0)
   - **Duplicate Executions** (must be 0)
   - **Lost Work Steps**

5. **Mandatory Merge Policy**:
   Any new distributed-systems or consensus feature merged into SentinelMesh must add at least one corresponding fault scenario to the chaos suite in `test/chaos/`.

## Consequences
- **Positive**: Direct empirical proof of fault-tolerance invariants with quantitative P50/P95/P99 latency distributions across N=30 repetitions.
- **Positive**: 100% reproducible debugging for distributed race conditions and edge cases.
- **Positive**: Zero flaky tests; pure-Go in-memory simulation runs in <1s for CI/CD pipelines.
- **Positive**: Establishes a deterministic baseline necessary before introducing Stage 16 performance benchmarks and Stages 17-18 predictive/adaptive AI schedulers.
